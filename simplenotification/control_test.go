package simplenotification_test

import (
	"testing"

	sdk "github.com/sacloud/sacloud-sdk-go/api/simple-notification"
	v1 "github.com/sacloud/sacloud-sdk-go/api/simple-notification/apis/v1"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	"github.com/sacloud/sakumock/simplenotification"
)

func newControlPlaneClient(t *testing.T, serverURL string) *v1.Client {
	t.Helper()
	var sa saclient.Client
	if err := sa.SetEnviron([]string{
		"SAKURA_ENDPOINTS_SIMPLE_NOTIFICATION=" + serverURL,
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
	}); err != nil {
		t.Fatal(err)
	}
	client, err := sdk.NewClient(&sa)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestDestinationAndGroupLifecycle(t *testing.T) {
	srv := simplenotification.NewTestServer(simplenotification.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newControlPlaneClient(t, srv.TestURL())

	destOp := sdk.NewDestinationOp(client)
	groupOp := sdk.NewGroupOp(client)

	// Create a destination.
	dest, err := destOp.Create(ctx, v1.PostCommonServiceItemRequest{
		CommonServiceItem: v1.PostCommonServiceItemRequestCommonServiceItem{
			Name:        "mail-dest",
			Description: "a destination",
			Icon:        v1.NilCommonServiceItemIcon{Null: true},
			Settings: v1.CommonServiceItemSettings{
				DestinationSettings: v1.DestinationSettings{
					Type:  v1.DestinationSettingsType("email"),
					Value: "ops@example.com",
				},
			},
			Tags: []string{},
		},
	})
	if err != nil {
		t.Fatalf("create destination: %v", err)
	}
	destID := dest.CommonServiceItem.ID
	if destID == "" {
		t.Fatal("expected a non-empty destination ID")
	}

	// Read it back.
	gotDest, err := destOp.Read(ctx, destID)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if gotDest.CommonServiceItem.Name != "mail-dest" {
		t.Errorf("destination name = %q, want mail-dest", gotDest.CommonServiceItem.Name)
	}

	// Create a group referencing the destination.
	group, err := groupOp.Create(ctx, v1.PostCommonServiceItemRequest{
		CommonServiceItem: v1.PostCommonServiceItemRequestCommonServiceItem{
			Name:        "alert-group",
			Description: "a group",
			Icon:        v1.NilCommonServiceItemIcon{Null: true},
			Settings: v1.CommonServiceItemSettings{
				GroupSettings: v1.GroupSettings{
					Destinations: []string{destID},
				},
			},
			Tags: []string{},
		},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	groupID := group.CommonServiceItem.ID
	if groupID == "" {
		t.Fatal("expected a non-empty group ID")
	}

	// The group list is filtered to groups only (not the destination).
	groups, err := groupOp.List(ctx)
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	foundGroup, sawDestination := false, false
	for _, it := range groups.CommonServiceItems {
		if it.ID == groupID {
			foundGroup = true
		}
		if it.ID == destID {
			sawDestination = true
		}
	}
	if !foundGroup {
		t.Errorf("created group %s not found in group list", groupID)
	}
	if sawDestination {
		t.Errorf("destination %s leaked into the group list", destID)
	}

	// Delete both.
	if err := groupOp.Delete(ctx, groupID); err != nil {
		t.Fatalf("delete group: %v", err)
	}
	if err := destOp.Delete(ctx, destID); err != nil {
		t.Fatalf("delete destination: %v", err)
	}
	if _, err := destOp.Read(ctx, destID); err == nil {
		t.Error("expected read after delete to fail")
	}
}
