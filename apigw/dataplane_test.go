package apigw_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	apigwsdk "github.com/sacloud/sacloud-sdk-go/api/apigw"
	v1 "github.com/sacloud/sacloud-sdk-go/api/apigw/apis/v1"

	"github.com/sacloud/sakumock/apigw"
)

// echoUpstream records what the proxied request looked like from the
// upstream's point of view.
type echoUpstream struct {
	Path          string `json:"path"`
	Query         string `json:"query"`
	Host          string `json:"host"`
	XFHost        string `json:"xfHost"`
	XFProto       string `json:"xfProto"`
	XFFor         string `json:"xfFor"`
	Authorization string `json:"authorization"`
}

func newEchoUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(echoUpstream{
			Path:          r.URL.Path,
			Query:         r.URL.RawQuery,
			Host:          r.Host,
			XFHost:        r.Header.Get("X-Forwarded-Host"),
			XFProto:       r.Header.Get("X-Forwarded-Proto"),
			XFFor:         r.Header.Get("X-Forwarded-For"),
			Authorization: r.Header.Get("Authorization"),
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newGateway starts a mock with the data plane enabled and returns the SDK
// client plus the data plane address.
func newGateway(t *testing.T) (*v1.Client, string) {
	t.Helper()
	srv := apigw.NewTestServer(apigw.Config{
		EnableDataPlane: true,
		DataPlaneAddr:   "127.0.0.1:0",
	})
	t.Cleanup(srv.Close)
	if srv.DataPlaneAddr() == "" {
		t.Fatal("data plane address should be set")
	}
	return newClient(t, srv.TestURL()), srv.DataPlaneAddr()
}

// createUpstreamService creates a subscription and a service pointing at the
// given upstream URL, returning the service (with its routeHost).
func createUpstreamService(t *testing.T, client *v1.Client, name, upstreamURL string, mutate func(*v1.ServiceDetailRequest)) *v1.ServiceDetailResponse {
	t.Helper()
	ctx := t.Context()
	u, err := url.Parse(upstreamURL)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(u.Port())
	sub := createSubscription(t, client, name+"_sub")
	req := &v1.ServiceDetailRequest{
		Name:         v1.Name(name),
		Protocol:     v1.ServiceDetailRequestProtocolHTTP,
		Host:         u.Hostname(),
		Port:         v1.NewOptInt(port),
		Subscription: v1.ServiceSubscriptionRequest{ID: sub.ID.Value},
	}
	if mutate != nil {
		mutate(req)
	}
	svcOp := apigwsdk.NewServiceOp(client)
	created, err := svcOp.Create(ctx, req)
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	read, err := svcOp.Read(ctx, created.ID.Value)
	if err != nil {
		t.Fatal(err)
	}
	return read
}

func createGatewayRoute(t *testing.T, client *v1.Client, serviceID uuid.UUID, rt *v1.RouteDetail) *v1.RouteDetail {
	t.Helper()
	created, err := apigwsdk.NewRouteOp(client, serviceID).Create(t.Context(), rt)
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	return created
}

// gwDo sends a request to the data plane listener with the given Host header.
func gwDo(t *testing.T, dpAddr, method, host, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, "http://"+dpAddr+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = host
	client := &http.Client{
		// Redirect responses are asserted as-is.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func decodeEcho(t *testing.T, resp *http.Response) echoUpstream {
	t.Helper()
	var e echoUpstream
	if err := json.NewDecoder(resp.Body).Decode(&e); err != nil {
		t.Fatalf("decode upstream echo: %v", err)
	}
	return e
}

func TestDataPlaneProxy(t *testing.T) {
	client, dpAddr := newGateway(t)
	upstream := newEchoUpstream(t)
	svc := createUpstreamService(t, client, "dp_service", upstream.URL, nil)
	serviceID := uuid.MustParse(svc.ID.Value.String())
	routeHost := svc.RouteHost.Value

	createGatewayRoute(t, client, serviceID, &v1.RouteDetail{
		Name:      v1.NewOptName("api_route"),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Path:      v1.NewOptString("/api"),
		Methods:   []v1.HTTPMethod{v1.HTTPMethodGET},
	})

	t.Run("strips the route path by default", func(t *testing.T) {
		resp := gwDo(t, dpAddr, "GET", routeHost, "/api/items?q=1")
		if resp.StatusCode != 200 {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		e := decodeEcho(t, resp)
		if e.Path != "/items" {
			t.Errorf("upstream path = %q, want /items", e.Path)
		}
		if e.Query != "q=1" {
			t.Errorf("upstream query = %q, want q=1", e.Query)
		}
		if !strings.HasPrefix(e.Host, "127.0.0.1") {
			t.Errorf("upstream host = %q, want the upstream address (preserveHost=false)", e.Host)
		}
		if e.XFHost != routeHost {
			t.Errorf("X-Forwarded-Host = %q, want %q", e.XFHost, routeHost)
		}
		if e.XFProto != "http" {
			t.Errorf("X-Forwarded-Proto = %q, want http", e.XFProto)
		}
		if e.XFFor == "" {
			t.Error("X-Forwarded-For should be set")
		}
	})

	t.Run("method not allowed does not match", func(t *testing.T) {
		resp := gwDo(t, dpAddr, "POST", routeHost, "/api/items")
		if resp.StatusCode != 404 {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("host matching is case-insensitive", func(t *testing.T) {
		resp := gwDo(t, dpAddr, "GET", strings.ToUpper(routeHost), "/api/items")
		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
	})

	t.Run("unknown host does not match", func(t *testing.T) {
		resp := gwDo(t, dpAddr, "GET", "nope.localhost", "/api/items")
		if resp.StatusCode != 404 {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "no Route matched with those values") {
			t.Errorf("body = %s", body)
		}
	})

	t.Run("unknown path does not match", func(t *testing.T) {
		resp := gwDo(t, dpAddr, "GET", routeHost, "/other")
		if resp.StatusCode != 404 {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})
}

func TestDataPlaneRouteOptions(t *testing.T) {
	client, dpAddr := newGateway(t)
	upstream := newEchoUpstream(t)
	svc := createUpstreamService(t, client, "dp_options", upstream.URL, func(req *v1.ServiceDetailRequest) {
		req.Path = v1.NewOptString("/base")
	})
	serviceID := uuid.MustParse(svc.ID.Value.String())
	routeHost := svc.RouteHost.Value

	createGatewayRoute(t, client, serviceID, &v1.RouteDetail{
		Name:         v1.NewOptName("keep_route"),
		Protocols:    v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Path:         v1.NewOptString("/keep"),
		Methods:      []v1.HTTPMethod{v1.HTTPMethodGET},
		StripPath:    v1.NewOptBool(false),
		PreserveHost: v1.NewOptBool(true),
	})

	resp := gwDo(t, dpAddr, "GET", routeHost, "/keep/x")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	e := decodeEcho(t, resp)
	// stripPath=false keeps the route path, and the service base path is
	// prepended.
	if e.Path != "/base/keep/x" {
		t.Errorf("upstream path = %q, want /base/keep/x", e.Path)
	}
	// preserveHost=true forwards the client's Host header.
	if e.Host != routeHost {
		t.Errorf("upstream host = %q, want %q", e.Host, routeHost)
	}
}

func TestDataPlaneCustomDomain(t *testing.T) {
	client, dpAddr := newGateway(t)
	upstream := newEchoUpstream(t)
	svc := createUpstreamService(t, client, "dp_domain", upstream.URL, nil)
	serviceID := uuid.MustParse(svc.ID.Value.String())

	if _, err := apigwsdk.NewDomainOp(client).Create(t.Context(), &v1.Domain{
		DomainName: "gw.example.com",
	}); err != nil {
		t.Fatal(err)
	}
	createGatewayRoute(t, client, serviceID, &v1.RouteDetail{
		Name:      v1.NewOptName("domain_route"),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Hosts:     []string{"gw.example.com"},
		Methods:   []v1.HTTPMethod{v1.HTTPMethodGET},
	})

	if resp := gwDo(t, dpAddr, "GET", "gw.example.com", "/x"); resp.StatusCode != 200 {
		t.Errorf("custom domain: status = %d, want 200", resp.StatusCode)
	}
	// With hosts set, the auto-issued host is no longer effective.
	if resp := gwDo(t, dpAddr, "GET", svc.RouteHost.Value, "/x"); resp.StatusCode != 404 {
		t.Errorf("auto host with hosts set: status = %d, want 404", resp.StatusCode)
	}
}

func TestDataPlaneRegexRoutes(t *testing.T) {
	client, dpAddr := newGateway(t)
	upstream := newEchoUpstream(t)
	svc := createUpstreamService(t, client, "dp_regex", upstream.URL, nil)
	serviceID := uuid.MustParse(svc.ID.Value.String())
	routeHost := svc.RouteHost.Value

	// A catch-all prefix route plus two overlapping regex routes with
	// different priorities (0 is the highest).
	createGatewayRoute(t, client, serviceID, &v1.RouteDetail{
		Name:      v1.NewOptName("catch_all"),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Path:      v1.NewOptString("/"),
		Methods:   []v1.HTTPMethod{v1.HTTPMethodGET},
		StripPath: v1.NewOptBool(false),
	})
	createGatewayRoute(t, client, serviceID, &v1.RouteDetail{
		Name:          v1.NewOptName("regex_low"),
		Protocols:     v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Path:          v1.NewOptString(`~/items/\d+`),
		Methods:       []v1.HTTPMethod{v1.HTTPMethodGET},
		RegexPriority: v1.NewOptInt(5),
		StripPath:     v1.NewOptBool(true),
	})
	createGatewayRoute(t, client, serviceID, &v1.RouteDetail{
		Name:          v1.NewOptName("regex_high"),
		Protocols:     v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Path:          v1.NewOptString(`~/items/1\d*`),
		Methods:       []v1.HTTPMethod{v1.HTTPMethodGET},
		RegexPriority: v1.NewOptInt(0),
		StripPath:     v1.NewOptBool(false),
	})

	// Both regexes match /items/123; priority 0 must win over 5, and regex
	// routes must win over the catch-all prefix. regex_high keeps the path.
	resp := gwDo(t, dpAddr, "GET", routeHost, "/items/123")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if e := decodeEcho(t, resp); e.Path != "/items/123" {
		t.Errorf("upstream path = %q, want /items/123 (regex_high, stripPath=false)", e.Path)
	}

	// Only regex_low matches /items/456; stripPath removes the matched part.
	resp = gwDo(t, dpAddr, "GET", routeHost, "/items/456/details")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if e := decodeEcho(t, resp); e.Path != "/details" {
		t.Errorf("upstream path = %q, want /details (regex_low, stripPath=true)", e.Path)
	}

	// Neither regex matches /other; the catch-all takes it.
	resp = gwDo(t, dpAddr, "GET", routeHost, "/other")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if e := decodeEcho(t, resp); e.Path != "/other" {
		t.Errorf("upstream path = %q, want /other (catch_all)", e.Path)
	}
}

func TestDataPlaneHTTPSRedirect(t *testing.T) {
	client, dpAddr := newGateway(t)
	upstream := newEchoUpstream(t)
	svc := createUpstreamService(t, client, "dp_https", upstream.URL, nil)
	serviceID := uuid.MustParse(svc.ID.Value.String())
	routeHost := svc.RouteHost.Value

	createGatewayRoute(t, client, serviceID, &v1.RouteDetail{
		Name:      v1.NewOptName("upgrade_route"),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTPS),
		Path:      v1.NewOptString("/upgrade"),
		Methods:   []v1.HTTPMethod{v1.HTTPMethodGET},
	})
	createGatewayRoute(t, client, serviceID, &v1.RouteDetail{
		Name:                    v1.NewOptName("redirect_route"),
		Protocols:               v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTPS),
		Path:                    v1.NewOptString("/redirect"),
		Methods:                 []v1.HTTPMethod{v1.HTTPMethodGET},
		HttpsRedirectStatusCode: v1.NewOptRouteDetailHttpsRedirectStatusCode(v1.RouteDetailHttpsRedirectStatusCode301),
	})

	resp := gwDo(t, dpAddr, "GET", routeHost, "/upgrade")
	if resp.StatusCode != 426 {
		t.Errorf("status = %d, want 426", resp.StatusCode)
	}
	if got := resp.Header.Get("Upgrade"); !strings.Contains(got, "TLS") {
		t.Errorf("Upgrade header = %q", got)
	}

	resp = gwDo(t, dpAddr, "GET", routeHost, "/redirect")
	if resp.StatusCode != 301 {
		t.Errorf("status = %d, want 301", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "https://"+routeHost+"/redirect" {
		t.Errorf("Location = %q", loc)
	}
}

func TestDataPlaneUpstreamErrors(t *testing.T) {
	client, dpAddr := newGateway(t)

	t.Run("unreachable upstream returns 502", func(t *testing.T) {
		down := httptest.NewServer(http.NotFoundHandler())
		downURL := down.URL
		down.Close()
		svc := createUpstreamService(t, client, "dp_down", downURL, nil)
		createGatewayRoute(t, client, uuid.MustParse(svc.ID.Value.String()), &v1.RouteDetail{
			Name:      v1.NewOptName("down_route"),
			Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
			Methods:   []v1.HTTPMethod{v1.HTTPMethodGET},
		})
		resp := gwDo(t, dpAddr, "GET", svc.RouteHost.Value, "/")
		if resp.StatusCode != 502 {
			t.Fatalf("status = %d, want 502", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "invalid response was received from the upstream") {
			t.Errorf("body = %s", body)
		}
	})

	t.Run("stalled write returns 504", func(t *testing.T) {
		// The handler never reads the request body, so a large upload stalls
		// once the kernel buffers fill and the writeTimeout fires.
		stall := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(1 * time.Second)
		}))
		t.Cleanup(stall.Close)
		svc := createUpstreamService(t, client, "dp_stall", stall.URL, func(req *v1.ServiceDetailRequest) {
			req.WriteTimeout = v1.NewOptInt(100) // ms
			req.Retries = v1.NewOptInt(0)
		})
		createGatewayRoute(t, client, uuid.MustParse(svc.ID.Value.String()), &v1.RouteDetail{
			Name:      v1.NewOptName("stall_route"),
			Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
			Methods:   []v1.HTTPMethod{v1.HTTPMethodPOST},
		})
		req, err := http.NewRequestWithContext(t.Context(), "POST", "http://"+dpAddr+"/",
			bytes.NewReader(make([]byte, 32<<20)))
		if err != nil {
			t.Fatal(err)
		}
		req.Host = svc.RouteHost.Value
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 504 {
			t.Fatalf("status = %d, want 504", resp.StatusCode)
		}
	})

	t.Run("slow upstream returns 504", func(t *testing.T) {
		slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(500 * time.Millisecond)
		}))
		t.Cleanup(slow.Close)
		svc := createUpstreamService(t, client, "dp_slow", slow.URL, func(req *v1.ServiceDetailRequest) {
			req.ReadTimeout = v1.NewOptInt(100) // ms
			req.Retries = v1.NewOptInt(0)
		})
		createGatewayRoute(t, client, uuid.MustParse(svc.ID.Value.String()), &v1.RouteDetail{
			Name:      v1.NewOptName("slow_route"),
			Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
			Methods:   []v1.HTTPMethod{v1.HTTPMethodGET},
		})
		resp := gwDo(t, dpAddr, "GET", svc.RouteHost.Value, "/")
		if resp.StatusCode != 504 {
			t.Fatalf("status = %d, want 504", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "timing out") {
			t.Errorf("body = %s", body)
		}
	})
}
