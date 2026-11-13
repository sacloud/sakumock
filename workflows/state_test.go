package workflows_test

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	wf "github.com/sacloud/sacloud-sdk-go/api/workflows"
	v1 "github.com/sacloud/sacloud-sdk-go/api/workflows/apis/v1"

	"github.com/sacloud/sakumock/core"
	"github.com/sacloud/sakumock/core/coretest"
	"github.com/sacloud/sakumock/workflows"
)

const doubleRunbook = `
meta:
  description: state test
args:
  x:
    type: number
steps:
  done:
    return: ${args.x * 2}
`

// sleepRunbook keeps its execution running until it is canceled.
const sleepRunbook = `
meta:
  description: state test (never finishes by itself)
steps:
  wait:
    call: sys.sleep
    args:
      seconds: 600
  done:
    return: "unreachable"
`

const interruptedReason = "sakumock was restarted"

func createWorkflow(t *testing.T, url, runbook string) string {
	t.Helper()
	created, err := wf.NewWorkflowOp(newClient(t, url)).Create(t.Context(), v1.CreateWorkflowReq{
		Name:    "persisted",
		Runbook: runbook,
		Publish: true,
		Logging: true,
	})
	if err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	return created.ID
}

// waitExecution polls the execution until its status is want.
func waitExecution(t *testing.T, url, workflowID, executionID string, want v1.ReadExecutionOKExecutionStatus) *v1.ReadExecutionOKExecution {
	t.Helper()
	op := wf.NewExecutionOp(newClient(t, url))
	var got *v1.ReadExecutionOKExecution
	for range 200 {
		var err error
		got, err = op.Read(t.Context(), workflowID, executionID)
		if err != nil {
			t.Fatalf("read execution: %v", err)
		}
		if got.Status == want {
			return got
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("execution status = %q, want %q", got.Status, want)
	return nil
}

// waitStateFile polls the state file until it contains want.
func waitStateFile(t *testing.T, path, want string) []byte {
	t.Helper()
	for range 200 {
		if data, err := os.ReadFile(path); err == nil && strings.Contains(string(data), want) {
			return data
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("state file %s never contained %q", path, want)
	return nil
}

func startStateServer(t *testing.T, dir string) (*workflows.Server, *httptest.Server) {
	t.Helper()
	srv, err := workflows.Config{EnableDataPlane: true}.NewServer(core.ServerOptions{StateDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	return srv.(*workflows.Server), httptest.NewServer(srv)
}

// assertInterrupted checks that the execution ended as Canceled with the
// restart as its reason, in its record and as the last entry of its history.
func assertInterrupted(t *testing.T, url, workflowID, executionID string) {
	t.Helper()
	ctx := t.Context()
	op := wf.NewExecutionOp(newClient(t, url))
	got, err := op.Read(ctx, workflowID, executionID)
	if err != nil {
		t.Fatalf("read execution: %v", err)
	}
	if got.Status != v1.ReadExecutionOKExecutionStatusCanceled || !strings.Contains(got.Error, interruptedReason) {
		t.Errorf("interrupted execution = status %q error %q, want Canceled because %q", got.Status, got.Error, interruptedReason)
	}
	history, err := op.ListHistory(ctx, v1.ListExecutionHistoriesParams{ID: workflowID, ExecutionId: executionID})
	if err != nil {
		t.Fatalf("list history: %v", err)
	}
	if n := len(history.Histories); n == 0 || history.Histories[n-1].Type != "workflowDidCanceled" {
		t.Errorf("history of the interrupted execution = %+v, want it to end with workflowDidCanceled", history.Histories)
	}
}

func TestStateSurvivesRestart(t *testing.T) {
	var workflowID, executionID string
	var historyTotal int

	coretest.StateRestart(t, workflows.Config{EnableDataPlane: true},
		func(t *testing.T, url string) {
			ctx := t.Context()
			client := newClient(t, url)
			created, err := wf.NewWorkflowOp(client).Create(ctx, v1.CreateWorkflowReq{
				Name:        "persisted",
				Description: v1.NewOptString("desc"),
				Runbook:     doubleRunbook,
				Publish:     true,
				Logging:     true,
				Tags:        []v1.CreateWorkflowReqTagsItem{{Name: "a"}},
			})
			if err != nil {
				t.Fatalf("create workflow: %v", err)
			}
			workflowID = created.ID
			if _, err := wf.NewRevisionOp(client).Create(ctx, workflowID, v1.CreateRevisionReq{
				Runbook:       doubleRunbook,
				RevisionAlias: v1.NewOptString("v2"),
			}); err != nil {
				t.Fatalf("create revision: %v", err)
			}

			executionOp := wf.NewExecutionOp(client)
			exec, err := executionOp.Create(ctx, workflowID, v1.OptCreateExecutionReq{
				Set:   true,
				Value: v1.CreateExecutionReq{Args: v1.NewOptString(`{"x": 21}`)},
			})
			if err != nil {
				t.Fatalf("create execution: %v", err)
			}
			executionID = exec.ExecutionId
			waitExecution(t, url, workflowID, executionID, v1.ReadExecutionOKExecutionStatusSucceeded)
			history, err := executionOp.ListHistory(ctx, v1.ListExecutionHistoriesParams{ID: workflowID, ExecutionId: executionID})
			if err != nil {
				t.Fatalf("list history: %v", err)
			}
			historyTotal = history.Total

			if err := wf.NewSubscriptionOp(client).Create(ctx, v1.CreateSubscriptionReq{PlanId: 1}); err != nil {
				t.Fatalf("create subscription: %v", err)
			}

			// A deleted workflow must not come back.
			deleted := createWorkflow(t, url, doubleRunbook)
			if err := wf.NewWorkflowOp(client).Delete(ctx, deleted); err != nil {
				t.Fatalf("delete workflow: %v", err)
			}
		},
		func(t *testing.T, url string) {
			ctx := t.Context()
			client := newClient(t, url)
			list, err := wf.NewWorkflowOp(client).List(ctx, v1.ListWorkflowsParams{})
			if err != nil {
				t.Fatalf("list workflows: %v", err)
			}
			if list.Total != 1 {
				t.Fatalf("got %d workflows after restart, want 1", list.Total)
			}
			got, err := wf.NewWorkflowOp(client).Read(ctx, workflowID)
			if err != nil {
				t.Fatalf("read workflow: %v", err)
			}
			if got.Name != "persisted" || !got.Publish || len(got.Tags) != 1 || got.Tags[0].Name != "a" {
				t.Errorf("restored workflow = %+v", got)
			}

			revisionOp := wf.NewRevisionOp(client)
			revisions, err := revisionOp.List(ctx, v1.ListRevisionsParams{ID: workflowID})
			if err != nil {
				t.Fatalf("list revisions: %v", err)
			}
			if revisions.Total != 2 {
				t.Errorf("got %d revisions after restart, want 2", revisions.Total)
			}
			rev2, err := revisionOp.Read(ctx, workflowID, 2)
			if err != nil {
				t.Fatalf("read revision 2: %v", err)
			}
			if rev2.Runbook != doubleRunbook || rev2.RevisionAlias != v1.NewOptString("v2") {
				t.Errorf("restored revision 2 = %+v", rev2)
			}

			executionOp := wf.NewExecutionOp(client)
			exec, err := executionOp.Read(ctx, workflowID, executionID)
			if err != nil {
				t.Fatalf("read execution: %v", err)
			}
			if exec.Status != v1.ReadExecutionOKExecutionStatusSucceeded || exec.Result != "42" {
				t.Errorf("restored execution = status %q result %q, want Succeeded 42", exec.Status, exec.Result)
			}
			history, err := executionOp.ListHistory(ctx, v1.ListExecutionHistoriesParams{ID: workflowID, ExecutionId: executionID})
			if err != nil {
				t.Fatalf("list history: %v", err)
			}
			if history.Total != historyTotal || historyTotal == 0 {
				t.Errorf("restored history has %d entries, want %d (non-zero)", history.Total, historyTotal)
			}

			sub, err := wf.NewSubscriptionOp(client).Read(ctx)
			if err != nil {
				t.Fatalf("read subscription: %v", err)
			}
			if plan, ok := sub.CurrentPlan.Get(); !ok || plan.PlanId != 1 {
				t.Errorf("restored subscription = %+v, want plan 1", sub.CurrentPlan)
			}
		})
}

// A workflow created after a restart never reuses a restored ID.
func TestStateRestoredIDsAreNotReused(t *testing.T) {
	var workflowID string
	coretest.StateRestart(t, workflows.Config{},
		func(t *testing.T, url string) { workflowID = createWorkflow(t, url, testRunbook) },
		func(t *testing.T, url string) {
			if id := createWorkflow(t, url, testRunbook); id == workflowID {
				t.Errorf("new workflow reused the restored ID %s", id)
			}
		}, true)
}

// An execution finishes in a goroutine, outside of any request: its outcome
// must reach the state file without waiting for the next request or Close.
func TestStateSavesFinishedExecution(t *testing.T) {
	dir := t.TempDir()
	srv, ts := startStateServer(t, dir)
	defer closeAndCheck(t, srv)
	defer ts.Close()

	workflowID := createWorkflow(t, ts.URL, doubleRunbook)
	exec, err := wf.NewExecutionOp(newClient(t, ts.URL)).Create(t.Context(), workflowID, v1.OptCreateExecutionReq{
		Set:   true,
		Value: v1.CreateExecutionReq{Args: v1.NewOptString(`{"x": 21}`)},
	})
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}
	waitExecution(t, ts.URL, workflowID, exec.ExecutionId, v1.ReadExecutionOKExecutionStatusSucceeded)
	waitStateFile(t, filepath.Join(dir, "workflows.json"), `"status": "Succeeded"`)
}

// An execution still running at shutdown is canceled by it, and comes back
// that way: terminal, with the restart as its reason, and deletable.
func TestStateExecutionRunningAtShutdown(t *testing.T) {
	var workflowID, executionID string
	coretest.StateRestart(t, workflows.Config{EnableDataPlane: true},
		func(t *testing.T, url string) {
			workflowID = createWorkflow(t, url, sleepRunbook)
			exec, err := wf.NewExecutionOp(newClient(t, url)).Create(t.Context(), workflowID, v1.OptCreateExecutionReq{})
			if err != nil {
				t.Fatalf("create execution: %v", err)
			}
			executionID = exec.ExecutionId
			waitExecution(t, url, workflowID, executionID, v1.ReadExecutionOKExecutionStatusRunning)
		},
		func(t *testing.T, url string) {
			assertInterrupted(t, url, workflowID, executionID)
		})
}

// A process killed mid-execution leaves an active execution in the state file
// with no goroutine to finish it: the restart ends it as Canceled and saves
// that at once.
func TestStateInterruptedExecutionIsCanceledOnRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "workflows.json")

	srv, ts := startStateServer(t, dir)
	workflowID := createWorkflow(t, ts.URL, sleepRunbook)
	exec, err := wf.NewExecutionOp(newClient(t, ts.URL)).Create(t.Context(), workflowID, v1.OptCreateExecutionReq{})
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}
	executionID := exec.ExecutionId
	waitExecution(t, ts.URL, workflowID, executionID, v1.ReadExecutionOKExecutionStatusRunning)
	// What a killed process leaves behind: the state as of the last request.
	killed := waitStateFile(t, path, executionID)
	if strings.Contains(string(killed), `"status": "Canceled"`) {
		t.Fatalf("the state of the running server already holds a canceled execution:\n%s", killed)
	}
	ts.Close()
	closeAndCheck(t, srv)
	if err := os.WriteFile(path, killed, 0o600); err != nil {
		t.Fatal(err)
	}

	srv, ts = startStateServer(t, dir)
	defer closeAndCheck(t, srv)
	defer ts.Close()
	assertInterrupted(t, ts.URL, workflowID, executionID)
	if saved, err := os.ReadFile(path); err != nil || !strings.Contains(string(saved), interruptedReason) {
		t.Errorf("the canceled execution was not saved at startup (%v):\n%s", err, saved)
	}

	// Nothing keeps the execution or its workflow from being deleted.
	if err := wf.NewExecutionOp(newClient(t, ts.URL)).Delete(t.Context(), workflowID, executionID); err != nil {
		t.Errorf("delete the interrupted execution: %v", err)
	}
	if err := wf.NewWorkflowOp(newClient(t, ts.URL)).Delete(t.Context(), workflowID); err != nil {
		t.Errorf("delete its workflow: %v", err)
	}
}
