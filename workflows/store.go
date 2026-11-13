package workflows

import "time"

// The json tags of the record types below are their form in the state file
// (--state-dir), not in API responses.

// Tag is a label attached to a workflow.
type Tag struct {
	Name string `json:"name"`
}

// WorkflowRecord is a workflow and the revision currently published.
type WorkflowRecord struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	Publish            bool      `json:"publish"`
	Logging            bool      `json:"logging"`
	Tags               []Tag     `json:"tags"`
	ServicePrincipalID string    `json:"service_principal_id"`
	ConcurrencyMode    string    `json:"concurrency_mode"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// RevisionRecord is one stored revision of a workflow's runbook.
type RevisionRecord struct {
	RevisionID    int       `json:"revision_id"`
	WorkflowID    string    `json:"workflow_id"`
	RevisionAlias string    `json:"revision_alias"`
	Runbook       string    `json:"runbook"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ExecutionRecord is one run of a workflow.
type ExecutionRecord struct {
	ExecutionID       string     `json:"execution_id"`
	Name              string     `json:"name"`
	WorkflowID        string     `json:"workflow_id"`
	Status            string     `json:"status"`
	Revision          int        `json:"revision"`
	RevisionAlias     string     `json:"revision_alias"`
	Args              string     `json:"args"`
	StepCount         int        `json:"step_count"`
	Result            string     `json:"result"`
	Error             string     `json:"error"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	RunAt             *time.Time `json:"run_at,omitempty"`
	FailedAt          *time.Time `json:"failed_at,omitempty"`
	SucceededAt       *time.Time `json:"succeeded_at,omitempty"`
	CancelRequestedAt *time.Time `json:"cancel_requested_at,omitempty"`
	CanceledAt        *time.Time `json:"canceled_at,omitempty"`
}

// SubscriptionRecord is the plan the organization is subscribed to.
type SubscriptionRecord struct {
	ID            string     `json:"id"`
	AccountID     string     `json:"account_id"`
	ContractID    string     `json:"contract_id"`
	PlanID        int        `json:"plan_id"`
	PlanName      string     `json:"plan_name"`
	ActivateFrom  time.Time  `json:"activate_from"`
	ActivateUntil *time.Time `json:"activate_until,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// HistoryRecord is one step-level entry of an execution's history.
type HistoryRecord struct {
	WorkflowExecutionID string    `json:"workflow_execution_id"`
	JobID               string    `json:"job_id"`
	ThreadID            string    `json:"thread_id"`
	Type                string    `json:"type"`
	CreatedAt           time.Time `json:"created_at"`
	Meta                string    `json:"meta"`
	StackTrace          string    `json:"stack_trace"`
	Variables           string    `json:"variables"`
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
	// CanceledAt is ignored for an execution that already has one (canceled
	// through the API before its runner stopped).
	CanceledAt *time.Time
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
