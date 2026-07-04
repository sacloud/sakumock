package apigw

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/sacloud/sakumock/core"
)

// dataPlane is the gateway itself: a separate listener that matches requests
// by Host header, path, and method against the configured routes and reverse
// proxies them to the owning service's upstream. It reads the store directly
// on every request, so control-plane changes take effect immediately (no
// config synchronization step).
type dataPlane struct {
	listener net.Listener
	server   *http.Server
	store    *MemoryStore
	logger   *slog.Logger
	// scheme is the protocol this listener serves ("https" when the shared
	// TLS files are set), used for route protocol matching and redirects.
	scheme string

	mu            sync.Mutex
	transports    map[string]*cachedTransport
	regexps       map[string]*regexp.Regexp
	oidcProviders map[string]*oidc.Provider
	pendingLogins map[string]pendingLogin
	sessions      map[string]gwSession
	s3Clients     map[string]*cachedS3Client
}

// cachedTransport is a per-service HTTP transport, rebuilt when the service's
// connection settings change (fingerprint mismatch).
type cachedTransport struct {
	fingerprint string
	rt          http.RoundTripper
}

func startDataPlane(cfg Config, store *MemoryStore, logger *slog.Logger) (*dataPlane, error) {
	ln, err := net.Listen("tcp", cfg.DataPlaneAddr)
	if err != nil {
		return nil, fmt.Errorf("data plane listen: %w", err)
	}

	scheme := "http"
	if cfg.tls.Enabled() {
		scheme = "https"
	}
	dp := &dataPlane{
		listener:      ln,
		store:         store,
		logger:        logger,
		scheme:        scheme,
		transports:    make(map[string]*cachedTransport),
		regexps:       make(map[string]*regexp.Regexp),
		oidcProviders: make(map[string]*oidc.Provider),
		pendingLogins: make(map[string]pendingLogin),
		sessions:      make(map[string]gwSession),
		s3Clients:     make(map[string]*cachedS3Client),
	}

	dp.server = &http.Server{Handler: core.TraceHandler(cfg.Name(), dp)}
	go func() {
		if err := core.ServeListener(dp.server, ln, cfg.tls); err != nil && err != http.ErrServerClosed {
			logger.Error("data plane serve error", "error", err)
		}
	}()

	logger.Info("data plane started", "addr", ln.Addr().String(), "scheme", scheme)
	return dp, nil
}

func (dp *dataPlane) Addr() string {
	if dp == nil || dp.listener == nil {
		return ""
	}
	return dp.listener.Addr().String()
}

func (dp *dataPlane) Close() {
	if dp == nil {
		return
	}
	if dp.server != nil {
		dp.server.Close()
	}
}

// matchResult is a matched route with its owning service and the path to
// forward upstream (stripPath already applied).
type matchResult struct {
	route        Route
	service      Service
	upstreamPath string
	// stripAuthorization drops the Authorization header before proxying
	// (the OIDC configuration's hideCredentials).
	stripAuthorization bool
	// stripSessionCookie drops the gateway-internal session cookie before
	// proxying.
	stripSessionCookie bool
}

func (dp *dataPlane) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rw := core.NewResponseRecorder(w)

	m := dp.match(rw, r)
	switch {
	case m == nil:
	case dp.handleCORSPreflight(rw, r, m):
		// Preflights are answered before authentication: browsers send them
		// without credentials.
	case dp.authorize(rw, r, m):
		dp.proxy(rw, r, m)
	}

	args := core.RequestLogArgs(r, rw, "host", r.Host)
	if m != nil {
		args = append(args, "route", m.route.Name, "upstream", m.service.Host)
	}
	dp.logger.Info("data plane request", args...)
}

