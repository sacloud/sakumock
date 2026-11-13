package workflows

import (
	"strings"
	"testing"
	"time"
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

// A canceled execution can be deleted while its runner is still stopping. A
// history entry arriving after that must not bring the history back: it would
// be saved without its execution and rejected at the next start.
func TestAppendHistoryIgnoresDeletedExecution(t *testing.T) {
	store := NewMemoryStore(nil)
	w := store.CreateWorkflow("w", "", "steps: {}", true, false, nil, "", "", "")
	e, err := store.CreateExecution(w.ID, ExecutionInput{InitialStatus: "Queued"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CancelExecution(w.ID, e.ExecutionID); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteExecution(w.ID, e.ExecutionID); err != nil {
		t.Fatal(err)
	}
	store.AppendHistory(w.ID, e.ExecutionID, HistoryRecord{WorkflowExecutionID: e.ExecutionID, Type: "workflowDidCanceled"})

	snapshot, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(snapshot), e.ExecutionID) {
		t.Errorf("history of the deleted execution is in the snapshot:\n%s", snapshot)
	}
	if err := NewMemoryStore(nil).Restore(snapshot); err != nil {
		t.Errorf("snapshot taken after the late history entry does not restore: %v", err)
	}
}

// The executor reports a cancellation it caused itself (shutdown, timeout)
// with its time; one already canceled through the API keeps the API's time.
func TestUpdateExecutionStatusCanceledAt(t *testing.T) {
	store := NewMemoryStore(nil)
	w := store.CreateWorkflow("w", "", "steps: {}", true, false, nil, "", "", "")
	later := time.Now().Add(time.Hour)

	byExecutor, err := store.CreateExecution(w.ID, ExecutionInput{InitialStatus: "Queued"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateExecutionStatus(w.ID, byExecutor.ExecutionID, ExecutionStatusUpdate{Status: "Canceled", CanceledAt: &later}); err != nil {
		t.Fatal(err)
	}
	if got, ok := store.GetExecution(w.ID, byExecutor.ExecutionID); !ok || got.CanceledAt == nil || !got.CanceledAt.Equal(later) {
		t.Errorf("executor-canceled execution = %+v, want CanceledAt %v", got, later)
	}

	byAPI, err := store.CreateExecution(w.ID, ExecutionInput{InitialStatus: "Queued"})
	if err != nil {
		t.Fatal(err)
	}
	canceled, err := store.CancelExecution(w.ID, byAPI.ExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	apiTime := *canceled.CanceledAt
	if err := store.UpdateExecutionStatus(w.ID, byAPI.ExecutionID, ExecutionStatusUpdate{Status: "Canceled", CanceledAt: &later}); err != nil {
		t.Fatal(err)
	}
	if got, ok := store.GetExecution(w.ID, byAPI.ExecutionID); !ok || got.CanceledAt == nil || !got.CanceledAt.Equal(apiTime) {
		t.Errorf("API-canceled execution = %+v, want the API's CanceledAt %v", got, apiTime)
	}
}
