package runbook

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/sacloud/sakumock/workflows/expr"
)

type returnSignal struct {
	value expr.Value
}

func (r *returnSignal) Error() string { return "return" }

type nextSignal struct {
	target string
}

func (n *nextSignal) Error() string { return "next: " + n.target }

type CallFunc func(ctx context.Context, env *expr.Env, call *CallStep, opts CallOpts) (expr.Value, error)

type CallOpts struct {
	AllowLocalNet bool
	HTTPClient    *http.Client
}

type EventType string

const (
	EventWorkflowWillStart   EventType = "workflowWillStart"
	EventWorkflowDidComplete EventType = "workflowDidCompleted"
	EventWorkflowDidFail     EventType = "workflowDidFailed"
	EventWorkflowDidCancel   EventType = "workflowDidCanceled"
	EventStepWillExecute     EventType = "stepWillExecute"
	EventStepDidExecute      EventType = "stepDidExecuted"
	EventFunctionWillCall    EventType = "functionWillCall"
	EventFunctionDidRun      EventType = "functionDidRun"
	EventFunctionDidFail     EventType = "functionDidFailed"
)

type Event struct {
	Type      EventType
	StepName  string
	Meta      string
	Variables string
}

type EventHandler func(Event)

type Runner struct {
	CallFuncs  map[string]CallFunc
	HTTPClient *http.Client
	Logger     *slog.Logger
	OnEvent    EventHandler
	// AllowLocalNet permits HTTP calls to localhost, private, and link-local
	// addresses. Enabled by default because sakumock is a local mock server
	// where calling other local services (e.g. other mocks) is a normal use case.
	// Set to false to simulate the real Workflows service's URL blocking.
	AllowLocalNet bool
}

func (r *Runner) emit(e Event) {
	if r.OnEvent != nil {
		r.OnEvent(e)
	}
}

func NewRunner() *Runner {
	return &Runner{
		CallFuncs:     defaultCallFuncs(),
		HTTPClient:    newDefaultHTTPClient(),
		Logger:        slog.Default(),
		AllowLocalNet: true,
	}
}

type Result struct {
	Value expr.Value
	Err   error
}

func (r *Runner) Run(ctx context.Context, rb *Runbook, args map[string]expr.Value) Result {
	env := expr.NewEnv()
	if args != nil {
		env.Set("args", expr.Object(args))
		r.Logger.Info("runbook start", "description", rb.Meta.Description, "args", expr.Object(args).ToString())
	} else {
		env.Set("args", expr.Object(nil))
		r.Logger.Info("runbook start", "description", rb.Meta.Description)
	}
	r.emit(Event{Type: EventWorkflowWillStart})

	val, err := r.execSteps(ctx, env, rb.Steps)
	if err != nil {
		if ret, ok := err.(*returnSignal); ok {
			r.Logger.Info("runbook end", "result", ret.value.ToString())
			r.emit(Event{Type: EventWorkflowDidComplete, Meta: ret.value.ToString()})
			return Result{Value: ret.value}
		}
		r.Logger.Info("runbook end", "error", err.Error())
		if ctx.Err() != nil {
			r.emit(Event{Type: EventWorkflowDidCancel, Meta: err.Error()})
		} else {
			r.emit(Event{Type: EventWorkflowDidFail, Meta: err.Error()})
		}
		return Result{Err: err}
	}
	r.Logger.Info("runbook end")
	r.emit(Event{Type: EventWorkflowDidComplete})
	return Result{Value: val}
}

func (r *Runner) execSteps(ctx context.Context, env *expr.Env, steps []NamedStep) (expr.Value, error) {
	if err := ctx.Err(); err != nil {
		return expr.Null, err
	}

	i := 0
	for i < len(steps) {
		if err := ctx.Err(); err != nil {
			return expr.Null, err
		}

		step := steps[i]
		r.Logger.Debug("step execute", "step", step.Name)
		r.emit(Event{Type: EventStepWillExecute, StepName: step.Name})
		err := r.execStep(ctx, env, &step.Step)
		if err == nil {
			r.emit(Event{Type: EventStepDidExecute, StepName: step.Name})
		}
		if err != nil {
			if n, ok := err.(*nextSignal); ok {
				found := false
				for j, s := range steps {
					if s.Name == n.target {
						i = j
						found = true
						break
					}
				}
				if !found {
					return expr.Null, fmt.Errorf("step %q not found (next from %q)", n.target, step.Name)
				}
				continue
			}
			return expr.Null, err
		}
		i++
	}
	return expr.Null, nil
}

