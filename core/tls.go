package core

import (
	"errors"
	"net"
	"net/http"
)

// TLSFiles is the common, optional TLS certificate/key pair sakumock serves all
// of a process's listeners (every control plane and data plane) with. TLS is
// enabled only when both files are set; otherwise listeners stay plain HTTP.
//
// It is embedded in a Command/Config with a kong prefix/envprefix, e.g.
//
//	TLS core.TLSFiles `embed:"" prefix:"tls-" envprefix:"SERVICE_TLS_"`
//
// which yields --tls-cert/--tls-key flags and SERVICE_TLS_CERT/SERVICE_TLS_KEY
// env vars. Pass it to Serve (control plane) or ServeListener (data plane), or
// hand the files to an external data-plane process (e.g. versitygw --cert/--key).
type TLSFiles struct {
	CertFile string `name:"cert" help:"TLS certificate file; with the matching --tls-key, all listeners (control plane and data plane) serve HTTPS instead of plain HTTP." env:"CERT"`
	KeyFile  string `name:"key" help:"TLS key file (see --tls-cert)." env:"KEY"`
}

// Enabled reports whether both the cert and key are set (i.e. serve HTTPS).
func (t TLSFiles) Enabled() bool { return t.CertFile != "" && t.KeyFile != "" }

// Validate reports an error when exactly one of the cert/key is set. TLS needs
// both, so a half-configured pair is a mistake — returning an error turns it
// into a startup failure instead of silently serving plain HTTP. Each command's
// Run calls it before serving.
func (t TLSFiles) Validate() error {
	if (t.CertFile == "") != (t.KeyFile == "") {
		return errors.New("both --tls-cert and --tls-key must be set to enable TLS (or neither)")
	}
	return nil
}

// Scheme returns "https" when Enabled, otherwise "http".
func (t TLSFiles) Scheme() string {
	if t.Enabled() {
		return "https"
	}
	return "http"
}

// ServeListener serves srv on ln — over HTTPS when tls.Enabled(), otherwise
// plain HTTP. It blocks until the server stops, so callers typically run it in a
// goroutine and stop it via srv.Close/srv.Shutdown; it returns the underlying
// Serve error (http.ErrServerClosed on a normal shutdown).
func ServeListener(srv *http.Server, ln net.Listener, tls TLSFiles) error {
	if tls.Enabled() {
		return srv.ServeTLS(ln, tls.CertFile, tls.KeyFile)
	}
	return srv.Serve(ln)
}
