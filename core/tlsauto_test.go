package core

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestTLSAutoValidate(t *testing.T) {
	tests := []struct {
		name    string
		auto    TLSAuto
		files   TLSFiles
		wantErr bool
	}{
		{"off", TLSAuto{}, TLSFiles{CertFile: "c", KeyFile: "k"}, false},
		{"ca dir", TLSAuto{Auto: true, CADir: "d"}, TLSFiles{}, false},
		{"ca out", TLSAuto{Auto: true, CAOut: "o"}, TLSFiles{}, false},
		{"nowhere to read the CA", TLSAuto{Auto: true}, TLSFiles{}, true},
		{"with cert files", TLSAuto{Auto: true, CADir: "d"}, TLSFiles{CertFile: "c", KeyFile: "k"}, true},
		{"options without auto", TLSAuto{SAN: []string{"x"}}, TLSFiles{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.auto.Validate(tt.files); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTLSAutoCACertPath(t *testing.T) {
	if got := (TLSAuto{CADir: "d"}).CACertPath(); got != "" {
		t.Errorf("off: CACertPath() = %q, want \"\"", got)
	}
	if got, want := (TLSAuto{Auto: true, CADir: "d"}).CACertPath(), filepath.Join("d", "ca.pem"); got != want {
		t.Errorf("ca dir: CACertPath() = %q, want %q", got, want)
	}
	if got := (TLSAuto{Auto: true, CADir: "d", CAOut: "o.pem"}).CACertPath(); got != "o.pem" {
		t.Errorf("ca out wins: CACertPath() = %q", got)
	}
	if got, want := (TLSAuto{Auto: true}).WithProfileDir("p").CACertPath(), filepath.Join("p", "tls", "ca.pem"); got != want {
		t.Errorf("profile dir: CACertPath() = %q, want %q", got, want)
	}
	if got, want := (TLSAuto{Auto: true, CADir: "d"}).WithProfileDir("p").CACertPath(), filepath.Join("d", "ca.pem"); got != want {
		t.Errorf("explicit ca dir wins: CACertPath() = %q, want %q", got, want)
	}
	if got := (TLSAuto{}).WithProfileDir("p"); got.CADir != "" {
		t.Errorf("off: WithProfileDir set CADir = %q", got.CADir)
	}
}

// The generated pair verifies against the CA for every requested name, and the
// CA file handed to clients holds no key.
func TestTLSAutoGenerate(t *testing.T) {
	dir := t.TempDir()
	a := TLSAuto{Auto: true, SAN: []string{"sakumock", "192.0.2.10"}, CADir: filepath.Join(dir, "ca"), CAOut: filepath.Join(dir, "out", "ca.pem")}
	if err := os.Mkdir(filepath.Join(dir, "out"), 0o755); err != nil {
		t.Fatal(err)
	}
	files, cleanup, err := a.Generate("0.0.0.0", "")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	pair, err := tls.LoadX509KeyPair(files.CertFile, files.KeyFile)
	if err != nil {
		t.Fatalf("generated pair does not load: %v", err)
	}

	caPEM, err := os.ReadFile(a.CACertPath())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(caPEM, []byte("PRIVATE KEY")) {
		t.Error("--tls-ca-out holds a private key")
	}
	if kept, err := os.ReadFile(filepath.Join(a.CADir, "ca.pem")); err != nil || !bytes.Equal(kept, caPEM) {
		t.Errorf("--tls-ca-dir and --tls-ca-out certificates differ (err %v)", err)
	}
	if info, err := os.Stat(filepath.Join(a.CADir, "ca-key.pem")); err != nil {
		t.Fatal(err)
	} else if perm := info.Mode().Perm(); runtime.GOOS != "windows" && perm&0o077 != 0 {
		t.Errorf("CA key mode = %o, want owner-only", perm)
	}

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatal("CA certificate does not parse")
	}
	for _, name := range []string{"localhost", "site-0123.localhost", "127.0.0.1", "::1", "sakumock", "192.0.2.10", "0.0.0.0"} {
		if _, err := pair.Leaf.Verify(x509.VerifyOptions{Roots: roots, DNSName: name}); err != nil {
			t.Errorf("certificate does not verify for %q: %v", name, err)
		}
	}

	cleanup()
	if _, err := os.Stat(files.CertFile); !os.IsNotExist(err) {
		t.Errorf("cleanup left the server certificate behind (err %v)", err)
	}
	if _, err := os.Stat(a.CACertPath()); err != nil {
		t.Errorf("cleanup removed the CA certificate: %v", err)
	}
}

// A client that keeps trusting the CA must not be served any other host by it:
// the CA's Name Constraints reject a certificate signed for another name.
func TestTLSAutoNameConstraints(t *testing.T) {
	a := TLSAuto{Auto: true, CADir: t.TempDir()}
	names, ips := a.subjects(nil)
	now := time.Now()
	ca, err := newCA(names, ips, now)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"secure.sakura.ad.jp"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca.cert)
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: roots, DNSName: "secure.sakura.ad.jp"}); err == nil {
		t.Error("a certificate for a name outside the CA's Name Constraints verified")
	}
}

