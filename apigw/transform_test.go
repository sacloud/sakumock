package apigw_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	v1 "github.com/sacloud/sacloud-sdk-go/api/apigw/apis/v1"
)

// transformEcho reports the upstream's view of a transformed request.
type transformEcho struct {
	Method  string              `json:"method"`
	Query   map[string][]string `json:"query"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body"`
}

func newTransformUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.NewEncoder(w).Encode(transformEcho{
			Method:  r.Method,
			Query:   r.URL.Query(),
			Headers: r.Header,
			Body:    string(body),
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// putJSON sends a raw control-plane PUT (terser than the generated types for
// the deeply nested transformation bodies).
func putJSON(t *testing.T, cpURL, path, body string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), "PUT", cpURL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 204 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("PUT %s: status %d: %s", path, resp.StatusCode, b)
	}
}

func decodeTransformEcho(t *testing.T, resp *http.Response) transformEcho {
	t.Helper()
	var e transformEcho
	if err := json.NewDecoder(resp.Body).Decode(&e); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestDataPlaneRequestTransformation(t *testing.T) {
	srv, client, dpAddr := newGatewayWithControlPlane(t)
	upstream := newTransformUpstream(t)
	svc := createUpstreamService(t, client, "req_transform", upstream.URL, nil)
	serviceID := uuid.MustParse(svc.ID.Value.String())
	route := createGatewayRoute(t, client, serviceID, &v1.RouteDetail{
		Name:      v1.NewOptName("req_transform_route"),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Methods:   []v1.HTTPMethod{v1.HTTPMethodGET, v1.HTTPMethodPOST},
	})
	host := svc.RouteHost.Value

	putJSON(t, srv, "/services/"+svc.ID.Value.String()+"/routes/"+route.ID.Value.String()+"/request", `{
		"remove":  {"headerKeys": ["X-Remove"], "queryParams": ["q_del"], "body": ["drop"]},
		"rename":  {"headers": [{"from": "X-From", "to": "X-To"}], "queryParams": [{"from": "q_from", "to": "q_to"}], "body": [{"from": "old", "to": "new"}]},
		"replace": {"headers": [{"key": "X-Replace", "value": "replaced"}, {"key": "X-Absent", "value": "never"}], "body": [{"key": "exist", "value": "b"}]},
		"add":     {"headers": [{"key": "X-Added", "value": "added"}, {"key": "X-Replace", "value": "not-me"}], "queryParams": [{"key": "q_new", "value": "nv"}], "body": [{"key": "fresh", "value": "c"}]},
		"append":  {"headers": [{"key": "X-Multi", "value": "second"}], "body": [{"key": "tag", "value": "t2"}]}
	}`)

	req, err := http.NewRequestWithContext(t.Context(), "POST",
		"http://"+dpAddr+"/?q_del=1&q_from=f&q_keep=k",
		strings.NewReader(`{"drop":"x","old":"v","exist":"a","tag":"t1","keep":true}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Host = host
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Remove", "1")
	req.Header.Set("X-From", "moved")
	req.Header.Set("X-Replace", "original")
	req.Header.Set("X-Multi", "first")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	e := decodeTransformEcho(t, resp)

	// Headers.
	if _, ok := e.Headers["X-Remove"]; ok {
		t.Error("X-Remove should be removed")
	}
	if got := e.Headers["X-To"]; len(got) != 1 || got[0] != "moved" {
		t.Errorf("X-To = %v, want [moved]", got)
	}
	if got := e.Headers["X-Replace"]; len(got) != 1 || got[0] != "replaced" {
		t.Errorf("X-Replace = %v, want [replaced] (replace wins, add must not touch it)", got)
	}
	if _, ok := e.Headers["X-Absent"]; ok {
		t.Error("replace must not create X-Absent")
	}
	if got := e.Headers["X-Added"]; len(got) != 1 || got[0] != "added" {
		t.Errorf("X-Added = %v, want [added]", got)
	}
	if got := e.Headers["X-Multi"]; len(got) != 2 {
		t.Errorf("X-Multi = %v, want two values (append)", got)
	}

	// Query parameters.
	if _, ok := e.Query["q_del"]; ok {
		t.Error("q_del should be removed")
	}
	if got := e.Query["q_to"]; len(got) != 1 || got[0] != "f" {
		t.Errorf("q_to = %v, want [f]", got)
	}
	if got := e.Query["q_new"]; len(got) != 1 || got[0] != "nv" {
		t.Errorf("q_new = %v, want [nv]", got)
	}
	if got := e.Query["q_keep"]; len(got) != 1 || got[0] != "k" {
		t.Errorf("q_keep = %v, want untouched [k]", got)
	}

	// JSON body.
	var body map[string]any
	if err := json.Unmarshal([]byte(e.Body), &body); err != nil {
		t.Fatalf("upstream body = %q: %v", e.Body, err)
	}
	if _, ok := body["drop"]; ok {
		t.Error("body.drop should be removed")
	}
	if body["new"] != "v" {
		t.Errorf("body.new = %v, want v (renamed from old)", body["new"])
	}
	if body["exist"] != "b" {
		t.Errorf("body.exist = %v, want b (replaced)", body["exist"])
	}
	if body["fresh"] != "c" {
		t.Errorf("body.fresh = %v, want c (added)", body["fresh"])
	}
	if arr, ok := body["tag"].([]any); !ok || len(arr) != 2 {
		t.Errorf("body.tag = %v, want a two-element array (append)", body["tag"])
	}
	if body["keep"] != true {
		t.Errorf("body.keep = %v, want untouched true", body["keep"])
	}
}

func TestDataPlaneRequestTransformationMethodOverride(t *testing.T) {
	srv, client, dpAddr := newGatewayWithControlPlane(t)
	upstream := newTransformUpstream(t)
	svc := createUpstreamService(t, client, "method_transform", upstream.URL, nil)
	serviceID := uuid.MustParse(svc.ID.Value.String())
	route := createGatewayRoute(t, client, serviceID, &v1.RouteDetail{
		Name:      v1.NewOptName("method_route"),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Methods:   []v1.HTTPMethod{v1.HTTPMethodGET},
	})
	putJSON(t, srv, "/services/"+svc.ID.Value.String()+"/routes/"+route.ID.Value.String()+"/request",
		`{"httpMethod": "POST"}`)

	resp := gwDo(t, dpAddr, "GET", svc.RouteHost.Value, "/")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if e := decodeTransformEcho(t, resp); e.Method != "POST" {
		t.Errorf("upstream method = %q, want POST (httpMethod override)", e.Method)
	}
}

func TestDataPlaneResponseTransformation(t *testing.T) {
	srv, client, dpAddr := newGatewayWithControlPlane(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Strip", "1")
		w.Write([]byte(`{"secret":"x","name":"n"}`))
	}))
	t.Cleanup(upstream.Close)
	svc := createUpstreamService(t, client, "resp_transform", upstream.URL, nil)
	serviceID := uuid.MustParse(svc.ID.Value.String())
	route := createGatewayRoute(t, client, serviceID, &v1.RouteDetail{
		Name:      v1.NewOptName("resp_transform_route"),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Methods:   []v1.HTTPMethod{v1.HTTPMethodGET},
	})
	host := svc.RouteHost.Value
	trPath := "/services/" + svc.ID.Value.String() + "/routes/" + route.ID.Value.String() + "/response"

	putJSON(t, srv, trPath, `{
		"remove": {"headerKeys": ["X-Strip"], "jsonKeys": ["secret"]},
		"rename": {"json": [{"from": "name", "to": "title"}]},
		"add":    {"headers": [{"key": "X-Resp-Added", "value": "yes"}], "json": [{"key": "extra", "value": "e"}]}
	}`)

	resp := gwDo(t, dpAddr, "GET", host, "/")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Strip"); got != "" {
		t.Errorf("X-Strip = %q, want removed", got)
	}
	if got := resp.Header.Get("X-Resp-Added"); got != "yes" {
		t.Errorf("X-Resp-Added = %q, want yes", got)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["secret"]; ok {
		t.Error("secret should be removed from the response body")
	}
	if body["title"] != "n" {
		t.Errorf("title = %v, want n (renamed from name)", body["title"])
	}
	if body["extra"] != "e" {
		t.Errorf("extra = %v, want e (added)", body["extra"])
	}

	// ifStatusCode gates the operation: with 404-only conditions nothing
	// applies to this 200 response.
	putJSON(t, srv, trPath, `{
		"remove": {"ifStatusCode": [404], "headerKeys": ["X-Strip"], "jsonKeys": ["secret"]}
	}`)
	resp = gwDo(t, dpAddr, "GET", host, "/")
	if got := resp.Header.Get("X-Strip"); got != "1" {
		t.Errorf("X-Strip = %q, want kept (ifStatusCode 404 must not match 200)", got)
	}
	body = map[string]any{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["secret"]; !ok {
		t.Error("secret should be kept (ifStatusCode 404 must not match 200)")
	}

	// Whole-body replacement.
	putJSON(t, srv, trPath, `{
		"replace": {"body": "gone"}
	}`)
	resp = gwDo(t, dpAddr, "GET", host, "/")
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "gone" {
		t.Errorf("body = %q, want gone (replace.body)", b)
	}

	// Presence, not emptiness, triggers the replacement: an explicit empty
	// string clears the body.
	putJSON(t, srv, trPath, `{
		"replace": {"body": ""}
	}`)
	resp = gwDo(t, dpAddr, "GET", host, "/")
	b, _ = io.ReadAll(resp.Body)
	if string(b) != "" {
		t.Errorf("body = %q, want empty (replace.body: \"\")", b)
	}
}

// newGatewayWithControlPlane is newGateway plus the control-plane base URL
// for raw requests.
func newGatewayWithControlPlane(t *testing.T) (string, *v1.Client, string) {
	t.Helper()
	srv := newGatewayServer(t)
	return srv.TestURL(), newClient(t, srv.TestURL()), srv.DataPlaneAddr()
}