// match selects the route for the request, writing the error/redirect
// response and returning nil when nothing should be proxied.
func (dp *dataPlane) match(w http.ResponseWriter, r *http.Request) *matchResult {
	host := stripPort(r.Host)

	type candidate struct {
		route    Route
		stripped string // request path with the matched portion removed
		regex    bool
		prefLen  int // matched prefix length, for longest-prefix ordering
	}
	var matched []candidate
	var httpsOnly *Route

	for _, rt := range dp.store.RoutesByHost(host) {
		stripped, prefLen, isRegex, ok := dp.matchPath(rt, r.URL.Path)
		if !ok || !methodAllowed(rt.Methods, r.Method) {
			continue
		}
		if !protocolsInclude(rt.Protocols, dp.scheme) {
			// Host/path/method matched but the protocol did not: remember an
			// https-only route so an http request gets the upgrade response
			// instead of a plain 404.
			if rt.Protocols == "https" && dp.scheme == "http" && httpsOnly == nil {
				cp := rt
				httpsOnly = &cp
			}
			continue
		}
		matched = append(matched, candidate{route: rt, stripped: stripped, regex: isRegex, prefLen: prefLen})
	}

	if len(matched) == 0 {
		if httpsOnly != nil {
			dp.writeHTTPSRedirect(w, r, *httpsOnly)
			return nil
		}
		// Kong's exact message for an unmatched request.
		writeError(w, http.StatusNotFound, "no Route matched with those values")
		return nil
	}

	// Regex routes rank above plain-prefix routes. Among regex routes the
	// lowest regexPriority wins (0 is highest per the spec); among prefix
	// routes the longest matched prefix wins. RoutesByHost returns creation
	// order, and the sort is stable, so ties go to the older route.
	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].regex != matched[j].regex {
			return matched[i].regex
		}
		if matched[i].regex {
			return matched[i].route.RegexPriority < matched[j].route.RegexPriority
		}
		return matched[i].prefLen > matched[j].prefLen
	})
	best := matched[0]

	svc, err := dp.store.GetService(best.route.ServiceID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "service for route not found")
		return nil
	}

	upstreamPath := r.URL.Path
	if best.route.StripPath == nil || *best.route.StripPath {
		upstreamPath = best.stripped
	}
	return &matchResult{route: best.route, service: svc, upstreamPath: upstreamPath}
}

// matchPath reports whether the request path matches the route's path, and
// returns the path with the matched portion removed (for stripPath) plus the
// matched length for longest-prefix ranking. A path starting with "~/" is a
// regular expression anchored at the start (Kong semantics); anything else is
// a literal prefix. An empty route path matches everything.
func (dp *dataPlane) matchPath(rt Route, reqPath string) (stripped string, prefLen int, isRegex, ok bool) {
	routePath := rt.Path
	if routePath == "" {
		routePath = "/"
	}
	if strings.HasPrefix(routePath, "~/") {
		re, err := dp.compiledRegexp(routePath[1:])
		if err != nil {
			return "", 0, true, false
		}
		loc := re.FindStringIndex(reqPath)
		if loc == nil || loc[0] != 0 {
			return "", 0, true, false
		}
		return ensureLeadingSlash(reqPath[loc[1]:]), loc[1], true, true
	}
	if !strings.HasPrefix(reqPath, routePath) {
		return "", 0, false, false
	}
	return ensureLeadingSlash(strings.TrimPrefix(reqPath, routePath)), len(routePath), false, true
}

func (dp *dataPlane) compiledRegexp(pattern string) (*regexp.Regexp, error) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	if re, ok := dp.regexps[pattern]; ok {
		return re, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	dp.regexps[pattern] = re
	return re, nil
}

func ensureLeadingSlash(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

func methodAllowed(methods []string, method string) bool {
	return len(methods) == 0 || slices.Contains(methods, method)
}

// protocolsInclude reports whether the route's protocols value (e.g. "http",
// "https", "http,https") includes the scheme. Substring matching would be
// wrong here: "https" contains "http".
func protocolsInclude(protocols, scheme string) bool {
	return protocols == "" || slices.Contains(strings.Split(protocols, ","), scheme)
}

// writeHTTPSRedirect answers an http request that matched an https-only
// route: a redirect for 3xx codes, or Kong's 426 upgrade response.
func (dp *dataPlane) writeHTTPSRedirect(w http.ResponseWriter, r *http.Request, rt Route) {
	code := rt.HTTPSRedirectStatusCode
	if code == 0 {
		code = 426
	}
	if code == http.StatusUpgradeRequired {
		w.Header().Set("Connection", "Upgrade")
		w.Header().Set("Upgrade", "TLS/1.2, HTTP/1.1")
		writeError(w, code, "Please use HTTPS protocol")
		return
	}
	u := *r.URL
	u.Scheme = "https"
	u.Host = stripPort(r.Host)
	http.Redirect(w, r, u.String(), code)
}

func (dp *dataPlane) proxy(w http.ResponseWriter, r *http.Request, m *matchResult) {
	if m.service.ObjectStorage != nil {
		dp.serveObjectStorage(w, r, m)
		return
	}
	svc := m.service
	upstreamHost := net.JoinHostPort(svc.Host, fmt.Sprintf("%d", svc.Port))
	originalHost := r.Host
	requestOrigin := r.Header.Get("Origin")

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = svc.Protocol
			req.URL.Host = upstreamHost
			req.URL.Path = joinURLPath(svc.Path, m.upstreamPath)
			if m.route.PreserveHost {
				req.Host = originalHost
			} else {
				req.Host = upstreamHost
			}
			req.Header.Set("X-Forwarded-Proto", dp.scheme)
			req.Header.Set("X-Forwarded-Host", originalHost)
			if m.stripAuthorization {
				req.Header.Del("Authorization")
			}
			if m.stripSessionCookie {
				removeCookie(req, sessionCookieName)
			}
			applyRequestTransformation(req, m.route.RequestTransform)
			// A request without a body may be retried safely; dropping the
			// zero-length body makes that explicit for the retry transport.
			if req.ContentLength == 0 {
				req.Body = nil
			}
		},
		ModifyResponse: func(resp *http.Response) error {
			applyCORSResponseHeaders(resp.Header, svc.CorsConfig, requestOrigin)
			return applyResponseTransformation(resp, m.route.ResponseTransform)
		},
		Transport: dp.transportFor(svc),
		ErrorHandler: func(rw http.ResponseWriter, req *http.Request, err error) {
			dp.logger.Debug("upstream error", "route", m.route.Name, "upstream", upstreamHost, "error", err)
			if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
				// Kong's upstream timeout message.
				writeError(rw, http.StatusGatewayTimeout, "The upstream server is timing out")
				return
			}
			// Kong's generic upstream failure message.
			writeError(rw, http.StatusBadGateway, "An invalid response was received from the upstream server")
		},
	}
	proxy.ServeHTTP(w, r)
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// joinURLPath joins the service base path and the (possibly stripped) request
// path without doubling the slash between them.
func joinURLPath(base, p string) string {
	base = strings.TrimSuffix(base, "/")
	if p == "" {
		p = "/"
	}
	return base + p
}

