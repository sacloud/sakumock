package iam_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/sacloud/sacloud-sdk-go/api/iam/apis/user"
	"github.com/sacloud/sacloud-sdk-go/api/iam/apis/user2fa"
	v1 "github.com/sacloud/sacloud-sdk-go/api/iam/apis/v1"

	"github.com/sacloud/sakumock/iam"
)

func seed(t *testing.T, baseURL, path string, body any, out any) {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.Post(baseURL+path, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("POST %s: status %d", path, res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		t.Fatal(err)
	}
}

func createTestUser(t *testing.T, client *v1.Client, code string) *v1.User {
	t.Helper()
	email := code + "@example.com"
	created, err := user.NewUserOp(client).Create(t.Context(), user.CreateParams{
		Name:        code,
		Password:    "Password12345!",
		Code:        code,
		Description: "2fa test user",
		Email:       &email,
	})
	if err != nil {
		t.Fatal(err)
	}
	return created
}

func TestUserTrustedDevices(t *testing.T) {
	srv := iam.NewTestServer(iam.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	u := createTestUser(t, client, "device-user")
	op := user2fa.NewUser2FAOp(client, u)
	base := srv.TestURL() + "/_sakumock/users/" + strconv.Itoa(u.GetID())

	// No device is trusted until one is seeded.
	list, err := op.ListTrustedDevices(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 || list.Count != 0 {
		t.Fatalf("expected no trusted devices, got %+v", list)
	}

	var first, second struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	seed(t, base, "/trusted-devices", map[string]string{"name": "laptop"}, &first)

	// Every field is optional, and so is the body itself.
	res, err := http.Post(base+"/trusted-devices", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("seeding without a body: status %d", res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(&second); err != nil {
		t.Fatal(err)
	}
	if second.Name != "mock-device" {
		t.Errorf("default name = %q, want mock-device", second.Name)
	}

	// Seeding for a user that does not exist is a 404.
	missing, err := http.Post(srv.TestURL()+"/_sakumock/users/999/trusted-devices", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	missing.Body.Close()
	if missing.StatusCode != http.StatusNotFound {
		t.Errorf("seeding for an unknown user: status %d, want 404", missing.StatusCode)
	}

	list, err = op.ListTrustedDevices(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 2 || list.Count != 2 {
		t.Fatalf("expected 2 trusted devices, got %+v", list)
	}
	if list.Items[0].ID != first.ID || list.Items[0].Name != "laptop" {
		t.Errorf("unexpected first device: %+v", list.Items[0])
	}

	// Delete one, then clear the rest.
	if err := op.DeleteTrustedDevice(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	list, err = op.ListTrustedDevices(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].ID != second.ID {
		t.Fatalf("unexpected devices after delete: %+v", list.Items)
	}
	if err := op.DeleteTrustedDevice(ctx, first.ID); err == nil {
		t.Error("expected an error deleting an unknown trusted device")
	}

	if err := op.ClearTrustedDevices(ctx); err != nil {
		t.Fatal(err)
	}
	list, err = op.ListTrustedDevices(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("expected no trusted devices after clear, got %+v", list.Items)
	}
}

func TestUserSecurityKeys(t *testing.T) {
	srv := iam.NewTestServer(iam.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	userOp := user.NewUserOp(client)
	u := createTestUser(t, client, "key-user")
	op := user2fa.NewUser2FAOp(client, u)
	base := srv.TestURL() + "/_sakumock/users/" + strconv.Itoa(u.GetID())

	list, err := op.ListSecurityKeys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("expected no security keys, got %+v", list.Items)
	}
	if u.IsSecurityKeyRegistered {
		t.Error("is_security_key_registered = true before any key is registered")
	}

	var key struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		AAGUID string `json:"aaguid"`
	}
	seed(t, base, "/security-keys", map[string]any{"name": "yubikey", "sign_count": 3}, &key)
	if key.AAGUID == "" {
		t.Error("expected a generated aaguid")
	}

	// The user now reports a registered security key.
	read, err := userOp.Read(ctx, u.GetID())
	if err != nil {
		t.Fatal(err)
	}
	if !read.IsSecurityKeyRegistered {
		t.Error("is_security_key_registered = false after registering a key")
	}

	got, err := op.ReadSecurityKey(ctx, key.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "yubikey" || got.SignCount != 3 {
		t.Errorf("unexpected security key: %+v", got)
	}
	if _, ok := got.LastUsedAt.Get(); ok {
		t.Errorf("last_used_at = %v, want null", got.LastUsedAt)
	}

	updated, err := op.UpdateSecurityKey(ctx, key.ID, "renamed")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "renamed" {
		t.Errorf("name = %q, want renamed", updated.Name)
	}

	if err := op.DeleteSecurityKey(ctx, key.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := op.ReadSecurityKey(ctx, key.ID); err == nil {
		t.Error("expected an error reading a deleted security key")
	}
}

// Every 2FA endpoint is scoped to a user, and 2FA state does not outlive it.
func TestUser2FAScopedToUser(t *testing.T) {
	srv := iam.NewTestServer(iam.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	userOp := user.NewUserOp(client)

	owner := createTestUser(t, client, "owner-user")
	other := createTestUser(t, client, "other-user")
	ownerOp := user2fa.NewUser2FAOp(client, owner)
	otherOp := user2fa.NewUser2FAOp(client, other)
	base := srv.TestURL() + "/_sakumock/users/" + strconv.Itoa(owner.GetID())

	var device struct {
		ID int `json:"id"`
	}
	var key struct {
		ID int `json:"id"`
	}
	seed(t, base, "/trusted-devices", map[string]string{"name": "laptop"}, &device)
	seed(t, base, "/security-keys", map[string]string{"name": "yubikey"}, &key)

	// Another user cannot see or touch them.
	otherDevices, err := otherOp.ListTrustedDevices(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherDevices.Items) != 0 {
		t.Errorf("another user sees %d trusted devices", len(otherDevices.Items))
	}
	if err := otherOp.DeleteTrustedDevice(ctx, device.ID); err == nil {
		t.Error("expected an error deleting another user's trusted device")
	}
	if _, err := otherOp.ReadSecurityKey(ctx, key.ID); err == nil {
		t.Error("expected an error reading another user's security key")
	}

	// OTP deactivation is accepted for an existing user.
	if err := ownerOp.DeactivateOTP(ctx); err != nil {
		t.Fatal(err)
	}

	// Deleting the user drops their 2FA state.
	if err := userOp.Delete(ctx, owner.GetID()); err != nil {
		t.Fatal(err)
	}
	if _, err := ownerOp.ListTrustedDevices(ctx); err == nil {
		t.Error("expected an error listing trusted devices of a deleted user")
	}
	if err := ownerOp.DeactivateOTP(ctx); err == nil {
		t.Error("expected an error deactivating OTP of a deleted user")
	}
}
