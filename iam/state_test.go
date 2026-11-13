package iam_test

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sacloud/sacloud-sdk-go/api/iam/apis/auth"
	"github.com/sacloud/sacloud-sdk-go/api/iam/apis/folder"
	"github.com/sacloud/sacloud-sdk-go/api/iam/apis/group"
	"github.com/sacloud/sacloud-sdk-go/api/iam/apis/iampolicy"
	"github.com/sacloud/sacloud-sdk-go/api/iam/apis/iamrole"
	"github.com/sacloud/sacloud-sdk-go/api/iam/apis/idpolicy"
	"github.com/sacloud/sacloud-sdk-go/api/iam/apis/idrole"
	"github.com/sacloud/sacloud-sdk-go/api/iam/apis/organization"
	"github.com/sacloud/sacloud-sdk-go/api/iam/apis/project"
	"github.com/sacloud/sacloud-sdk-go/api/iam/apis/projectapikey"
	"github.com/sacloud/sacloud-sdk-go/api/iam/apis/scim"
	"github.com/sacloud/sacloud-sdk-go/api/iam/apis/servicepolicy"
	"github.com/sacloud/sacloud-sdk-go/api/iam/apis/serviceprincipal"
	"github.com/sacloud/sacloud-sdk-go/api/iam/apis/sso"
	"github.com/sacloud/sacloud-sdk-go/api/iam/apis/user"
	"github.com/sacloud/sacloud-sdk-go/api/iam/apis/user2fa"
	v1 "github.com/sacloud/sacloud-sdk-go/api/iam/apis/v1"

	"github.com/sacloud/sakumock/core/coretest"
	"github.com/sacloud/sakumock/iam"
)

func iamBinding(role string, principalID int) []v1.IamPolicy {
	return []v1.IamPolicy{{
		Role: v1.NewOptIamPolicyRole(v1.IamPolicyRole{
			Type: v1.NewOptIamPolicyRoleType("preset"),
			ID:   v1.NewOptString(role),
		}),
		Principals: []v1.Principal{{Type: v1.NewOptString("user"), ID: v1.NewOptInt(principalID)}},
	}}
}

// checkIAMBinding fails unless bindings is the single binding iamBinding made.
func checkIAMBinding(t *testing.T, what string, bindings []v1.IamPolicy, role string, principalID int) {
	t.Helper()
	if len(bindings) != 1 || bindings[0].Role.Value.ID.Value != role ||
		len(bindings[0].Principals) != 1 || bindings[0].Principals[0].ID.Value != principalID {
		t.Errorf("restored %s = %+v, want role %s for principal %d", what, bindings, role, principalID)
	}
}