// transportFor returns the cached per-service transport, rebuilding it when
// the service's connection settings changed. Stale entries for deleted
// services are left behind; the cache is bounded by the number of services
// ever configured, which is fine for a mock.
func (dp *dataPlane) transportFor(svc Service) http.RoundTripper {
	retries := 5
	if svc.Retries != nil {
		retries = *svc.Retries
	}
	fp := fmt.Sprintf("%s|%d|%d|%d|%d", svc.Protocol, svc.ConnectTimeout, svc.WriteTimeout, svc.ReadTimeout, retries)

	dp.mu.Lock()
	defer dp.mu.Unlock()
	if ct, ok := dp.transports[svc.ID]; ok && ct.fingerprint == fp {
		return ct.rt
	}
	dialer := &net.Dialer{Timeout: time.Duration(svc.ConnectTimeout) * time.Millisecond}
	readTimeout := time.Duration(svc.ReadTimeout) * time.Millisecond
	writeTimeout := time.Duration(svc.WriteTimeout) * time.Millisecond
	tr := &http.Transport{
		// readTimeout/writeTimeout bound the idle time between two successive
		// read/write operations on the upstream connection (Kong semantics),
		// implemented as per-operation deadlines on the dialed conn. Waiting
		// for response headers is a read, so no ResponseHeaderTimeout needed.
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := dialer.DialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			return &timeoutConn{Conn: conn, readTimeout: readTimeout, writeTimeout: writeTimeout}, nil
		},
		// The mock does not verify upstream TLS certificates so local
		// upstreams with self-signed certificates work out of the box.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	rt := &retryRoundTripper{base: tr, retries: retries}
	dp.transports[svc.ID] = &cachedTransport{fingerprint: fp, rt: rt}
	return rt
}

// retryRoundTripper retries connection-level failures for bodyless requests
// (best-effort approximation of the service's retries setting; requests with
// a body are never replayed).
type retryRoundTripper struct {
	base    http.RoundTripper
	retries int
}

func (t *retryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err == nil || req.Body != nil {
		return resp, err
	}
	for range t.retries {
		if req.Context().Err() != nil {
			break
		}
		if resp, retryErr := t.base.RoundTrip(req); retryErr == nil {
			return resp, nil
		}
	}
	return resp, err
}

// timeoutConn applies the service's readTimeout/writeTimeout as a deadline
// per read/write operation, mirroring how the real gateway's nginx-style
// timeouts bound the interval between successive operations rather than the
// whole exchange.
type timeoutConn struct {
	net.Conn
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func (c *timeoutConn) Read(b []byte) (int, error) {
	if c.readTimeout > 0 {
		if err := c.Conn.SetReadDeadline(time.Now().Add(c.readTimeout)); err != nil {
			return 0, err
		}
	}
	return c.Conn.Read(b)
}

func (c *timeoutConn) Write(b []byte) (int, error) {
	if c.writeTimeout > 0 {
		if err := c.Conn.SetWriteDeadline(time.Now().Add(c.writeTimeout)); err != nil {
			return 0, err
		}
	}
	return c.Conn.Write(b)
}

func stripPort(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}
