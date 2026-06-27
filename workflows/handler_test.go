package workflows_test

import (
	"testing"
	"time"

	wf "github.com/sacloud/sacloud-sdk-go/api/workflows"
	v1 "github.com/sacloud/sacloud-sdk-go/api/workflows/apis/v1"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	"github.com/sacloud/sakumock/workflows"
)

const testRunbook = `
meta:
  description: test
steps:
  done:
    return: "hello"
`

func newClient(t *testing.T, serverURL string) *v1.Client {
	t.Helper()
	var sa saclient.Client
	if err := sa.SetEnviron([]string{
		"SAKURA_ENDPOINTS_WORKFLOWS=" + serverURL,
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
	}); err != nil {
		t.Fatal(err)
	}
	client, err := wf.NewClient(&sa)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestWorkflowLifecycle(t *testing.T) {
	srv := workflows.NewTestServer(workflows.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newClient(t, srv.TestURL())
	workflowOp := wf.NewWorkflowOp(client)

	created, err := workflowOp.Create(ctx, v1.CreateWorkflowReq{
		Name:    "test-workflow",
		Runbook: testRunbook,
		Publish: true,
		Logging: true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty workflow ID")
	}
	if created.Name != "test-workflow" {
		t.Errorf("name = %q, want test-workflow", created.Name)
	}
	id := created.ID

	got, err := workflowOp.Read(ctx, id)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.ID != id || got.Name != "test-workflow" {
		t.Errorf("unexpected read result: %+v", got)
	}
	if !got.Publish || !got.Logging {
		t.Errorf("expected Publish=true Logging=true, got %v %v", got.Publish, got.Logging)
	}

	list, err := workflowOp.List(ctx, v1.ListWorkflowParams{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if list.Total != 1 {
		t.Errorf("total = %d, want 1", list.Total)
	}

	updated, err := workflowOp.Update(ctx, id, v1.UpdateWorkflowReq{
		Name:        v1.NewOptString("updated-workflow"),
		Description: v1.NewOptString("updated description"),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "updated-workflow" {
		t.Errorf("updated name = %q, want updated-workflow", updated.Name)
	}

	if err := workflowOp.Delete(ctx, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := workflowOp.Read(ctx, id); err == nil {
		t.Error("expected read after delete to fail")
	}
}

func TestRevisionLifecycle(t *testing.T) {
	srv := workflows.NewTestServer(workflows.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newClient(t, srv.TestURL())
	workflowOp := wf.NewWorkflowOp(client)
	revisionOp := wf.NewRevisionOp(client)

	created, err := workflowOp.Create(ctx, v1.CreateWorkflowReq{
		Name:    "rev-test",
		Runbook: testRunbook,
		Publish: true,
		Logging: true,
	})
	if err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	wfID := created.ID

	revisions, err := revisionOp.List(ctx, v1.ListWorkflowRevisionsParams{ID: wfID})
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if revisions.Total != 1 {
		t.Errorf("initial revision count = %d, want 1", revisions.Total)
	}

	rev, err := revisionOp.Read(ctx, wfID, 1)
	if err != nil {
		t.Fatalf("read revision 1: %v", err)
	}
	if rev.Runbook != testRunbook {
		t.Errorf("revision runbook mismatch")
	}

	rev2, err := revisionOp.Create(ctx, wfID, v1.CreateWorkflowRevisionReq{
		Runbook:       testRunbook,
		RevisionAlias: v1.NewOptString("v2"),
	})
	if err != nil {
		t.Fatalf("create revision: %v", err)
	}
	if rev2.RevisionId != 2 {
		t.Errorf("revision id = %d, want 2", rev2.RevisionId)
	}
	if rev2.RevisionAlias != v1.NewOptString("v2") {
		t.Errorf("revision alias = %v, want v2", rev2.RevisionAlias)
	}

	updatedRev, err := revisionOp.UpdateAlias(ctx, wfID, 1, v1.UpdateWorkflowRevisionAliasReq{
		RevisionAlias: "v1",
	})
	if err != nil {
		t.Fatalf("update alias: %v", err)
	}
	if updatedRev.RevisionAlias != v1.NewOptString("v1") {
		t.Errorf("updated alias = %v, want v1", updatedRev.RevisionAlias)
	}

	if err := revisionOp.DeleteAlias(ctx, wfID, 1); err != nil {
		t.Fatalf("delete alias: %v", err)
	}

	if err := workflowOp.Delete(ctx, wfID); err != nil {
		t.Fatalf("delete workflow: %v", err)
	}
}

func TestExecutionLifecycle(t *testing.T) {
	srv := workflows.NewTestServer(workflows.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newClient(t, srv.TestURL())
	workflowOp := wf.NewWorkflowOp(client)
	executionOp := wf.NewExecutionOp(client)

	created, err := workflowOp.Create(ctx, v1.CreateWorkflowReq{
		Name:    "exec-test",
		Runbook: testRunbook,
		Publish: true,
		Logging: true,
	})
	if err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	wfID := created.ID

	exec, err := executionOp.Create(ctx, wfID, v1.OptCreateExecutionReq{})
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}
	if exec.ExecutionId == "" {
		t.Fatal("expected non-empty execution ID")
	}
	if exec.Status != v1.CreateExecutionCreatedExecutionStatusSucceeded {
		t.Errorf("status = %q, want Succeeded", exec.Status)
	}

	gotExec, err := executionOp.Read(ctx, wfID, exec.ExecutionId)
	if err != nil {
		t.Fatalf("read execution: %v", err)
	}
	if gotExec.ExecutionId != exec.ExecutionId {
		t.Errorf("execution id mismatch")
	}

	execList, err := executionOp.List(ctx, v1.ListExecutionParams{ID: wfID})
	if err != nil {
		t.Fatalf("list executions: %v", err)
	}
	if execList.Total != 1 {
		t.Errorf("execution count = %d, want 1", execList.Total)
	}

	if err := executionOp.Delete(ctx, wfID, exec.ExecutionId); err != nil {
		t.Fatalf("delete execution: %v", err)
	}

	if err := workflowOp.Delete(ctx, wfID); err != nil {
		t.Fatalf("delete workflow: %v", err)
	}
}

func TestExecutionDataPlane(t *testing.T) {
	runbookYAML := `
meta:
  description: data plane test
args:
  x:
    type: number
steps:
  done:
    return: ${args.x * 2}
`
	srv := workflows.NewTestServer(workflows.Config{EnableDataPlane: true})
	defer srv.Close()
	ctx := t.Context()
	client := newClient(t, srv.TestURL())
	workflowOp := wf.NewWorkflowOp(client)
	executionOp := wf.NewExecutionOp(client)

	created, err := workflowOp.Create(ctx, v1.CreateWorkflowReq{
		Name:    "dp-test",
		Runbook: runbookYAML,
		Publish: true,
		Logging: true,
	})
	if err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	wfID := created.ID

	exec, err := executionOp.Create(ctx, wfID, v1.OptCreateExecutionReq{
		Set: true,
		Value: v1.CreateExecutionReq{
			Args: v1.NewOptString(`{"x": 21}`),
		},
	})
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}

	// Wait for async execution to complete
	var gotExec *v1.GetExecutionOKExecution
	for range 50 {
		time.Sleep(10 * time.Millisecond)
		gotExec, err = executionOp.Read(ctx, wfID, exec.ExecutionId)
		if err != nil {
			t.Fatalf("read execution: %v", err)
		}
		if gotExec.Status == v1.GetExecutionOKExecutionStatusSucceeded {
			break
		}
	}
	if gotExec.Status != v1.GetExecutionOKExecutionStatusSucceeded {
		t.Fatalf("execution status = %q, want Succeeded", gotExec.Status)
	}
	if gotExec.Result != "42" {
		t.Errorf("result = %q, want 42", gotExec.Result)
	}

	if err := executionOp.Delete(ctx, wfID, exec.ExecutionId); err != nil {
		t.Fatalf("delete execution: %v", err)
	}
	if err := workflowOp.Delete(ctx, wfID); err != nil {
		t.Fatalf("delete workflow: %v", err)
	}
}

func TestSubscription(t *testing.T) {
	srv := workflows.NewTestServer(workflows.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newClient(t, srv.TestURL())
	subscriptionOp := wf.NewSubscriptionOp(client)

	plans, err := subscriptionOp.ListPlans(ctx)
	if err != nil {
		t.Fatalf("list plans: %v", err)
	}
	if len(plans.Plans) == 0 {
		t.Fatal("expected at least one plan")
	}

	if err := subscriptionOp.Create(ctx, v1.CreateSubscriptionReq{PlanId: 1}); err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	sub, err := subscriptionOp.Read(ctx)
	if err != nil {
		t.Fatalf("read subscription: %v", err)
	}
	if sub.CurrentPlan.Value.PlanId != 1 {
		t.Errorf("plan id = %d, want 1", sub.CurrentPlan.Value.PlanId)
	}

	if err := subscriptionOp.Delete(ctx); err != nil {
		t.Fatalf("delete subscription: %v", err)
	}
}
