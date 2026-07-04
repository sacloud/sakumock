package apigw_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	apigwsdk "github.com/sacloud/sacloud-sdk-go/api/apigw"
	v1 "github.com/sacloud/sacloud-sdk-go/api/apigw/apis/v1"
)

// gwDoWith sends a request to the data plane with the Host header set,
// letting the caller decorate the request (credentials etc.).
func gwDoWith(t *testing.T, dpAddr, method, host, path string, decorate func(*http.Request)) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, "http://"+dpAddr+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = host
	if decorate != nil {
		decorate(req)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func assertStatus(t *testing.T, resp *http.Response, want int, wantBody string) {
	t.Helper()
	if resp.StatusCode != want {
		t.Errorf("status = %d, want %d", resp.StatusCode, want)
	}
	if wantBody != "" {
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), wantBody) {
			t.Errorf("body = %s, want it to contain %q", body, wantBody)
		}
	}
}

// createAuthUser creates a user with the given credentials and returns its ID.
func createAuthUser(t *testing.T, client *v1.Client, name string, auth v1.UserAuthentication) uuid.UUID {
	t.Helper()
	user, err := apigwsdk.NewUserOp(client).Create(t.Context(), &v1.UserDetail{Name: v1.Name(name)})
	if err != nil {
		t.Fatal(err)
	}
	userID := uuid.MustParse(user.ID.Value.String())
	if err := apigwsdk.NewUserExtraOp(client, userID).UpdateAuth(t.Context(), auth); err != nil {
		t.Fatal(err)
	}
	return userID
}

func TestDataPlaneBasicAuth(t *testing.T) {
	client, dpAddr := newGateway(t)
	upstream := newEchoUpstream(t)
	svc := createUpstreamService(t, client, "auth_basic", upstream.URL, func(req *v1.ServiceDetailRequest) {
		req.Authentication = v1.NewOptServiceDetailRequestAuthentication(v1.ServiceDetailRequestAuthenticationBasic)
	})
	createGatewayRoute(t, client, uuid.MustParse(svc.ID.Value.String()), &v1.RouteDetail{
		Name:      v1.NewOptName("basic_route"),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Methods:   []v1.HTTPMethod{v1.HTTPMethodGET},
	})
	createAuthUser(t, client, "basic_user", v1.UserAuthentication{
		BasicAuth: v1.NewOptBasicAuth(v1.BasicAuth{UserName: "alice", Password: "open sesame"}),
	})
	host := svc.RouteHost.Value

	t.Run("missing credentials", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/", nil)
		assertStatus(t, resp, 401, "Unauthorized")
		if got := resp.Header.Get("WWW-Authenticate"); !strings.HasPrefix(got, "Basic") {
			t.Errorf("WWW-Authenticate = %q", got)
		}
	})
	t.Run("wrong password", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/", func(r *http.Request) { r.SetBasicAuth("alice", "nope") })
		assertStatus(t, resp, 401, "Invalid authentication credentials")
	})
	t.Run("unknown user", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/", func(r *http.Request) { r.SetBasicAuth("mallory", "open sesame") })
		assertStatus(t, resp, 401, "Invalid authentication credentials")
	})
	t.Run("valid credentials", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/", func(r *http.Request) { r.SetBasicAuth("alice", "open sesame") })
		assertStatus(t, resp, 200, "")
	})
}

func signJWT(t *testing.T, method jwt.SigningMethod, iss, secret string, exp time.Time) string {
	t.Helper()
	token, err := jwt.NewWithClaims(method, jwt.MapClaims{
		"iss": iss,
		"exp": exp.Unix(),
	}).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestDataPlaneJwtAuth(t *testing.T) {
	client, dpAddr := newGateway(t)
	upstream := newEchoUpstream(t)
	svc := createUpstreamService(t, client, "auth_jwt", upstream.URL, func(req *v1.ServiceDetailRequest) {
		req.Authentication = v1.NewOptServiceDetailRequestAuthentication(v1.ServiceDetailRequestAuthenticationJwt)
	})
	createGatewayRoute(t, client, uuid.MustParse(svc.ID.Value.String()), &v1.RouteDetail{
		Name:      v1.NewOptName("jwt_route"),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Methods:   []v1.HTTPMethod{v1.HTTPMethodGET},
	})
	createAuthUser(t, client, "jwt_user", v1.UserAuthentication{
		Jwt: v1.NewOptJwt(v1.Jwt{Key: "issuer-key", Secret: "jwt-secret", Algorithm: v1.JwtAlgorithmHS256}),
	})
	host := svc.RouteHost.Value
	bearer := func(token string) func(*http.Request) {
		return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) }
	}
	in1h := time.Now().Add(time.Hour)

	t.Run("missing token", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/", nil)
		assertStatus(t, resp, 401, "Unauthorized")
	})
	t.Run("unknown iss", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/", bearer(signJWT(t, jwt.SigningMethodHS256, "other-key", "jwt-secret", in1h)))
		assertStatus(t, resp, 401, "No credentials found for given 'iss'")
	})
	t.Run("wrong secret", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/", bearer(signJWT(t, jwt.SigningMethodHS256, "issuer-key", "wrong", in1h)))
		assertStatus(t, resp, 401, "Invalid signature")
	})
	t.Run("algorithm mismatch", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/", bearer(signJWT(t, jwt.SigningMethodHS384, "issuer-key", "jwt-secret", in1h)))
		assertStatus(t, resp, 401, "Invalid signature")
	})
	t.Run("expired token", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/", bearer(signJWT(t, jwt.SigningMethodHS256, "issuer-key", "jwt-secret", time.Now().Add(-time.Hour))))
		assertStatus(t, resp, 401, "token expired")
	})
	t.Run("valid token", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/", bearer(signJWT(t, jwt.SigningMethodHS256, "issuer-key", "jwt-secret", in1h)))
		assertStatus(t, resp, 200, "")
	})
}

