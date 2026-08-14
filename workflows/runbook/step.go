package runbook

// Runbook is a parsed workflow definition.
type Runbook struct {
	Meta  Meta
	Args  map[string]ArgDef
	Steps []NamedStep
}

// Meta describes the runbook and declares the arguments it accepts.
type Meta struct {
	Description string
}

// ArgDef declares one argument of a runbook.
type ArgDef struct {
	Type        string
	Description string
}

// NamedStep is a step together with the name it is addressed by.
type NamedStep struct {
	Name string
	Step Step
}

// Step is one instruction of a runbook. Exactly one of its fields is set,
// which decides what the step does.
type Step struct {
	Assign   []Assignment
	Return   *string
	Call     *CallStep
	Switch   []SwitchCase
	For      *ForStep
	Parallel *ParallelStep
	Try      *TryStep
	Next     string
}

// Assignment binds an expression's result to a variable.
type Assignment struct {
	Name       string
	Expression string
}

// CallStep invokes a call function with the given arguments.
type CallStep struct {
	Func   string
	Args   map[string]string
	Result string
}

// SwitchCase is one branch of a switch step.
type SwitchCase struct {
	Condition string
	Steps     []NamedStep
	Next      string
	Return    *string
}

// ForStep repeats its steps over a range or a collection.
type ForStep struct {
	In    string
	As    string
	Steps []NamedStep
}

// ParallelStep runs its branches concurrently.
type ParallelStep struct {
	Shared           map[string]string
	ConcurrencyLimit int
	Branches         []Branch
	In               string
	As               string
	Steps            []NamedStep
}

// Branch is one concurrently executed sequence of a parallel step.
type Branch struct {
	Name  string
	Steps []NamedStep
}

// TryStep runs its steps and handles a failure with the except steps.
type TryStep struct {
	Steps        []NamedStep
	ExceptAs     string
	ExceptSteps  []NamedStep
	ExceptReturn *string
}
