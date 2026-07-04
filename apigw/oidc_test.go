package apigw_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	apigwsdk "github.com/sacloud/sacloud-sdk-go/api/apigw"
	v1 "github.com/sacloud/sacloud-sdk-go/api/apigw/apis/v1"
)

// fakeIdP is an in-process OpenID Connect provider: it serves the discovery
// document and JWKS, and signs RS256 ID tokens with its key. It exercises
// exactly the discovery + JWKS path a real IdP (e.g. Google) would.
type fakeIdP struct {
	srv *httptest.Server
	key *rsa.PrivateKey
	// codes issued by /auth, keyed by the authorization code, for /token.
	codes map[string]fakeIdPCode
}

type fakeIdPCode struct {
	nonce    string
	clientID string
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	idp := &fakeIdP{key: key, codes: make(map[string]fakeIdPCode)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /auth", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		code := uuid.NewString()
		idp.codes[code] = fakeIdPCode{nonce: q.Get("nonce"), clientID: q.Get("client_id")}
		u, err := url.Parse(q.Get("redirect_uri"))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		ru := u.Query()
		ru.Set("code", code)
		ru.Set("state", q.Get("state"))
		u.RawQuery = ru.Encode()
		http.Redirect(w, r, u.String(), http.StatusFound)
	})
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		issued, ok := idp.codes[r.PostFormValue("code")]
		if !ok {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"iss":   idp.srv.URL,
			"sub":   "test-subject",
			"aud":   issued.clientID,
			"nonce": issued.nonce,
			"exp":   time.Now().Add(time.Hour).Unix(),
			"iat":   time.Now().Unix(),
		})
		token.Header["kid"] = "test-key"
		signed, err := token.SignedString(idp.key)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "fake-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     signed,
		})
	})
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                idp.srv.URL,
			"authorization_endpoint":                idp.srv.URL + "/auth",
			"token_endpoint":                        idp.srv.URL + "/token",
			"jwks_uri":                              idp.srv.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("GET /jwks", func(w http.ResponseWriter, r *http.Request) {
		pub := &idp.key.PublicKey
		json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{{
				"kty": "RSA",
				"alg": "RS256",
				"use": "sig",
				"kid": "test-key",
				"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
			}},
		})
	})
	idp.srv = httptest.NewServer(mux)
	t.Cleanup(idp.srv.Close)
	return idp
}

func (idp *fakeIdP) issuer() string { return idp.srv.URL }

// sign issues an RS256 ID token with the given audience and expiry.
func (idp *fakeIdP) sign(t *testing.T, aud string, exp time.Time) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": idp.srv.URL,
		"sub": "test-subject",
		"aud": aud,
		"exp": exp.Unix(),
		"iat": time.Now().Unix(),
	})
	token.Header["kid"] = "test-key"
	signed, err := token.SignedString(idp.key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

// createOidcConfig registers an OIDC configuration and returns its ID.
func createOidcConfig(t *testing.T, client *v1.Client, cfg *v1.Oidc) uuid.UUID {
	t.Helper()
	res, err := client.AddOidc(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	created, ok := res.(*v1.AddOidcCreated)
	if !ok {
		t.Fatalf("add oidc response = %T", res)
	}
	return created.Apigw.Oidc.Value.ID.Value
}

// setupOidcGateway wires a gateway service protected by the fake IdP and
// returns the data plane address and the service.
func setupOidcGateway(t *testing.T, client *v1.Client, name string, upstreamURL string, oidcID uuid.UUID) *v1.ServiceDetailResponse {
	t.Helper()
	svc := createUpstreamService(t, client, name, upstreamURL, func(req *v1.ServiceDetailRequest) {
		req.Authentication = v1.NewOptServiceDetailRequestAuthentication(v1.ServiceDetailRequestAuthenticationOidc)
		req.Oidc = v1.NewOptOidcSummary(v1.OidcSummary{ID: v1.NewOptUUID(oidcID)})
	})
	createGatewayRoute(t, client, uuid.MustParse(svc.ID.Value.String()), &v1.RouteDetail{
		Name:      v1.NewOptName(v1.Name(name + "_route")),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Methods:   []v1.HTTPMethod{v1.HTTPMethodGET},
	})
	return svc
}

func bearerToken(token string) func(*http.Request) {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) }
}

