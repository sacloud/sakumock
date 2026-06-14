package eventbus_test

import (
	"encoding/json"
	"strings"
	"testing"

	sdk "github.com/sacloud/sacloud-sdk-go/api/eventbus"
	v1 "github.com/sacloud/sacloud-sdk-go/api/eventbus/apis/v1"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	"github.com/sacloud/sakumock/eventbus"
)

func newTestClient(t *testing.T, serverURL string) *v1.Client {
	t.Helper()
	var sa saclient.Client
	// The trailing slash matters: see Config.ClientEnv.
	if err := sa.SetEnviron([]string{
		"SAKURA_ENDPOINTS_EVENTBUS=" + serverURL + "/",
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

// createProcessConfiguration creates a process configuration through the SDK
// and returns its ID.
func createProcessConfiguration(t *testing.T, op sdk.ProcessConfigurationAPI) string {
	t.Helper()
	created, err := op.Create(t.Context(), v1.CreateCommonServiceItemRequest{
		CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
			Name:     "test-pc",
			Settings: sdk.CreateSimpleNotificationSettings("123456789012", "hello"),
			Tags:     []string{"tag1"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return created.ID
}

func TestProcessConfigurationCRUD(t *testing.T) {
	srv := eventbus.NewTestServer(eventbus.Config{})
	defer srv.Close()
	ctx := t.Context()
	op := sdk.NewProcessConfigurationOp(newTestClient(t, srv.TestURL()))

	id := createProcessConfiguration(t, op)

	got, err := op.Read(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "test-pc" {
		t.Errorf("unexpected name: %s", got.Name)
	}
	if got.Provider.Class != v1.ProviderClassEventbusprocessconfiguration {
		t.Errorf("unexpected provider class: %s", got.Provider.Class)
	}
	st, ok := got.Settings.GetProcessConfigurationSettings()
	if !ok {
		t.Fatalf("settings is not ProcessConfigurationSettings: %+v", got.Settings)
	}
	if st.Destination != v1.ProcessConfigurationSettingsDestinationSimplenotification {
		t.Errorf("unexpected destination: %s", st.Destination)
	}
	if !strings.Contains(st.Parameters, "123456789012") {
		t.Errorf("unexpected parameters: %s", st.Parameters)
	}

	updated, err := op.Update(ctx, id, v1.UpdateCommonServiceItemRequest{
		CommonServiceItem: v1.UpdateCommonServiceItemRequestCommonServiceItem{
			Name:     v1.NewOptString("test-pc-renamed"),
			Settings: v1.NewOptSettings(sdk.CreateSimpleMqSettings("test-queue", "content")),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "test-pc-renamed" {
		t.Errorf("unexpected name after update: %s", updated.Name)
	}
	if st, _ := updated.Settings.GetProcessConfigurationSettings(); st.Destination != v1.ProcessConfigurationSettingsDestinationSimplemq {
		t.Errorf("unexpected destination after update: %s", st.Destination)
	}

	if err := op.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := op.Read(ctx, id); err == nil {
		t.Fatal("expected error reading deleted item")
	}
}

func TestScheduleCRUD(t *testing.T) {
	srv := eventbus.NewTestServer(eventbus.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	pcOp := sdk.NewProcessConfigurationOp(client)
	scheduleOp := sdk.NewScheduleOp(client)

	pcID := createProcessConfiguration(t, pcOp)

	created, err := scheduleOp.Create(ctx, v1.CreateCommonServiceItemRequest{
		CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
			Name: "test-schedule",
			Settings: v1.NewScheduleSettingsSettings(v1.ScheduleSettings{
				ProcessConfigurationID: pcID,
				StartsAt:               v1.NewInt64ScheduleSettingsStartsAt(1700000000000),
				RecurringStep:          v1.NewOptInt(1),
				RecurringUnit:          v1.NewOptScheduleSettingsRecurringUnit(v1.ScheduleSettingsRecurringUnitDay),
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	st, ok := created.Settings.GetScheduleSettings()
	if !ok {
		t.Fatalf("settings is not ScheduleSettings: %+v", created.Settings)
	}
	if st.ProcessConfigurationID != pcID {
		t.Errorf("unexpected process configuration id: %s", st.ProcessConfigurationID)
	}
	// The API accepts StartsAt as an integer but returns it as a string.
	if got, ok := st.StartsAt.GetString(); !ok || got != "1700000000000" {
		t.Errorf("expected StartsAt as string %q, got %+v", "1700000000000", st.StartsAt)
	}

	got, err := scheduleOp.Read(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "test-schedule" {
		t.Errorf("unexpected name: %s", got.Name)
	}

	if err := scheduleOp.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
}

func TestTriggerCRUD(t *testing.T) {
	srv := eventbus.NewTestServer(eventbus.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	pcOp := sdk.NewProcessConfigurationOp(client)
	triggerOp := sdk.NewTriggerOp(client)

	pcID := createProcessConfiguration(t, pcOp)

	created, err := triggerOp.Create(ctx, v1.CreateCommonServiceItemRequest{
		CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
			Name: "test-trigger",
			Settings: v1.NewTriggerSettingsSettings(v1.TriggerSettings{
				Source:                 "test-source",
				Types:                  v1.NewOptNilStringArray([]string{"type1"}),
				ProcessConfigurationID: pcID,
				Conditions: v1.NewOptNilTriggerSettingsConditionsItemArray([]v1.TriggerSettingsConditionsItem{
					v1.NewTriggerConditionEqTriggerSettingsConditionsItem(v1.TriggerConditionEq{
						Key: "key1", Op: v1.TriggerConditionEqOpEq, Values: []string{"foo"},
					}),
					v1.NewTriggerConditionInTriggerSettingsConditionsItem(v1.TriggerConditionIn{
						Key: "key2", Op: v1.TriggerConditionInOpIn, Values: []string{"bar", "buz"},
					}),
				}),
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := triggerOp.Read(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	st, ok := got.Settings.GetTriggerSettings()
	if !ok {
		t.Fatalf("settings is not TriggerSettings: %+v", got.Settings)
	}
	if st.Source != "test-source" {
		t.Errorf("unexpected source: %s", st.Source)
	}
	conditions, _ := st.Conditions.Get()
	if len(conditions) != 2 {
		t.Fatalf("expected 2 conditions, got %d", len(conditions))
	}
	if eq, ok := conditions[0].GetTriggerConditionEq(); !ok || eq.Key != "key1" {
		t.Errorf("unexpected first condition: %+v", conditions[0])
	}
	if in, ok := conditions[1].GetTriggerConditionIn(); !ok || len(in.Values) != 2 {
		t.Errorf("unexpected second condition: %+v", conditions[1])
	}

	if err := triggerOp.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
}

func TestListFiltersByProviderClass(t *testing.T) {
	srv := eventbus.NewTestServer(eventbus.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	pcOp := sdk.NewProcessConfigurationOp(client)
	scheduleOp := sdk.NewScheduleOp(client)
	triggerOp := sdk.NewTriggerOp(client)

	pcID := createProcessConfiguration(t, pcOp)
	if _, err := scheduleOp.Create(ctx, v1.CreateCommonServiceItemRequest{
		CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
			Name: "test-schedule",
			Settings: v1.NewScheduleSettingsSettings(v1.ScheduleSettings{
				ProcessConfigurationID: pcID,
				StartsAt:               v1.NewInt64ScheduleSettingsStartsAt(1700000000000),
				Crontab:                v1.NewOptString("*/15 * * * *"),
			}),
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := triggerOp.Create(ctx, v1.CreateCommonServiceItemRequest{
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

	pcs, err := pcOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pcs) != 1 || pcs[0].Provider.Class != v1.ProviderClassEventbusprocessconfiguration {
		t.Errorf("unexpected process configuration list: %+v", pcs)
	}
	schedules, err := scheduleOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(schedules) != 1 || schedules[0].Provider.Class != v1.ProviderClassEventbusschedule {
		t.Errorf("unexpected schedule list: %+v", schedules)
	}
	triggers, err := triggerOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(triggers) != 1 || triggers[0].Provider.Class != v1.ProviderClassEventbustrigger {
		t.Errorf("unexpected trigger list: %+v", triggers)
	}
}

func TestScheduleRejectsInvalidCrontab(t *testing.T) {
	srv := eventbus.NewTestServer(eventbus.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	pcOp := sdk.NewProcessConfigurationOp(client)
	scheduleOp := sdk.NewScheduleOp(client)

	pcID := createProcessConfiguration(t, pcOp)

	newReq := func(crontab string) v1.CreateCommonServiceItemRequest {
		return v1.CreateCommonServiceItemRequest{
			CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
				Name: "test-schedule",
				Settings: v1.NewScheduleSettingsSettings(v1.ScheduleSettings{
					ProcessConfigurationID: pcID,
					StartsAt:               v1.NewInt64ScheduleSettingsStartsAt(1700000000000),
					Crontab:                v1.NewOptString(crontab),
				}),
			},
		}
	}

	// Day-of-week 7 and aliases are documented as invalid.
	for _, crontab := range []string{"* * * * 7", "0 0 * JAN *"} {
		if _, err := scheduleOp.Create(ctx, newReq(crontab)); err == nil {
			t.Errorf("expected error creating schedule with crontab %q", crontab)
		}
	}

	created, err := scheduleOp.Create(ctx, newReq("*/15 * * * *"))
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := created.Settings.GetScheduleSettings(); st.Crontab.Value != "*/15 * * * *" {
		t.Errorf("unexpected crontab after create: %+v", st.Crontab)
	}
}

func TestScheduleRequiresExistingProcessConfiguration(t *testing.T) {
	srv := eventbus.NewTestServer(eventbus.Config{})
	defer srv.Close()
	scheduleOp := sdk.NewScheduleOp(newTestClient(t, srv.TestURL()))

	_, err := scheduleOp.Create(t.Context(), v1.CreateCommonServiceItemRequest{
		CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
			Name: "test-schedule",
			Settings: v1.NewScheduleSettingsSettings(v1.ScheduleSettings{
				ProcessConfigurationID: "999999999999",
				StartsAt:               v1.NewInt64ScheduleSettingsStartsAt(1700000000000),
				Crontab:                v1.NewOptString("*/15 * * * *"),
			}),
		},
	})
	if err == nil {
		t.Fatal("expected error creating schedule referencing a missing process configuration")
	}
}

func TestScheduleRequiresCronOrRecurring(t *testing.T) {
	srv := eventbus.NewTestServer(eventbus.Config{})
	defer srv.Close()
	client := newTestClient(t, srv.TestURL())
	pcOp := sdk.NewProcessConfigurationOp(client)
	scheduleOp := sdk.NewScheduleOp(client)

	pcID := createProcessConfiguration(t, pcOp)

	// A schedule's type is exactly one of Crontab or recurring — the control
	// panel presents a mutually exclusive choice. Both the empty and the
	// both-specified cases are rejected, though the OpenAPI marks them optional.
	cases := map[string]v1.ScheduleSettings{
		"neither": {
			ProcessConfigurationID: pcID,
			StartsAt:               v1.NewInt64ScheduleSettingsStartsAt(1700000000000),
		},
		"both": {
			ProcessConfigurationID: pcID,
			StartsAt:               v1.NewInt64ScheduleSettingsStartsAt(1700000000000),
			Crontab:                v1.NewOptString("*/15 * * * *"),
			RecurringStep:          v1.NewOptInt(1),
			RecurringUnit:          v1.NewOptScheduleSettingsRecurringUnit(v1.ScheduleSettingsRecurringUnitDay),
		},
	}
	for name, settings := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := scheduleOp.Create(t.Context(), v1.CreateCommonServiceItemRequest{
				CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
					Name:     "test-schedule",
					Settings: v1.NewScheduleSettingsSettings(settings),
				},
			})
			if err == nil {
				t.Fatalf("expected error creating a schedule with %s of Crontab/recurring", name)
			}
		})
	}
}

func TestSetSecret(t *testing.T) {
	srv := eventbus.NewTestServer(eventbus.Config{})
	defer srv.Close()
	ctx := t.Context()
	op := sdk.NewProcessConfigurationOp(newTestClient(t, srv.TestURL()))

	id := createProcessConfiguration(t, op)

	if _, ok := srv.Secret(id); ok {
		t.Fatal("expected no secret before set-secret")
	}

	err := op.UpdateSecret(ctx, id, v1.SetSecretRequest{
		Secret: v1.NewSacloudAPISecretSetSecretRequestSecret(v1.SacloudAPISecret{
			AccessToken:       "token",
			AccessTokenSecret: "secret",
		}),
	})
	if err != nil {
		t.Fatal(err)
	}

	secret, ok := srv.Secret(id)
	if !ok {
		t.Fatal("expected secret to be stored")
	}
	var got struct {
		AccessToken       string `json:"AccessToken"`
		AccessTokenSecret string `json:"AccessTokenSecret"`
	}
	if err := json.Unmarshal(secret, &got); err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != "token" || got.AccessTokenSecret != "secret" {
		t.Errorf("unexpected secret: %s", secret)
	}

	// SimpleMQ secrets are accepted too.
	err = op.UpdateSecret(ctx, id, v1.SetSecretRequest{
		Secret: v1.NewSimpleMQSecretSetSecretRequestSecret(v1.SimpleMQSecret{APIKey: "apikey"}),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSetSecretOnNonProcessConfiguration(t *testing.T) {
	srv := eventbus.NewTestServer(eventbus.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	pcOp := sdk.NewProcessConfigurationOp(client)
	triggerOp := sdk.NewTriggerOp(client)

	pcID := createProcessConfiguration(t, pcOp)
	trigger, err := triggerOp.Create(ctx, v1.CreateCommonServiceItemRequest{
		CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
			Name: "test-trigger",
			Settings: v1.NewTriggerSettingsSettings(v1.TriggerSettings{
				Source:                 "test-source",
				ProcessConfigurationID: pcID,
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = pcOp.UpdateSecret(ctx, trigger.ID, v1.SetSecretRequest{
		Secret: v1.NewSimpleMQSecretSetSecretRequestSecret(v1.SimpleMQSecret{APIKey: "apikey"}),
	})
	if err == nil {
		t.Fatal("expected error setting a secret on a trigger")
	}
}

func TestUpdateRejectsClassMismatch(t *testing.T) {
	srv := eventbus.NewTestServer(eventbus.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	pcOp := sdk.NewProcessConfigurationOp(client)
	scheduleOp := sdk.NewScheduleOp(client)

	pcID := createProcessConfiguration(t, pcOp)

	// Addressing a process configuration through the schedule op must fail.
	_, err := scheduleOp.Update(ctx, pcID, v1.UpdateCommonServiceItemRequest{
		CommonServiceItem: v1.UpdateCommonServiceItemRequestCommonServiceItem{
			Name: v1.NewOptString("renamed"),
			Settings: v1.NewOptSettings(v1.NewScheduleSettingsSettings(v1.ScheduleSettings{
				ProcessConfigurationID: pcID,
				StartsAt:               v1.NewInt64ScheduleSettingsStartsAt(1700000000000),
			})),
		},
	})
	if err == nil {
		t.Fatal("expected error updating a process configuration via the schedule op")
	}
}
