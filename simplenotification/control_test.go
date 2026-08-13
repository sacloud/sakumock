package simplenotification_test

import (
	"testing"

	sdk "github.com/sacloud/sacloud-sdk-go/api/simple-notification"
	v1 "github.com/sacloud/sacloud-sdk-go/api/simple-notification/apis/v1"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	"github.com/sacloud/sakumock/simplenotification"
)

// closeAndCheck closes srv, failing the test if any handler response
// diverged from the OpenAPI spec.
func closeAndCheck(t *testing.T, srv *simplenotification.Server) {
	t.Helper()
	if v := srv.SpecViolations(); len(v) != 0 {
		t.Errorf("spec violations recorded: %+v", v)
	}
	srv.Close()
}

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
	defer closeAndCheck(t, srv)
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

func TestItemStatusSourcesAndHistory(t *testing.T) {
	srv := simplenotification.NewTestServer(simplenotification.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	client := newControlPlaneClient(t, srv.TestURL())

	destOp := sdk.NewDestinationOp(client)
	groupOp := sdk.NewGroupOp(client)
	routingOp := sdk.NewRoutingOp(client)
	historyOp := sdk.NewHistoryOp(client)

	// Sources: the mock's static catalog.
	sources, err := routingOp.ListSource(ctx)
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	if len(sources.Sources) == 0 || sources.Sources[0].ID == "" || sources.Sources[0].Name == "" {
		t.Fatalf("unexpected sources: %+v", sources.Sources)
	}

	// Destination status: valid while enabled.
	dest, err := destOp.Create(ctx, v1.PostCommonServiceItemRequest{
		CommonServiceItem: v1.PostCommonServiceItemRequestCommonServiceItem{
			Name: "status-dest",
			Icon: v1.NilCommonServiceItemIcon{Null: true},
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
	status, err := destOp.ReadStatus(ctx, destID)
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if !status.NotificationStatus.IsValid {
		t.Errorf("expected destination to be valid: %+v", status.NotificationStatus)
	}

	// History: a sent message appears with a per-destination status.
	group, err := groupOp.Create(ctx, v1.PostCommonServiceItemRequest{
		CommonServiceItem: v1.PostCommonServiceItemRequestCommonServiceItem{
			Name: "history-group",
			Icon: v1.NilCommonServiceItemIcon{Null: true},
			Settings: v1.CommonServiceItemSettings{
				GroupSettings: v1.GroupSettings{Destinations: []string{destID}},
			},
			Tags: []string{},
		},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	groupID := group.CommonServiceItem.ID
	if _, err := groupOp.SendMessage(ctx, groupID, v1.SendNotificationMessageRequest{Message: "hello history"}); err != nil {
		t.Fatalf("send message: %v", err)
	}

	histories, err := historyOp.List(ctx)
	if err != nil {
		t.Fatalf("list histories: %v", err)
	}
	if len(histories.NotificationHistories) != 1 {
		t.Fatalf("expected 1 history, got %d", len(histories.NotificationHistories))
	}
	h := histories.NotificationHistories[0]
	if h.Message.Body != "hello history" {
		t.Errorf("unexpected history message body: %q", h.Message.Body)
	}
	if len(h.Statuses) != 1 || h.Statuses[0].GroupID != groupID || h.Statuses[0].DestinationID != destID {
		t.Errorf("unexpected history statuses: %+v", h.Statuses)
	}

	got, err := historyOp.Read(ctx, h.RequestID)
	if err != nil {
		t.Fatalf("read history: %v", err)
	}
	if got.NotificationHistory.RequestID != h.RequestID {
		t.Errorf("unexpected history: %+v", got.NotificationHistory)
	}
	if _, err := historyOp.Read(ctx, "999999"); err == nil {
		t.Error("expected error reading a nonexistent history")
	}
}

func TestRoutingReorder(t *testing.T) {
	srv := simplenotification.NewTestServer(simplenotification.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	client := newControlPlaneClient(t, srv.TestURL())
	routingOp := sdk.NewRoutingOp(client)
	groupOp := sdk.NewGroupOp(client)

	group, err := groupOp.Create(ctx, v1.PostCommonServiceItemRequest{
		CommonServiceItem: v1.PostCommonServiceItemRequestCommonServiceItem{
			Name: "target-group",
			Icon: v1.NilCommonServiceItemIcon{Null: true},
			Settings: v1.CommonServiceItemSettings{
				GroupSettings: v1.GroupSettings{Destinations: []string{}},
			},
			Tags: []string{},
		},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	newRouting := func(name string, rank int) string {
		t.Helper()
		r, err := routingOp.Create(ctx, v1.PostCommonServiceItemRequest{
			CommonServiceItem: v1.PostCommonServiceItemRequestCommonServiceItem{
				Name: name,
				Icon: v1.NilCommonServiceItemIcon{Null: true},
				Settings: v1.CommonServiceItemSettings{
					RoutingSettings: v1.RoutingSettings{
						SourceID:      "1",
						MatchLabels:   []v1.RoutingSettingsMatchLabelsItem{},
						TargetGroupID: group.CommonServiceItem.ID,
						PriorityRank:  rank,
					},
				},
				Tags: []string{},
			},
		})
		if err != nil {
			t.Fatalf("create routing %s: %v", name, err)
		}
		return r.CommonServiceItem.ID
	}
	r1 := newRouting("routing-1", 1)
	r2 := newRouting("routing-2", 2)

	res, err := routingOp.Reorder(ctx, v1.PutCommonServiceItemRoutingReorderRequest{
		Orders: []v1.PutCommonServiceItemRoutingReorderRequestOrdersItem{
			{RoutingID: r1, PriorityRank: 2},
			{RoutingID: r2, PriorityRank: 1},
		},
	})
	if err != nil {
		t.Fatalf("reorder: %v", err)
	}
	if ok, _ := res.IsOk.Get(); !ok {
		t.Errorf("expected is_ok=true, got %+v", res)
	}

	// The new ranks are visible on the routings.
	got, err := routingOp.Read(ctx, r1)
	if err != nil {
		t.Fatalf("read routing: %v", err)
	}
	if got.CommonServiceItem.Settings.RoutingSettings.PriorityRank != 2 {
		t.Errorf("expected routing-1 rank 2, got %+v", got.CommonServiceItem.Settings.RoutingSettings)
	}

	// Duplicate ranks are rejected.
	if _, err := routingOp.Reorder(ctx, v1.PutCommonServiceItemRoutingReorderRequest{
		Orders: []v1.PutCommonServiceItemRoutingReorderRequestOrdersItem{
			{RoutingID: r1, PriorityRank: 1},
			{RoutingID: r2, PriorityRank: 1},
		},
	}); err == nil {
		t.Error("expected duplicate-rank reorder to fail")
	}

	// Unknown routing IDs are rejected.
	if _, err := routingOp.Reorder(ctx, v1.PutCommonServiceItemRoutingReorderRequest{
		Orders: []v1.PutCommonServiceItemRoutingReorderRequestOrdersItem{
			{RoutingID: "999999999999", PriorityRank: 3},
		},
	}); err == nil {
		t.Error("expected reorder of unknown routing to fail")
	}
}