func TestDataPlaneOidcBearer(t *testing.T) {
	client, dpAddr := newGateway(t)
	upstream := newEchoUpstream(t)
	idp := newFakeIdP(t)

	oidcID := createOidcConfig(t, client, &v1.Oidc{
		Name:                  "google_like",
		AuthenticationMethods: v1.AuthenticationMethods{v1.AuthenticationMethodsItemAccessToken},
		Issuer:                idp.issuer(),
		ClientId:              "my-client",
		ClientSecret:          "my-secret",
	})
	svc := setupOidcGateway(t, client, "oidc_svc", upstream.URL, oidcID)
	host := svc.RouteHost.Value
	in1h := time.Now().Add(time.Hour)

	t.Run("missing token", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/", nil)
		assertStatus(t, resp, 401, "Unauthorized")
	})
	t.Run("expired token", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/", bearerToken(idp.sign(t, "my-client", time.Now().Add(-time.Hour))))
		assertStatus(t, resp, 401, "expired")
	})
	t.Run("audience mismatch", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/", bearerToken(idp.sign(t, "someone-else", in1h)))
		assertStatus(t, resp, 401, "audience")
	})
	t.Run("token from another IdP", func(t *testing.T) {
		// Signed by a different key, so signature verification fails against
		// the configured issuer's JWKS.
		other := newFakeIdP(t)
		resp := gwDoWith(t, dpAddr, "GET", host, "/", bearerToken(other.sign(t, "my-client", in1h)))
		assertStatus(t, resp, 401, "Unauthorized")
	})
	t.Run("valid token is proxied with credentials kept", func(t *testing.T) {
		resp := gwDoWith(t, dpAddr, "GET", host, "/", bearerToken(idp.sign(t, "my-client", in1h)))
		assertStatus(t, resp, 200, "")
		// Without hideCredentials, the Authorization header reaches the
		// upstream.
		if e := decodeEcho(t, resp); !strings.HasPrefix(e.Authorization, "Bearer ") {
			t.Errorf("upstream Authorization = %q, want the bearer token", e.Authorization)
		}
	})
	t.Run("config without accessToken method rejects bearer", func(t *testing.T) {
		codeOnlyID := createOidcConfig(t, client, &v1.Oidc{
			Name:                  "code_only",
			AuthenticationMethods: v1.AuthenticationMethods{v1.AuthenticationMethodsItemAuthorizationCodeFlow},
			Issuer:                idp.issuer(),
			ClientId:              "my-client",
			ClientSecret:          "my-secret",
		})
		codeSvc := setupOidcGateway(t, client, "oidc_code_only", upstream.URL, codeOnlyID)
		resp := gwDoWith(t, dpAddr, "GET", codeSvc.RouteHost.Value, "/", bearerToken(idp.sign(t, "my-client", in1h)))
		assertStatus(t, resp, 401, "does not allow the accessToken method")
	})

	t.Run("ACL does not apply to OIDC consumers", func(t *testing.T) {
		// OIDC consumers are external to the user store; an enabled route
		// authorization must not lock them out.
		routes, err := apigwsdk.NewRouteOp(client, uuid.MustParse(svc.ID.Value.String())).List(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		group, err := apigwsdk.NewGroupOp(client).Create(t.Context(), &v1.Group{Name: v1.NewOptName("oidc_acl_group")})
		if err != nil {
			t.Fatal(err)
		}
		if err := apigwsdk.NewRouteExtraOp(client, uuid.MustParse(svc.ID.Value.String()), routes[0].ID.Value).
			EnableAuthorization(t.Context(), []v1.RouteAuthorization{{ID: group.ID, Enabled: v1.NewOptBool(true)}}); err != nil {
			t.Fatal(err)
		}
		resp := gwDoWith(t, dpAddr, "GET", host, "/", bearerToken(idp.sign(t, "my-client", in1h)))
		assertStatus(t, resp, 200, "")
	})
}

func TestDataPlaneOidcTokenAudiences(t *testing.T) {
	client, dpAddr := newGateway(t)
	upstream := newEchoUpstream(t)
	idp := newFakeIdP(t)

	oidcID := createOidcConfig(t, client, &v1.Oidc{
		Name:                  "aud_config",
		AuthenticationMethods: v1.AuthenticationMethods{v1.AuthenticationMethodsItemAccessToken},
		Issuer:                idp.issuer(),
		ClientId:              "my-client",
		ClientSecret:          "my-secret",
		TokenAudiences:        []string{"api://allowed"},
	})
	svc := setupOidcGateway(t, client, "oidc_aud", upstream.URL, oidcID)
	host := svc.RouteHost.Value
	in1h := time.Now().Add(time.Hour)

	// With tokenAudiences configured, the allowed audience passes even though
	// it differs from the client ID...
	resp := gwDoWith(t, dpAddr, "GET", host, "/", bearerToken(idp.sign(t, "api://allowed", in1h)))
	assertStatus(t, resp, 200, "")
	// ...and other audiences (including the client ID) are rejected.
	resp = gwDoWith(t, dpAddr, "GET", host, "/", bearerToken(idp.sign(t, "my-client", in1h)))
	assertStatus(t, resp, 401, "audience is not allowed")
}

