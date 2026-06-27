package workflows

import (
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sacloud/sakumock/core"
)

type MemoryStore struct {
	mu           sync.RWMutex
	workflows    map[string]*WorkflowRecord
	revisions    map[string][]*RevisionRecord  // keyed by workflow ID
	executions   map[string][]*ExecutionRecord // keyed by workflow ID
	histories    map[string][]HistoryRecord    // keyed by execution ID
	subscription *SubscriptionRecord
	ids          *core.IDGenerator
	logger       *slog.Logger
}

func NewMemoryStore(logger *slog.Logger) *MemoryStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &MemoryStore{
		workflows:  make(map[string]*WorkflowRecord),
		revisions:  make(map[string][]*RevisionRecord),
		executions: make(map[string][]*ExecutionRecord),
		histories:  make(map[string][]HistoryRecord),
		ids:        core.NewIDGenerator(core.DefaultIDBase()),
		logger:     logger,
	}
}

func copyWorkflow(w *WorkflowRecord) *WorkflowRecord {
	c := *w
	c.Tags = make([]Tag, len(w.Tags))
	copy(c.Tags, w.Tags)
	return &c
}

func copyRevision(r *RevisionRecord) *RevisionRecord {
	c := *r
	return &c
}

func copyExecution(e *ExecutionRecord) *ExecutionRecord {
	c := *e
	return &c
}