// hmacAuthorize signs the request draft-cavage style with headers
// "date request-line" and sets the Authorization and Date headers.
func hmacAuthorize(username, secret, date string) func(*http.Request) {
	return func(r *http.Request) {
		r.Header.Set("Date", date)
		signing := "date: " + date + "\n" + r.Method + " " + r.URL.RequestURI() + " HTTP/1.1"
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(signing))
		sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		r.Header.Set("Authorization",
			`hmac username="`+username+`",algorithm="hmac-sha256",headers="date request-line",signature="`+sig+`"`)
	}
}

func TestDataPlaneHmacAuth(t *testing.T) {
	client, dpAddr := newGateway(t)
	upstream := newEchoUpstream(t)
	svc := createUpstreamService(t, client, "auth_hmac", upstream.URL, func(req *v1.ServiceDetailRequest) {
		req.Authentication = v1.NewOptServiceDetailRequestAuthentication(v1.ServiceDetailRequestAuthenticationHmac)
	})
	createGatewayRoute(t, client, uuid.MustParse(svc.ID.Value.String()), &v1.RouteDetail{
		Name:      v1.NewOptName("hmac_route"),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Methods:   []v1.HTTPMethod{v1.HTTPMethodGET},
	})
	createAuthUser(t, client, "hmac_user", v1.UserAuthentication{
		HmacAuth: v1.NewOptHmacAuth(v1.HmacAuth{UserName: "bob", Secret: "hmac-secret"}),
	})
	host := svc.RouteHost.Value
	now := time.Now().UTC().Format(http.TimeFormat)

	t.Run("missing authorization", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/x", nil)
		assertStatus(t, resp, 401, "Unauthorized")
	})
	t.Run("wrong secret", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/x", hmacAuthorize("bob", "wrong", now))
		assertStatus(t, resp, 401, "HMAC signature cannot be verified")
	})
	t.Run("unknown user", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/x", hmacAuthorize("nobody", "hmac-secret", now))
		assertStatus(t, resp, 401, "HMAC signature cannot be verified")
	})
	t.Run("stale date", func(t *testing.T) {
		stale := time.Now().UTC().Add(-10 * time.Minute).Format(http.TimeFormat)
		resp := gwDoWith(t, dpAddr, "GET", host, "/x", hmacAuthorize("bob", "hmac-secret", stale))
		assertStatus(t, resp, 401, "HMAC signature cannot be verified")
	})
	t.Run("valid signature", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/x", hmacAuthorize("bob", "hmac-secret", now))
		assertStatus(t, resp, 200, "")
	})
	t.Run("signature does not cover a tampered path", func(t *testing.T) {
		// Sign /x but request /y: request-line is part of the signing string.
		resp := gwDoWith(t, dpAddr, "GET", host, "/y", func(r *http.Request) {
			saved := *r.URL
			r.URL.Path = "/x"
			hmacAuthorize("bob", "hmac-secret", now)(r)
			*r.URL = saved
		})
		assertStatus(t, resp, 401, "HMAC signature cannot be verified")
	})
}

