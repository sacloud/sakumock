package workflows

import "time"

type Tag struct {
	Name string
}

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

type RevisionRecord struct {
	RevisionID    int
	WorkflowID    string
	RevisionAlias string
	Runbook       string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

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

type WorkflowUpdates struct {
	Name            *string
	Description     *string
	Publish         *bool
	Logging         *bool
	Tags            *[]Tag
	ConcurrencyMode *string
}

type ExecutionStatusUpdate struct {
	Status      string
	Result      string
	Error       string
	RunAt       *time.Time
	SucceededAt *time.Time
	FailedAt    *time.Time
}

type ExecutionInput struct {
	RevisionID    *int
	RevisionAlias string
	Args          string
	Name          string
	InitialStatus string
}

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
	ListExecutionHistory(workflowID, executionID string) ([]HistoryRecord, error)

	GetSubscription() *SubscriptionRecord
	CreateSubscription(planID int) error
	DeleteSubscription() bool

	Close() error
}
