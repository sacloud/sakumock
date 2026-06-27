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

type executor struct {
	store  *MemoryStore
	runner *runbook.Runner
	logger *slog.Logger

	mu      sync.Mutex
	running map[string]context.CancelFunc
}

func newExecutor(store *MemoryStore, logger *slog.Logger) *executor {
	return &executor{
		store:   store,
		runner:  runbook.NewRunner(),
		logger:  logger,
		running: make(map[string]context.CancelFunc),
	}
}

func (e *executor) submit(ctx context.Context, workflowID, executionID string, rb *runbook.Runbook, argsJSON string) {
	execCtx, cancel := context.WithCancel(ctx)

	e.mu.Lock()
	e.running[executionID] = cancel
	e.mu.Unlock()

	go func() {
		defer func() {
			e.mu.Lock()
			delete(e.running, executionID)
			e.mu.Unlock()
			cancel()
		}()

		now := time.Now()
		e.store.UpdateExecutionStatus(workflowID, executionID, ExecutionStatusUpdate{
			Status: "Running",
			RunAt:  &now,
		})

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

		result := e.runner.Run(execCtx, rb, args)

		finished := time.Now()
		if result.Err != nil {
			status := "Failed"
			if execCtx.Err() != nil {
				status = "Canceled"
			}
			update := ExecutionStatusUpdate{
				Status: status,
				Error:  result.Err.Error(),
			}
			if status == "Failed" {
				update.FailedAt = &finished
			}
			e.store.UpdateExecutionStatus(workflowID, executionID, update)
			e.logger.Info("execution finished",
				"workflow_id", workflowID,
				"execution_id", executionID,
				"status", status,
				"error", result.Err.Error(),
			)
			return
		}

		resultJSON := "null"
		if b, err := json.Marshal(result.Value.ToInterface()); err == nil {
			resultJSON = string(b)
		}

		e.store.UpdateExecutionStatus(workflowID, executionID, ExecutionStatusUpdate{
			Status:      "Succeeded",
			Result:      resultJSON,
			SucceededAt: &finished,
		})
		e.logger.Info("execution finished",
			"workflow_id", workflowID,
			"execution_id", executionID,
			"status", "Succeeded",
		)
	}()
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

func (e *executor) shutdown() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, cancel := range e.running {
		cancel()
	}
}