func (r *Runner) execStep(ctx context.Context, env *expr.Env, step *Step) error {
	switch {
	case len(step.Assign) > 0:
		return r.execAssign(env, step)
	case step.Return != nil:
		return r.execReturn(env, step)
	case step.Call != nil:
		return r.execCall(ctx, env, step)
	case step.Switch != nil:
		return r.execSwitch(ctx, env, step)
	case step.For != nil:
		return r.execFor(ctx, env, step)
	case step.Parallel != nil:
		return r.execParallel(ctx, env, step)
	case step.Try != nil:
		return r.execTry(ctx, env, step)
	case step.Next != "":
		return &nextSignal{target: step.Next}
	}
	return nil
}

func (r *Runner) execAssign(env *expr.Env, step *Step) error {
	for _, a := range step.Assign {
		val, err := expr.EvalInterpolated(a.Expression, env)
		if err != nil {
			return fmt.Errorf("assign %s: %w", a.Name, err)
		}
		env.Set(a.Name, val)
		r.Logger.Debug("assign", "name", a.Name, "value", val.ToString())
	}
	if step.Next != "" {
		return &nextSignal{target: step.Next}
	}
	return nil
}

func (r *Runner) execReturn(env *expr.Env, step *Step) error {
	val, err := expr.EvalInterpolated(*step.Return, env)
	if err != nil {
		return fmt.Errorf("return: %w", err)
	}
	r.Logger.Debug("return", "value", val.ToString())
	return &returnSignal{value: val}
}

func (r *Runner) execCall(ctx context.Context, env *expr.Env, step *Step) error {
	call := step.Call
	r.Logger.Debug("call", "func", call.Func)
	r.emit(Event{Type: EventFunctionWillCall, Meta: call.Func})
	fn, ok := r.CallFuncs[call.Func]
	if !ok {
		return fmt.Errorf("unknown call function: %s", call.Func)
	}

	result, err := fn(ctx, env, call, CallOpts{AllowLocalNet: r.AllowLocalNet, HTTPClient: r.HTTPClient})
	if err != nil {
		r.emit(Event{Type: EventFunctionDidFail, Meta: call.Func + ": " + err.Error()})
		return fmt.Errorf("call %s: %w", call.Func, err)
	}

	r.emit(Event{Type: EventFunctionDidRun, Meta: call.Func})
	if call.Result != "" {
		env.Set(call.Result, result)
		r.Logger.Debug("call result", "func", call.Func, "var", call.Result, "value", result.ToString())
	}
	if step.Next != "" {
		return &nextSignal{target: step.Next}
	}
	return nil
}

func (r *Runner) execSwitch(ctx context.Context, env *expr.Env, step *Step) error {
	for _, c := range step.Switch {
		cond, err := expr.EvalInterpolated(c.Condition, env)
		if err != nil {
			return fmt.Errorf("switch condition: %w", err)
		}
		if !cond.Truthy() {
			continue
		}

		if c.Return != nil {
			val, err := expr.EvalInterpolated(*c.Return, env)
			if err != nil {
				return fmt.Errorf("switch return: %w", err)
			}
			return &returnSignal{value: val}
		}
		if c.Next != "" {
			return &nextSignal{target: c.Next}
		}
		if len(c.Steps) > 0 {
			_, err := r.execSteps(ctx, env, c.Steps)
			return err
		}
		return nil
	}
	if step.Next != "" {
		return &nextSignal{target: step.Next}
	}
	return nil
}

func (r *Runner) execFor(ctx context.Context, env *expr.Env, step *Step) error {
	f := step.For
	arr, err := expr.EvalInterpolated(f.In, env)
	if err != nil {
		return fmt.Errorf("for in: %w", err)
	}
	if arr.Type() != expr.TypeArray {
		return fmt.Errorf("for in: expected array, got %s", arr.Type())
	}

	for _, item := range arr.AsArray() {
		if err := ctx.Err(); err != nil {
			return err
		}
		env.Set(f.As, item)
		_, err := r.execSteps(ctx, env, f.Steps)
		if err != nil {
			if _, ok := err.(*returnSignal); ok {
				return err
			}
			return err
		}
	}
	if step.Next != "" {
		return &nextSignal{target: step.Next}
	}
	return nil
}

