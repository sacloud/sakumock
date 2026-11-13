package simplenotification_test

import (
	"testing"

	sdk "github.com/sacloud/sacloud-sdk-go/api/simple-notification"
	v1 "github.com/sacloud/sacloud-sdk-go/api/simple-notification/apis/v1"

	"github.com/sacloud/sakumock/core/coretest"
	"github.com/sacloud/sakumock/simplenotification"
)

func createStateGroup(t *testing.T, url, name string, destinations []string) string {
	t.Helper()
	group, err := sdk.NewGroupOp(newControlPlaneClient(t, url)).Create(t.Context(), v1.PostCommonServiceItemRequest{
		CommonServiceItem: v1.PostCommonServiceItemRequestCommonServiceItem{
			Name:        name,
			Description: "a group",
			Icon:        v1.NilCommonServiceItemIcon{Null: true},
			Settings: v1.NewCommonServiceItemGroupSettingsPostCommonServiceItemRequestCommonServiceItemSettings(v1.CommonServiceItemGroupSettings{
				Destinations: destinations,
			}),
			Tags: []string{"a", "b"},
		},
	})
	if err != nil {
		t.Fatalf("create group %s: %v", name, err)
	}
	return group.CommonServiceItem.ID
}

func TestStateSurvivesRestart(t *testing.T) {
	var destID, groupID, routingID string

	coretest.StateRestart(t, simplenotification.Config{},
		func(t *testing.T, url string) {
			ctx := t.Context()
			client := newControlPlaneClient(t, url)
			dest, err := sdk.NewDestinationOp(client).Create(ctx, v1.PostCommonServiceItemRequest{
				CommonServiceItem: v1.PostCommonServiceItemRequestCommonServiceItem{
					Name: "mail-dest",
					Icon: v1.NilCommonServiceItemIcon{Null: true},
					Settings: v1.NewCommonServiceItemDestinationSettingsPostCommonServiceItemRequestCommonServiceItemSettings(v1.CommonServiceItemDestinationSettings{
						Type:  v1.CommonServiceItemDestinationSettingsTypeEmail,
						Value: "ops@example.com",
					}),
					Tags: []string{},
				},
			})
			if err != nil {
				t.Fatalf("create destination: %v", err)
			}
			destID = dest.CommonServiceItem.ID
			groupID = createStateGroup(t, url, "alert-group", []string{destID})
			routing, err := sdk.NewRoutingOp(client).Create(ctx, v1.PostCommonServiceItemRequest{
				CommonServiceItem: v1.PostCommonServiceItemRequestCommonServiceItem{
					Name: "routing-1",
					Icon: v1.NilCommonServiceItemIcon{Null: true},
					Settings: v1.NewCommonServiceItemRoutingSettingsPostCommonServiceItemRequestCommonServiceItemSettings(v1.CommonServiceItemRoutingSettings{
						SourceID:      "1",
						MatchLabels:   []v1.CommonServiceItemRoutingSettingsMatchLabelsItem{},
						TargetGroupID: groupID,
						PriorityRank:  3,
					}),
					Tags: []string{},
				},
			})
			if err != nil {
				t.Fatalf("create routing: %v", err)
			}
			routingID = routing.CommonServiceItem.ID
			for _, msg := range []string{"first", "second"} {
				if _, err := sdk.NewGroupOp(client).SendMessage(ctx, groupID, v1.SendNotificationRequest{Message: msg}); err != nil {
					t.Fatalf("send %s: %v", msg, err)
				}
			}
		},
		func(t *testing.T, url string) {
			ctx := t.Context()
			client := newControlPlaneClient(t, url)
			dest, err := sdk.NewDestinationOp(client).Read(ctx, destID)
			if err != nil {
				t.Fatalf("read destination: %v", err)
			}
			if dest.CommonServiceItem.Name != "mail-dest" || dest.CommonServiceItem.Settings.CommonServiceItemDestinationSettings.Value != "ops@example.com" {
				t.Errorf("restored destination = %+v", dest.CommonServiceItem)
			}
			group, err := sdk.NewGroupOp(client).Read(ctx, groupID)
			if err != nil {
				t.Fatalf("read group: %v", err)
			}
			if g := group.CommonServiceItem; g.Name != "alert-group" || g.Description != "a group" || len(g.Tags) != 2 ||
				len(g.Settings.CommonServiceItemGroupSettings.Destinations) != 1 {
				t.Errorf("restored group = %+v", g)
			}
			routing, err := sdk.NewRoutingOp(client).Read(ctx, routingID)
			if err != nil {
				t.Fatalf("read routing: %v", err)
			}
			if r := routing.CommonServiceItem.Settings.CommonServiceItemRoutingSettings; r.TargetGroupID != groupID || r.PriorityRank != 3 {
				t.Errorf("restored routing settings = %+v", r)
			}

			// The accepted messages come back in send order, still attributed
			// to the group's destination.
			histories, err := sdk.NewHistoryOp(client).List(ctx)
			if err != nil {
				t.Fatalf("list histories: %v", err)
			}
			bodies := map[string]bool{}
			for _, h := range histories.NotificationHistories {
				bodies[h.Message.Body] = true
				if len(h.Statuses) != 1 || h.Statuses[0].DestinationID != destID {
					t.Errorf("restored history statuses = %+v", h.Statuses)
				}
			}
			if len(histories.NotificationHistories) != 2 || !bodies["first"] || !bodies["second"] {
				t.Errorf("restored histories = %+v, want first and second", histories.NotificationHistories)
			}
			msgs, err := simplenotification.NewInspectionClient(url).Messages(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(msgs) != 2 || msgs[0].Message != "first" || msgs[1].Message != "second" {
				t.Errorf("restored messages = %+v, want first then second", msgs)
			}
		})
}

// An item or a message created after a restart never reuses an earlier ID,
// not even the ID of a message that was cleared before the restart.
func TestStateRestoredIDsAreNotReused(t *testing.T) {
	var groupID, messageID string
	send := func(t *testing.T, url, groupID string) string {
		t.Helper()
		ctx := t.Context()
		if _, err := sdk.NewGroupOp(newControlPlaneClient(t, url)).SendMessage(ctx, groupID, v1.SendNotificationRequest{Message: "hello"}); err != nil {
			t.Fatalf("send: %v", err)
		}
		msgs, err := simplenotification.NewInspectionClient(url).Messages(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(msgs) != 1 {
			t.Fatalf("messages = %+v, want one", msgs)
		}
		return msgs[0].ID
	}
	coretest.StateRestart(t, simplenotification.Config{},
		func(t *testing.T, url string) {
			groupID = createStateGroup(t, url, "first-group", []string{})
			messageID = send(t, url, groupID)
			if err := simplenotification.NewInspectionClient(url).ClearMessages(t.Context()); err != nil {
				t.Fatal(err)
			}
		},
		func(t *testing.T, url string) {
			if id := createStateGroup(t, url, "second-group", []string{}); id == groupID {
				t.Errorf("new group reused the restored ID %s", id)
			}
			if id := send(t, url, groupID); id == messageID {
				t.Errorf("new message reused the ID %s of a message cleared before the restart", id)
			}
		}, true)
}
