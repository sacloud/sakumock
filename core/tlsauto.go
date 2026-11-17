package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// CACertPathEnv is the SDK setting naming a PEM file of CA certificates the
// client trusts in addition to the system roots (sacloud-sdk-go v0.7.0+).
const CACertPathEnv = "SAKURA_CA_CERT_PATH"

const (
	caCertFile = "ca.pem"
	caKeyFile  = "ca-key.pem"

	caValidity   = 10 * 365 * 24 * time.Hour
	leafValidity = 365 * 24 * time.Hour
)

// TLSAuto makes sakumock generate the certificate it serves with: a private
// CA, which clients are told to trust, and a server certificate signed by it.
// It is the alternative to handing over a certificate with TLSFiles.
//
// It is embedded next to TLSFiles with the same kong prefix/envprefix, which
// yields --tls-auto, --tls-san, --tls-ca-dir and --tls-ca-out.
type TLSAuto struct {
	Auto  bool     `name:"auto" env:"AUTO" help:"Serve HTTPS with a generated certificate signed by a generated CA, instead of --tls-cert/--tls-key. Needs --tls-ca-dir or --tls-ca-out, which is where clients read the CA certificate they trust."`
	SAN   []string `name:"san" env:"SAN" placeholder:"NAME" help:"Extra host name or IP address the generated certificate is valid for, besides localhost, *.localhost, 127.0.0.1 and ::1 (e.g. the compose service name). Repeatable."`
	CADir string   `name:"ca-dir" env:"CA_DIR" type:"path" placeholder:"DIR" help:"Directory keeping the generated CA (ca.pem and its key ca-key.pem), so a restart keeps the CA clients already trust; unset regenerates the CA on every start."`
	CAOut string   `name:"ca-out" env:"CA_OUT" type:"path" placeholder:"PATH" help:"Write the CA certificate (public, no key) to this file, e.g. in a volume shared with clients."`
}

// Validate reports an error when the options contradict each other or name no
// place where a client could read the CA certificate.
func (a TLSAuto) Validate(files TLSFiles) error {
	if !a.Auto {
		if len(a.SAN) > 0 || a.CADir != "" || a.CAOut != "" {
			return errors.New("--tls-san, --tls-ca-dir and --tls-ca-out need --tls-auto")
		}
		return nil
	}
	if files.CertFile != "" || files.KeyFile != "" {
		return errors.New("--tls-auto cannot be combined with --tls-cert/--tls-key")
	}
	if a.CACertPath() == "" {
		return errors.New("--tls-auto needs --tls-ca-dir or --tls-ca-out: clients read the CA certificate to trust from there")
	}
	return nil
}

// CACertPath returns the CA certificate file a client is pointed at:
// --tls-ca-out when set, otherwise ca.pem in --tls-ca-dir. It is "" when
// automatic TLS is off.
func (a TLSAuto) CACertPath() string {
	switch {
	case !a.Auto:
		return ""
	case a.CAOut != "":
		return a.CAOut
	case a.CADir != "":
		return filepath.Join(a.CADir, caCertFile)
	}
	return ""
}

// ClientEnv returns the variable telling an SDK client to trust the CA, or
// nil when automatic TLS is off.
func (a TLSAuto) ClientEnv() []EnvVar {
	if path := a.CACertPath(); path != "" {
		return []EnvVar{{Key: CACertPathEnv, Value: path}}
	}
	return nil
}

// Generate prepares the CA and writes a fresh server certificate for
// localhost and its subdomains, the loopback addresses, a.SAN and hosts (empty ones are
// skipped). It returns the certificate/key pair
// to serve with and a cleanup removing those two files; the CA files stay.
//
// The CA is loaded from --tls-ca-dir when one is kept there and still covers
// the names, and generated otherwise. Its Name Constraints permit only those
// names, so a client that keeps trusting it cannot be served a certificate for
// any other host. A CA file that cannot be read back is an error.
func (a TLSAuto) Generate(hosts ...string) (TLSFiles, func(), error) {
	names, ips := a.subjects(hosts)
	now := time.Now()

	ca, err := a.loadCA(names, ips, now)
	if err != nil {
		return TLSFiles{}, nil, err
	}
	if ca == nil {
		if ca, err = newCA(names, ips, now); err != nil {
			return TLSFiles{}, nil, err
		}
		if a.CADir != "" {
			if err := ca.save(a.CADir); err != nil {
				return TLSFiles{}, nil, err
			}
		}
	}
	if a.CAOut != "" {
		if err := writePEM(a.CAOut, "CERTIFICATE", ca.cert.Raw, 0o644); err != nil {
			return TLSFiles{}, nil, err
		}
	}

	dir, err := os.MkdirTemp("", "sakumock-tls-")
	if err != nil {
		return TLSFiles{}, nil, err
	}
	cleanup := func() { os.RemoveAll(dir) }
	files, err := ca.issue(dir, names, ips, now)
	if err != nil {
		cleanup()
		return TLSFiles{}, nil, err
	}
	return files, cleanup, nil
}

// subjects splits the names the certificate covers into DNS names and IP
// addresses, sorted and without duplicates.
func (a TLSAuto) subjects(hosts []string) ([]string, []net.IP) {
	// The data planes routing by Host header serve <id>.localhost.
	names := []string{"localhost", "*.localhost"}
	ips := []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback}
	for _, h := range slices.Concat(a.SAN, hosts) {
		if h == "" {
			continue
		}
		if ip := net.ParseIP(h); ip != nil {
			ips = append(ips, ip)
			continue
		}
		names = append(names, h)
	}
	slices.Sort(names)
	slices.SortFunc(ips, func(x, y net.IP) int { return slices.Compare(x.To16(), y.To16()) })
	return slices.Compact(names), slices.CompactFunc(ips, net.IP.Equal)
}

