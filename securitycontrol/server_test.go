package securitycontrol_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	securitycontrol "github.com/sacloud/sacloud-sdk-go/api/security-control"
	v1 "github.com/sacloud/sacloud-sdk-go/api/security-control/apis/v1"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	sc "github.com/sacloud/sakumock/securitycontrol"
)

func newTestClient(t *testing.T, serverURL string) *v1.Client {
	t.Helper()
	var sa saclient.Client
	if err := sa.SetEnviron([]string{
		"SAKURA_ENDPOINTS_SECURITY_CONTROL=" + serverURL,
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
	}); err != nil {
		t.Fatal(err)
	}
	client, err := securitycontrol.NewClient(&sa)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func newServer(t *testing.T, cfg sc.Config) (*sc.Server, *v1.Client) {
	t.Helper()
	srv := sc.NewTestServer(cfg)
	t.Cleanup(func() {
		srv.Close()
		if v := srv.SpecViolations(); len(v) != 0 {
			t.Errorf("spec violations recorded: %+v", v)
		}
	})
	return srv, newTestClient(t, srv.TestURL())
}

// statusOf extracts the HTTP status carried by an SDK error ("" when none).
func statusOf(err error) string {
	if err == nil {
		return ""
	}
	for _, code := range []string{"400", "404", "409"} {
		if strings.Contains(err.Error(), code) {
			return code
		}
	}
	return err.Error()
}

func TestActivationLifecycle(t *testing.T) {
	_, client := newServer(t, sc.Config{})
	ctx := t.Context()
	op := securitycontrol.NewActivationOp(client)

	if _, err := op.Read(ctx); !saclient.IsNotFoundError(err) {
		t.Fatalf("read before activation: want 404, got %v", err)
	}
	if _, err := op.Update(ctx, "111111111111", false); !saclient.IsNotFoundError(err) {
		t.Fatalf("update before activation: want 404, got %v", err)
	}

	created, err := op.Create(ctx, "111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if !created.IsActive || created.ServicePrincipalId != "111111111111" || created.AutomatedActionLimit != sc.DefaultAutomatedActionLimit {
		t.Fatalf("unexpected activation: %+v", created)
	}
	if _, err := op.Create(ctx, "111111111111"); !saclient.IsConflictError(err) {
		t.Fatalf("second activation: want 409, got %v", err)
	}

	updated, err := op.Update(ctx, "222222222222", false)
	if err != nil {
		t.Fatal(err)
	}
	if updated.IsActive || updated.ServicePrincipalId != "222222222222" {
		t.Fatalf("unexpected updated activation: %+v", updated)
	}
	read, err := op.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if *read != *updated {
		t.Fatalf("read %+v, want %+v", read, updated)
	}
}

func simpleNotificationAction(name string) *v1.AutomatedActionInput {
	return &v1.AutomatedActionInput{
		Name:        name,
		Description: v1.NewOptString("example automated action"),
		IsEnabled:   true,
		Action: v1.ActionDefinition{
			OneOf: v1.ActionDefinitionSum{
				Type: v1.ActionDefinitionSimpleNotificationActionDefinitionSum,
				ActionDefinitionSimpleNotification: v1.ActionDefinitionSimpleNotification{
					ActionType: v1.ActionDefinitionSimpleNotificationActionType("simpleNotification"),
					ActionParameter: v1.SakuraSimpleNotification{
						ServicePrincipalId:  "111111111111",
						NotificationGroupId: "111111111112",
					},
				},
			},
		},
		ExecutionCondition: `event.evaluationResult.status == "REJECTED"`,
	}
}

func TestAutomatedActionLifecycle(t *testing.T) {
	_, client := newServer(t, sc.Config{})
	ctx := t.Context()
	op := securitycontrol.NewAutomatedActionsOp(client)

	created, err := op.Create(ctx, simpleNotificationAction("notify"))
	if err != nil {
		t.Fatal(err)
	}
	id := string(created.AutomatedActionId)
	if id == "" || created.CreatedAt.IsZero() || created.Description.Value != "example automated action" {
		t.Fatalf("unexpected created action: %+v", created)
	}
	param := created.Action.OneOf.ActionDefinitionSimpleNotification.ActionParameter
	if created.Action.OneOf.Type != v1.ActionDefinitionSimpleNotificationActionDefinitionSum || param.NotificationGroupId != "111111111112" {
		t.Fatalf("action not round-tripped: %+v", created.Action)
	}

	upd := &v1.AutomatedActionInput{
		Name:      "run-workflow",
		IsEnabled: false,
		Action: v1.ActionDefinition{
			OneOf: v1.ActionDefinitionSum{
				Type: v1.ActionDefinitionWorkflowsActionDefinitionSum,
				ActionDefinitionWorkflows: v1.ActionDefinitionWorkflows{
					ActionType: v1.ActionDefinitionWorkflowsActionType("workflows"),
					ActionParameter: v1.SakuraWorkflows{
						ServicePrincipalId: "111111111111",
						WorkflowId:         "333333333333",
						Args:               v1.NewOptString(`{"server_id": 12345}`),
					},
				},
			},
		},
		ExecutionCondition: `event.evaluationResult.status == "FAILED"`,
	}
	updated, err := op.Update(ctx, id, upd)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "run-workflow" || updated.IsEnabled || updated.Description.Set || !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Fatalf("unexpected updated action: %+v", updated)
	}
	if wf := updated.Action.OneOf.ActionDefinitionWorkflows.ActionParameter; wf.WorkflowId != "333333333333" || wf.Args.Value != `{"server_id": 12345}` {
		t.Fatalf("workflows action not round-tripped: %+v", updated.Action)
	}

	read, err := op.Read(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if read.Name != "run-workflow" {
		t.Fatalf("unexpected read action: %+v", read)
	}
	list, err := op.List(ctx, v1.AutomatedActionsListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.TotalSize != 1 || list.NextPageToken.Set || list.PrevPageToken.Set {
		t.Fatalf("unexpected list: %+v", list)
	}

	if err := op.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := op.Read(ctx, id); !saclient.IsNotFoundError(err) {
		t.Fatalf("read after delete: want 404, got %v", err)
	}
	if _, err := op.Update(ctx, id, upd); !saclient.IsNotFoundError(err) {
		t.Fatalf("update after delete: want 404, got %v", err)
	}
	// The spec declares no 404 for delete.
	if err := op.Delete(ctx, id); err != nil {
		t.Fatalf("delete of a deleted action: %v", err)
	}
}

func TestAutomatedActionLimit(t *testing.T) {
	_, client := newServer(t, sc.Config{AutomatedActionLimit: 2})
	ctx := t.Context()
	op := securitycontrol.NewAutomatedActionsOp(client)
	for i := range 2 {
		if _, err := op.Create(ctx, simpleNotificationAction(fmt.Sprintf("a%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := op.Create(ctx, simpleNotificationAction("over")); statusOf(err) != "400" {
		t.Fatalf("create over the limit: want 400, got %v", err)
	}
	act, err := securitycontrol.NewActivationOp(client).Create(ctx, "111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if act.AutomatedActionLimit != 2 {
		t.Fatalf("automatedActionLimit = %d, want 2", act.AutomatedActionLimit)
	}
}

func TestAutomatedActionPagination(t *testing.T) {
	_, client := newServer(t, sc.Config{AutomatedActionLimit: 5})
	ctx := t.Context()
	op := securitycontrol.NewAutomatedActionsOp(client)
	var ids []string
	for i := range 5 {
		a, err := op.Create(ctx, simpleNotificationAction(fmt.Sprintf("a%d", i)))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, string(a.AutomatedActionId))
	}

	var got []string
	params := v1.AutomatedActionsListParams{PageSize: v1.NewOptInt(2)}
	var pages int
	for {
		page, err := op.List(ctx, params)
		if err != nil {
			t.Fatal(err)
		}
		pages++
		for _, a := range page.Items {
			got = append(got, string(a.AutomatedActionId))
		}
		if pages > 1 && !page.PrevPageToken.Set {
			t.Fatalf("page %d has no prevPageToken", pages)
		}
		if !page.NextPageToken.Set {
			break
		}
		params.Next = page.NextPageToken
	}
	if pages != 3 || strings.Join(got, ",") != strings.Join(ids, ",") {
		t.Fatalf("paged %d pages %v, want 3 pages %v", pages, got, ids)
	}

	// The last page's prev token leads back to the second page.
	last, err := op.List(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	back, err := op.List(ctx, v1.AutomatedActionsListParams{PageSize: v1.NewOptInt(2), Prev: last.PrevPageToken})
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Items) != 2 || string(back.Items[0].AutomatedActionId) != ids[2] {
		t.Fatalf("prev page: %+v", back.Items)
	}

	for name, p := range map[string]v1.AutomatedActionsListParams{
		"next and prev":      {Next: params.Next, Prev: params.Next},
		"page_size mismatch": {PageSize: v1.NewOptInt(3), Next: params.Next},
		"broken token":       {Next: v1.NewOptString("!!")},
		"trailing garbage":   {Next: v1.NewOptString("MDo1MGp1bms")}, // "0:50junk"
		"zero page_size":     {PageSize: v1.NewOptInt(0)},
	} {
		if _, err := op.List(ctx, p); statusOf(err) != "400" {
			t.Errorf("%s: want 400, got %v", name, err)
		}
	}
}

func TestEvaluationRules(t *testing.T) {
	_, client := newServer(t, sc.Config{})
	ctx := t.Context()
	op := securitycontrol.NewEvaluationRulesOp(client)

	list, err := op.List(ctx, v1.EvaluationRulesListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 14 || list.NextPageToken.Set {
		t.Fatalf("expected all 14 rules on one page, got %d (next=%v)", len(list.Items), list.NextPageToken)
	}
	for _, r := range list.Items {
		if r.IsEnabled || r.Description.Value == "" {
			t.Errorf("unexpected initial rule: %+v", r)
		}
	}

	in := securitycontrol.SetupEvaluationRuleInput(&securitycontrol.EvaluationRuleInputParams{
		ID:                 "server-no-public-ip",
		ServicePrincipalID: "111111111111",
		Targets:            []string{"is1b", "tk1b"},
		Enabled:            true,
	})
	updated, err := op.Update(ctx, "server-no-public-ip", in)
	if err != nil {
		t.Fatal(err)
	}
	zoned := updated.Rule.OneOf.ServerNoPublicIP.Parameter.Value
	if !updated.IsEnabled || zoned.ServicePrincipalId.Value != "111111111111" || strings.Join(zoned.Zones, ",") != "is1b,tk1b" {
		t.Fatalf("unexpected updated rule: %+v", updated)
	}
	read, err := op.Read(ctx, "server-no-public-ip")
	if err != nil {
		t.Fatal(err)
	}
	if !read.IsEnabled || strings.Join(read.Rule.OneOf.ServerNoPublicIP.Parameter.Value.Zones, ",") != "is1b,tk1b" {
		t.Fatalf("unexpected read rule: %+v", read)
	}

	// The rule in the body must be the one in the path.
	if _, err := op.Update(ctx, "disk-encryption-enabled", in); statusOf(err) != "400" {
		t.Fatalf("mismatched rule: want 400, got %v", err)
	}
}

func TestUnknownEvaluationRule(t *testing.T) {
	srv, _ := newServer(t, sc.Config{})
	resp, err := http.Get(srv.TestURL() + "/evaluation_rules/no-such-rule")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestErrorShape(t *testing.T) {
	srv, _ := newServer(t, sc.Config{})
	resp, err := http.Post(srv.TestURL()+"/activation", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", resp.StatusCode, body)
	}
	var p struct {
		Title  string `json:"title"`
		Status int    `json:"status"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatal(err)
	}
	if p.Status != 400 || p.Detail != "invalid" || !strings.Contains(p.Title, "servicePrincipalId") {
		t.Fatalf("unexpected problem body: %s", body)
	}
}

func TestDiscriminatedUnionValidation(t *testing.T) {
	srv, _ := newServer(t, sc.Config{})
	cases := []struct {
		name, method, path, body, want string
	}{
		{"unknown actionType", "POST", "/automated_actions",
			`{"name":"a","executionCondition":"true","isEnabled":true,"action":{"actionType":"email"}}`,
			"action.actionType must be one of simpleNotification, workflows"},
		{"workflows without workflowId", "POST", "/automated_actions",
			`{"name":"a","executionCondition":"true","isEnabled":true,"action":{"actionType":"workflows","actionParameter":{"servicePrincipalId":"1"}}}`,
			"action.actionParameter.workflowId is required"},
		{"rule parameter of the wrong type", "PUT", "/evaluation_rules/server-no-public-ip",
			`{"isEnabled":true,"rule":{"evaluationRuleId":"server-no-public-ip","parameter":{"zones":"is1a"}}}`,
			"rule.parameter.zones must be an array"},
		{"rule without evaluationRuleId", "PUT", "/evaluation_rules/server-no-public-ip",
			`{"isEnabled":true,"rule":{}}`,
			"rule.evaluationRuleId is required"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), c.method, srv.TestURL()+c.path, strings.NewReader(c.body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			var p struct {
				Title string `json:"title"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusBadRequest || p.Title != c.want {
				t.Fatalf("got %d %q, want 400 %q", resp.StatusCode, p.Title, c.want)
			}
		})
	}
}