func TestDataPlaneACL(t *testing.T) {
	client, dpAddr := newGateway(t)
	upstream := newEchoUpstream(t)
	svc := createUpstreamService(t, client, "auth_acl", upstream.URL, func(req *v1.ServiceDetailRequest) {
		req.Authentication = v1.NewOptServiceDetailRequestAuthentication(v1.ServiceDetailRequestAuthenticationBasic)
	})
	serviceID := uuid.MustParse(svc.ID.Value.String())
	route := createGatewayRoute(t, client, serviceID, &v1.RouteDetail{
		Name:      v1.NewOptName("acl_route"),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Methods:   []v1.HTTPMethod{v1.HTTPMethodGET},
	})
	host := svc.RouteHost.Value

	memberID := createAuthUser(t, client, "acl_member", v1.UserAuthentication{
		BasicAuth: v1.NewOptBasicAuth(v1.BasicAuth{UserName: "member", Password: "pw"}),
	})
	createAuthUser(t, client, "acl_outsider", v1.UserAuthentication{
		BasicAuth: v1.NewOptBasicAuth(v1.BasicAuth{UserName: "outsider", Password: "pw"}),
	})

	group, err := apigwsdk.NewGroupOp(client).Create(t.Context(), &v1.Group{Name: v1.NewOptName("acl_group")})
	if err != nil {
		t.Fatal(err)
	}
	if err := apigwsdk.NewUserExtraOp(client, memberID).UpdateGroup(t.Context(), "acl_group", true); err != nil {
		t.Fatal(err)
	}
	extraOp := apigwsdk.NewRouteExtraOp(client, serviceID, uuid.MustParse(route.ID.Value.String()))
	if err := extraOp.EnableAuthorization(t.Context(), []v1.RouteAuthorization{
		{ID: group.ID, Enabled: v1.NewOptBool(true)},
	}); err != nil {
		t.Fatal(err)
	}

	basic := func(user string) func(*http.Request) {
		return func(r *http.Request) { r.SetBasicAuth(user, "pw") }
	}

	t.Run("group member passes", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/", basic("member"))
		assertStatus(t, resp, 200, "")
	})
	t.Run("outsider is forbidden", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/", basic("outsider"))
		assertStatus(t, resp, 403, "You cannot consume this service")
	})
	t.Run("disabled ACL lets everyone through", func(t *testing.T) {
		if err := extraOp.DisableAuthorization(t.Context()); err != nil {
			t.Fatal(err)
		}
		resp := gwDoWith(t, dpAddr, "GET", host, "/", basic("outsider"))
		assertStatus(t, resp, 200, "")
	})
}

func TestDataPlaneACLWithoutAuthentication(t *testing.T) {
	client, dpAddr := newGateway(t)
	upstream := newEchoUpstream(t)
	svc := createUpstreamService(t, client, "acl_none", upstream.URL, nil) // authentication: none
	serviceID := uuid.MustParse(svc.ID.Value.String())
	route := createGatewayRoute(t, client, serviceID, &v1.RouteDetail{
		Name:      v1.NewOptName("acl_none_route"),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Methods:   []v1.HTTPMethod{v1.HTTPMethodGET},
	})
	group, err := apigwsdk.NewGroupOp(client).Create(t.Context(), &v1.Group{Name: v1.NewOptName("some_group")})
	if err != nil {
		t.Fatal(err)
	}
	if err := apigwsdk.NewRouteExtraOp(client, serviceID, uuid.MustParse(route.ID.Value.String())).
		EnableAuthorization(t.Context(), []v1.RouteAuthorization{{ID: group.ID, Enabled: v1.NewOptBool(true)}}); err != nil {
		t.Fatal(err)
	}

	// An ACL without an authentication scheme can never identify a consumer.
	resp := gwDoWith(t, dpAddr, "GET", svc.RouteHost.Value, "/", nil)
	assertStatus(t, resp, 401, "Unauthorized")
}

