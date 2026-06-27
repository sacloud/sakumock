package runbook

import "fmt"

const (
	MaxRunbookSize        = 256 * 1024
	MaxSteps              = 5000
	MaxStepNameLen        = 31
	MaxSwitchBranches     = 50
	MaxSwitchesPerRunbook = 10
	MaxParallelSteps      = 100
	MaxParallelDepth      = 2
	ParallelTimeout       = 600
)

func ValidateRunbookSize(yamlData []byte) error {
	if len(yamlData) > MaxRunbookSize {
		return fmt.Errorf("runbook size %d bytes exceeds limit %d bytes", len(yamlData), MaxRunbookSize)
	}
	return nil
}

func Validate(rb *Runbook) error {
	v := &validator{}
	v.walkSteps(rb.Steps, 0)
	if v.err != nil {
		return v.err
	}
	if v.stepCount > MaxSteps {
		return fmt.Errorf("runbook has %d steps, exceeds limit %d", v.stepCount, MaxSteps)
	}
	if v.switchCount > MaxSwitchesPerRunbook {
		return fmt.Errorf("runbook has %d switches, exceeds limit %d", v.switchCount, MaxSwitchesPerRunbook)
	}
	return nil
}

type validator struct {
	stepCount   int
	switchCount int
	err         error
}

func (v *validator) walkSteps(steps []NamedStep, parallelDepth int) {
	for _, s := range steps {
		if v.err != nil {
			return
		}
		v.stepCount++
		if len(s.Name) > MaxStepNameLen {
			v.err = fmt.Errorf("step name %q length %d exceeds limit %d", s.Name, len(s.Name), MaxStepNameLen)
			return
		}
		v.walkStep(&s.Step, parallelDepth)
	}
}

func (v *validator) walkStep(s *Step, parallelDepth int) {
	if v.err != nil {
		return
	}
	if s.Switch != nil {
		v.switchCount++
		if len(s.Switch) > MaxSwitchBranches {
			v.err = fmt.Errorf("switch has %d branches, exceeds limit %d", len(s.Switch), MaxSwitchBranches)
			return
		}
		for _, c := range s.Switch {
			v.walkSteps(c.Steps, parallelDepth)
		}
	}
	if s.For != nil {
		v.walkSteps(s.For.Steps, parallelDepth)
	}
	if s.Parallel != nil {
		newDepth := parallelDepth + 1
		if newDepth > MaxParallelDepth {
			v.err = fmt.Errorf("parallel nesting depth %d exceeds limit %d", newDepth, MaxParallelDepth)
			return
		}
		for _, b := range s.Parallel.Branches {
			if len(b.Steps) > MaxParallelSteps {
				v.err = fmt.Errorf("parallel branch %q has %d steps, exceeds limit %d", b.Name, len(b.Steps), MaxParallelSteps)
				return
			}
			v.walkSteps(b.Steps, newDepth)
		}
		v.walkSteps(s.Parallel.Steps, newDepth)
	}
	if s.Try != nil {
		v.walkSteps(s.Try.Steps, parallelDepth)
		v.walkSteps(s.Try.ExceptSteps, parallelDepth)
	}
}