func (s *MemoryStore) CreateWorkflow(name, description, runbook string, publish, logging bool, tags []Tag, servicePrincipalID, concurrencyMode, revisionAlias string) *WorkflowRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	id := s.ids.Next()
	if tags == nil {
		tags = []Tag{}
	}
	if concurrencyMode == "" {
		concurrencyMode = "parallel"
	}
	w := &WorkflowRecord{
		ID:                 id,
		Name:               name,
		Description:        description,
		Publish:            publish,
		Logging:            logging,
		Tags:               tags,
		ServicePrincipalID: servicePrincipalID,
		ConcurrencyMode:    concurrencyMode,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	s.workflows[id] = w

	rev := &RevisionRecord{
		RevisionID:    1,
		WorkflowID:    id,
		RevisionAlias: revisionAlias,
		Runbook:       runbook,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	s.revisions[id] = []*RevisionRecord{rev}

	s.logger.Debug("workflow created", "id", id, "name", name)
	return copyWorkflow(w)
}

func (s *MemoryStore) GetWorkflow(id string) (*WorkflowRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w, ok := s.workflows[id]
	if !ok {
		return nil, false
	}
	return copyWorkflow(w), true
}

func (s *MemoryStore) ListWorkflows() []*WorkflowRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*WorkflowRecord, 0, len(s.workflows))
	for _, w := range s.workflows {
		result = append(result, copyWorkflow(w))
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

func (s *MemoryStore) UpdateWorkflow(id string, updates WorkflowUpdates) (*WorkflowRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	w, ok := s.workflows[id]
	if !ok {
		return nil, false
	}
	if updates.Name != nil {
		w.Name = *updates.Name
	}
	if updates.Description != nil {
		w.Description = *updates.Description
	}
	if updates.Publish != nil {
		w.Publish = *updates.Publish
	}
	if updates.Logging != nil {
		w.Logging = *updates.Logging
	}
	if updates.Tags != nil {
		w.Tags = *updates.Tags
	}
	if updates.ConcurrencyMode != nil {
		w.ConcurrencyMode = *updates.ConcurrencyMode
	}
	w.UpdatedAt = time.Now()
	s.logger.Debug("workflow updated", "id", id)
	return copyWorkflow(w), true
}

func (s *MemoryStore) DeleteWorkflow(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.workflows[id]; !ok {
		return fmt.Errorf("workflow %q not found", id)
	}

	for _, e := range s.executions[id] {
		if e.Status == "Queued" || e.Status == "Running" || e.Status == "Canceling" {
			return fmt.Errorf("workflow has active executions")
		}
	}

	delete(s.workflows, id)
	delete(s.revisions, id)
	delete(s.executions, id)
	s.logger.Debug("workflow deleted", "id", id)
	return nil
}

func (s *MemoryStore) CreateRevision(workflowID, runbook, alias string) (*RevisionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.workflows[workflowID]; !ok {
		return nil, fmt.Errorf("workflow %q not found", workflowID)
	}

	if alias != "" {
		for _, r := range s.revisions[workflowID] {
			if r.RevisionAlias == alias {
				return nil, fmt.Errorf("revision alias %q already exists", alias)
			}
		}
	}

	revs := s.revisions[workflowID]
	nextID := 1
	if len(revs) > 0 {
		nextID = revs[len(revs)-1].RevisionID + 1
	}

	now := time.Now()
	rev := &RevisionRecord{
		RevisionID:    nextID,
		WorkflowID:    workflowID,
		RevisionAlias: alias,
		Runbook:       runbook,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	s.revisions[workflowID] = append(s.revisions[workflowID], rev)
	s.workflows[workflowID].UpdatedAt = now
	s.logger.Debug("revision created", "workflow_id", workflowID, "revision_id", nextID)
	return copyRevision(rev), nil
}

func (s *MemoryStore) GetRevision(workflowID string, revisionID int) (*RevisionRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.revisions[workflowID] {
		if r.RevisionID == revisionID {
			return copyRevision(r), true
		}
	}
	return nil, false
}

func (s *MemoryStore) ListRevisions(workflowID string) []*RevisionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	revs := s.revisions[workflowID]
	result := make([]*RevisionRecord, len(revs))
	for i, r := range revs {
		result[i] = copyRevision(r)
	}
	return result
}

func (s *MemoryStore) UpdateRevisionAlias(workflowID string, revisionID int, alias string) (*RevisionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.workflows[workflowID]; !ok {
		return nil, fmt.Errorf("workflow %q not found", workflowID)
	}

	if alias != "" {
		for _, r := range s.revisions[workflowID] {
			if r.RevisionAlias == alias && r.RevisionID != revisionID {
				return nil, fmt.Errorf("revision alias %q already exists", alias)
			}
		}
	}

	for _, r := range s.revisions[workflowID] {
		if r.RevisionID == revisionID {
			r.RevisionAlias = alias
			r.UpdatedAt = time.Now()
			s.logger.Debug("revision alias updated", "workflow_id", workflowID, "revision_id", revisionID, "alias", alias)
			return copyRevision(r), nil
		}
	}
	return nil, fmt.Errorf("revision %d not found", revisionID)
}

func (s *MemoryStore) DeleteRevisionAlias(workflowID string, revisionID int) (*RevisionRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, r := range s.revisions[workflowID] {
		if r.RevisionID == revisionID {
			r.RevisionAlias = ""
			r.UpdatedAt = time.Now()
			s.logger.Debug("revision alias deleted", "workflow_id", workflowID, "revision_id", revisionID)
			return copyRevision(r), true
		}
	}
	return nil, false
}

func (s *MemoryStore) CreateExecution(workflowID string, input ExecutionInput) (*ExecutionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	w, ok := s.workflows[workflowID]
	if !ok {
		return nil, fmt.Errorf("workflow %q not found", workflowID)
	}

	if !w.Publish {
		return nil, fmt.Errorf("workflow is not executable")
	}

	revs := s.revisions[workflowID]
	if len(revs) == 0 {
		return nil, fmt.Errorf("workflow has no revisions")
	}

	var targetRev *RevisionRecord
	if input.RevisionID != nil {
		for _, r := range revs {
			if r.RevisionID == *input.RevisionID {
				targetRev = r
				break
			}
		}
		if targetRev == nil {
			return nil, fmt.Errorf("revision %d not found", *input.RevisionID)
		}
	} else if input.RevisionAlias != "" {
		for _, r := range revs {
			if r.RevisionAlias == input.RevisionAlias {
				targetRev = r
				break
			}
		}
		if targetRev == nil {
			return nil, fmt.Errorf("revision alias %q not found", input.RevisionAlias)
		}
	} else {
		targetRev = revs[len(revs)-1]
	}

	now := time.Now()
	execName := input.Name
	if execName == "" {
		execName = w.Name
	}
	args := input.Args
	if args == "" {
		args = "null"
	}

	status := input.InitialStatus
	if status == "" {
		status = "Succeeded"
	}
	exec := &ExecutionRecord{
		ExecutionID:   uuid.NewString(),
		Name:          execName,
		WorkflowID:    workflowID,
		Status:        status,
		Revision:      targetRev.RevisionID,
		RevisionAlias: targetRev.RevisionAlias,
		Args:          args,
		StepCount:     0,
		Result:        "null",
		Error:         "null",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if status == "Succeeded" {
		exec.RunAt = &now
		exec.SucceededAt = &now
	}
	s.executions[workflowID] = append(s.executions[workflowID], exec)
	s.logger.Debug("execution created", "workflow_id", workflowID, "execution_id", exec.ExecutionID)
	return copyExecution(exec), nil
}

func (s *MemoryStore) GetExecution(workflowID, executionID string) (*ExecutionRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, e := range s.executions[workflowID] {
		if e.ExecutionID == executionID {
			return copyExecution(e), true
		}
	}
	return nil, false
}

func (s *MemoryStore) ListExecutions(workflowID string) []*ExecutionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	execs := s.executions[workflowID]
	result := make([]*ExecutionRecord, len(execs))
	for i, e := range execs {
		result[i] = copyExecution(e)
	}
	return result
}

func (s *MemoryStore) UpdateExecutionStatus(workflowID, executionID string, update ExecutionStatusUpdate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, e := range s.executions[workflowID] {
		if e.ExecutionID == executionID {
			e.Status = update.Status
			if update.Result != "" {
				e.Result = update.Result
			}
			if update.Error != "" {
				e.Error = update.Error
			}
			if update.RunAt != nil {
				e.RunAt = update.RunAt
			}
			if update.SucceededAt != nil {
				e.SucceededAt = update.SucceededAt
			}
			if update.FailedAt != nil {
				e.FailedAt = update.FailedAt
			}
			e.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("execution %q not found", executionID)
}

func (s *MemoryStore) CancelExecution(workflowID, executionID string) (*ExecutionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, e := range s.executions[workflowID] {
		if e.ExecutionID == executionID {
			if e.Status != "Queued" && e.Status != "Running" {
				return nil, fmt.Errorf("execution cannot be canceled (status: %s)", e.Status)
			}
			now := time.Now()
			e.Status = "Canceled"
			e.CancelRequestedAt = &now
			e.CanceledAt = &now
			e.UpdatedAt = now
			s.logger.Debug("execution canceled", "workflow_id", workflowID, "execution_id", executionID)
			return copyExecution(e), nil
		}
	}
	return nil, fmt.Errorf("execution %q not found", executionID)
}

func (s *MemoryStore) DeleteExecution(workflowID, executionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	execs := s.executions[workflowID]
	for i, e := range execs {
		if e.ExecutionID == executionID {
			if e.Status == "Queued" || e.Status == "Running" || e.Status == "Canceling" {
				return fmt.Errorf("execution cannot be deleted (status: %s)", e.Status)
			}
			s.executions[workflowID] = append(execs[:i], execs[i+1:]...)
			s.logger.Debug("execution deleted", "workflow_id", workflowID, "execution_id", executionID)
			return nil
		}
	}
	return fmt.Errorf("execution %q not found", executionID)
}

func (s *MemoryStore) AppendHistory(workflowID, executionID string, record HistoryRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.histories[executionID] = append(s.histories[executionID], record)
}

func (s *MemoryStore) ListExecutionHistory(workflowID, executionID string) ([]HistoryRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	found := false
	for _, e := range s.executions[workflowID] {
		if e.ExecutionID == executionID {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("execution %q not found", executionID)
	}
	records := s.histories[executionID]
	result := make([]HistoryRecord, len(records))
	copy(result, records)
	return result, nil
}

func (s *MemoryStore) GetSubscription() *SubscriptionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.subscription
}

func (s *MemoryStore) CreateSubscription(planID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	plan, ok := plansByID[planID]
	if !ok {
		return fmt.Errorf("plan %d not found", planID)
	}

	now := time.Now()
	s.subscription = &SubscriptionRecord{
		ID:           uuid.NewString(),
		AccountID:    "100000000001",
		ContractID:   uuid.NewString(),
		PlanID:       planID,
		PlanName:     plan.Name,
		ActivateFrom: now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.logger.Debug("subscription created", "plan_id", planID)
	return nil
}

func (s *MemoryStore) DeleteSubscription() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.subscription == nil {
		return false
	}
	s.subscription = nil
	s.logger.Debug("subscription deleted")
	return true
}

func (s *MemoryStore) Close() error {
	return nil
}

type planDef struct {
	ID                  int
	Name                string
	Grade               int
	ServiceClassPath    string
	BasePrice           int
	IncludedSteps       int
	OverageStepUnit     int
	OveragePricePerUnit int
}

var staticPlans = []planDef{
	{ID: 1, Name: "200Kプラン", Grade: 200, ServiceClassPath: "cloud/workflow/200k", BasePrice: 4000, IncludedSteps: 200000, OverageStepUnit: 1000, OveragePricePerUnit: 80},
	{ID: 2, Name: "600Kプラン", Grade: 300, ServiceClassPath: "cloud/workflow/600k", BasePrice: 10000, IncludedSteps: 600000, OverageStepUnit: 1000, OveragePricePerUnit: 80},
}

var plansByID = func() map[int]planDef {
	m := make(map[int]planDef, len(staticPlans))
	for _, p := range staticPlans {
		m[p.ID] = p
	}
	return m
}()
