package core

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTLSFiles(t *testing.T) {
	cases := []struct {
		name       string
		tls        TLSFiles
		wantEnable bool
		wantScheme string
	}{
		{"empty", TLSFiles{}, false, "http"},
		{"cert only", TLSFiles{CertFile: "c"}, false, "http"},
		{"key only", TLSFiles{KeyFile: "k"}, false, "http"},
		{"both", TLSFiles{CertFile: "c", KeyFile: "k"}, true, "https"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.tls.Enabled(); got != tc.wantEnable {
				t.Errorf("Enabled() = %v, want %v", got, tc.wantEnable)
			}
			if got := tc.tls.Scheme(); got != tc.wantScheme {
				t.Errorf("Scheme() = %q, want %q", got, tc.wantScheme)
			}
		})
	}
}

func TestTLSFilesValidate(t *testing.T) {
	cases := []struct {
		name    string
		tls     TLSFiles
		wantErr bool
	}{
		{"empty", TLSFiles{}, false},
		{"both", TLSFiles{CertFile: "c", KeyFile: "k"}, false},
		{"cert only", TLSFiles{CertFile: "c"}, true},
		{"key only", TLSFiles{KeyFile: "k"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.tls.Validate(); (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestWithTLSScheme(t *testing.T) {
	in := []EnvVar{
		{Key: "SAKURA_ENDPOINTS_KMS", Value: "http://127.0.0.1:18081"},
		{Key: "AWS_ENDPOINT_URL_S3", Value: "http://127.0.0.1:28086"},
		{Key: "SAKURA_ACCESS_TOKEN", Value: "dummy"},
		{Key: "ALREADY", Value: "https://example.com"},
	}

	// Disabled: returned unchanged.
	if got := WithTLSScheme(in, false); &got[0] != &in[0] {
		t.Error("WithTLSScheme(_, false) should return the slice unchanged")
	}

	got := WithTLSScheme(in, true)
	want := map[string]string{
		"SAKURA_ENDPOINTS_KMS": "https://127.0.0.1:18081",
		"AWS_ENDPOINT_URL_S3":  "https://127.0.0.1:28086",
		"SAKURA_ACCESS_TOKEN":  "dummy",
		"ALREADY":              "https://example.com",
	}
	for _, v := range got {
		if want[v.Key] != v.Value {
			t.Errorf("%s = %q, want %q", v.Key, v.Value, want[v.Key])
		}
	}
	// Input must not be mutated.
	if in[0].Value != "http://127.0.0.1:18081" {
		t.Errorf("input mutated: %q", in[0].Value)
	}
}

// TestServeListenerTLS proves ServeListener serves HTTPS when given a cert/key
// (and plain HTTP otherwise), which is the mechanism every data plane uses.
func TestServeListenerTLS(t *testing.T) {
	certFile, keyFile := writeSelfSignedCert(t)

	for _, tc := range []struct {
		name   string
		files  TLSFiles
		scheme string
	}{
		{"https", TLSFiles{CertFile: certFile, KeyFile: keyFile}, "https"},
		{"http", TLSFiles{}, "http"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})}
			defer srv.Close()
			go func() { _ = ServeListener(srv, ln, tc.files) }()

			if got := tc.files.Scheme(); got != tc.scheme {
				t.Fatalf("Scheme() = %q, want %q", got, tc.scheme)
			}
			resp := getInsecure(t, tc.scheme+"://"+ln.Addr().String()+"/")
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusNoContent {
				t.Errorf("status = %d, want 204", resp.StatusCode)
			}
		})
	}
}

// TestServeTLS proves Serve (the control-plane path) serves HTTPS when given a
// cert/key, and shuts down cleanly when the context is canceled.
func TestServeTLS(t *testing.T) {
	certFile, keyFile := writeSelfSignedCert(t)

	// Reserve a free port for Serve, which binds the address itself.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errc := make(chan error, 1)
	go func() {
		errc <- Serve(ctx, addr, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}), TLSFiles{CertFile: certFile, KeyFile: keyFile})
	}()

	resp := getInsecure(t, "https://"+addr+"/")
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}

	cancel()
	if err := <-errc; err != nil {
		t.Errorf("Serve returned %v", err)
	}
}

// getInsecure issues a GET that skips TLS verification (self-signed certs) and
// tolerates the brief window before the server goroutine starts accepting,
// retrying until a short deadline.
func getInsecure(t *testing.T, url string) *http.Response {
	t.Helper()
	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := client.Get(url)
		if err == nil {
			return resp
		}
		if time.Now().After(deadline) {
			t.Fatalf("GET %s: %v", url, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// writeSelfSignedCert writes a self-signed cert/key (valid for 127.0.0.1) to
// temp files and returns their paths.
func writeSelfSignedCert(t *testing.T) (certFile, keyFile string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "sakumock-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o644); err != nil {
		t.Fatal(err)
	}
	return certFile, keyFile
}
