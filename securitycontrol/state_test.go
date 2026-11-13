package securitycontrol_test

import (
	"strings"
	"testing"

	securitycontrol "github.com/sacloud/sacloud-sdk-go/api/security-control"
	v1 "github.com/sacloud/sacloud-sdk-go/api/security-control/apis/v1"

	"github.com/sacloud/sakumock/core/coretest"
	sc "github.com/sacloud/sakumock/securitycontrol"
)

func TestStateSurvivesRestart(t *testing.T) {
	var actionID string

	coretest.StateRestart(t, sc.Config{},
		func(t *testing.T, url string) {
			ctx := t.Context()
			client := newTestClient(t, url)
			activationOp := securitycontrol.NewActivationOp(client)
			if _, err := activationOp.Create(ctx, "111111111111"); err != nil {
				t.Fatal(err)
			}
			if _, err := activationOp.Update(ctx, "222222222222", false); err != nil {
				t.Fatal(err)
			}
			created, err := securitycontrol.NewAutomatedActionsOp(client).Create(ctx, simpleNotificationAction("notify"))
			if err != nil {
				t.Fatal(err)
			}
			actionID = string(created.AutomatedActionId)
			in := securitycontrol.SetupEvaluationRuleInput(&securitycontrol.EvaluationRuleInputParams{
				ID:                 "server-no-public-ip",
				ServicePrincipalID: "111111111111",
				Targets:            []string{"is1b", "tk1b"},
				Enabled:            true,
			})
			if _, err := securitycontrol.NewEvaluationRulesOp(client).Update(ctx, "server-no-public-ip", in); err != nil {
				t.Fatal(err)
			}
		},
		func(t *testing.T, url string) {
			ctx := t.Context()
			client := newTestClient(t, url)
			activation, err := securitycontrol.NewActivationOp(client).Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if activation.IsActive || activation.ServicePrincipalId != "222222222222" {
				t.Errorf("restored activation = %+v", activation)
			}

			action, err := securitycontrol.NewAutomatedActionsOp(client).Read(ctx, actionID)
			if err != nil {
				t.Fatal(err)
			}
			param := action.Action.OneOf.ActionDefinitionSimpleNotification.ActionParameter
			if action.Name != "notify" || !action.IsEnabled || action.Description.Value != "example automated action" ||
				action.CreatedAt.IsZero() || param.NotificationGroupId != "111111111112" {
				t.Errorf("restored automated action = %+v", action)
			}

			// The catalog rules are restored in place, not added to the
			// ones a fresh store starts with.
			rules, err := securitycontrol.NewEvaluationRulesOp(client).List(ctx, v1.EvaluationRulesListParams{})
			if err != nil {
				t.Fatal(err)
			}
			if len(rules.Items) != 14 {
				t.Fatalf("got %d evaluation rules after restart, want 14", len(rules.Items))
			}
			enabled := 0
			for _, r := range rules.Items {
				if r.IsEnabled {
					enabled++
				}
			}
			if enabled != 1 {
				t.Errorf("got %d enabled rules after restart, want 1", enabled)
			}
			rule, err := securitycontrol.NewEvaluationRulesOp(client).Read(ctx, "server-no-public-ip")
			if err != nil {
				t.Fatal(err)
			}
			zoned := rule.Rule.OneOf.ServerNoPublicIP.Parameter.Value
			if !rule.IsEnabled || zoned.ServicePrincipalId.Value != "111111111111" || strings.Join(zoned.Zones, ",") != "is1b,tk1b" {
				t.Errorf("restored evaluation rule = %+v", rule)
			}
		})
}

// An automated action created after a restart never reuses a restored ID.
func TestStateRestoredIDsAreNotReused(t *testing.T) {
	var actionID string
	create := func(t *testing.T, url string) string {
		t.Helper()
		created, err := securitycontrol.NewAutomatedActionsOp(newTestClient(t, url)).Create(t.Context(), simpleNotificationAction("notify"))
		if err != nil {
			t.Fatal(err)
		}
		return string(created.AutomatedActionId)
	}
	coretest.StateRestart(t, sc.Config{},
		func(t *testing.T, url string) { actionID = create(t, url) },
		func(t *testing.T, url string) {
			if id := create(t, url); id == actionID {
				t.Errorf("new automated action reused the restored ID %s", id)
			}
		}, true)
}
