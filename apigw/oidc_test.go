package apigw_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
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
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	idp := &fakeIdP{key: key}
	mux := http.NewServeMux()
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
