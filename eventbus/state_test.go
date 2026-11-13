package eventbus_test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	sdk "github.com/sacloud/sacloud-sdk-go/api/eventbus"
	v1 "github.com/sacloud/sacloud-sdk-go/api/eventbus/apis/v1"

	"github.com/sacloud/sakumock/core"
	"github.com/sacloud/sakumock/core/coretest"
	"github.com/sacloud/sakumock/eventbus"
)

func createRecurringSchedule(t *testing.T, client *v1.Client, pcID string, start time.Time) string {
	t.Helper()
	created, err := sdk.NewScheduleOp(client).Create(t.Context(), v1.CreateCommonServiceItemRequest{
		CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
			Name: "persisted-schedule",
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
	return created.ID
}

func createTrigger(t *testing.T, client *v1.Client, pcID string) string {
	t.Helper()
	created, err := sdk.NewTriggerOp(client).Create(t.Context(), v1.CreateCommonServiceItemRequest{
		CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
			Name: "persisted-trigger",
			Settings: v1.NewTriggerSettingsSettings(v1.TriggerSettings{
				Source:                 "test-source",
				ProcessConfigurationID: pcID,
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return created.ID
}

func TestStateSurvivesRestart(t *testing.T) {
	var pcID, scheduleID, triggerID, scheduleHash string

	coretest.StateRestart(t, eventbus.Config{},
		func(t *testing.T, url string) {
			ctx := t.Context()
			client := newTestClient(t, url)
			pcOp := sdk.NewProcessConfigurationOp(client)
			pcID = createProcessConfiguration(t, pcOp)
			if err := pcOp.UpdateSecret(ctx, pcID, v1.SetSecretRequest{
				Secret: v1.NewSimpleMQSecretSetSecretRequestSecret(v1.SimpleMQSecret{APIKey: "apikey"}),
			}); err != nil {
				t.Fatal(err)
			}
			scheduleID = createRecurringSchedule(t, client, pcID, time.Now().Add(time.Hour))
			schedule, err := sdk.NewScheduleOp(client).Read(ctx, scheduleID)
			if err != nil {
				t.Fatal(err)
			}
			scheduleHash = schedule.SettingsHash
			triggerID = createTrigger(t, client, pcID)
			// Fire the trigger so its Status is part of the saved state.
			fired, err := eventbus.NewInspectionClient(url).InjectEvent(ctx, eventbus.Event{Source: "test-source", Type: "any"})
			if err != nil {
				t.Fatal(err)
			}
			if len(fired) != 1 {
				t.Fatalf("expected the trigger to fire once, got %d", len(fired))
			}
			// A deleted item must not come back.
			deleted := createProcessConfiguration(t, pcOp)
			if err := pcOp.Delete(ctx, deleted); err != nil {
				t.Fatal(err)
			}
		},
		func(t *testing.T, url string) {
			ctx := t.Context()
			client := newTestClient(t, url)
			pcOp := sdk.NewProcessConfigurationOp(client)
			pcs, err := pcOp.List(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(pcs) != 1 || pcs[0].ID != pcID {
				t.Fatalf("restored process configurations = %+v, want only %s", pcs, pcID)
			}
			pc, err := pcOp.Read(ctx, pcID)
			if err != nil {
				t.Fatal(err)
			}
			if st, ok := pc.Settings.GetProcessConfigurationSettings(); !ok ||
				st.Destination != v1.ProcessConfigurationSettingsDestinationSimplenotification ||
				pc.Name != "test-pc" || len(pc.Tags) != 1 {
				t.Errorf("restored process configuration = %+v", pc)
			}

			schedule, err := sdk.NewScheduleOp(client).Read(ctx, scheduleID)
			if err != nil {
				t.Fatal(err)
			}
			if st, ok := schedule.Settings.GetScheduleSettings(); !ok || st.ProcessConfigurationID != pcID {
				t.Errorf("restored schedule settings = %+v", schedule.Settings)
			}
			if schedule.SettingsHash != scheduleHash {
				t.Errorf("restored schedule SettingsHash = %s, want %s", schedule.SettingsHash, scheduleHash)
			}
			if _, ok := schedule.Status.Get(); ok {
				t.Errorf("schedule that never fired has a Status after restart: %+v", schedule.Status)
			}

			trigger, err := sdk.NewTriggerOp(client).Read(ctx, triggerID)
			if err != nil {
				t.Fatal(err)
			}
			status, ok := trigger.Status.Get()
			if !ok {
				t.Fatal("the trigger's Status was not restored")
			}
			if success, _ := status.Success.Get(); !success {
				t.Errorf("restored trigger Status = %+v, want success", status)
			}

			// The delivery log is not part of the state.
			deliveries, err := eventbus.NewInspectionClient(url).Deliveries(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(deliveries) != 0 {
				t.Errorf("expected no deliveries after restart, got %+v", deliveries)
			}
		})
}

// An item created after a restart never reuses a restored ID.
func TestStateRestoredIDsAreNotReused(t *testing.T) {
	var pcID string
	create := func(t *testing.T, url string) string {
		t.Helper()
		return createProcessConfiguration(t, sdk.NewProcessConfigurationOp(newTestClient(t, url)))
	}
	coretest.StateRestart(t, eventbus.Config{},
		func(t *testing.T, url string) { pcID = create(t, url) },
		func(t *testing.T, url string) {
			if id := create(t, url); id == pcID {
				t.Errorf("new process configuration reused the restored ID %s", id)
			}
		}, true)
}

// Restored schedules and triggers are live: the data plane fires them without
// their being created or updated again.
func TestStateRestoredScheduleAndTriggerFire(t *testing.T) {
	var scheduleID, triggerID string
	start := time.Now().Add(time.Hour)

	coretest.StateRestart(t, eventbus.Config{},
		func(t *testing.T, url string) {
			client := newTestClient(t, url)
			pcID := createProcessConfiguration(t, sdk.NewProcessConfigurationOp(client))
			scheduleID = createRecurringSchedule(t, client, pcID, start)
			triggerID = createTrigger(t, client, pcID)
		},
		func(t *testing.T, url string) {
			ctx := t.Context()
			ic := eventbus.NewInspectionClient(url)

			fired, err := ic.Tick(ctx, start.Add(30*time.Second))
			if err != nil {
				t.Fatal(err)
			}
			if len(fired) != 1 || fired[0].SourceID != scheduleID || fired[0].Error != "" {
				t.Errorf("tick after restart fired %+v, want the restored schedule %s once", fired, scheduleID)
			}
			schedule, err := sdk.NewScheduleOp(newTestClient(t, url)).Read(ctx, scheduleID)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := schedule.Status.Get(); !ok {
				t.Error("restored schedule has no Status after firing")
			}

			fired, err = ic.InjectEvent(ctx, eventbus.Event{Source: "test-source", Type: "any"})
			if err != nil {
				t.Fatal(err)
			}
			if len(fired) != 1 || fired[0].SourceID != triggerID || fired[0].Error != "" {
				t.Errorf("event after restart fired %+v, want the restored trigger %s once", fired, triggerID)
			}
		}, true)
}

// The write-only secret of a process configuration is restored too. The API
// cannot read it back, so this goes through Server.Secret.
func TestStateRestoresSecret(t *testing.T) {
	opts := core.ServerOptions{StateDir: t.TempDir()}
	start := func() (*eventbus.Server, *httptest.Server) {
		t.Helper()
		srv, err := eventbus.Config{}.NewServer(opts)
		if err != nil {
			t.Fatal(err)
		}
		return srv.(*eventbus.Server), httptest.NewServer(srv)
	}

	srv, ts := start()
	op := sdk.NewProcessConfigurationOp(newTestClient(t, ts.URL))
	id := createProcessConfiguration(t, op)
	if err := op.UpdateSecret(t.Context(), id, v1.SetSecretRequest{
		Secret: v1.NewSimpleMQSecretSetSecretRequestSecret(v1.SimpleMQSecret{APIKey: "apikey"}),
	}); err != nil {
		t.Fatal(err)
	}
	want, ok := srv.Secret(id)
	if !ok {
		t.Fatal("secret was not stored")
	}
	ts.Close()
	srv.Close()

	srv, ts = start()
	defer srv.Close()
	defer ts.Close()
	got, ok := srv.Secret(id)
	if !ok {
		t.Fatal("secret was not restored")
	}
	if string(got) != string(want) {
		t.Errorf("restored secret = %s, want %s", got, want)
	}
	var secret struct {
		APIKey string `json:"APIKey"`
	}
	if err := json.Unmarshal(got, &secret); err != nil || secret.APIKey != "apikey" {
		t.Errorf("restored secret = %s (%v), want APIKey apikey", got, err)
	}
}
