package runbook

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func Parse(yamlData []byte) (*Runbook, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(yamlData, &doc); err != nil {
		return nil, fmt.Errorf("parse runbook: %w", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, fmt.Errorf("parse runbook: empty document")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("parse runbook: root must be a mapping")
	}

	rb := &Runbook{}
	for i := 0; i < len(root.Content)-1; i += 2 {
		key := root.Content[i].Value
		val := root.Content[i+1]

		switch key {
		case "meta":
			meta, err := parseMeta(val)
			if err != nil {
				return nil, err
			}
			rb.Meta = meta
		case "args":
			args, err := parseArgs(val)
			if err != nil {
				return nil, err
			}
			rb.Args = args
		case "steps":
			steps, err := parseSteps(val)
			if err != nil {
				return nil, err
			}
			rb.Steps = steps
		}
	}
	return rb, nil
}

func parseMeta(n *yaml.Node) (Meta, error) {
	if n.Kind != yaml.MappingNode {
		return Meta{}, fmt.Errorf("meta: expected mapping")
	}
	var m Meta
	for i := 0; i < len(n.Content)-1; i += 2 {
		if n.Content[i].Value == "description" {
			m.Description = n.Content[i+1].Value
		}
	}
	return m, nil
}

func parseArgs(n *yaml.Node) (map[string]ArgDef, error) {
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("args: expected mapping")
	}
	args := make(map[string]ArgDef)
	for i := 0; i < len(n.Content)-1; i += 2 {
		name := n.Content[i].Value
		val := n.Content[i+1]
		var def ArgDef
		if val.Kind == yaml.MappingNode {
			for j := 0; j < len(val.Content)-1; j += 2 {
				switch val.Content[j].Value {
				case "type":
					def.Type = val.Content[j+1].Value
				case "description":
					def.Description = val.Content[j+1].Value
				}
			}
		}
		args[name] = def
	}
	return args, nil
}

func parseSteps(n *yaml.Node) ([]NamedStep, error) {
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("steps: expected mapping")
	}
	var steps []NamedStep
	for i := 0; i < len(n.Content)-1; i += 2 {
		name := n.Content[i].Value
		val := n.Content[i+1]
		step, err := parseStep(val)
		if err != nil {
			return nil, fmt.Errorf("step %s: %w", name, err)
		}
		steps = append(steps, NamedStep{Name: name, Step: step})
	}
	return steps, nil
}

func parseStep(n *yaml.Node) (Step, error) {
	if n.Kind == yaml.ScalarNode && n.Value == "" {
		return Step{}, nil
	}
	if n.Kind != yaml.MappingNode {
		return Step{}, fmt.Errorf("step: expected mapping, got %v", n.Kind)
	}

	var s Step
	for i := 0; i < len(n.Content)-1; i += 2 {
		key := n.Content[i].Value
		val := n.Content[i+1]

		switch key {
		case "assign":
			assigns, err := parseAssign(val)
			if err != nil {
				return Step{}, err
			}
			s.Assign = assigns
		case "return":
			expr := nodeToExpr(val)
			s.Return = &expr
		case "next":
			s.Next = val.Value
		case "call":
			if s.Call == nil {
				s.Call = &CallStep{}
			}
			s.Call.Func = val.Value
		case "args":
			if s.Call == nil {
				s.Call = &CallStep{}
			}
			args, err := parseCallArgs(val)
			if err != nil {
				return Step{}, err
			}
			s.Call.Args = args
		case "result":
			if s.Call == nil {
				s.Call = &CallStep{}
			}
			s.Call.Result = val.Value
		case "switch":
			cases, err := parseSwitchCases(val)
			if err != nil {
				return Step{}, err
			}
			s.Switch = cases
		case "for":
			f, err := parseFor(val)
			if err != nil {
				return Step{}, err
			}
			s.For = f
		case "parallel":
			p, err := parseParallel(val)
			if err != nil {
				return Step{}, err
			}
			s.Parallel = p
		case "try":
			if s.Try == nil {
				s.Try = &TryStep{}
			}
			steps, err := parseStepBody(val)
			if err != nil {
				return Step{}, fmt.Errorf("try: %w", err)
			}
			s.Try.Steps = steps
		case "except":
			if s.Try == nil {
				s.Try = &TryStep{}
			}
			if err := parseExcept(val, s.Try); err != nil {
				return Step{}, err
			}
		}
	}
	return s, nil
}

func parseAssign(n *yaml.Node) ([]Assignment, error) {
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("assign: expected mapping")
	}
	var assigns []Assignment
	for i := 0; i < len(n.Content)-1; i += 2 {
		name := n.Content[i].Value
		expr := nodeToExpr(n.Content[i+1])
		assigns = append(assigns, Assignment{Name: name, Expression: expr})
	}
	return assigns, nil
}

func parseCallArgs(n *yaml.Node) (map[string]string, error) {
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("call args: expected mapping")
	}
	args := make(map[string]string)
	for i := 0; i < len(n.Content)-1; i += 2 {
		args[n.Content[i].Value] = nodeToExpr(n.Content[i+1])
	}
	return args, nil
}

