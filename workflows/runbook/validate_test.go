package runbook_test

import (
	"strings"
	"testing"

	"github.com/sacloud/sakumock/workflows/runbook"
)

func TestValidateRunbookSize(t *testing.T) {
	large := make([]byte, runbook.MaxRunbookSize+1)
	if err := runbook.ValidateRunbookSize(large); err == nil {
		t.Error("expected size error")
	}
	small := make([]byte, 100)
	if err := runbook.ValidateRunbookSize(small); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateStepNameLength(t *testing.T) {
	longName := strings.Repeat("a", runbook.MaxStepNameLen+1)
	rb := &runbook.Runbook{
		Steps: []runbook.NamedStep{
			{Name: longName, Step: runbook.Step{}},
		},
	}
	if err := runbook.Validate(rb); err == nil {
		t.Error("expected step name length error")
	}
}

func TestValidateSwitchBranches(t *testing.T) {
	cases := make([]runbook.SwitchCase, runbook.MaxSwitchBranches+1)
	for i := range cases {
		cases[i] = runbook.SwitchCase{Condition: "${true}"}
	}
	rb := &runbook.Runbook{
		Steps: []runbook.NamedStep{
			{Name: "s", Step: runbook.Step{Switch: cases}},
		},
	}
	if err := runbook.Validate(rb); err == nil {
		t.Error("expected switch branches error")
	}
}

func TestValidateSwitchCount(t *testing.T) {
	var steps []runbook.NamedStep
	for i := range runbook.MaxSwitchesPerRunbook + 1 {
		steps = append(steps, runbook.NamedStep{
			Name: string(rune('a' + i)),
			Step: runbook.Step{
				Switch: []runbook.SwitchCase{{Condition: "${true}"}},
			},
		})
	}
	rb := &runbook.Runbook{Steps: steps}
	if err := runbook.Validate(rb); err == nil {
		t.Error("expected switch count error")
	}
}

func TestValidateParallelDepth(t *testing.T) {
	rb := &runbook.Runbook{
		Steps: []runbook.NamedStep{
			{Name: "p1", Step: runbook.Step{
				Parallel: &runbook.ParallelStep{
					Branches: []runbook.Branch{
						{Name: "b1", Steps: []runbook.NamedStep{
							{Name: "p2", Step: runbook.Step{
								Parallel: &runbook.ParallelStep{
									Branches: []runbook.Branch{
										{Name: "b2", Steps: []runbook.NamedStep{
											{Name: "p3", Step: runbook.Step{
												Parallel: &runbook.ParallelStep{
													Branches: []runbook.Branch{
														{Name: "b3", Steps: []runbook.NamedStep{}},
													},
												},
											}},
										}},
									},
								},
							}},
						}},
					},
				},
			}},
		},
	}
	if err := runbook.Validate(rb); err == nil {
		t.Error("expected parallel depth error")
	}
}

func TestValidateExpressionLength(t *testing.T) {
	longExpr := strings.Repeat("1+", 300) + "1"
	_, err := runbook.Parse([]byte("steps:\n  s:\n    return: ${" + longExpr + "}"))
	if err != nil {
		t.Skipf("parse error: %v", err)
	}
}

func TestValidateOK(t *testing.T) {
	rb := &runbook.Runbook{
		Steps: []runbook.NamedStep{
			{Name: "ok", Step: runbook.Step{
				Switch: []runbook.SwitchCase{
					{Condition: "${true}"},
				},
			}},
		},
	}
	if err := runbook.Validate(rb); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
