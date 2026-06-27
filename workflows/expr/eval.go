package expr

import (
	"fmt"
	"maps"
	"math"
)

const (
	DefaultMaxSteps    = 100_000
	DefaultMaxArrayLen = 1_000_000
)

type Env struct {
	vars        map[string]Value
	funcs       map[string]Func
	steps       int
	maxSteps    int
	MaxArrayLen int
	testCounter int
}

type Func func(env *Env, args []Value) (Value, error)

func NewEnv() *Env {
	return &Env{
		vars:        make(map[string]Value),
		funcs:       builtinFuncs(),
		maxSteps:    DefaultMaxSteps,
		MaxArrayLen: DefaultMaxArrayLen,
	}
}

func (e *Env) SetMaxSteps(n int) { e.maxSteps = n }

func (e *Env) step() error {
	e.steps++
	if e.steps > e.maxSteps {
		return fmt.Errorf("evaluation exceeded %d steps", e.maxSteps)
	}
	return nil
}

func (e *Env) resetSteps() { e.steps = 0 }

func (e *Env) Set(name string, val Value) {
	e.vars[name] = val
}

func (e *Env) Get(name string) (Value, bool) {
	v, ok := e.vars[name]
	return v, ok
}

func (e *Env) SetFunc(name string, f Func) {
	e.funcs[name] = f
}

func (e *Env) Clone() *Env {
	vars := make(map[string]Value, len(e.vars))
	maps.Copy(vars, e.vars)
	return &Env{vars: vars, funcs: e.funcs, maxSteps: e.maxSteps, MaxArrayLen: e.MaxArrayLen}
}

func eval(n node, env *Env) (Value, error) {
	if err := env.step(); err != nil {
		return Null, err
	}
	switch nd := n.(type) {
	case *literalNode:
		return nd.value, nil

	case *identNode:
		v, ok := env.Get(nd.name)
		if !ok {
			return Null, nil
		}
		return v, nil

	case *unaryNode:
		return evalUnary(nd, env)

	case *binaryNode:
		return evalBinary(nd, env)

	case *ternaryNode:
		cond, err := eval(nd.cond, env)
		if err != nil {
			return Null, err
		}
		if cond.Truthy() {
			return eval(nd.consequent, env)
		}
		return eval(nd.alternate, env)

	case *memberNode:
		obj, err := eval(nd.object, env)
		if err != nil {
			return Null, err
		}
		v, _ := obj.GetMember(nd.property)
		return v, nil

	case *indexNode:
		obj, err := eval(nd.object, env)
		if err != nil {
			return Null, err
		}
		idx, err := eval(nd.index, env)
		if err != nil {
			return Null, err
		}
		v, _ := obj.GetIndex(idx)
		return v, nil

	case *callNode:
		return evalCall(nd, env)

	case *arrayNode:
		items := make([]Value, len(nd.elements))
		for i, elem := range nd.elements {
			v, err := eval(elem, env)
			if err != nil {
				return Null, err
			}
			items[i] = v
		}
		return Array(items...), nil

	case *objectNode:
		m := make(map[string]Value, len(nd.keys))
		for i, key := range nd.keys {
			v, err := eval(nd.values[i], env)
			if err != nil {
				return Null, err
			}
			m[key] = v
		}
		return Object(m), nil

	default:
		return Null, fmt.Errorf("unknown node type: %T", n)
	}
}

func evalUnary(nd *unaryNode, env *Env) (Value, error) {
	operand, err := eval(nd.operand, env)
	if err != nil {
		return Null, err
	}
	switch nd.op {
	case "-":
		return Number(-operand.ToNumber()), nil
	case "+":
		return Number(operand.ToNumber()), nil
	case "!":
		return Bool(!operand.Truthy()), nil
	default:
		return Null, fmt.Errorf("unknown unary operator: %s", nd.op)
	}
}

func evalBinary(nd *binaryNode, env *Env) (Value, error) {
	if nd.op == "&&" {
		left, err := eval(nd.left, env)
		if err != nil {
			return Null, err
		}
		if !left.Truthy() {
			return left, nil
		}
		return eval(nd.right, env)
	}
	if nd.op == "||" {
		left, err := eval(nd.left, env)
		if err != nil {
			return Null, err
		}
		if left.Truthy() {
			return left, nil
		}
		return eval(nd.right, env)
	}

	left, err := eval(nd.left, env)
	if err != nil {
		return Null, err
	}
	right, err := eval(nd.right, env)
	if err != nil {
		return Null, err
	}

	switch nd.op {
	case "+":
		if left.typ == TypeString || right.typ == TypeString {
			return String(left.ToString() + right.ToString()), nil
		}
		return Number(left.ToNumber() + right.ToNumber()), nil
	case "-":
		return Number(left.ToNumber() - right.ToNumber()), nil
	case "*":
		return Number(left.ToNumber() * right.ToNumber()), nil
	case "/":
		r := right.ToNumber()
		if r == 0 {
			if left.ToNumber() == 0 {
				return Number(math.NaN()), nil
			}
			return Number(math.Inf(1)), nil
		}
		return Number(left.ToNumber() / r), nil
	case "%":
		return Number(math.Mod(left.ToNumber(), right.ToNumber())), nil
	case "**":
		return Number(math.Pow(left.ToNumber(), right.ToNumber())), nil
	case "==":
		return Bool(left.Equal(right)), nil
	case "!=":
		return Bool(!left.Equal(right)), nil
	case "===":
		return Bool(left.StrictEqual(right)), nil
	case "!==":
		return Bool(!left.StrictEqual(right)), nil
	case "<":
		return Bool(left.Less(right)), nil
	case ">":
		return Bool(right.Less(left)), nil
	case "<=":
		return Bool(!right.Less(left)), nil
	case ">=":
		return Bool(!left.Less(right)), nil
	default:
		return Null, fmt.Errorf("unknown binary operator: %s", nd.op)
	}
}

func evalCall(nd *callNode, env *Env) (Value, error) {
	name := resolveFuncName(nd.callee)
	if name == "" {
		return Null, fmt.Errorf("cannot call non-function value")
	}

	fn, ok := env.funcs[name]
	if !ok {
		return Null, fmt.Errorf("undefined function: %s", name)
	}

	args := make([]Value, len(nd.args))
	for i, arg := range nd.args {
		v, err := eval(arg, env)
		if err != nil {
			return Null, err
		}
		args[i] = v
	}
	return fn(env, args)
}

func resolveFuncName(n node) string {
	switch nd := n.(type) {
	case *identNode:
		return nd.name
	case *memberNode:
		parent := resolveFuncName(nd.object)
		if parent == "" {
			return ""
		}
		return parent + "." + nd.property
	default:
		return ""
	}
}