type certAuthority struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

// loadCA returns the CA kept in --tls-ca-dir, or nil when there is none or it
// has to be replaced: it would expire before a certificate issued now, or its
// Name Constraints do not permit every requested name.
func (a TLSAuto) loadCA(names []string, ips []net.IP, now time.Time) (*certAuthority, error) {
	if a.CADir == "" {
		return nil, nil
	}
	certPath, keyPath := filepath.Join(a.CADir, caCertFile), filepath.Join(a.CADir, caKeyFile)
	certDER, err := readPEM(certPath, "CERTIFICATE")
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	keyDER, err := readPEM(keyPath, "EC PRIVATE KEY")
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("%s: %w (delete the directory's CA files to generate a new CA)", certPath, err)
	}
	key, err := x509.ParseECPrivateKey(keyDER)
	if err != nil {
		return nil, fmt.Errorf("%s: %w (delete the directory's CA files to generate a new CA)", keyPath, err)
	}
	if !key.PublicKey.Equal(cert.PublicKey) {
		return nil, fmt.Errorf("%s does not match %s (delete both to generate a new CA)", keyPath, certPath)
	}
	if cert.NotAfter.Before(now.Add(leafValidity)) || !permits(cert, names, ips) {
		return nil, nil
	}
	return &certAuthority{cert: cert, key: key}, nil
}

// permits reports whether the CA's Name Constraints cover names and ips.
func permits(ca *x509.Certificate, names []string, ips []net.IP) bool {
	for _, n := range constraintDomains(names) {
		if !slices.Contains(ca.PermittedDNSDomains, n) {
			return false
		}
	}
	for _, ip := range ips {
		if !slices.ContainsFunc(ca.PermittedIPRanges, func(r *net.IPNet) bool { return r.Contains(ip) }) {
			return false
		}
	}
	return true
}

// constraintDomains turns certificate names into Name Constraints domains: a
// domain permits its subdomains too, so a wildcard name is its parent domain.
func constraintDomains(names []string) []string {
	domains := make([]string, 0, len(names))
	for _, n := range names {
		domains = append(domains, strings.TrimPrefix(n, "*."))
	}
	slices.Sort(domains)
	return slices.Compact(domains)
}

func newCA(names []string, ips []net.IP, now time.Time) (*certAuthority, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	serial, err := newSerial()
	if err != nil {
		return nil, err
	}
	ranges := make([]*net.IPNet, 0, len(ips))
	for _, ip := range ips {
		// The whole loopback network, a single address otherwise.
		switch {
		case ip.To4() != nil && ip.IsLoopback():
			ranges = append(ranges, &net.IPNet{IP: net.IPv4(127, 0, 0, 0).To4(), Mask: net.CIDRMask(8, 32)})
		case ip.To4() != nil:
			ranges = append(ranges, &net.IPNet{IP: ip.To4(), Mask: net.CIDRMask(32, 32)})
		default:
			ranges = append(ranges, &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)})
		}
	}
	tmpl := &x509.Certificate{
		SerialNumber:                serial,
		Subject:                     pkix.Name{CommonName: "sakumock local CA"},
		NotBefore:                   now.Add(-time.Hour),
		NotAfter:                    now.Add(caValidity),
		KeyUsage:                    x509.KeyUsageCertSign,
		BasicConstraintsValid:       true,
		IsCA:                        true,
		MaxPathLenZero:              true,
		PermittedDNSDomainsCritical: true,
		PermittedDNSDomains:         constraintDomains(names),
		PermittedIPRanges:           ranges,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &certAuthority{cert: cert, key: key}, nil
}

// save keeps the CA in dir; the key is owner-readable only.
func (ca *certAuthority) save(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	keyDER, err := x509.MarshalECPrivateKey(ca.key)
	if err != nil {
		return err
	}
	if err := writePEM(filepath.Join(dir, caKeyFile), "EC PRIVATE KEY", keyDER, 0o600); err != nil {
		return err
	}
	return writePEM(filepath.Join(dir, caCertFile), "CERTIFICATE", ca.cert.Raw, 0o644)
}

// issue writes a server certificate for names and ips, and its key, into dir.
func (ca *certAuthority) issue(dir string, names []string, ips []net.IP, now time.Time) (TLSFiles, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return TLSFiles{}, err
	}
	serial, err := newSerial()
	if err != nil {
		return TLSFiles{}, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "sakumock"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(leafValidity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     names,
		IPAddresses:  ips,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		return TLSFiles{}, err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return TLSFiles{}, err
	}
	files := TLSFiles{CertFile: filepath.Join(dir, "cert.pem"), KeyFile: filepath.Join(dir, "key.pem")}
	if err := writePEM(files.CertFile, "CERTIFICATE", der, 0o644); err != nil {
		return TLSFiles{}, err
	}
	if err := writePEM(files.KeyFile, "EC PRIVATE KEY", keyDER, 0o600); err != nil {
		return TLSFiles{}, err
	}
	return files, nil
}

func newSerial() (*big.Int, error) {
	return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
}

func writePEM(path, blockType string, der []byte, perm os.FileMode) error {
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der}), perm)
}

func readPEM(path, blockType string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != blockType {
		return nil, fmt.Errorf("%s: no %s PEM block (delete the directory's CA files to generate a new CA)", path, blockType)
	}
	return block.Bytes, nil
}
