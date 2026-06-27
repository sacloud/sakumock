package runbook

type Runbook struct {
	Meta  Meta
	Args  map[string]ArgDef
	Steps []NamedStep
}

type Meta struct {
	Description string
}

type ArgDef struct {
	Type        string
	Description string
}

type NamedStep struct {
	Name string
	Step Step
}

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

type Assignment struct {
	Name       string
	Expression string
}

type CallStep struct {
	Func   string
	Args   map[string]string
	Result string
}

type SwitchCase struct {
	Condition string
	Steps     []NamedStep
	Next      string
	Return    *string
}

type ForStep struct {
	In    string
	As    string
	Steps []NamedStep
}

type ParallelStep struct {
	Shared           map[string]string
	ConcurrencyLimit int
	Branches         []Branch
	In               string
	As               string
	Steps            []NamedStep
}

type Branch struct {
	Name  string
	Steps []NamedStep
}

type TryStep struct {
	Steps        []NamedStep
	ExceptAs     string
	ExceptSteps  []NamedStep
	ExceptReturn *string
}
