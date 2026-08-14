package apigw_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	apigwsdk "github.com/sacloud/sacloud-sdk-go/api/apigw"
	v1 "github.com/sacloud/sacloud-sdk-go/api/apigw/apis/v1"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	"github.com/sacloud/sakumock/apigw"
)

// closeAndCheck closes srv, failing the test if any handler response
// diverged from the OpenAPI spec.
func closeAndCheck(t *testing.T, srv *apigw.Server) {
	t.Helper()
	if v := srv.SpecViolations(); len(v) != 0 {
		t.Errorf("spec violations recorded: %+v", v)
	}
	srv.Close()
}

func newClient(t *testing.T, serverURL string) *v1.Client {
	t.Helper()
	var sa saclient.Client
	if err := sa.SetEnviron([]string{
		"SAKURA_ENDPOINTS_APIGW=" + serverURL,
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
	}); err != nil {
		t.Fatal(err)
	}
	client, err := apigwsdk.NewClient(&sa)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// createSubscription subscribes to the first plan and returns the
// subscription found by name (POST /subscriptions returns 204 with no body,
// so lookup via the list is the real client flow).
func createSubscription(t *testing.T, client *v1.Client, name string) v1.Subscription {
	t.Helper()
	ctx := t.Context()
	subOp := apigwsdk.NewSubscriptionOp(client)
	plans, err := subOp.ListPlans(ctx)
	if err != nil {
		t.Fatalf("list plans: %v", err)
	}
	if len(plans) == 0 {
		t.Fatal("no plans seeded")
	}
	if err := subOp.Create(ctx, plans[0].ID.Value, name); err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	subs, err := subOp.List(ctx)
	if err != nil {
		t.Fatalf("list subscriptions: %v", err)
	}
	for _, sub := range subs {
		if string(sub.Name.Value) == name {
			return sub
		}
	}
	t.Fatalf("subscription %s not found after create", name)
	return v1.Subscription{}
}

func createService(t *testing.T, client *v1.Client, name string, subscriptionID uuid.UUID) *v1.ServiceDetailResponse {
	t.Helper()
	ctx := t.Context()
	svcOp := apigwsdk.NewServiceOp(client)
	created, err := svcOp.Create(ctx, &v1.ServiceDetailRequest{
		Name:     v1.Name(name),
		Protocol: v1.ServiceDetailRequestProtocolHTTP,
		Host:     "upstream.example.com",
		Subscription: v1.ServiceSubscriptionRequest{
			ID: subscriptionID,
		},
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	read, err := svcOp.Read(ctx, created.ID.Value)
	if err != nil {
		t.Fatalf("read service: %v", err)
	}
	return read
}

func TestPlansAndSubscriptionLifecycle(t *testing.T) {
	srv := apigw.NewTestServer(apigw.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	client := newClient(t, srv.TestURL())
	subOp := apigwsdk.NewSubscriptionOp(client)

	plans, err := subOp.ListPlans(ctx)
	if err != nil {
		t.Fatalf("list plans: %v", err)
	}
	if len(plans) != 2 {
		t.Fatalf("plans = %d, want 2", len(plans))
	}
	var enterprise *v1.Plan
	for i := range plans {
		if strings.Contains(string(plans[i].Name.Value), "エンタープライズ") {
			enterprise = &plans[i]
		}
	}
	if enterprise == nil {
		t.Fatal("エンタープライズ plan not seeded")
	}
	if got := enterprise.MaxRequestsUnit.Value; string(got) != "month" {
		t.Errorf("maxRequestsUnit = %q, want month", got)
	}

	sub := createSubscription(t, client, "test_subscription")
	if sub.ResourceId.Value == 0 {
		t.Error("resourceId should be a non-zero numeric ID")
	}

	detail, err := subOp.Read(ctx, sub.ID.Value)
	if err != nil {
		t.Fatalf("read subscription: %v", err)
	}
	if got := detail.Plan.Value.PlanName.Value; got != string(enterprise.Name.Value) && got != "トライアル" {
		t.Errorf("plan name = %q", got)
	}

	if err := subOp.Update(ctx, sub.ID.Value, "renamed_subscription"); err != nil {
		t.Fatalf("update subscription: %v", err)
	}
	if err := subOp.Delete(ctx, sub.ID.Value); err != nil {
		t.Fatalf("delete subscription: %v", err)
	}
	subs, err := subOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 0 {
		t.Errorf("subscriptions after delete = %d, want 0", len(subs))
	}
}

func TestServiceLifecycle(t *testing.T) {
	srv := apigw.NewTestServer(apigw.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	client := newClient(t, srv.TestURL())
	svcOp := apigwsdk.NewServiceOp(client)

	sub := createSubscription(t, client, "svc_sub")
	svc := createService(t, client, "test_service", sub.ID.Value)

	if !strings.HasPrefix(svc.RouteHost.Value, "site-") || !strings.HasSuffix(svc.RouteHost.Value, ".localhost") {
		t.Errorf("routeHost = %q, want site-*.localhost", svc.RouteHost.Value)
	}
	// Spec-declared defaults are filled server-side.
	if svc.Port.Value != 80 {
		t.Errorf("port = %d, want 80", svc.Port.Value)
	}
	if svc.Retries.Value != 5 {
		t.Errorf("retries = %d, want 5", svc.Retries.Value)
	}
	if string(svc.Authentication.Value) != "none" {
		t.Errorf("authentication = %q, want none", svc.Authentication.Value)
	}
	if svc.Subscription.Name != "svc_sub" {
		t.Errorf("subscription.name = %q, want svc_sub", svc.Subscription.Name)
	}

	// The subscription is bound: a second service on it must conflict.
	if _, err := svcOp.Create(ctx, &v1.ServiceDetailRequest{
		Name:         "another_service",
		Protocol:     v1.ServiceDetailRequestProtocolHTTP,
		Host:         "other.example.com",
		Subscription: v1.ServiceSubscriptionRequest{ID: sub.ID.Value},
	}); err == nil {
		t.Error("second service on the same subscription should fail")
	}

	if err := svcOp.Update(ctx, &v1.ServiceDetail{
		Name:     "renamed_service",
		Protocol: v1.ServiceDetailProtocolHTTPS,
		Host:     "upstream.example.com",
	}, svc.ID.Value); err != nil {
		t.Fatalf("update service: %v", err)
	}
	updated, err := svcOp.Read(ctx, svc.ID.Value)
	if err != nil {
		t.Fatal(err)
	}
	if string(updated.Name) != "renamed_service" {
		t.Errorf("name = %q after update", updated.Name)
	}
	if updated.RouteHost.Value != svc.RouteHost.Value {
		t.Errorf("routeHost changed on update: %q -> %q", svc.RouteHost.Value, updated.RouteHost.Value)
	}
	if updated.Port.Value != 443 {
		t.Errorf("port = %d after switching to https, want 443", updated.Port.Value)
	}

	if err := svcOp.Delete(ctx, svc.ID.Value); err != nil {
		t.Fatalf("delete service: %v", err)
	}
	// Deleting unbinds the subscription, so a new service can use it again.
	svc2 := createService(t, client, "test_service2", sub.ID.Value)
	if svc2.Subscription.ID.String() != sub.ID.Value.String() {
		t.Errorf("subscription id = %q", svc2.Subscription.ID)
	}
}

func TestRouteLifecycle(t *testing.T) {
	srv := apigw.NewTestServer(apigw.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	client := newClient(t, srv.TestURL())

	sub := createSubscription(t, client, "route_sub")
	svc := createService(t, client, "route_service", sub.ID.Value)
	serviceID := uuid.MustParse(svc.ID.Value.String())
	routeOp := apigwsdk.NewRouteOp(client, serviceID)

	created, err := routeOp.Create(ctx, &v1.RouteDetail{
		Name:      v1.NewOptName("test_route"),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Path:      v1.NewOptString("/api"),
		Hosts:     []string{svc.RouteHost.Value},
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	if created.Host.Value != svc.RouteHost.Value {
		t.Errorf("route host = %q, want %q", created.Host.Value, svc.RouteHost.Value)
	}
	if !created.StripPath.Value {
		t.Error("stripPath default should be true")
	}
	if created.HttpsRedirectStatusCode.Value != 426 {
		t.Errorf("httpsRedirectStatusCode = %d, want 426", created.HttpsRedirectStatusCode.Value)
	}
	if len(created.Methods) == 0 {
		t.Error("methods should be filled")
	}

	// A hosts entry that is neither routeHost nor a registered domain is a 400.
	if _, err := routeOp.Create(ctx, &v1.RouteDetail{
		Name:      v1.NewOptName("bad_host_route"),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Hosts:     []string{"unregistered.example.com"},
	}); err == nil {
		t.Error("route with unregistered host should fail")
	}

	routeID := uuid.MustParse(created.ID.Value.String())

	// While a route exists, the service refuses deletion.
	if err := apigwsdk.NewServiceOp(client).Delete(ctx, serviceID); err == nil {
		t.Error("service delete with routes should fail")
	}

	// Authorization: initially disabled (empty response body).
	extraOp := apigwsdk.NewRouteExtraOp(client, serviceID, routeID)
	auth, err := extraOp.ReadAuthorization(ctx)
	if err != nil {
		t.Fatalf("read authorization: %v", err)
	}
	if auth.IsACLEnabled {
		t.Error("ACL should be disabled initially")
	}

	group, err := apigwsdk.NewGroupOp(client).Create(ctx, &v1.Group{Name: v1.NewOptName("route_group")})
	if err != nil {
		t.Fatal(err)
	}
	if err := extraOp.EnableAuthorization(ctx, []v1.RouteAuthorization{
		{ID: group.ID, Enabled: v1.NewOptBool(true)},
	}); err != nil {
		t.Fatalf("enable authorization: %v", err)
	}
	auth, err = extraOp.ReadAuthorization(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !auth.IsACLEnabled || len(auth.Groups) != 1 {
		t.Errorf("authorization = %+v, want enabled with 1 group", auth)
	}
	if string(auth.Groups[0].Name.Value) != "route_group" {
		t.Errorf("authorization group name = %q", auth.Groups[0].Name.Value)
	}
	if err := extraOp.DisableAuthorization(ctx); err != nil {
		t.Fatalf("disable authorization: %v", err)
	}

	// Transformations round-trip.
	if err := extraOp.UpdateRequestTransformation(ctx, &v1.RequestTransformation{
		Remove: v1.NewOptRequestRemoveDetail(v1.RequestRemoveDetail{
			HeaderKeys: []v1.RequestHeaderKey{"X-Debug"},
		}),
	}); err != nil {
		t.Fatalf("update request transformation: %v", err)
	}
	reqTr, err := extraOp.ReadRequestTransformation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(reqTr.Remove.Value.HeaderKeys) != 1 {
		t.Errorf("request transformation = %+v", reqTr)
	}

	if err := extraOp.UpdateResponseTransformation(ctx, &v1.ResponseTransformation{
		Remove: v1.NewOptResponseRemoveDetail(v1.ResponseRemoveDetail{
			HeaderKeys: []v1.ResponseHeaderKey{"Server"},
		}),
	}); err != nil {
		t.Fatalf("update response transformation: %v", err)
	}
	respTr, err := extraOp.ReadResponseTransformation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(respTr.Remove.Value.HeaderKeys) != 1 {
		t.Errorf("response transformation = %+v", respTr)
	}

	if err := routeOp.Update(ctx, &v1.RouteDetail{
		Name:      v1.NewOptName("renamed_route"),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTPHTTPS),
		Methods:   []v1.HTTPMethod{v1.HTTPMethodGET, v1.HTTPMethodPOST},
	}, routeID); err != nil {
		t.Fatalf("update route: %v", err)
	}
	got, err := routeOp.Read(ctx, routeID)
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Name.Value) != "renamed_route" {
		t.Errorf("name = %q after update", got.Name.Value)
	}
	// Extras survive a route update.
	reqTr, err = extraOp.ReadRequestTransformation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(reqTr.Remove.Value.HeaderKeys) != 1 {
		t.Error("request transformation lost after route update")
	}

	if err := routeOp.Delete(ctx, routeID); err != nil {
		t.Fatalf("delete route: %v", err)
	}
	routes, err := routeOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Errorf("routes after delete = %d, want 0", len(routes))
	}
}

func TestUserGroupLifecycle(t *testing.T) {
	srv := apigw.NewTestServer(apigw.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	client := newClient(t, srv.TestURL())
	userOp := apigwsdk.NewUserOp(client)
	groupOp := apigwsdk.NewGroupOp(client)

	user, err := userOp.Create(ctx, &v1.UserDetail{
		Name:     "test_user",
		CustomID: v1.NewOptString("custom-1"),
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := userOp.Create(ctx, &v1.UserDetail{Name: "test_user"}); err == nil {
		t.Error("duplicate user name should conflict")
	}

	group, err := groupOp.Create(ctx, &v1.Group{Name: v1.NewOptName("test_group")})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	userID := uuid.MustParse(user.ID.Value.String())
	extraOp := apigwsdk.NewUserExtraOp(client, userID)

	// Assign by name, verify via the assignment list and the user detail.
	if err := extraOp.UpdateGroup(ctx, "test_group", true); err != nil {
		t.Fatalf("assign group: %v", err)
	}
	assignments, err := extraOp.ListGroup(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(assignments) != 1 || !assignments[0].IsAssigned {
		t.Errorf("assignments = %+v, want 1 assigned", assignments)
	}
	detail, err := userOp.Read(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Groups) != 1 || string(detail.Groups[0].Name.Value) != "test_group" {
		t.Errorf("user groups = %+v", detail.Groups)
	}

	if err := extraOp.UpdateAuth(ctx, v1.UserAuthentication{
		BasicAuth: v1.NewOptBasicAuth(v1.BasicAuth{UserName: "alice", Password: "secret"}),
		Jwt: v1.NewOptJwt(v1.Jwt{
			Key:       "issuer-key",
			Secret:    "jwt-secret",
			Algorithm: v1.JwtAlgorithmHS256,
		}),
	}); err != nil {
		t.Fatalf("update auth: %v", err)
	}
	auth, err := extraOp.ReadAuth(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if auth.BasicAuth.Value.UserName != "alice" {
		t.Errorf("basic userName = %q", auth.BasicAuth.Value.UserName)
	}
	if string(auth.Jwt.Value.Algorithm) != "HS256" {
		t.Errorf("jwt algorithm = %q", auth.Jwt.Value.Algorithm)
	}
	if auth.HmacAuth.Set {
		t.Error("hmac should be unset")
	}
	// An empty credential value is a spec-gap 400 (WithNonEmpty overlay).
	if err := extraOp.UpdateAuth(ctx, v1.UserAuthentication{
		BasicAuth: v1.NewOptBasicAuth(v1.BasicAuth{UserName: "alice", Password: ""}),
	}); err == nil {
		t.Error("empty password should be rejected")
	}

	// Group delete cascades out of user membership.
	if err := groupOp.Delete(ctx, uuid.MustParse(group.ID.Value.String())); err != nil {
		t.Fatalf("delete group: %v", err)
	}
	detail, err = userOp.Read(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Groups) != 0 {
		t.Errorf("user groups after group delete = %+v", detail.Groups)
	}

	if err := userOp.Delete(ctx, userID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
}

func testCertPEM(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "example.com"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return string(certPEM), string(keyPEM)
}

func TestDomainCertificateLifecycle(t *testing.T) {
	srv := apigw.NewTestServer(apigw.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	client := newClient(t, srv.TestURL())
	certOp := apigwsdk.NewCertificateOp(client)
	domainOp := apigwsdk.NewDomainOp(client)

	certPEM, keyPEM := testCertPEM(t)
	cert, err := certOp.Create(ctx, &v1.Certificate{
		Name: v1.NewOptName("test_cert"),
		Rsa: v1.NewOptCertificateDetails(v1.CertificateDetails{
			Cert: v1.NewOptString(certPEM),
			Key:  v1.NewOptString(keyPEM),
		}),
	})
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	if cert.Rsa.Value.ExpiredAt.Value.IsZero() {
		t.Error("expiredAt should be derived from the PEM")
	}

	domain, err := domainOp.Create(ctx, &v1.Domain{
		DomainName:    "api.example.com",
		CertificateId: cert.ID,
	})
	if err != nil {
		t.Fatalf("create domain: %v", err)
	}
	if domain.CertificateName.Value != "test_cert" {
		t.Errorf("certificateName = %q", domain.CertificateName.Value)
	}

	// A certificate referenced by a domain refuses deletion.
	if err := certOp.Delete(ctx, cert.ID.Value); err == nil {
		t.Error("certificate delete while referenced should fail")
	}

	// Routes can use the domain in hosts; the domain then refuses deletion.
	sub := createSubscription(t, client, "domain_sub")
	svc := createService(t, client, "domain_service", sub.ID.Value)
	serviceID := uuid.MustParse(svc.ID.Value.String())
	routeOp := apigwsdk.NewRouteOp(client, serviceID)
	route, err := routeOp.Create(ctx, &v1.RouteDetail{
		Name:      v1.NewOptName("domain_route"),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTPHTTPS),
		Hosts:     []string{"api.example.com"},
	})
	if err != nil {
		t.Fatalf("create route with domain host: %v", err)
	}
	domainID := uuid.MustParse(domain.ID.Value.String())
	if err := domainOp.Delete(ctx, domainID); err == nil {
		t.Error("domain delete while referenced by a route should fail")
	}

	if err := routeOp.Delete(ctx, uuid.MustParse(route.ID.Value.String())); err != nil {
		t.Fatal(err)
	}
	if err := domainOp.Update(ctx, &v1.DomainPUT{}, domainID); err != nil {
		t.Fatalf("update domain (clear certificate): %v", err)
	}
	if err := certOp.Delete(ctx, cert.ID.Value); err != nil {
		t.Fatalf("delete certificate after unlink: %v", err)
	}
	if err := domainOp.Delete(ctx, domainID); err != nil {
		t.Fatalf("delete domain: %v", err)
	}
}

func TestOidcLifecycle(t *testing.T) {
	srv := apigw.NewTestServer(apigw.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	client := newClient(t, srv.TestURL())

	created, err := client.AddOidc(ctx, &v1.Oidc{
		Name:                  "test_oidc",
		AuthenticationMethods: v1.AuthenticationMethods{v1.AuthenticationMethodsItemAccessToken},
		Issuer:                "https://accounts.google.com",
		ClientId:              "client-id",
		ClientSecret:          "client-secret",
		Scopes:                []string{"openid"},
	})
	if err != nil {
		t.Fatalf("add oidc: %v", err)
	}
	oidc, ok := created.(*v1.AddOidcCreated)
	if !ok {
		t.Fatalf("add oidc response = %T", created)
	}
	oidcID := oidc.Apigw.Oidc.Value.ID.Value

	// Empty issuer is a spec-gap 400 (WithNonEmpty overlay).
	res, err := client.AddOidc(ctx, &v1.Oidc{
		Name:                  "bad_oidc",
		AuthenticationMethods: v1.AuthenticationMethods{v1.AuthenticationMethodsItemAccessToken},
		ClientId:              "x",
		ClientSecret:          "y",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := res.(*v1.AddOidcBadRequest); !ok {
		t.Errorf("empty issuer response = %T, want AddOidcBadRequest", res)
	}

	// Attach to a service, then deleting the OIDC config must conflict.
	sub := createSubscription(t, client, "oidc_sub")
	svcOp := apigwsdk.NewServiceOp(client)
	svc, err := svcOp.Create(ctx, &v1.ServiceDetailRequest{
		Name:           "oidc_service",
		Protocol:       v1.ServiceDetailRequestProtocolHTTPS,
		Host:           "upstream.example.com",
		Authentication: v1.NewOptServiceDetailRequestAuthentication(v1.ServiceDetailRequestAuthenticationOidc),
		Oidc:           v1.NewOptOidcSummary(v1.OidcSummary{ID: v1.NewOptUUID(oidcID)}),
		Subscription:   v1.ServiceSubscriptionRequest{ID: sub.ID.Value},
	})
	if err != nil {
		t.Fatalf("create service with oidc: %v", err)
	}

	got, err := client.GetOidcById(ctx, v1.GetOidcByIdParams{OidcId: oidcID})
	if err != nil {
		t.Fatal(err)
	}
	detail, ok := got.(*v1.GetOidcByIdOK)
	if !ok {
		t.Fatalf("get oidc response = %T", got)
	}
	services := detail.Apigw.Oidc.Value.Services
	if len(services) != 1 || string(services[0].Name.Value) != "oidc_service" {
		t.Errorf("oidc services = %+v", services)
	}

	delRes, err := client.DeleteOidc(ctx, v1.DeleteOidcParams{OidcId: oidcID})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := delRes.(*v1.DeleteOidcConflict); !ok {
		t.Errorf("delete oidc while referenced = %T, want DeleteOidcConflict", delRes)
	}

	if err := svcOp.Delete(ctx, svc.ID.Value); err != nil {
		t.Fatal(err)
	}
	delRes, err = client.DeleteOidc(ctx, v1.DeleteOidcParams{OidcId: oidcID})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := delRes.(*v1.DeleteOidcNoContent); !ok {
		t.Errorf("delete oidc = %T, want DeleteOidcNoContent", delRes)
	}
}

// TestServiceValidation exercises the spec-derived validator paths that the
// SDK client cannot produce (it always sends well-formed typed values).
func TestServiceValidation(t *testing.T) {
	srv := apigw.NewTestServer(apigw.Config{})
	defer closeAndCheck(t, srv)

	post := func(body string) int {
		t.Helper()
		resp, err := http.Post(srv.TestURL()+"/services", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}

	// subscription is required by the generated schema.
	if got := post(`{"name":"v","protocol":"http","host":"upstream.example.com"}`); got != 400 {
		t.Errorf("missing subscription: status = %d, want 400", got)
	}
	// subscription.id is required by the generated schema.
	if got := post(`{"name":"v","protocol":"http","host":"upstream.example.com","subscription":{}}`); got != 400 {
		t.Errorf("missing subscription.id: status = %d, want 400", got)
	}
	// An empty subscription.id is a spec gap covered by the WithNonEmpty overlay.
	if got := post(`{"name":"v","protocol":"http","host":"upstream.example.com","subscription":{"id":""}}`); got != 400 {
		t.Errorf("empty subscription.id: status = %d, want 400", got)
	}
}

// TestCertificateValidation verifies the mock returns debuggable errors for
// broken PEM input instead of a vague 400.
func TestCertificateValidation(t *testing.T) {
	srv := apigw.NewTestServer(apigw.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	client := newClient(t, srv.TestURL())
	certOp := apigwsdk.NewCertificateOp(client)

	certPEM, _ := testCertPEM(t)
	_, otherKeyPEM := testCertPEM(t)

	// Values that fail the spec pattern (e.g. a non-PEM string) are rejected
	// by the ogen SDK client itself, so only pattern-passing garbage reaches
	// the mock.
	tests := []struct {
		name, cert, key, wantErr string
	}{
		{"broken base64", "-----BEGIN CERTIFICATE-----\nA\n-----END CERTIFICATE-----\n", "", "no decodable PEM block found"},
		{"not an x509 certificate", "-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n", "", "invalid certificate: x509"},
		{"key mismatch", certPEM, otherKeyPEM, "private key does not match"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := certOp.Create(ctx, &v1.Certificate{
				Name: v1.NewOptName("bad_cert"),
				Rsa: v1.NewOptCertificateDetails(v1.CertificateDetails{
					Cert: v1.NewOptString(tt.cert),
					Key:  v1.NewOptString(tt.key),
				}),
			})
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}
