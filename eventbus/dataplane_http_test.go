package eventbus_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	sdk "github.com/sacloud/sacloud-sdk-go/api/eventbus"
	v1 "github.com/sacloud/sacloud-sdk-go/api/eventbus/apis/v1"

	"github.com/sacloud/sakumock/eventbus"
)

// delivery mirrors eventbus.Delivery for decoding the inspection-endpoint JSON.
type delivery struct {
	SourceID               string `json:"SourceID"`
	SourceClass            string `json:"SourceClass"`
	ProcessConfigurationID string `json:"ProcessConfigurationID"`
	Destination            string `json:"Destination"`
	Parameters             string `json:"Parameters"`
	Error                  string `json:"Error"`
}

type deliveriesResp struct {
	Deliveries []delivery `json:"Deliveries"`
	Count      int        `json:"Count"`
}

func postJSON(t *testing.T, url string, body any) deliveriesResp {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	resp, err := http.Post(url, "application/json", &buf)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s: status %d", url, resp.StatusCode)
	}
	var out deliveriesResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestInjectEventFiresMatchingTrigger(t *testing.T) {
	srv := eventbus.NewTestServer(eventbus.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	pcOp := sdk.NewProcessConfigurationOp(client)
	triggerOp := sdk.NewTriggerOp(client)

	pcID := createProcessConfiguration(t, pcOp) // destination simplenotification
	trigger, err := triggerOp.Create(ctx, v1.CreateCommonServiceItemRequest{
		CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
			Name: "test-trigger",
			Settings: v1.NewTriggerSettingsSettings(v1.TriggerSettings{
				Source:                 "test-source",
				Types:                  v1.NewOptNilStringArray([]string{"type1"}),
				ProcessConfigurationID: pcID,
				Conditions: v1.NewOptNilTriggerSettingsConditionsItemArray([]v1.TriggerSettingsConditionsItem{
					v1.NewTriggerConditionEqTriggerSettingsConditionsItem(v1.TriggerConditionEq{
						Key: "status", Op: v1.TriggerConditionEqOpEq, Values: []string{"critical"},
					}),
				}),
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// A non-matching event (wrong status) fires nothing.
	nonMatch := postJSON(t, srv.TestURL()+"/_sakumock/events", map[string]any{
		"Source": "test-source", "Type": "type1",
		"Attributes": map[string]any{"status": "ok"},
	})
	if nonMatch.Count != 0 {
		t.Fatalf("non-matching event fired %d deliveries", nonMatch.Count)
	}

	// A matching event fires the trigger and delivers the process config's job.
	got := postJSON(t, srv.TestURL()+"/_sakumock/events", map[string]any{
		"Source": "test-source", "Type": "type1",
		"Attributes": map[string]any{"status": "critical"},
		"Data":       map[string]any{"detail": "disk full"},
	})
	if got.Count != 1 {
		t.Fatalf("expected 1 delivery, got %d", got.Count)
	}
	d := got.Deliveries[0]
	if d.SourceID != trigger.ID || d.SourceClass != "eventbustrigger" {
		t.Errorf("unexpected delivery source: %+v", d)
	}
	if d.ProcessConfigurationID != pcID || d.Destination != "simplenotification" {
		t.Errorf("unexpected delivery target: %+v", d)
	}
	if d.Error != "" {
		t.Errorf("unexpected delivery error: %s", d.Error)
	}

	// The firing is also retained and the trigger's Status reflects success.
	if list := srv.Deliveries(); len(list) != 1 {
		t.Errorf("expected 1 recorded delivery, got %d", len(list))
	}
	read, err := triggerOp.Read(ctx, trigger.ID)
	if err != nil {
		t.Fatal(err)
	}
	st, ok := read.Status.Get()
	if !ok {
		t.Fatal("expected Status to be populated after firing")
	}
	if success, _ := st.Success.Get(); !success {
		t.Errorf("expected Status.Success true, got %+v", st)
	}
}

func TestTickFiresRecurringSchedule(t *testing.T) {
	srv := eventbus.NewTestServer(eventbus.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	pcOp := sdk.NewProcessConfigurationOp(client)
	scheduleOp := sdk.NewScheduleOp(client)

	pcID := createProcessConfiguration(t, pcOp)
	// StartsAt strictly after the server's construction (its initial tick
	// baseline), so the boundary at StartsAt itself is in the first window.
	start := time.Now().Add(time.Second)
	schedule, err := scheduleOp.Create(ctx, v1.CreateCommonServiceItemRequest{
		CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
			Name: "test-schedule",
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

	// Tick 150s past StartsAt using a human-readable RFC3339 time: boundaries at
	// StartsAt, +1m, +2m fall due.
	at := url.QueryEscape(start.Add(150 * time.Second).Format(time.RFC3339))
	got := postJSON(t, srv.TestURL()+"/_sakumock/tick?at="+at, nil)
	if got.Count != 3 {
		t.Fatalf("expected 3 deliveries, got %d: %+v", got.Count, got.Deliveries)
	}
	if got.Deliveries[0].SourceID != schedule.ID || got.Deliveries[0].SourceClass != "eventbusschedule" {
		t.Errorf("unexpected delivery source: %+v", got.Deliveries[0])
	}

	// Ticking the same time again is idempotent: the boundaries already fired.
	again := postJSON(t, srv.TestURL()+"/_sakumock/tick?at="+at, nil)
	if again.Count != 0 {
		t.Errorf("re-tick should fire nothing, got %d", again.Count)
	}
}

func TestClearDeliveries(t *testing.T) {
	srv := eventbus.NewTestServer(eventbus.Config{})
	defer srv.Close()
	client := newTestClient(t, srv.TestURL())
	pcOp := sdk.NewProcessConfigurationOp(client)
	triggerOp := sdk.NewTriggerOp(client)

	pcID := createProcessConfiguration(t, pcOp)
	if _, err := triggerOp.Create(t.Context(), v1.CreateCommonServiceItemRequest{
		CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
			Name: "test-trigger",
			Settings: v1.NewTriggerSettingsSettings(v1.TriggerSettings{
				Source:                 "test-source",
				ProcessConfigurationID: pcID,
			}),
		},
	}); err != nil {
		t.Fatal(err)
	}
	postJSON(t, srv.TestURL()+"/_sakumock/events", map[string]any{"Source": "test-source"})
	if len(srv.Deliveries()) == 0 {
		t.Fatal("expected a recorded delivery")
	}

	req, _ := http.NewRequest(http.MethodDelete, srv.TestURL()+"/_sakumock/deliveries", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE deliveries: status %d", resp.StatusCode)
	}
	if len(srv.Deliveries()) != 0 {
		t.Errorf("expected deliveries cleared, got %d", len(srv.Deliveries()))
	}
}
