package workflows

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sacloud/sakumock/workflows/runbook"
)

// interruptedMessage is the reason recorded on an execution that was cut
// short because the mock stopped, not because of anything in its runbook.
const interruptedMessage = "execution interrupted: sakumock was restarted"

// storeSnapshot is the store's form in the state file (--state-dir).
type storeSnapshot struct {
	Workflows map[string]*WorkflowRecord `json:"workflows"`
	// Revisions and Executions are keyed by workflow ID, oldest first.
	Revisions  map[string][]*RevisionRecord  `json:"revisions"`
	Executions map[string][]*ExecutionRecord `json:"executions"`
	// Histories is keyed by execution ID, oldest first.
	Histories    map[string][]HistoryRecord `json:"histories"`
	Subscription *SubscriptionRecord        `json:"subscription"`
}

// Snapshot returns every workflow, revision, execution, history entry and the
// subscription as JSON, for core.StateFile.
func (s *MemoryStore) Snapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.MarshalIndent(storeSnapshot{
		Workflows:    s.workflows,
		Revisions:    s.revisions,
		Executions:   s.executions,
		Histories:    s.histories,
		Subscription: s.subscription,
	}, "", "  ")
}

// Restore replaces the store's state with a snapshot, and advances the ID
// generator past the restored workflow IDs so new workflows never reuse one.
//
// An execution saved as Queued, Running or Canceling was being run by a
// goroutine of the process that wrote the snapshot, and that goroutine is
// gone: it is restored as Canceled, with a reason and a closing history entry
// saying so, instead of staying active forever (which would also block
// deleting it and its workflow).
func (s *MemoryStore) Restore(data []byte) error {
	var snap storeSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	if snap.Workflows == nil {
		snap.Workflows = make(map[string]*WorkflowRecord)
	}
	if snap.Revisions == nil {
		snap.Revisions = make(map[string][]*RevisionRecord)
	}
	if snap.Executions == nil {
		snap.Executions = make(map[string][]*ExecutionRecord)
	}
	if snap.Histories == nil {
		snap.Histories = make(map[string][]HistoryRecord)
	}
	if err := snap.validate(); err != nil {
		return err
	}
	for _, w := range snap.Workflows {
		if w.Tags == nil {
			w.Tags = []Tag{}
		}
	}
	interrupted := snap.cancelInterrupted(time.Now())

	s.mu.Lock()
	defer s.mu.Unlock()
	s.workflows = snap.Workflows
	s.revisions = snap.Revisions
	s.executions = snap.Executions
	s.histories = snap.Histories
	s.subscription = snap.Subscription
	s.interrupted = len(interrupted)
	for id := range s.workflows {
		s.ids.Observe(id)
	}
	for _, e := range interrupted {
		s.logger.Info("execution was interrupted by the restart, marked as Canceled",
			"workflow_id", e.WorkflowID, "execution_id", e.ExecutionID)
	}
	return nil
}

// validate reports what would otherwise surface as a wrong answer at request
// time: records filed under the wrong key, or referring to nothing.
func (snap *storeSnapshot) validate() error {
	for id, w := range snap.Workflows {
		if w == nil || w.ID != id {
			return fmt.Errorf("workflow %q: record is missing or has a different id", id)
		}
	}
	for workflowID, revs := range snap.Revisions {
		if _, ok := snap.Workflows[workflowID]; !ok {
			return fmt.Errorf("revisions of workflow %q: no such workflow", workflowID)
		}
		last := 0
		for _, r := range revs {
			if r == nil || r.WorkflowID != workflowID {
				return fmt.Errorf("revisions of workflow %q: record is missing or belongs to another workflow", workflowID)
			}
			// CreateRevision numbers the next revision after the last one.
			if r.RevisionID <= last {
				return fmt.Errorf("revisions of workflow %q: revision ids are not increasing at %d", workflowID, r.RevisionID)
			}
			last = r.RevisionID
		}
	}
	executionIDs := make(map[string]bool)
	for workflowID, execs := range snap.Executions {
		if _, ok := snap.Workflows[workflowID]; !ok {
			return fmt.Errorf("executions of workflow %q: no such workflow", workflowID)
		}
		for _, e := range execs {
			if e == nil || e.WorkflowID != workflowID {
				return fmt.Errorf("executions of workflow %q: record is missing or belongs to another workflow", workflowID)
			}
			if e.ExecutionID == "" || executionIDs[e.ExecutionID] {
				return fmt.Errorf("executions of workflow %q: execution id %q is empty or duplicated", workflowID, e.ExecutionID)
			}
			executionIDs[e.ExecutionID] = true
			switch e.Status {
			case "Queued", "Running", "Succeeded", "Failed", "Canceling", "Canceled":
			default:
				return fmt.Errorf("execution %q: unknown status %q", e.ExecutionID, e.Status)
			}
		}
	}
	for executionID := range snap.Histories {
		if !executionIDs[executionID] {
			return fmt.Errorf("history of execution %q: no such execution", executionID)
		}
	}
	if sub := snap.Subscription; sub != nil {
		if _, ok := plansByID[sub.PlanID]; !ok {
			return fmt.Errorf("subscription: plan %d not found", sub.PlanID)
		}
	}
	return nil
}

// cancelInterrupted ends every execution that was still active when the
// snapshot was taken, and returns them.
func (snap *storeSnapshot) cancelInterrupted(now time.Time) []*ExecutionRecord {
	var interrupted []*ExecutionRecord
	for _, execs := range snap.Executions {
		for _, e := range execs {
			switch e.Status {
			case "Queued", "Running", "Canceling":
			default:
				continue
			}
			e.Status = "Canceled"
			e.Error = interruptedMessage
			e.CanceledAt = &now
			e.UpdatedAt = now
			snap.Histories[e.ExecutionID] = append(snap.Histories[e.ExecutionID], HistoryRecord{
				WorkflowExecutionID: e.ExecutionID,
				JobID:               e.ExecutionID,
				ThreadID:            "main",
				Type:                string(runbook.EventWorkflowDidCancel),
				CreatedAt:           now,
				Meta:                interruptedMessage,
				StackTrace:          "-",
				Variables:           "{}",
			})
			interrupted = append(interrupted, e)
		}
	}
	return interrupted
}