func (r *Runner) execParallel(ctx context.Context, env *expr.Env, step *Step) error {
	p := step.Parallel

	if p.Shared != nil {
		for k, v := range p.Shared {
			val, err := expr.EvalInterpolated(v, env)
			if err != nil {
				return fmt.Errorf("parallel shared %s: %w", k, err)
			}
			env.Set(k, val)
		}
	}

	if len(p.Branches) > 0 {
		return r.execParallelBranches(ctx, env, p, step.Next)
	}
	if p.In != "" {
		return r.execParallelIteration(ctx, env, p, step.Next)
	}
	return nil
}

func (r *Runner) execParallelBranches(ctx context.Context, env *expr.Env, p *ParallelStep, next string) error {
	type branchResult struct {
		idx int
		val expr.Value
		err error
	}

	results := make([]expr.Value, len(p.Branches))
	ch := make(chan branchResult, len(p.Branches))
	limit := p.ConcurrencyLimit
	if limit <= 0 {
		limit = len(p.Branches)
	}
	sem := make(chan struct{}, limit)

	var wg sync.WaitGroup
	for i, branch := range p.Branches {
		wg.Add(1)
		go func(idx int, b Branch) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			branchEnv := env.Clone()
			val, err := r.execSteps(ctx, branchEnv, b.Steps)
			if err != nil {
				if ret, ok := err.(*returnSignal); ok {
					ch <- branchResult{idx: idx, val: ret.value}
					return
				}
				ch <- branchResult{idx: idx, err: err}
				return
			}
			ch <- branchResult{idx: idx, val: val}
		}(i, branch)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for br := range ch {
		if br.err != nil {
			return br.err
		}
		results[br.idx] = br.val
	}
	env.Set("results", expr.Array(results...))

	if next != "" {
		return &nextSignal{target: next}
	}
	return nil
}

func (r *Runner) execParallelIteration(ctx context.Context, env *expr.Env, p *ParallelStep, next string) error {
	arr, err := expr.EvalInterpolated(p.In, env)
	if err != nil {
		return fmt.Errorf("parallel in: %w", err)
	}
	if arr.Type() != expr.TypeArray {
		return fmt.Errorf("parallel in: expected array, got %s", arr.Type())
	}

	items := arr.AsArray()
	type iterResult struct {
		idx int
		val expr.Value
		err error
	}

	results := make([]expr.Value, len(items))
	ch := make(chan iterResult, len(items))
	limit := p.ConcurrencyLimit
	if limit <= 0 {
		limit = len(items)
	}
	sem := make(chan struct{}, limit)

	var wg sync.WaitGroup
	for i, item := range items {
		wg.Add(1)
		go func(idx int, it expr.Value) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			iterEnv := env.Clone()
			iterEnv.Set(p.As, it)
			val, err := r.execSteps(ctx, iterEnv, p.Steps)
			if err != nil {
				if ret, ok := err.(*returnSignal); ok {
					ch <- iterResult{idx: idx, val: ret.value}
					return
				}
				ch <- iterResult{idx: idx, err: err}
				return
			}
			ch <- iterResult{idx: idx, val: val}
		}(i, item)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for ir := range ch {
		if ir.err != nil {
			return ir.err
		}
		results[ir.idx] = ir.val
	}
	env.Set("results", expr.Array(results...))

	if next != "" {
		return &nextSignal{target: next}
	}
	return nil
}

func (r *Runner) execTry(ctx context.Context, env *expr.Env, step *Step) error {
	t := step.Try
	_, err := r.execSteps(ctx, env, t.Steps)
	if err != nil {
		if _, ok := err.(*returnSignal); ok {
			return err
		}
		errVal := expr.Object(map[string]expr.Value{
			"message": expr.String(err.Error()),
		})
		if t.ExceptAs != "" {
			env.Set(t.ExceptAs, errVal)
		}

		if t.ExceptReturn != nil {
			val, evalErr := expr.EvalInterpolated(*t.ExceptReturn, env)
			if evalErr != nil {
				return fmt.Errorf("except return: %w", evalErr)
			}
			return &returnSignal{value: val}
		}
		if len(t.ExceptSteps) > 0 {
			_, execErr := r.execSteps(ctx, env, t.ExceptSteps)
			return execErr
		}
		return nil
	}
	if step.Next != "" {
		return &nextSignal{target: step.Next}
	}
	return nil
}