// A kept CA survives a restart, and is replaced only when it no longer covers
// the requested names.
func TestTLSAutoKeepsCA(t *testing.T) {
	a := TLSAuto{Auto: true, CADir: t.TempDir()}
	readCA := func() []byte {
		t.Helper()
		data, err := os.ReadFile(a.CACertPath())
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	generate := func() {
		t.Helper()
		_, cleanup, err := a.Generate()
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		cleanup()
	}

	generate()
	first := readCA()
	generate()
	if !bytes.Equal(first, readCA()) {
		t.Error("the CA changed across a restart with the same names")
	}

	// A subdomain of a permitted domain needs no new CA.
	a.SAN = []string{"App.Localhost"}
	generate()
	if !bytes.Equal(first, readCA()) {
		t.Error("the CA changed although it already permits the added name")
	}

	a.SAN = []string{"sakumock"}
	generate()
	if bytes.Equal(first, readCA()) {
		t.Error("the CA was kept although it does not permit the added name")
	}
}

// A CA replacement interrupted between its two files leaves a key that does
// not belong to the certificate; the next start replaces the pair.
func TestTLSAutoMismatchedCA(t *testing.T) {
	a, other := TLSAuto{Auto: true, CADir: t.TempDir()}, TLSAuto{Auto: true, CADir: t.TempDir()}
	for _, x := range []TLSAuto{a, other} {
		_, cleanup, err := x.Generate()
		if err != nil {
			t.Fatal(err)
		}
		cleanup()
	}
	key, err := os.ReadFile(filepath.Join(other.CADir, "ca-key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.CADir, "ca-key.pem"), key, 0o600); err != nil {
		t.Fatal(err)
	}

	files, cleanup, err := a.Generate()
	if err != nil {
		t.Fatalf("Generate with a mismatched CA pair: %v", err)
	}
	defer cleanup()
	pair, err := tls.LoadX509KeyPair(files.CertFile, files.KeyFile)
	if err != nil {
		t.Fatal(err)
	}
	caPEM, err := os.ReadFile(a.CACertPath())
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(caPEM)
	if _, err := pair.Leaf.Verify(x509.VerifyOptions{Roots: roots, DNSName: "localhost"}); err != nil {
		t.Errorf("certificate does not verify against the replaced CA: %v", err)
	}
}

// A CA that cannot be read back fails the start instead of being replaced
// silently.
func TestTLSAutoBrokenCA(t *testing.T) {
	a := TLSAuto{Auto: true, CADir: t.TempDir()}
	if _, cleanup, err := a.Generate(); err != nil {
		t.Fatal(err)
	} else {
		cleanup()
	}
	if err := os.WriteFile(filepath.Join(a.CADir, "ca-key.pem"), []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := a.Generate()
	if err == nil || !strings.Contains(err.Error(), "delete") {
		t.Errorf("Generate() error = %v, want one telling to delete the CA files", err)
	}
}
