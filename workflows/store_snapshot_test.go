package workflows

import (
	"strings"
	"testing"
)

// Restore rejects a snapshot whose records are filed inconsistently, instead
// of loading it and answering wrongly at request time.
func TestRestoreRejectsInconsistentSnapshot(t *testing.T) {
	store := NewMemoryStore(nil)
	w := store.CreateWorkflow("w", "", "steps: {}", true, false, nil, "", "", "")
	e, err := store.CreateExecution(w.ID, ExecutionInput{})
	if err != nil {
		t.Fatal(err)
	}
	store.AppendHistory(w.ID, e.ExecutionID, HistoryRecord{WorkflowExecutionID: e.ExecutionID, Type: "workflowWillStart"})
	if err := store.CreateSubscription(1); err != nil {
		t.Fatal(err)
	}
	good, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := NewMemoryStore(nil).Restore(good); err != nil {
		t.Fatalf("restore of an untouched snapshot: %v", err)
	}

	for name, broken := range map[string]string{
		"workflow under another id":    strings.Replace(string(good), `"id": "`+w.ID+`"`, `"id": "1"`, 1),
		"revision of another workflow": strings.Replace(string(good), `"workflow_id": "`+w.ID+`"`, `"workflow_id": "1"`, 1),
		"unknown execution status":     strings.Replace(string(good), `"status": "Succeeded"`, `"status": "Done"`, 1),
		"history of no execution":      strings.Replace(string(good), `"`+e.ExecutionID+`": [`, `"orphan": [`, 1),
		"unknown plan":                 strings.Replace(string(good), `"plan_id": 1`, `"plan_id": 99`, 1),
	} {
		if broken == string(good) {
			t.Fatalf("%s: the snapshot was not altered", name)
		}
		if err := NewMemoryStore(nil).Restore([]byte(broken)); err == nil {
			t.Errorf("%s: restore succeeded, want an error", name)
		}
	}
}