func TestDataPlaneIPRestriction(t *testing.T) {
	client, dpAddr := newGateway(t)
	upstream := newEchoUpstream(t)

	t.Run("route-level deny", func(t *testing.T) {
		svc := createUpstreamService(t, client, "ip_deny", upstream.URL, nil)
		createGatewayRoute(t, client, uuid.MustParse(svc.ID.Value.String()), &v1.RouteDetail{
			Name:      v1.NewOptName("deny_route"),
			Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
			Methods:   []v1.HTTPMethod{v1.HTTPMethodGET},
			IpRestrictionConfig: v1.NewOptIpRestrictionConfig(v1.IpRestrictionConfig{
				Protocols:    v1.IpRestrictionConfigProtocolsHTTPHTTPS,
				RestrictedBy: v1.IpRestrictionConfigRestrictedByDenyIps,
				Ips:          []string{"127.0.0.1"},
			}),
		})
		resp := gwDoWith(t, dpAddr, "GET", svc.RouteHost.Value, "/", nil)
		assertStatus(t, resp, 403, "Your IP address is not allowed")
	})

	t.Run("route-level allow list", func(t *testing.T) {
		svc := createUpstreamService(t, client, "ip_allow", upstream.URL, nil)
		createGatewayRoute(t, client, uuid.MustParse(svc.ID.Value.String()), &v1.RouteDetail{
			Name:      v1.NewOptName("allow_route"),
			Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
			Methods:   []v1.HTTPMethod{v1.HTTPMethodGET},
			IpRestrictionConfig: v1.NewOptIpRestrictionConfig(v1.IpRestrictionConfig{
				Protocols:    v1.IpRestrictionConfigProtocolsHTTPHTTPS,
				RestrictedBy: v1.IpRestrictionConfigRestrictedByAllowIps,
				Ips:          []string{"127.0.0.1"},
			}),
		})
		resp := gwDoWith(t, dpAddr, "GET", svc.RouteHost.Value, "/", nil)
		assertStatus(t, resp, 200, "")
	})

	t.Run("route-level allow list excluding the client", func(t *testing.T) {
		svc := createUpstreamService(t, client, "ip_allow_other", upstream.URL, nil)
		createGatewayRoute(t, client, uuid.MustParse(svc.ID.Value.String()), &v1.RouteDetail{
			Name:      v1.NewOptName("allow_other_route"),
			Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
			Methods:   []v1.HTTPMethod{v1.HTTPMethodGET},
			IpRestrictionConfig: v1.NewOptIpRestrictionConfig(v1.IpRestrictionConfig{
				Protocols:    v1.IpRestrictionConfigProtocolsHTTPHTTPS,
				RestrictedBy: v1.IpRestrictionConfigRestrictedByAllowIps,
				Ips:          []string{"10.0.0.1"},
			}),
		})
		resp := gwDoWith(t, dpAddr, "GET", svc.RouteHost.Value, "/", nil)
		assertStatus(t, resp, 403, "Your IP address is not allowed")
	})

	t.Run("user-level deny applies after authentication", func(t *testing.T) {
		svc := createUpstreamService(t, client, "ip_user", upstream.URL, func(req *v1.ServiceDetailRequest) {
			req.Authentication = v1.NewOptServiceDetailRequestAuthentication(v1.ServiceDetailRequestAuthenticationBasic)
		})
		createGatewayRoute(t, client, uuid.MustParse(svc.ID.Value.String()), &v1.RouteDetail{
			Name:      v1.NewOptName("user_ip_route"),
			Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
			Methods:   []v1.HTTPMethod{v1.HTTPMethodGET},
		})
		user, err := apigwsdk.NewUserOp(client).Create(t.Context(), &v1.UserDetail{
			Name: "ip_user",
			IpRestrictionConfig: v1.NewOptIpRestrictionConfig(v1.IpRestrictionConfig{
				Protocols:    v1.IpRestrictionConfigProtocolsHTTPHTTPS,
				RestrictedBy: v1.IpRestrictionConfigRestrictedByDenyIps,
				Ips:          []string{"127.0.0.1"},
			}),
		})
		if err != nil {
			t.Fatal(err)
		}
		userID := uuid.MustParse(user.ID.Value.String())
		if err := apigwsdk.NewUserExtraOp(client, userID).UpdateAuth(t.Context(), v1.UserAuthentication{
			BasicAuth: v1.NewOptBasicAuth(v1.BasicAuth{UserName: "carol", Password: "pw"}),
		}); err != nil {
			t.Fatal(err)
		}
		resp := gwDoWith(t, dpAddr, "GET", svc.RouteHost.Value, "/", func(r *http.Request) { r.SetBasicAuth("carol", "pw") })
		assertStatus(t, resp, 403, "Your IP address is not allowed")
	})
}

func TestCredentialUniqueness(t *testing.T) {
	client, _ := newGateway(t)
	createAuthUser(t, client, "uniq_one", v1.UserAuthentication{
		BasicAuth: v1.NewOptBasicAuth(v1.BasicAuth{UserName: "shared", Password: "pw"}),
	})
	user2, err := apigwsdk.NewUserOp(client).Create(t.Context(), &v1.UserDetail{Name: "uniq_two"})
	if err != nil {
		t.Fatal(err)
	}
	err = apigwsdk.NewUserExtraOp(client, uuid.MustParse(user2.ID.Value.String())).UpdateAuth(t.Context(), v1.UserAuthentication{
		BasicAuth: v1.NewOptBasicAuth(v1.BasicAuth{UserName: "shared", Password: "pw2"}),
	})
	if err == nil {
		t.Error("reusing another user's basicAuth userName should fail")
	}
}
