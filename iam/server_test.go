package iam_test

// TODO: The following routes are not covered by SDK-based tests because the SDK
// does not yet expose operations for them. Add tests when SDK support arrives.
//
//   POST   /compat/users/{user_id}/deactivate-otp
//   GET    /compat/users/{user_id}/trusted-devices
//   DELETE /compat/users/{user_id}/trusted-devices/{trusted_device_id}
//   POST   /compat/users/{user_id}/clear-trusted-devices
//   GET    /compat/users/{user_id}/security-keys
//   PUT    /compat/users/{user_id}/security-keys/{security_key_id}
//   DELETE /compat/users/{user_id}/security-keys/{security_key_id}
//   POST   /move-projects
//   POST   /move-folders
//   GET    /folders/{folder_id}/iam-policy
//   PUT    /folders/{folder_id}/iam-policy
//   GET    /organization-iam-policy
//   PUT    /organization-iam-policy
//   GET    /organization-auth-conditions
//   PUT    /organization-auth-conditions
//   PUT    /sso-profiles/{sso_profile_id}
//   POST   /service-principals/oauth2/token
//   POST   /enable-service-policy
//   POST   /disable-service-policy
//   GET    /service-policy-status
//   GET    /organization-service-policy
//   PUT    /organization-service-policy
//   GET    /service-policy-rule-templates

import (
	"testing"

	iamsdk "github.com/sacloud/sacloud-sdk-go/api/iam"
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
	"github.com/sacloud/sacloud-sdk-go/api/iam/apis/serviceprincipal"
	"github.com/sacloud/sacloud-sdk-go/api/iam/apis/sso"
	"github.com/sacloud/sacloud-sdk-go/api/iam/apis/user"
	v1 "github.com/sacloud/sacloud-sdk-go/api/iam/apis/v1"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	"github.com/sacloud/sakumock/iam"
)