func TestDataPlaneOidcHideCredentials(t *testing.T) {
	client, dpAddr := newGateway(t)
	idp := newFakeIdP(t)

	// The echo upstream reports whether Authorization survived the proxy.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"authorization": r.Header.Get("Authorization")})
	}))
	t.Cleanup(upstream.Close)

	hide := true
	oidcID := createOidcConfig(t, client, &v1.Oidc{
		Name:                  "hide_config",
		AuthenticationMethods: v1.AuthenticationMethods{v1.AuthenticationMethodsItemAccessToken},
		Issuer:                idp.issuer(),
		ClientId:              "my-client",
		ClientSecret:          "my-secret",
		HideCredentials:       v1.NewOptBool(hide),
	})
	svc := setupOidcGateway(t, client, "oidc_hide", upstream.URL, oidcID)

	resp := gwDoWith(t, dpAddr, "GET", svc.RouteHost.Value, "/", bearerToken(idp.sign(t, "my-client", time.Now().Add(time.Hour))))
	assertStatus(t, resp, 200, "")
	var got map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["authorization"] != "" {
		t.Errorf("Authorization reached the upstream despite hideCredentials: %q", got["authorization"])
	}
}

// flowDo sends one step of the code flow to the data plane without following
// redirects.
func flowDo(t *testing.T, dpAddr, host, pathAndQuery string, cookies []*http.Cookie) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), "GET", "http://"+dpAddr+pathAndQuery, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = host
	for _, c := range cookies {
		req.AddCookie(c)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestDataPlaneOidcCodeFlow(t *testing.T) {
	client, dpAddr := newGateway(t)
	upstream := newEchoUpstream(t)
	idp := newFakeIdP(t)

	oidcID := createOidcConfig(t, client, &v1.Oidc{
		Name:                  "code_flow",
		AuthenticationMethods: v1.AuthenticationMethods{v1.AuthenticationMethodsItemAuthorizationCodeFlow},
		Issuer:                idp.issuer(),
		ClientId:              "my-client",
		ClientSecret:          "my-secret",
		UseSession:            v1.NewOptBool(true),
	})
	svc := setupOidcGateway(t, client, "oidc_flow", upstream.URL, oidcID)
	// The Host header includes the data plane port so redirect_uri round-trips.
	_, dpPort, err := net.SplitHostPort(dpAddr)
	if err != nil {
		t.Fatal(err)
	}
	host := svc.RouteHost.Value + ":" + dpPort

	// Step 1: the unauthenticated request is redirected to the IdP.
	resp := flowDo(t, dpAddr, host, "/app", nil)
	if resp.StatusCode != 302 {
		t.Fatalf("step1 status = %d, want 302", resp.StatusCode)
	}
	authURL := resp.Header.Get("Location")
	if !strings.HasPrefix(authURL, idp.issuer()+"/auth") {
		t.Fatalf("step1 Location = %q, want the IdP authorization endpoint", authURL)
	}
	if !strings.Contains(authURL, "redirect_uri="+url.QueryEscape("http://"+host+"/app")) {
		t.Errorf("step1 Location misses the original URL as redirect_uri: %q", authURL)
	}

	// Step 2: the IdP authenticates the user and redirects back with a code.
	idpClient := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	idpResp, err := idpClient.Get(authURL)
	if err != nil {
		t.Fatal(err)
	}
	defer idpResp.Body.Close()
	callback, err := url.Parse(idpResp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if callback.Host != host {
		t.Fatalf("callback host = %q, want %q", callback.Host, host)
	}

	// Step 3: the gateway exchanges the code, sets the session cookie, and
	// redirects to the originally requested URL.
	resp = flowDo(t, dpAddr, host, callback.Path+"?"+callback.RawQuery, nil)
	if resp.StatusCode != 302 {
		t.Fatalf("step3 status = %d, want 302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "http://"+host+"/app" {
		t.Errorf("step3 Location = %q, want the original URL", loc)
	}
	var session *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "apigw_session" {
			session = c
		}
	}
	if session == nil {
		t.Fatal("step3 set no session cookie")
	}

	// Step 4: the session cookie authenticates subsequent requests, and the
	// gateway-internal cookie does not leak to the upstream.
	resp = flowDo(t, dpAddr, host, "/app", []*http.Cookie{session})
	if resp.StatusCode != 200 {
		t.Fatalf("step4 status = %d, want 200", resp.StatusCode)
	}
	if e := decodeEcho(t, resp); strings.Contains(e.Cookie, "apigw_session") {
		t.Errorf("session cookie leaked to the upstream: %q", e.Cookie)
	}

	// An unknown state on the callback is rejected.
	resp = flowDo(t, dpAddr, host, "/app?code=whatever&state=forged", nil)
	assertStatus(t, resp, 401, "unknown or expired login state")
}
