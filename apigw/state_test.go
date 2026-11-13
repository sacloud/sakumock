package apigw_test

import (
	"net"
	"net/http"
	"testing"

	"github.com/google/uuid"
	apigwsdk "github.com/sacloud/sacloud-sdk-go/api/apigw"
	v1 "github.com/sacloud/sacloud-sdk-go/api/apigw/apis/v1"

	"github.com/sacloud/sakumock/apigw"
	"github.com/sacloud/sakumock/core/coretest"
)

// freeAddr returns a loopback address nothing listens on, for a data plane
// that has to come back on the same address after a restart.
func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

func TestStateSurvivesRestart(t *testing.T) {
	upstream := newEchoUpstream(t)
	dpAddr := freeAddr(t)
	const domainName = "api.example.com"
	var (
		subID, idleSubID, oidcSubID uuid.UUID
		serviceID, oidcServiceID    uuid.UUID
		routeID, userID, oidcID     uuid.UUID
		routeHost                   string
		resourceID                  int64
	)
	member := func(r *http.Request) { r.SetBasicAuth("alice", "open sesame") }
	outsider := func(r *http.Request) { r.SetBasicAuth("bob", "pw") }

	coretest.StateRestart(t, apigw.Config{EnableDataPlane: true, DataPlaneAddr: dpAddr},
		func(t *testing.T, url string) {
			ctx := t.Context()
			client := newClient(t, url)

			certPEM, keyPEM := testCertPEM(t)
			cert, err := apigwsdk.NewCertificateOp(client).Create(ctx, &v1.Certificate{
				Name: v1.NewOptName("state_cert"),
				Rsa: v1.NewOptCertificateDetails(v1.CertificateDetails{
					Cert: v1.NewOptString(certPEM),
					Key:  v1.NewOptString(keyPEM),
				}),
			})
			if err != nil {
				t.Fatalf("create certificate: %v", err)
			}
			if _, err := apigwsdk.NewDomainOp(client).Create(ctx, &v1.Domain{
				DomainName:    domainName,
				CertificateId: cert.ID,
			}); err != nil {
				t.Fatalf("create domain: %v", err)
			}

			group, err := apigwsdk.NewGroupOp(client).Create(ctx, &v1.Group{
				Name: v1.NewOptName("state_group"),
				Tags: []string{"g"},
			})
			if err != nil {
				t.Fatalf("create group: %v", err)
			}
			userID = createAuthUser(t, client, "state_member", v1.UserAuthentication{
				BasicAuth: v1.NewOptBasicAuth(v1.BasicAuth{UserName: "alice", Password: "open sesame"}),
				Jwt:       v1.NewOptJwt(v1.Jwt{Key: "issuer-key", Secret: "jwt-secret", Algorithm: v1.JwtAlgorithmHS256}),
				HmacAuth:  v1.NewOptHmacAuth(v1.HmacAuth{UserName: "alice-hmac", Secret: "hmac-secret"}),
			})
			if err := apigwsdk.NewUserExtraOp(client, userID).UpdateGroup(ctx, "state_group", true); err != nil {
				t.Fatalf("assign group: %v", err)
			}
			createAuthUser(t, client, "state_outsider", v1.UserAuthentication{
				BasicAuth: v1.NewOptBasicAuth(v1.BasicAuth{UserName: "bob", Password: "pw"}),
			})

			svc := createUpstreamService(t, client, "state_service", upstream.URL, func(req *v1.ServiceDetailRequest) {
				req.Authentication = v1.NewOptServiceDetailRequestAuthentication(v1.ServiceDetailRequestAuthenticationBasic)
				req.CorsConfig = v1.NewOptCorsConfig(v1.CorsConfig{
					AccessControlAllowOrigins: v1.NewOptString("http://app.example.com"),
					AccessControlAllowMethods: []v1.HTTPMethod{v1.HTTPMethodGET},
					MaxAge:                    v1.NewOptInt32(3600),
				})
			})
			serviceID = svc.ID.Value
			routeHost = svc.RouteHost.Value
			route := createGatewayRoute(t, client, serviceID, &v1.RouteDetail{
				Name:      v1.NewOptName("state_route"),
				Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTPS),
				Path:      v1.NewOptString("/api"),
				Hosts:     []string{routeHost, domainName},
				Methods:   []v1.HTTPMethod{v1.HTTPMethodGET},
			})
			routeID = route.ID.Value
			extraOp := apigwsdk.NewRouteExtraOp(client, serviceID, routeID)
			if err := extraOp.EnableAuthorization(ctx, []v1.RouteAuthorization{
				{ID: group.ID, Enabled: v1.NewOptBool(true)},
			}); err != nil {
				t.Fatalf("enable authorization: %v", err)
			}
			if err := extraOp.UpdateRequestTransformation(ctx, &v1.RequestTransformation{
				Remove: v1.NewOptRequestRemoveDetail(v1.RequestRemoveDetail{
					HeaderKeys: []v1.RequestHeaderKey{"X-Debug"},
				}),
			}); err != nil {
				t.Fatalf("update request transformation: %v", err)
			}
			if err := extraOp.UpdateResponseTransformation(ctx, &v1.ResponseTransformation{
				Remove: v1.NewOptResponseRemoveDetail(v1.ResponseRemoveDetail{
					HeaderKeys: []v1.ResponseHeaderKey{"Server"},
				}),
			}); err != nil {
				t.Fatalf("update response transformation: %v", err)
			}

			oidcID = createOidcConfig(t, client, &v1.Oidc{
				Name:                  "state_oidc",
				AuthenticationMethods: v1.AuthenticationMethods{v1.AuthenticationMethodsItemAccessToken},
				Issuer:                "https://idp.example.com",
				ClientId:              "client-id",
				ClientSecret:          "client-secret",
				Scopes:                []string{"openid"},
			})
			oidcSvc := createUpstreamService(t, client, "state_oidc_service", upstream.URL, func(req *v1.ServiceDetailRequest) {
				req.Authentication = v1.NewOptServiceDetailRequestAuthentication(v1.ServiceDetailRequestAuthenticationOidc)
				req.Oidc = v1.NewOptOidcSummary(v1.OidcSummary{ID: v1.NewOptUUID(oidcID)})
			})
			oidcServiceID = oidcSvc.ID.Value

			idle := createSubscription(t, client, "state_idle_sub")
			idleSubID = idle.ID.Value
			resourceID = idle.ResourceId.Value
			subs, err := apigwsdk.NewSubscriptionOp(client).List(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for _, sub := range subs {
				switch string(sub.Name.Value) {
				case "state_service_sub":
					subID = sub.ID.Value
				case "state_oidc_service_sub":
					oidcSubID = sub.ID.Value
				}
			}
		},
		func(t *testing.T, url string) {
			ctx := t.Context()
			client := newClient(t, url)

			// The data plane serves the restored configuration: the route and its
			// service, the credentials, the group membership and the allow-list.
			assertStatus(t, gwDoWith(t, dpAddr, "GET", routeHost, "/api/x", member), 200, "")
			assertStatus(t, gwDoWith(t, dpAddr, "GET", domainName, "/api/x", member), 200, "")
			assertStatus(t, gwDoWith(t, dpAddr, "GET", routeHost, "/api/x", outsider), 403, "")
			assertStatus(t, gwDoWith(t, dpAddr, "GET", routeHost, "/api/x", nil), 401, "")

			subOp := apigwsdk.NewSubscriptionOp(client)
			subs, err := subOp.List(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(subs) != 3 {
				t.Errorf("got %d subscriptions after restart, want 3", len(subs))
			}
			sub, err := subOp.Read(ctx, subID)
			if err != nil {
				t.Fatalf("read subscription: %v", err)
			}
			if sub.Service.Value.ID != serviceID || sub.Service.Value.Name != "state_service" {
				t.Errorf("restored subscription service = %+v", sub.Service)
			}
			if sub.Plan.Value.PlanName.Value == "" {
				t.Errorf("restored subscription lost its plan: %+v", sub.Plan)
			}
			idle, err := subOp.Read(ctx, idleSubID)
			if err != nil {
				t.Fatalf("read idle subscription: %v", err)
			}
			if idle.Service.Set || idle.ResourceId.Value != resourceID {
				t.Errorf("restored idle subscription = %+v, want no service and resourceId %d", idle, resourceID)
			}
			// The binding is restored on the subscription side too.
			if err := subOp.Delete(ctx, oidcSubID); err == nil {
				t.Error("deleting a subscription bound to a restored service should fail")
			}

			svc, err := apigwsdk.NewServiceOp(client).Read(ctx, serviceID)
			if err != nil {
				t.Fatalf("read service: %v", err)
			}
			if string(svc.Name) != "state_service" || svc.RouteHost.Value != routeHost ||
				string(svc.Authentication.Value) != "basic" || svc.Subscription.ID != subID ||
				svc.CorsConfig.Value.MaxAge.Value != 3600 {
				t.Errorf("restored service = %+v", svc)
			}

			route, err := apigwsdk.NewRouteOp(client, serviceID).Read(ctx, routeID)
			if err != nil {
				t.Fatalf("read route: %v", err)
			}
			if string(route.Name.Value) != "state_route" || route.Path.Value != "/api" || len(route.Hosts) != 2 {
				t.Errorf("restored route = %+v", route)
			}
			extraOp := apigwsdk.NewRouteExtraOp(client, serviceID, routeID)
			auth, err := extraOp.ReadAuthorization(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if !auth.IsACLEnabled || len(auth.Groups) != 1 || string(auth.Groups[0].Name.Value) != "state_group" {
				t.Errorf("restored authorization = %+v", auth)
			}
			reqTr, err := extraOp.ReadRequestTransformation(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(reqTr.Remove.Value.HeaderKeys) != 1 {
				t.Errorf("restored request transformation = %+v", reqTr)
			}
			respTr, err := extraOp.ReadResponseTransformation(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(respTr.Remove.Value.HeaderKeys) != 1 {
				t.Errorf("restored response transformation = %+v", respTr)
			}

			user, err := apigwsdk.NewUserOp(client).Read(ctx, userID)
			if err != nil {
				t.Fatalf("read user: %v", err)
			}
			if len(user.Groups) != 1 || string(user.Groups[0].Name.Value) != "state_group" {
				t.Errorf("restored user groups = %+v", user.Groups)
			}
			creds, err := apigwsdk.NewUserExtraOp(client, userID).ReadAuth(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if creds.BasicAuth.Value.Password != "open sesame" || creds.Jwt.Value.Secret != "jwt-secret" ||
				creds.HmacAuth.Value.Secret != "hmac-secret" {
				t.Errorf("restored credentials = %+v", creds)
			}

			groups, err := apigwsdk.NewGroupOp(client).List(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(groups) != 1 || len(groups[0].Tags) != 1 {
				t.Errorf("restored groups = %+v", groups)
			}
			certs, err := apigwsdk.NewCertificateOp(client).List(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(certs) != 1 || certs[0].Rsa.Value.ExpiredAt.Value.IsZero() {
				t.Errorf("restored certificates = %+v", certs)
			}
			domains, err := apigwsdk.NewDomainOp(client).List(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(domains) != 1 || domains[0].DomainName != domainName ||
				domains[0].CertificateId != certs[0].ID || domains[0].CertificateName.Value != "state_cert" {
				t.Errorf("restored domains = %+v", domains)
			}

			res, err := client.GetOidcById(ctx, v1.GetOidcByIdParams{OidcId: oidcID})
			if err != nil {
				t.Fatal(err)
			}
			detail, ok := res.(*v1.GetOidcByIdOK)
			if !ok {
				t.Fatalf("get oidc response = %T", res)
			}
			oidc := detail.Apigw.Oidc.Value
			if oidc.Issuer != "https://idp.example.com" || oidc.ClientSecret != "client-secret" ||
				len(oidc.Services) != 1 || oidc.Services[0].ID.Value != oidcServiceID {
				t.Errorf("restored oidc = %+v", oidc)
			}
		})
}

// A subscription created after a restart never reuses a restored resource ID.
func TestStateRestoredIDsAreNotReused(t *testing.T) {
	var resourceID int64
	coretest.StateRestart(t, apigw.Config{},
		func(t *testing.T, url string) {
			resourceID = createSubscription(t, newClient(t, url), "first").ResourceId.Value
		},
		func(t *testing.T, url string) {
			if id := createSubscription(t, newClient(t, url), "second").ResourceId.Value; id <= resourceID {
				t.Errorf("new subscription got resource ID %d, want one after the restored %d", id, resourceID)
			}
		}, true)
}