func TestStateSurvivesRestart(t *testing.T) {
	const authConditions = `{"ip_restriction":{"enabled":false,"subnets":[]},"require_two_factor_auth":{"enabled":true},"datetime_restriction":{"after":null,"before":null}}`
	var (
		userID, deletedUserID, groupID, folderID, projectID, spID, apiKeyID, ssoID int
		deviceID, securityKeyID                                                    int
		spKeyID                                                                    uuid.UUID
		scimID                                                                     string
		apiKeyToken                                                                string
	)

	coretest.StateRestart(t, iam.Config{},
		func(t *testing.T, url string) {
			ctx := t.Context()
			client := newTestClient(t, url)

			u := createTestUser(t, client, "persisted")
			userID = u.ID
			// A deleted row must stay deleted, and must not disturb the order
			// of the rows around it.
			deletedUserID = createTestUser(t, client, "deleted").ID
			createTestUser(t, client, "second")
			if err := user.NewUserOp(client).Delete(ctx, deletedUserID); err != nil {
				t.Fatal(err)
			}

			var seeded struct {
				ID int `json:"id"`
			}
			base := url + "/_sakumock/users/" + strconv.Itoa(userID)
			seed(t, base, "/trusted-devices", map[string]string{"name": "laptop"}, &seeded)
			deviceID = seeded.ID
			seed(t, base, "/security-keys", map[string]string{"name": "yubikey"}, &seeded)
			securityKeyID = seeded.ID

			groupOp := group.NewGroupOp(client)
			g, err := groupOp.Create(ctx, "persisted-group", "desc")
			if err != nil {
				t.Fatal(err)
			}
			groupID = g.ID
			if _, err := groupOp.UpdateMemberships(ctx, groupID, []int{userID}); err != nil {
				t.Fatal(err)
			}

			f, err := folder.NewFolderOp(client).Create(ctx, folder.CreateParams{Name: "persisted-folder"})
			if err != nil {
				t.Fatal(err)
			}
			folderID = f.ID
			p, err := project.NewProjectOp(client).Create(ctx, project.CreateParams{
				Code: "persisted", Name: "persisted-project", Description: "desc", ParentFolderID: &folderID,
			})
			if err != nil {
				t.Fatal(err)
			}
			projectID = p.ID

			spOp := serviceprincipal.NewServicePrincipalOp(client)
			sp, err := spOp.Create(ctx, v1.CreateServicePrincipalReq{ProjectID: projectID, Name: "persisted-sp", Description: "desc"})
			if err != nil {
				t.Fatal(err)
			}
			spID = sp.ID
			key, err := spOp.UploadKey(ctx, spID, "ssh-rsa AAAAB3NzaC1yc2EAAA...")
			if err != nil {
				t.Fatal(err)
			}
			spKeyID = key.ID
			if _, err := spOp.DisableKey(ctx, spID, spKeyID); err != nil {
				t.Fatal(err)
			}

			apiKey, err := projectapikey.NewProjectAPIKeyOp(client).Create(ctx, projectapikey.CreateParams{
				ProjectID: projectID, Name: "persisted-key", Description: "desc", IamRoles: []string{"owner"},
			})
			if err != nil {
				t.Fatal(err)
			}
			apiKeyID, apiKeyToken = apiKey.ID, apiKey.AccessToken

			ssoOp := sso.NewSSOOp(client)
			profile, err := ssoOp.Create(ctx, v1.CreateSsoProfileReq{
				Name:           "persisted-sso",
				Description:    "desc",
				IdpEntityID:    "https://idp.example.com/entity",
				IdpLoginURL:    "https://idp.example.com/login",
				IdpLogoutURL:   "https://idp.example.com/logout",
				IdpCertificate: "MIICpDCCAYwCCQD...",
			})
			if err != nil {
				t.Fatal(err)
			}
			ssoID = profile.ID
			if _, err := ssoOp.Link(ctx, ssoID); err != nil {
				t.Fatal(err)
			}

			sc, err := scim.NewScimOp(client).Create(ctx, scim.CreateParams{Name: "persisted-scim"})
			if err != nil {
				t.Fatal(err)
			}
			scimID = sc.ID.String()

			policyOp := iampolicy.NewIAMPolicyOp(client)
			if _, err := policyOp.UpdateOrganizationPolicy(ctx, iamBinding("organization-admin", userID)); err != nil {
				t.Fatal(err)
			}
			if _, err := policyOp.UpdateProjectPolicy(ctx, projectID, iamBinding("owner", userID)); err != nil {
				t.Fatal(err)
			}
			if _, err := policyOp.UpdateFolderPolicy(ctx, folderID, iamBinding("viewer", userID)); err != nil {
				t.Fatal(err)
			}
			if _, err := idpolicy.NewIDPolicyOp(client).UpdateOrganizationIdPolicy(ctx, []v1.IdPolicy{{
				Role: v1.NewOptIdPolicyRole(v1.IdPolicyRole{
					Type: v1.NewOptIdPolicyRoleType("preset"),
					ID:   v1.NewOptString("admin"),
				}),
				Principals: []v1.Principal{{Type: v1.NewOptString("user"), ID: v1.NewOptInt(userID)}},
			}}); err != nil {
				t.Fatal(err)
			}

			if _, err := organization.NewOrganizationOp(client).Update(ctx, "persisted-org"); err != nil {
				t.Fatal(err)
			}
			authOp := auth.NewAuthOp(client)
			if _, err := authOp.UpdatePasswordPolicy(ctx, v1.PasswordPolicy{MinLength: 16, RequireUppercase: true, RequireSymbols: true}); err != nil {
				t.Fatal(err)
			}
			// The SDK cannot decode the auth conditions, so they go over raw HTTP.
			req, err := http.NewRequestWithContext(ctx, http.MethodPut, url+"/organization-auth-conditions", strings.NewReader(authConditions))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if res.StatusCode != http.StatusOK {
				t.Fatalf("PUT /organization-auth-conditions: status %d", res.StatusCode)
			}

			spolOp := servicepolicy.NewServicePolicyOp(client)
			if err := spolOp.Enable(ctx); err != nil {
				t.Fatal(err)
			}
			if _, err := client.UpdateOrganizationServicePolicy(ctx, &v1.UpdateOrganizationServicePolicyReq{
				Rules: []v1.Rule{{Code: "example.rule.bool", IsActive: true}},
			}); err != nil {
				t.Fatal(err)
			}
		},
		func(t *testing.T, url string) {
			ctx := t.Context()
			client := newTestClient(t, url)

			userOp := user.NewUserOp(client)
			users, err := userOp.List(ctx, user.ListParams{})
			if err != nil {
				t.Fatal(err)
			}
			if len(users.Items) != 2 || users.Items[0].ID != userID || users.Items[0].Name != "persisted" || users.Items[1].Name != "second" {
				t.Errorf("restored users = %+v, want persisted then second", users.Items)
			}
			u, err := userOp.Read(ctx, userID)
			if err != nil {
				t.Fatal(err)
			}
			if u.Email != "persisted@example.com" || !u.IsSecurityKeyRegistered {
				t.Errorf("restored user = %+v", u)
			}
			twoFA := user2fa.NewUser2FAOp(client, u)
			devices, err := twoFA.ListTrustedDevices(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(devices.Items) != 1 || devices.Items[0].ID != deviceID || devices.Items[0].Name != "laptop" {
				t.Errorf("restored trusted devices = %+v", devices.Items)
			}
			securityKey, err := twoFA.ReadSecurityKey(ctx, securityKeyID)
			if err != nil {
				t.Fatal(err)
			}
			if securityKey.Name != "yubikey" {
				t.Errorf("restored security key = %+v", securityKey)
			}

			groupOp := group.NewGroupOp(client)
			g, err := groupOp.Read(ctx, groupID)
			if err != nil {
				t.Fatal(err)
			}
			if g.Name != "persisted-group" {
				t.Errorf("restored group = %+v", g)
			}
			members, err := groupOp.ReadMemberships(ctx, groupID)
			if err != nil {
				t.Fatal(err)
			}
			if len(members) != 1 || members[0].ID != userID {
				t.Errorf("restored memberships = %+v, want user %d", members, userID)
			}

			f, err := folder.NewFolderOp(client).Read(ctx, folderID)
			if err != nil {
				t.Fatal(err)
			}
			if f.Name != "persisted-folder" {
				t.Errorf("restored folder = %+v", f)
			}
			p, err := project.NewProjectOp(client).Read(ctx, projectID)
			if err != nil {
				t.Fatal(err)
			}
			if p.Code != "persisted" || p.ParentFolderID.Value != folderID {
				t.Errorf("restored project = %+v", p)
			}

			spOp := serviceprincipal.NewServicePrincipalOp(client)
			sp, err := spOp.Read(ctx, spID)
			if err != nil {
				t.Fatal(err)
			}
			if sp.Name != "persisted-sp" || sp.ProjectID != projectID {
				t.Errorf("restored service principal = %+v", sp)
			}
			keys, err := spOp.ListKeys(ctx, spID, serviceprincipal.ListKeysParams{})
			if err != nil {
				t.Fatal(err)
			}
			if len(keys.Items) != 1 || keys.Items[0].ID != spKeyID || keys.Items[0].Status != "disabled" {
				t.Errorf("restored service principal keys = %+v, want %s disabled", keys.Items, spKeyID)
			}

			apiKey, err := projectapikey.NewProjectAPIKeyOp(client).Read(ctx, apiKeyID)
			if err != nil {
				t.Fatal(err)
			}
			if apiKey.Name != "persisted-key" || apiKey.AccessToken != apiKeyToken || len(apiKey.IamRoles) != 1 {
				t.Errorf("restored API key = %+v", apiKey)
			}

			profile, err := sso.NewSSOOp(client).Read(ctx, ssoID)
			if err != nil {
				t.Fatal(err)
			}
			if profile.Name != "persisted-sso" || !profile.Assigned {
				t.Errorf("restored SSO profile = %+v", profile)
			}
			sc, err := scim.NewScimOp(client).Read(ctx, scimID)
			if err != nil {
				t.Fatal(err)
			}
			if sc.Name != "persisted-scim" {
				t.Errorf("restored SCIM configuration = %+v", sc)
			}

			policyOp := iampolicy.NewIAMPolicyOp(client)
			orgPolicy, err := policyOp.ReadOrganizationPolicy(ctx)
			if err != nil {
				t.Fatal(err)
			}
			checkIAMBinding(t, "organization IAM policy", orgPolicy, "organization-admin", userID)
			projectPolicy, err := policyOp.ReadProjectPolicy(ctx, projectID)
			if err != nil {
				t.Fatal(err)
			}
			checkIAMBinding(t, "project IAM policy", projectPolicy, "owner", userID)
			folderPolicy, err := policyOp.ReadFolderPolicy(ctx, folderID)
			if err != nil {
				t.Fatal(err)
			}
			checkIAMBinding(t, "folder IAM policy", folderPolicy, "viewer", userID)
			idPolicy, err := idpolicy.NewIDPolicyOp(client).ReadOrganizationIdPolicy(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(idPolicy) != 1 || idPolicy[0].Role.Value.ID.Value != "admin" || len(idPolicy[0].Principals) != 1 {
				t.Errorf("restored organization ID policy = %+v", idPolicy)
			}

			org, err := organization.NewOrganizationOp(client).Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if org.Name != "persisted-org" {
				t.Errorf("restored organization = %+v", org)
			}
			authOp := auth.NewAuthOp(client)
			pp, err := authOp.ReadPasswordPolicy(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if pp.MinLength != 16 || !pp.RequireUppercase || pp.RequireLowercase || !pp.RequireSymbols {
				t.Errorf("restored password policy = %+v", pp)
			}
			// The document is served as it was stored, not as the state file
			// indents it.
			res, err := http.Get(url + "/organization-auth-conditions")
			if err != nil {
				t.Fatal(err)
			}
			conditions, err := io.ReadAll(res.Body)
			res.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			if string(conditions) != authConditions {
				t.Errorf("restored auth conditions = %s, want %s", conditions, authConditions)
			}

			enabled, err := servicepolicy.NewServicePolicyOp(client).IsEnabled(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if !enabled {
				t.Error("restored service policy is disabled, want enabled")
			}
			policyRes, err := client.ReadOrganizationServicePolicy(ctx, v1.ReadOrganizationServicePolicyParams{})
			if err != nil {
				t.Fatal(err)
			}
			rules, ok := policyRes.(*v1.ReadOrganizationServicePolicyOK)
			if !ok || len(rules.Rules) != 1 || rules.Rules[0].Code.Value != "example.rule.bool" {
				t.Errorf("restored service policy rules = %#v", policyRes)
			}

			// The built-in roles are seeded once, not once per start.
			iamRoles, err := iamrole.NewIAMRoleOp(client).List(ctx, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			idRoles, err := idrole.NewIdRoleOp(client).List(ctx, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(iamRoles.Items) != 5 || len(idRoles.Items) != 2 {
				t.Errorf("built-in roles after restart: %d IAM, %d ID, want 5 and 2", len(iamRoles.Items), len(idRoles.Items))
			}
		})
}

// A resource created after a restart never reuses a restored ID, whichever
// table the restored one lives in.
func TestStateRestoredIDsAreNotReused(t *testing.T) {
	restored := map[int]string{}
	coretest.StateRestart(t, iam.Config{},
		func(t *testing.T, url string) {
			ctx := t.Context()
			client := newTestClient(t, url)
			u := createTestUser(t, client, "first")
			restored[u.ID] = "user"
			g, err := group.NewGroupOp(client).Create(ctx, "g", "desc")
			if err != nil {
				t.Fatal(err)
			}
			restored[g.ID] = "group"
			// The last ID handed out before the restart belongs to a trusted
			// device, which only the mock's own endpoint can create.
			var device struct {
				ID int `json:"id"`
			}
			seed(t, url+"/_sakumock/users/"+strconv.Itoa(u.ID), "/trusted-devices", map[string]string{"name": "laptop"}, &device)
			restored[device.ID] = "trusted device"
		},
		func(t *testing.T, url string) {
			client := newTestClient(t, url)
			p, err := project.NewProjectOp(client).Create(t.Context(), project.CreateParams{Code: "new", Name: "new", Description: "desc"})
			if err != nil {
				t.Fatal(err)
			}
			if kind, ok := restored[p.ID]; ok {
				t.Errorf("new project reused the ID %d of a restored %s", p.ID, kind)
			}
		}, true)
}

// A state file missing what a fresh store always has is rejected at startup
// instead of breaking requests later.
func TestStateRejectsIncompleteFile(t *testing.T) {
	for name, state := range map[string]string{
		"no organization":    `{"auth_conditions":{}}`,
		"no auth conditions": `{"organization":{"id":1,"name":"x"}}`,
		"null record":        `{"organization":{"id":1,"name":"x"},"auth_conditions":{},"users":[null]}`,
		"duplicate id":       `{"organization":{"id":1,"name":"x"},"auth_conditions":{},"groups":[{"id":7},{"id":7}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			store := iam.NewMemoryStore(nil)
			if err := store.Restore([]byte(state)); err == nil {
				t.Error("Restore accepted the state, want an error")
			}
		})
	}
}
