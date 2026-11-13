package eventbus

import (
	"encoding/json"
	"github.com/sacloud/sakumock/core"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A firing by the autonomous scheduler happens outside of any request, so it
// has to save the Status it sets by itself.
func TestAutonomousFiringSavesState(t *testing.T) {
	dir := t.TempDir()
	srv, err := NewHandler(Config{stateDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	start := time.Now().Add(time.Hour)
	pc := srv.store.CreateItem(ServiceItem{
		ProviderClass: classProcessConfiguration,
		Settings:      json.RawMessage(`{"Destination":"simplemq","Parameters":"{}"}`),
	})
	settings, err := json.Marshal(scheduleSettings{
		ProcessConfigurationID: pc.ID,
		StartsAt:               startsAtMs(start),
		RecurringStep:          1,
		RecurringUnit:          "min",
	})
	if err != nil {
		t.Fatal(err)
	}
	schedule := srv.store.CreateItem(ServiceItem{ProviderClass: classSchedule, Settings: settings})

	srv.dataPlane.now = func() time.Time { return start.Add(time.Second) }
	srv.dataPlane.autoTick(t.Context())

	if _, err := os.Stat(filepath.Join(dir, "eventbus.json")); err != nil {
		t.Fatalf("state not saved after an autonomous firing: %v", err)
	}
	restored := NewMemoryStore(nil)
	if _, err := core.OpenState(dir, "eventbus", restored, nil); err != nil {
		t.Fatal(err)
	}
	got, ok := restored.GetItem(schedule.ID)
	if !ok || got.Status == nil || !got.Status.Success {
		t.Errorf("saved schedule = %+v, want the Status of its firing", got)
	}
}

// Restore rejects a snapshot whose items could not be served, instead of
// loading it and misbehaving on the first request.
func TestRestoreRejectsInconsistentSnapshot(t *testing.T) {
	store := NewMemoryStore(nil)
	it := store.CreateItem(ServiceItem{ProviderClass: classTrigger, Settings: json.RawMessage(`{"Source":"s"}`)})
	good, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := NewMemoryStore(nil).Restore(good); err != nil {
		t.Fatalf("restore of an untouched snapshot: %v", err)
	}

	for name, broken := range map[string]string{
		"different id":           strings.Replace(string(good), `"id": "`+it.ID+`"`, `"id": "1"`, 1),
		"unknown provider class": strings.Replace(string(good), classTrigger, "eventbusunknown", 1),
		"invalid settings":       strings.Replace(string(good), `"settings": "{`, `"settings": "`, 1),
	} {
		if broken == string(good) {
			t.Fatalf("%s: the snapshot was not altered", name)
		}
		if err := NewMemoryStore(nil).Restore([]byte(broken)); err == nil {
			t.Errorf("%s: restore succeeded, want an error", name)
		}
	}
}