func parseSwitchCases(n *yaml.Node) ([]SwitchCase, error) {
	if n.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("switch: expected sequence")
	}
	var cases []SwitchCase
	for _, item := range n.Content {
		if item.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("switch case: expected mapping")
		}
		var c SwitchCase
		for i := 0; i < len(item.Content)-1; i += 2 {
			key := item.Content[i].Value
			val := item.Content[i+1]
			switch key {
			case "condition":
				c.Condition = nodeToExpr(val)
			case "steps":
				steps, err := parseSteps(val)
				if err != nil {
					return nil, fmt.Errorf("switch steps: %w", err)
				}
				c.Steps = steps
			case "next":
				c.Next = val.Value
			case "return":
				expr := nodeToExpr(val)
				c.Return = &expr
			}
		}
		cases = append(cases, c)
	}
	return cases, nil
}

func parseFor(n *yaml.Node) (*ForStep, error) {
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("for: expected mapping")
	}
	f := &ForStep{}
	for i := 0; i < len(n.Content)-1; i += 2 {
		key := n.Content[i].Value
		val := n.Content[i+1]
		switch key {
		case "in":
			f.In = nodeToExpr(val)
		case "as":
			f.As = val.Value
		case "steps":
			steps, err := parseSteps(val)
			if err != nil {
				return nil, fmt.Errorf("for steps: %w", err)
			}
			f.Steps = steps
		}
	}
	return f, nil
}

func parseParallel(n *yaml.Node) (*ParallelStep, error) {
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("parallel: expected mapping")
	}
	p := &ParallelStep{}
	for i := 0; i < len(n.Content)-1; i += 2 {
		key := n.Content[i].Value
		val := n.Content[i+1]
		switch key {
		case "shared":
			shared, err := parseCallArgs(val)
			if err != nil {
				return nil, fmt.Errorf("parallel shared: %w", err)
			}
			p.Shared = shared
		case "concurrencyLimit":
			v, err := strconv.Atoi(val.Value)
			if err != nil {
				return nil, fmt.Errorf("parallel concurrencyLimit: invalid integer %q", val.Value)
			}
			p.ConcurrencyLimit = v
		case "branches":
			branches, err := parseBranches(val)
			if err != nil {
				return nil, err
			}
			p.Branches = branches
		case "in":
			p.In = nodeToExpr(val)
		case "as":
			p.As = val.Value
		case "steps":
			steps, err := parseSteps(val)
			if err != nil {
				return nil, fmt.Errorf("parallel steps: %w", err)
			}
			p.Steps = steps
		}
	}
	return p, nil
}

func parseBranches(n *yaml.Node) ([]Branch, error) {
	if n.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("branches: expected sequence")
	}
	var branches []Branch
	for _, item := range n.Content {
		if item.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("branch: expected mapping")
		}
		var b Branch
		steps, err := parseSteps(item)
		if err != nil {
			return nil, fmt.Errorf("branch: %w", err)
		}
		if len(steps) > 0 {
			b.Name = steps[0].Name
		}
		b.Steps = steps
		branches = append(branches, b)
	}
	return branches, nil
}

func parseExcept(n *yaml.Node, t *TryStep) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("except: expected mapping")
	}
	for i := 0; i < len(n.Content)-1; i += 2 {
		key := n.Content[i].Value
		val := n.Content[i+1]
		switch key {
		case "as":
			t.ExceptAs = val.Value
		case "steps":
			steps, err := parseSteps(val)
			if err != nil {
				return fmt.Errorf("except steps: %w", err)
			}
			t.ExceptSteps = steps
		case "return":
			expr := nodeToExpr(val)
			t.ExceptReturn = &expr
		}
	}
	return nil
}

func parseStepBody(n *yaml.Node) ([]NamedStep, error) {
	if n.Kind == yaml.MappingNode {
		step, err := parseStep(n)
		if err != nil {
			return nil, err
		}
		return []NamedStep{{Name: "_try", Step: step}}, nil
	}
	return nil, fmt.Errorf("step body: expected mapping")
}

func nodeToExpr(n *yaml.Node) string {
	switch n.Kind {
	case yaml.ScalarNode:
		return n.Value
	case yaml.SequenceNode:
		return "${" + nodeToJSON(n) + "}"
	case yaml.MappingNode:
		return "${" + nodeToJSON(n) + "}"
	default:
		return n.Value
	}
}

func nodeToJSON(n *yaml.Node) string {
	var raw any
	if err := n.Decode(&raw); err != nil {
		return ""
	}
	return fmt.Sprintf("%v", toExprLiteral(raw))
}

func toExprLiteral(v any) string {
	switch val := v.(type) {
	case nil:
		return "null"
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(val)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case string:
		escaped := strings.ReplaceAll(val, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		return `"` + escaped + `"`
	case []any:
		var s strings.Builder
		s.WriteString("[")
		for i, item := range val {
			if i > 0 {
				s.WriteString(", ")
			}
			s.WriteString(toExprLiteral(item))
		}
		return s.String() + "]"
	case map[string]any:
		var s strings.Builder
		s.WriteString("{")
		first := true
		for k, item := range val {
			if !first {
				s.WriteString(", ")
			}
			first = false
			s.WriteString(`"` + k + `": ` + toExprLiteral(item))
		}
		return s.String() + "}"
	default:
		return fmt.Sprintf("%v", val)
	}
}
