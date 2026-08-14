package apigw_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	apigwsdk "github.com/sacloud/sacloud-sdk-go/api/apigw"
	v1 "github.com/sacloud/sacloud-sdk-go/api/apigw/apis/v1"
)

func preflight(origin, requestMethod string) func(*http.Request) {
	return func(r *http.Request) {
		r.Header.Set("Origin", origin)
		r.Header.Set("Access-Control-Request-Method", requestMethod)
	}
}

func corsService(t *testing.T, client *v1.Client, name, upstreamURL string, cors v1.CorsConfig, mutate func(*v1.ServiceDetailRequest)) *v1.ServiceDetailResponse {
	t.Helper()
	return createUpstreamService(t, client, name, upstreamURL, func(req *v1.ServiceDetailRequest) {
		req.CorsConfig = v1.NewOptCorsConfig(cors)
		if mutate != nil {
			mutate(req)
		}
	})
}

func TestDataPlaneCORS(t *testing.T) {
	client, dpAddr := newGateway(t)
	upstream := newEchoUpstream(t)

	svc := corsService(t, client, "cors_svc", upstream.URL, v1.CorsConfig{
		AccessControlAllowOrigins:   v1.NewOptString("http://app.example.com, http://other.example.com"),
		AccessControlAllowMethods:   []v1.HTTPMethod{v1.HTTPMethodGET, v1.HTTPMethodPOST},
		AccessControlAllowHeaders:   v1.NewOptString("X-Custom"),
		AccessControlExposedHeaders: v1.NewOptString("X-Expose"),
		MaxAge:                      v1.NewOptInt32(3600),
		Credentials:                 v1.NewOptBool(true),
	}, nil)
	createGatewayRoute(t, client, uuid.MustParse(svc.ID.Value.String()), &v1.RouteDetail{
		Name:      v1.NewOptName("cors_route"),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Methods:   []v1.HTTPMethod{v1.HTTPMethodGET, v1.HTTPMethodPOST, v1.HTTPMethodOPTIONS},
	})
	host := svc.RouteHost.Value

	t.Run("preflight is answered by the gateway", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "OPTIONS", host, "/", preflight("http://app.example.com", "POST"))
		if resp.StatusCode != 200 {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		h := resp.Header
		if got := h.Get("Access-Control-Allow-Origin"); got != "http://app.example.com" {
			t.Errorf("Allow-Origin = %q", got)
		}
		if got := h.Get("Access-Control-Allow-Methods"); got != "GET,POST" {
			t.Errorf("Allow-Methods = %q", got)
		}
		if got := h.Get("Access-Control-Allow-Headers"); got != "X-Custom" {
			t.Errorf("Allow-Headers = %q", got)
		}
		if got := h.Get("Access-Control-Max-Age"); got != "3600" {
			t.Errorf("Max-Age = %q", got)
		}
		if got := h.Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Errorf("Allow-Credentials = %q", got)
		}
		// The gateway answers directly; the response is not the upstream echo.
		if ct := h.Get("Content-Type"); ct != "" {
			t.Errorf("preflight should not be proxied (Content-Type = %q)", ct)
		}
	})

	t.Run("actual response gets the CORS headers", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/", func(r *http.Request) {
			r.Header.Set("Origin", "http://app.example.com")
		})
		if resp.StatusCode != 200 {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://app.example.com" {
			t.Errorf("Allow-Origin = %q", got)
		}
		if got := resp.Header.Get("Access-Control-Expose-Headers"); got != "X-Expose" {
			t.Errorf("Expose-Headers = %q", got)
		}
		if got := resp.Header.Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Errorf("Allow-Credentials = %q", got)
		}
		if got := resp.Header.Get("Vary"); got != "Origin" {
			t.Errorf("Vary = %q", got)
		}
	})

	t.Run("disallowed origin gets no CORS headers", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/", func(r *http.Request) {
			r.Header.Set("Origin", "http://evil.example.com")
		})
		// The request is still proxied (the browser is what blocks it).
		if resp.StatusCode != 200 {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("Allow-Origin = %q, want none", got)
		}
	})

	t.Run("requests without Origin are untouched", func(t *testing.T) {
		resp := gwDo(t, dpAddr, "GET", host, "/")
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("Allow-Origin = %q, want none", got)
		}
	})
}

func TestDataPlaneCORSWildcard(t *testing.T) {
	client, dpAddr := newGateway(t)
	upstream := newEchoUpstream(t)

	t.Run("wildcard without credentials", func(t *testing.T) {
		svc := corsService(t, client, "cors_wild", upstream.URL, v1.CorsConfig{
			AccessControlAllowOrigins: v1.NewOptString("*"),
		}, nil)
		createGatewayRoute(t, client, uuid.MustParse(svc.ID.Value.String()), &v1.RouteDetail{
			Name:      v1.NewOptName("wild_route"),
			Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
			Methods:   []v1.HTTPMethod{v1.HTTPMethodGET, v1.HTTPMethodOPTIONS},
		})
		resp := gwDoWith(t, dpAddr, "OPTIONS", svc.RouteHost.Value, "/", preflight("http://anywhere.example.com", "GET"))
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
			t.Errorf("Allow-Origin = %q, want *", got)
		}
		// Unconfigured methods reflect the gateway default allow-list.
		if got := resp.Header.Get("Access-Control-Allow-Methods"); got == "" {
			t.Error("Allow-Methods should default to the full method list")
		}
	})

	t.Run("wildcard with credentials echoes the origin", func(t *testing.T) {
		svc := corsService(t, client, "cors_wild_cred", upstream.URL, v1.CorsConfig{
			AccessControlAllowOrigins: v1.NewOptString("*"),
			Credentials:               v1.NewOptBool(true),
		}, nil)
		createGatewayRoute(t, client, uuid.MustParse(svc.ID.Value.String()), &v1.RouteDetail{
			Name:      v1.NewOptName("wild_cred_route"),
			Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
			Methods:   []v1.HTTPMethod{v1.HTTPMethodGET, v1.HTTPMethodOPTIONS},
		})
		resp := gwDoWith(t, dpAddr, "OPTIONS", svc.RouteHost.Value, "/", preflight("http://spa.example.com", "GET"))
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://spa.example.com" {
			t.Errorf("Allow-Origin = %q, want the echoed origin ('*' is invalid with credentials)", got)
		}
	})
}

