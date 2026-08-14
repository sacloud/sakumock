package workflows

import "time"

// Tag is a label attached to a workflow.
type Tag struct {
	Name string
}

// WorkflowRecord is a workflow and the revision currently published.
type WorkflowRecord struct {
	ID                 string
	Name               string
	Description        string
	Publish            bool
	Logging            bool
	Tags               []Tag
	ServicePrincipalID string
	ConcurrencyMode    string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// RevisionRecord is one stored revision of a workflow's runbook.
type RevisionRecord struct {
	RevisionID    int
	WorkflowID    string
	RevisionAlias string
	Runbook       string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ExecutionRecord is one run of a workflow.
type ExecutionRecord struct {
	ExecutionID       string
	Name              string
	WorkflowID        string
	Status            string
	Revision          int
	RevisionAlias     string
	Args              string
	StepCount         int
	Result            string
	Error             string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	RunAt             *time.Time
	FailedAt          *time.Time
	SucceededAt       *time.Time
	CancelRequestedAt *time.Time
	CanceledAt        *time.Time
}

// SubscriptionRecord is the plan the organization is subscribed to.
type SubscriptionRecord struct {
	ID            string
	AccountID     string
	ContractID    string
	PlanID        int
	PlanName      string
	ActivateFrom  time.Time
	ActivateUntil *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// HistoryRecord is one step-level entry of an execution's history.
type HistoryRecord struct {
	WorkflowExecutionID string
	JobID               string
	ThreadID            string
	Type                string
	CreatedAt           time.Time
	Meta                string
	StackTrace          string
	Variables           string
}

// WorkflowUpdates carries the fields UpdateWorkflow may change; a nil field is
// left untouched.
type WorkflowUpdates struct {
	Name            *string
	Description     *string
	Publish         *bool
	Logging         *bool
	Tags            *[]Tag
	ConcurrencyMode *string
}

// ExecutionStatusUpdate carries the fields UpdateExecutionStatus may change.
type ExecutionStatusUpdate struct {
	Status      string
	Result      string
	Error       string
	RunAt       *time.Time
	SucceededAt *time.Time
	FailedAt    *time.Time
}

// ExecutionInput is what an execution is started with.
type ExecutionInput struct {
	RevisionID    *int
	RevisionAlias string
	Args          string
	Name          string
	InitialStatus string
}

// Store is the storage backend for workflows, their revisions, and their
// executions.
type Store interface {
	CreateWorkflow(name, description, runbook string, publish, logging bool, tags []Tag, servicePrincipalID, concurrencyMode, revisionAlias string) *WorkflowRecord
	GetWorkflow(id string) (*WorkflowRecord, bool)
	ListWorkflows() []*WorkflowRecord
	UpdateWorkflow(id string, updates WorkflowUpdates) (*WorkflowRecord, bool)
	DeleteWorkflow(id string) error

	CreateRevision(workflowID, runbook, alias string) (*RevisionRecord, error)
	GetRevision(workflowID string, revisionID int) (*RevisionRecord, bool)
	ListRevisions(workflowID string) []*RevisionRecord
	UpdateRevisionAlias(workflowID string, revisionID int, alias string) (*RevisionRecord, error)
	DeleteRevisionAlias(workflowID string, revisionID int) (*RevisionRecord, bool)

	CreateExecution(workflowID string, input ExecutionInput) (*ExecutionRecord, error)
	GetExecution(workflowID, executionID string) (*ExecutionRecord, bool)
	ListExecutions(workflowID string) []*ExecutionRecord
	UpdateExecutionStatus(workflowID, executionID string, update ExecutionStatusUpdate) error
	CancelExecution(workflowID, executionID string) (*ExecutionRecord, error)
	DeleteExecution(workflowID, executionID string) error
	AppendHistory(workflowID, executionID string, record HistoryRecord)
	ListExecutionHistory(workflowID, executionID string) ([]HistoryRecord, error)

	GetSubscription() *SubscriptionRecord
	CreateSubscription(planID int) error
	DeleteSubscription() bool

	Close() error
}
