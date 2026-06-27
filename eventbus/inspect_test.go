package eventbus_test

import (
	"testing"
	"time"

	sdk "github.com/sacloud/sacloud-sdk-go/api/eventbus"
	v1 "github.com/sacloud/sacloud-sdk-go/api/eventbus/apis/v1"

	"github.com/sacloud/sakumock/eventbus"
)

func TestInspectionClientInjectEvent(t *testing.T) {
	srv := eventbus.NewTestServer(eventbus.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	ic := eventbus.NewInspectionClient(srv.TestURL())

	pcOp := sdk.NewProcessConfigurationOp(client)
	triggerOp := sdk.NewTriggerOp(client)

	pcID := createProcessConfiguration(t, pcOp)
	_, err := triggerOp.Create(ctx, v1.CreateCommonServiceItemRequest{
		CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
			Name: "ic-trigger",
			Settings: v1.NewTriggerSettingsSettings(v1.TriggerSettings{
				Source:                 "ic-source",
				ProcessConfigurationID: pcID,
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	ds, err := ic.InjectEvent(ctx, eventbus.Event{Source: "ic-source"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(ds))
	}
	if ds[0].Destination != "simplenotification" {
		t.Errorf("unexpected destination: %s", ds[0].Destination)
	}
}

func TestInspectionClientDeliveries(t *testing.T) {
	srv := eventbus.NewTestServer(eventbus.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	ic := eventbus.NewInspectionClient(srv.TestURL())

	pcOp := sdk.NewProcessConfigurationOp(client)
	triggerOp := sdk.NewTriggerOp(client)

	pcID := createProcessConfiguration(t, pcOp)
	_, err := triggerOp.Create(ctx, v1.CreateCommonServiceItemRequest{
		CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
			Name: "ic-trigger",
			Settings: v1.NewTriggerSettingsSettings(v1.TriggerSettings{
				Source:                 "ic-source",
				ProcessConfigurationID: pcID,
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ic.InjectEvent(ctx, eventbus.Event{Source: "ic-source"}); err != nil {
		t.Fatal(err)
	}

	ds, err := ic.Deliveries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(ds))
	}

	if err := ic.ClearDeliveries(ctx); err != nil {
		t.Fatal(err)
	}
	ds, err = ic.Deliveries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 0 {
		t.Fatalf("expected 0 deliveries after clear, got %d", len(ds))
	}
}

func TestInspectionClientTick(t *testing.T) {
	srv := eventbus.NewTestServer(eventbus.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	ic := eventbus.NewInspectionClient(srv.TestURL())

	pcOp := sdk.NewProcessConfigurationOp(client)
	scheduleOp := sdk.NewScheduleOp(client)

	pcID := createProcessConfiguration(t, pcOp)
	// StartsAt strictly after server construction (its initial tick baseline),
	// so the boundary at StartsAt itself is in the first window.
	start := time.Now().Add(time.Second)
	_, err := scheduleOp.Create(ctx, v1.CreateCommonServiceItemRequest{
		CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
			Name: "ic-schedule",
			Settings: v1.NewScheduleSettingsSettings(v1.ScheduleSettings{
				ProcessConfigurationID: pcID,
				StartsAt:               v1.NewInt64ScheduleSettingsStartsAt(start.UnixMilli()),
				RecurringStep:          v1.NewOptInt(1),
				RecurringUnit:          v1.NewOptScheduleSettingsRecurringUnit(v1.ScheduleSettingsRecurringUnitMin),
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Tick 150s past StartsAt: boundaries at StartsAt, +1m, +2m fall due.
	at := start.Add(150 * time.Second)
	ds, err := ic.Tick(ctx, at)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 3 {
		t.Fatalf("expected 3 deliveries from tick, got %d", len(ds))
	}

	// Tick with zero time (server uses current time); no error expected.
	ds, err = ic.Tick(ctx, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	_ = ds
}