func TestDataPlaneCORSPrecedesAuthentication(t *testing.T) {
	client, dpAddr := newGateway(t)
	upstream := newEchoUpstream(t)
	svc := corsService(t, client, "cors_auth", upstream.URL, v1.CorsConfig{
		AccessControlAllowOrigins: v1.NewOptString("http://app.example.com"),
	}, func(req *v1.ServiceDetailRequest) {
		req.Authentication = v1.NewOptServiceDetailRequestAuthentication(v1.ServiceDetailRequestAuthenticationBasic)
	})
	createGatewayRoute(t, client, uuid.MustParse(svc.ID.Value.String()), &v1.RouteDetail{
		Name:      v1.NewOptName("cors_auth_route"),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Methods:   []v1.HTTPMethod{v1.HTTPMethodGET, v1.HTTPMethodOPTIONS},
	})
	host := svc.RouteHost.Value

	// Browsers send preflights without credentials; the gateway must answer
	// them before authentication.
	resp := gwDoWith(t, dpAddr, "OPTIONS", host, "/", preflight("http://app.example.com", "GET"))
	if resp.StatusCode != 200 {
		t.Fatalf("preflight status = %d, want 200 (must precede authentication)", resp.StatusCode)
	}

	// The actual request is still authenticated.
	resp = gwDoWith(t, dpAddr, "GET", host, "/", func(r *http.Request) {
		r.Header.Set("Origin", "http://app.example.com")
	})
	assertStatus(t, resp, 401, "Unauthorized")
}

func TestDataPlaneCORSPreflightContinue(t *testing.T) {
	client, dpAddr := newGateway(t)
	upstream := newEchoUpstream(t)
	svc := corsService(t, client, "cors_continue", upstream.URL, v1.CorsConfig{
		AccessControlAllowOrigins: v1.NewOptString("http://app.example.com"),
		PreflightContinue:         v1.NewOptBool(true),
	}, nil)
	createGatewayRoute(t, client, uuid.MustParse(svc.ID.Value.String()), &v1.RouteDetail{
		Name:      v1.NewOptName("continue_route"),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Methods:   []v1.HTTPMethod{v1.HTTPMethodGET, v1.HTTPMethodOPTIONS},
	})

	// With preflightContinue the OPTIONS request reaches the upstream.
	resp := gwDoWith(t, dpAddr, "OPTIONS", svc.RouteHost.Value, "/", preflight("http://app.example.com", "GET"))
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if e := decodeEcho(t, resp); e.XFHost == "" {
		t.Error("preflight should have been proxied to the upstream")
	}
}

func TestDataPlaneCORSObjectStoragePreflightContinue(t *testing.T) {
	client, dpAddr := newGateway(t)
	fakeS3 := newFakeS3(t, map[string]string{"pf-bucket/hello.txt": "hi"})

	sub := createSubscription(t, client, "cors_s3_sub")
	svcOp := apigwsdk.NewServiceOp(client)
	created, err := svcOp.Create(t.Context(), &v1.ServiceDetailRequest{
		Name:         "cors_s3",
		Protocol:     v1.ServiceDetailRequestProtocolHTTPS,
		Host:         "storage.example.com",
		Subscription: v1.ServiceSubscriptionRequest{ID: sub.ID.Value},
		ObjectStorageConfig: v1.NewOptObjectStorageConfig(v1.ObjectStorageConfig{
			BucketName:       "pf-bucket",
			Endpoint:         fakeS3.URL,
			Region:           "jp-north-1",
			AccessKeyID:      "k",
			SecretAccessKey:  "s",
			UseDocumentIndex: true,
		}),
		CorsConfig: v1.NewOptCorsConfig(v1.CorsConfig{
			AccessControlAllowOrigins: v1.NewOptString("http://app.example.com"),
			PreflightContinue:         v1.NewOptBool(true),
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := svcOp.Read(t.Context(), created.ID.Value)
	if err != nil {
		t.Fatal(err)
	}
	createGatewayRoute(t, client, uuid.MustParse(svc.ID.Value.String()), &v1.RouteDetail{
		Name:      v1.NewOptName("cors_s3_route"),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Methods:   []v1.HTTPMethod{v1.HTTPMethodGET, v1.HTTPMethodOPTIONS},
	})

	// The preflight is forwarded to the S3 endpoint, whose bucket-CORS
	// answer is relayed.
	resp := gwDoWith(t, dpAddr, "OPTIONS", svc.RouteHost.Value, "/hello.txt", preflight("http://app.example.com", "GET"))
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://app.example.com" {
		t.Errorf("Allow-Origin = %q, want the bucket CORS echo", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Methods"); got != "GET" {
		t.Errorf("Allow-Methods = %q, want the bucket CORS value", got)
	}
}
