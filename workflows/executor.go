package workflows

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/sacloud/sakumock/workflows/expr"
	"github.com/sacloud/sakumock/workflows/runbook"
)

const defaultExecutionTimeout = 10 * time.Minute

type executor struct {
	store            *MemoryStore
	logger           *slog.Logger
	executionTimeout time.Duration
	allowLocalNet    bool
	// onFinished, when set, runs after an execution reached a terminal status:
	// that happens outside of any HTTP request, so the server saves its state
	// here.
	onFinished func()

	mu      sync.Mutex
	running map[string]context.CancelFunc
	// closed is set by shutdown: no execution starts after it.
	closed bool
	wg     sync.WaitGroup
}

func newExecutor(store *MemoryStore, logger *slog.Logger) *executor {
	return &executor{
		executionTimeout: defaultExecutionTimeout,
		store:            store,
		logger:           logger,
		running:          make(map[string]context.CancelFunc),
	}
}

func (e *executor) newRunner(workflowID, executionID string) *runbook.Runner {
	r := runbook.NewRunner()
	r.Logger = e.logger
	r.AllowLocalNet = e.allowLocalNet
	r.HTTPClient = runbook.NewHTTPClient(e.allowLocalNet)
	r.OnEvent = func(ev runbook.Event) {
		meta := ev.Meta
		if meta == "" {
			meta = "{}"
		}
		stackTrace := ev.StepName
		if stackTrace == "" {
			stackTrace = "-"
		}
		variables := ev.Variables
		if variables == "" {
			variables = "{}"
		}
		e.store.AppendHistory(workflowID, executionID, HistoryRecord{
			WorkflowExecutionID: executionID,
			JobID:               executionID,
			ThreadID:            "main",
			Type:                string(ev.Type),
			CreatedAt:           time.Now(),
			Meta:                meta,
			StackTrace:          stackTrace,
			Variables:           variables,
		})
	}
	return r
}

// submit runs the execution in a goroutine. After shutdown it does nothing:
// the execution stays Queued.
func (e *executor) submit(ctx context.Context, workflowID, executionID string, rb *runbook.Runbook, argsJSON string) {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	execCtx, cancel := context.WithTimeout(ctx, e.executionTimeout)
	e.running[executionID] = cancel
	e.wg.Add(1)
	e.mu.Unlock()

	go func() {
		defer e.wg.Done()
		defer func() {
			e.mu.Lock()
			delete(e.running, executionID)
			e.mu.Unlock()
			cancel()
		}()

		e.logger.Info("execution started",
			"workflow_id", workflowID,
			"execution_id", executionID,
		)

		now := time.Now()
		if err := e.store.UpdateExecutionStatus(workflowID, executionID, ExecutionStatusUpdate{
			Status: "Running",
			RunAt:  &now,
		}); err != nil {
			e.logger.Error("update execution status", "error", err)
			return
		}

		var args map[string]expr.Value
		if argsJSON != "" && argsJSON != "null" {
			var raw map[string]any
			if err := json.Unmarshal([]byte(argsJSON), &raw); err == nil {
				args = make(map[string]expr.Value, len(raw))
				for k, v := range raw {
					args[k] = expr.FromInterface(v)
				}
			}
		}

		runner := e.newRunner(workflowID, executionID)
		result := runner.Run(execCtx, rb, args)

		finished := time.Now()
		if result.Err != nil {
			status := "Failed"
			if execCtx.Err() != nil {
				status = "Canceled"
			}
			reason := result.Err.Error()
			if status == "Canceled" && e.isClosed() {
				reason = interruptedMessage
			}
			update := ExecutionStatusUpdate{
				Status: status,
				Error:  reason,
			}
			if status == "Failed" {
				update.FailedAt = &finished
			}
			if err := e.store.UpdateExecutionStatus(workflowID, executionID, update); err != nil {
				e.logger.Error("update execution status", "error", err)
			}
			e.logger.Info("execution finished",
				"workflow_id", workflowID,
				"execution_id", executionID,
				"status", status,
				"error", reason,
			)
			e.finished()
			return
		}

		resultJSON := "null"
		if b, err := json.Marshal(result.Value.ToInterface()); err == nil {
			resultJSON = string(b)
		}

		if err := e.store.UpdateExecutionStatus(workflowID, executionID, ExecutionStatusUpdate{
			Status:      "Succeeded",
			Result:      resultJSON,
			SucceededAt: &finished,
		}); err != nil {
			e.logger.Error("update execution status", "error", err)
		}
		e.logger.Info("execution finished",
			"workflow_id", workflowID,
			"execution_id", executionID,
			"status", "Succeeded",
		)
		e.finished()
	}()
}

func (e *executor) finished() {
	if e.onFinished != nil {
		e.onFinished()
	}
}

func (e *executor) isClosed() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.closed
}

func (e *executor) cancel(executionID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	cancel, ok := e.running[executionID]
	if !ok {
		return false
	}
	cancel()
	return true
}

// shutdown cancels the running executions and waits until each has recorded
// its final status, so the state saved at Close holds no execution that is
// still changing.
func (e *executor) shutdown() {
	e.mu.Lock()
	e.closed = true
	for _, cancel := range e.running {
		cancel()
	}
	e.mu.Unlock()

	e.wg.Wait()
}