func newTestClient(t *testing.T, serverURL string) *v1.Client {
	t.Helper()
	var sa saclient.Client
	if err := sa.SetEnviron([]string{
		"SAKURA_ENDPOINTS_IAM=" + serverURL,
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
	}); err != nil {
		t.Fatal(err)
	}
	client, err := iamsdk.NewClient(&sa)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestUserLifecycle(t *testing.T) {
	srv := iam.NewTestServer(iam.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	userOp := user.NewUserOp(client)

	// List: initially empty
	list, err := userOp.List(ctx, user.ListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("expected 0 users, got %d", len(list.Items))
	}

	// Create
	email := "test@example.com"
	created, err := userOp.Create(ctx, user.CreateParams{
		Name:        "test-user",
		Password:    "Password12345!",
		Code:        "test-code",
		Description: "test description",
		Email:       &email,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "test-user" {
		t.Fatalf("unexpected name: %s", created.Name)
	}
	userID := created.ID

	// List: 1
	list, err = userOp.List(ctx, user.ListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 user, got %d", len(list.Items))
	}

	// Read
	read, err := userOp.Read(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Name != "test-user" || read.Email != "test@example.com" {
		t.Fatalf("unexpected read: %+v", read)
	}

	// Update
	updated, err := userOp.Update(ctx, userID, user.UpdateParams{
		Name:        "updated-user",
		Description: "updated desc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "updated-user" {
		t.Fatalf("unexpected update: %+v", updated)
	}

	// Register email
	if err := userOp.RegisterEmail(ctx, userID, "new@example.com"); err != nil {
		t.Fatal(err)
	}
	read, err = userOp.Read(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Email != "new@example.com" {
		t.Fatalf("expected new email, got: %s", read.Email)
	}

	// Unregister email
	if err := userOp.UnregisterEmail(ctx, userID); err != nil {
		t.Fatal(err)
	}
	read, err = userOp.Read(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Email != "" {
		t.Fatalf("expected empty email, got: %s", read.Email)
	}

	// Delete
	if err := userOp.Delete(ctx, userID); err != nil {
		t.Fatal(err)
	}

	// List: empty
	list, err = userOp.List(ctx, user.ListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("expected 0 users after delete, got %d", len(list.Items))
	}
}

func TestGroupLifecycle(t *testing.T) {
	srv := iam.NewTestServer(iam.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	groupOp := group.NewGroupOp(client)
	userOp := user.NewUserOp(client)

	// Create group
	created, err := groupOp.Create(ctx, "test-group", "test description")
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "test-group" {
		t.Fatalf("unexpected name: %s", created.Name)
	}
	groupID := created.ID

	// Read
	read, err := groupOp.Read(ctx, groupID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Name != "test-group" {
		t.Fatalf("unexpected read: %+v", read)
	}

	// Update
	updated, err := groupOp.Update(ctx, groupID, "updated-group", "updated desc")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "updated-group" {
		t.Fatalf("unexpected update: %+v", updated)
	}

	// Read memberships (empty)
	members, err := groupOp.ReadMemberships(ctx, groupID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 0 {
		t.Fatalf("expected 0 members, got %d", len(members))
	}

	// Create user then add to group
	u, err := userOp.Create(ctx, user.CreateParams{
		Name: "member-user", Password: "Password12345!", Code: "mem",
	})
	if err != nil {
		t.Fatal(err)
	}
	newMembers, err := groupOp.UpdateMemberships(ctx, groupID, []int{u.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(newMembers) != 1 {
		t.Fatalf("expected 1 member, got %d", len(newMembers))
	}

	// Delete
	if err := groupOp.Delete(ctx, groupID); err != nil {
		t.Fatal(err)
	}
}

func TestProjectLifecycle(t *testing.T) {
	srv := iam.NewTestServer(iam.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	projectOp := project.NewProjectOp(client)

	// Create
	created, err := projectOp.Create(ctx, project.CreateParams{
		Code:        "test-code",
		Name:        "test-project",
		Description: "test description",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "test-project" || created.Code != "test-code" {
		t.Fatalf("unexpected: %+v", created)
	}
	projectID := created.ID

	// Read
	read, err := projectOp.Read(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Name != "test-project" {
		t.Fatalf("unexpected read: %+v", read)
	}

	// Update
	updated, err := projectOp.Update(ctx, projectID, "updated-project", "updated desc")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "updated-project" {
		t.Fatalf("unexpected update: %+v", updated)
	}

	// Delete
	if err := projectOp.Delete(ctx, projectID); err != nil {
		t.Fatal(err)
	}
}

func TestFolderLifecycle(t *testing.T) {
	srv := iam.NewTestServer(iam.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	folderOp := folder.NewFolderOp(client)

	// Create
	created, err := folderOp.Create(ctx, folder.CreateParams{
		Name: "test-folder",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "test-folder" {
		t.Fatalf("unexpected: %+v", created)
	}
	folderID := created.ID

	// Read
	read, err := folderOp.Read(ctx, folderID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Name != "test-folder" {
		t.Fatalf("unexpected read: %+v", read)
	}

	// Update
	updated, err := folderOp.Update(ctx, folderID, "updated-folder", nil)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "updated-folder" {
		t.Fatalf("unexpected update: %+v", updated)
	}

	// Delete
	if err := folderOp.Delete(ctx, folderID); err != nil {
		t.Fatal(err)
	}
}

func TestServicePrincipalLifecycle(t *testing.T) {
	srv := iam.NewTestServer(iam.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	spOp := serviceprincipal.NewServicePrincipalOp(client)
	projectOp := project.NewProjectOp(client)

	// Create project first
	proj, err := projectOp.Create(ctx, project.CreateParams{
		Code: "sp-project", Name: "sp-project", Description: "for SP",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create service principal
	created, err := spOp.Create(ctx, v1.ServicePrincipalsPostReq{
		ProjectID:   proj.ID,
		Name:        "test-sp",
		Description: "test description",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "test-sp" {
		t.Fatalf("unexpected: %+v", created)
	}
	spID := created.ID

	// Read
	read, err := spOp.Read(ctx, spID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Name != "test-sp" {
		t.Fatalf("unexpected read: %+v", read)
	}

	// Update
	updated, err := spOp.Update(ctx, spID, serviceprincipal.UpdateParams{
		Name:        "updated-sp",
		Description: v1.NewOptString("updated desc"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "updated-sp" {
		t.Fatalf("unexpected update: %+v", updated)
	}

	// Upload key
	key, err := spOp.UploadKey(ctx, spID, "ssh-rsa AAAAB3NzaC1yc2EAAA...")
	if err != nil {
		t.Fatal(err)
	}
	if key.Status != "enabled" {
		t.Fatalf("expected enabled, got %s", key.Status)
	}

	// List keys
	keys, err := spOp.ListKeys(ctx, spID, serviceprincipal.ListKeysParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(keys.Items) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys.Items))
	}

	// Disable key
	disabled, err := spOp.DisableKey(ctx, spID, key.ID)
	if err != nil {
		t.Fatal(err)
	}
	if disabled.Status != "disabled" {
		t.Fatalf("expected disabled, got %s", disabled.Status)
	}

	// Enable key
	enabled, err := spOp.EnableKey(ctx, spID, key.ID)
	if err != nil {
		t.Fatal(err)
	}
	if enabled.Status != "enabled" {
		t.Fatalf("expected enabled, got %s", enabled.Status)
	}

	// Delete key
	if err := spOp.DeleteKey(ctx, spID, key.ID); err != nil {
		t.Fatal(err)
	}

	// Delete service principal
	if err := spOp.Delete(ctx, spID); err != nil {
		t.Fatal(err)
	}
}

func TestProjectAPIKeyLifecycle(t *testing.T) {
	srv := iam.NewTestServer(iam.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	apiKeyOp := projectapikey.NewProjectAPIKeyOp(client)
	projectOp := project.NewProjectOp(client)

	// Create project
	proj, err := projectOp.Create(ctx, project.CreateParams{
		Code: "apikey-project", Name: "apikey-project", Description: "for API key",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create API key
	created, err := apiKeyOp.Create(ctx, projectapikey.CreateParams{
		ProjectID:   proj.ID,
		Name:        "test-key",
		Description: "test description",
		IamRoles:    []string{"owner"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "test-key" {
		t.Fatalf("unexpected: %+v", created)
	}
	if created.AccessToken == "" {
		t.Fatal("expected non-empty access token")
	}
	if created.AccessTokenSecret == "" {
		t.Fatal("expected non-empty access token secret")
	}
	keyID := created.ID

	// Read
	read, err := apiKeyOp.Read(ctx, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Name != "test-key" {
		t.Fatalf("unexpected read: %+v", read)
	}

	// Update
	updated, err := apiKeyOp.Update(ctx, keyID, projectapikey.UpdateParams{
		Name:        "updated-key",
		Description: "updated desc",
		IamRoles:    []string{"editor"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "updated-key" {
		t.Fatalf("unexpected update: %+v", updated)
	}

	// Delete
	if err := apiKeyOp.Delete(ctx, keyID); err != nil {
		t.Fatal(err)
	}
}

func TestIAMRoleListAndRead(t *testing.T) {
	srv := iam.NewTestServer(iam.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	roleOp := iamrole.NewIAMRoleOp(client)

	// List (pre-seeded)
	list, err := roleOp.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) < 3 {
		t.Fatalf("expected at least 3 IAM roles, got %d", len(list.Items))
	}

	// Read by ID
	read, err := roleOp.Read(ctx, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if read.ID != "owner" {
		t.Fatalf("unexpected: %+v", read)
	}
}

func TestIDRoleListAndRead(t *testing.T) {
	srv := iam.NewTestServer(iam.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	roleOp := idrole.NewIdRoleOp(client)

	// List (pre-seeded)
	list, err := roleOp.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) < 2 {
		t.Fatalf("expected at least 2 ID roles, got %d", len(list.Items))
	}

	// Read by ID
	read, err := roleOp.Read(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if read.ID != "admin" {
		t.Fatalf("unexpected: %+v", read)
	}
}

func TestIAMPolicyProject(t *testing.T) {
	srv := iam.NewTestServer(iam.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	policyOp := iampolicy.NewIAMPolicyOp(client)
	projectOp := project.NewProjectOp(client)

	// Create project
	proj, err := projectOp.Create(ctx, project.CreateParams{
		Code: "policy-project", Name: "policy-project", Description: "for policy",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Read (empty)
	bindings, err := policyOp.ReadProjectPolicy(ctx, proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 0 {
		t.Fatalf("expected 0 bindings, got %d", len(bindings))
	}

	// Update
	newBindings := []v1.IamPolicy{
		{
			Role: v1.NewOptIamPolicyRole(v1.IamPolicyRole{
				Type: v1.NewOptIamPolicyRoleType("preset"),
				ID:   v1.NewOptString("owner"),
			}),
			Principals: []v1.Principal{
				{
					Type: v1.NewOptString("service-principal"),
					ID:   v1.NewOptInt(1),
				},
			},
		},
	}
	updated, err := policyOp.UpdateProjectPolicy(ctx, proj.ID, newBindings)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(updated))
	}

	// Read again
	bindings, err = policyOp.ReadProjectPolicy(ctx, proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(bindings))
	}
}

func TestIDPolicyOrganization(t *testing.T) {
	srv := iam.NewTestServer(iam.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	policyOp := idpolicy.NewIDPolicyOp(client)

	// Read (empty)
	bindings, err := policyOp.ReadOrganizationIdPolicy(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 0 {
		t.Fatalf("expected 0 bindings, got %d", len(bindings))
	}

	// Update
	newBindings := []v1.IdPolicy{
		{
			Role: v1.NewOptIdPolicyRole(v1.IdPolicyRole{
				Type: v1.NewOptIdPolicyRoleType("preset"),
				ID:   v1.NewOptString("admin"),
			}),
			Principals: []v1.Principal{
				{
					Type: v1.NewOptString("user"),
					ID:   v1.NewOptInt(1),
				},
			},
		},
	}
	updated, err := policyOp.UpdateOrganizationIdPolicy(ctx, newBindings)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(updated))
	}
}

func TestOrganizationReadUpdate(t *testing.T) {
	srv := iam.NewTestServer(iam.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	orgOp := organization.NewOrganizationOp(client)

	// Read (pre-seeded)
	read, err := orgOp.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if read.Name != "mock-organization" {
		t.Fatalf("unexpected: %+v", read)
	}

	// Update
	updated, err := orgOp.Update(ctx, "new-org-name")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "new-org-name" {
		t.Fatalf("unexpected update: %+v", updated)
	}
}

func TestPasswordPolicy(t *testing.T) {
	srv := iam.NewTestServer(iam.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	authOp := auth.NewAuthOp(client)

	// Read
	pp, err := authOp.ReadPasswordPolicy(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if pp.MinLength != 12 {
		t.Fatalf("expected min_length 12, got %d", pp.MinLength)
	}

	// Update
	updated, err := authOp.UpdatePasswordPolicy(ctx, v1.PasswordPolicy{
		MinLength:        12,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireSymbols:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.MinLength != 12 || !updated.RequireUppercase {
		t.Fatalf("unexpected update: %+v", updated)
	}
}

func TestAuthContext(t *testing.T) {
	srv := iam.NewTestServer(iam.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	authOp := auth.NewAuthOp(client)

	ac, err := authOp.ReadAuthContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ac.AuthType != "apikey" {
		t.Fatalf("unexpected auth_type: %s", ac.AuthType)
	}
}

func TestSSOProfileLifecycle(t *testing.T) {
	srv := iam.NewTestServer(iam.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	ssoOp := sso.NewSSOOp(client)

	// Create
	created, err := ssoOp.Create(ctx, v1.SSOProfilesPostReq{
		Name:           "test-sso",
		Description:    "test SSO profile",
		IdpEntityID:    "https://idp.example.com/entity",
		IdpLoginURL:    "https://idp.example.com/login",
		IdpLogoutURL:   "https://idp.example.com/logout",
		IdpCertificate: "MIICpDCCAYwCCQD...",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "test-sso" {
		t.Fatalf("unexpected: %+v", created)
	}
	ssoID := created.ID

	// Read
	read, err := ssoOp.Read(ctx, ssoID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Name != "test-sso" {
		t.Fatalf("unexpected read: %+v", read)
	}

	// Link
	linked, err := ssoOp.Link(ctx, ssoID)
	if err != nil {
		t.Fatal(err)
	}
	if !linked.Assigned {
		t.Fatal("expected assigned=true after link")
	}

	// Unlink
	unlinked, err := ssoOp.Unlink(ctx, ssoID)
	if err != nil {
		t.Fatal(err)
	}
	if unlinked.Assigned {
		t.Fatal("expected assigned=false after unlink")
	}

	// Delete
	if err := ssoOp.Delete(ctx, ssoID); err != nil {
		t.Fatal(err)
	}
}

func TestScimLifecycle(t *testing.T) {
	srv := iam.NewTestServer(iam.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	scimOp := scim.NewScimOp(client)

	// Create
	created, err := scimOp.Create(ctx, scim.CreateParams{Name: "test-scim"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "test-scim" {
		t.Fatalf("unexpected: %+v", created)
	}
	scimID := created.ID.String()

	// Read
	read, err := scimOp.Read(ctx, scimID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Name != "test-scim" {
		t.Fatalf("unexpected read: %+v", read)
	}

	// Update
	updated, err := scimOp.Update(ctx, scimID, scim.UpdateParams{Name: "updated-scim"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "updated-scim" {
		t.Fatalf("unexpected update: %+v", updated)
	}

	// Regenerate token
	tokenResp, err := scimOp.RegenerateToken(ctx, scimID)
	if err != nil {
		t.Fatal(err)
	}
	if !tokenResp.SecretToken.Set {
		t.Fatal("expected non-empty secret token")
	}

	// Delete
	if err := scimOp.Delete(ctx, scimID); err != nil {
		t.Fatal(err)
	}
}
