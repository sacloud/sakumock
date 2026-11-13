package securitycontrol

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	// ErrNotFound reports a missing activation, automated action or
	// evaluation rule.
	ErrNotFound = errors.New("not found")
	// ErrAlreadyActivated reports a second activation of the project.
	ErrAlreadyActivated = errors.New("security control is already activated")
	// ErrActionLimit reports that the automated-action limit is reached.
	ErrActionLimit = errors.New("automated action limit reached")
)

// Activation is the project's security-control activation state. The json
// tags of the stored types are their form in the state file (--state-dir),
// not in API responses.
type Activation struct {
	ServicePrincipalID string `json:"service_principal_id"`
	IsActive           bool   `json:"is_active"`
}

// AutomatedAction is a stored automated action. Action keeps the request's
// action definition (a discriminated union) as sent.
type AutomatedAction struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Description        *string         `json:"description"`
	Action             json.RawMessage `json:"action"`
	ExecutionCondition string          `json:"execution_condition"`
	IsEnabled          bool            `json:"is_enabled"`
	CreatedAt          time.Time       `json:"created_at"`
}

// AutomatedActionInput holds the client-settable fields of an automated action.
type AutomatedActionInput struct {
	Name               string
	Description        *string
	Action             json.RawMessage
	ExecutionCondition string
	IsEnabled          bool
}

// EvaluationRule is one of the fixed evaluation rules of the project. Rule
// keeps the rule union (evaluationRuleId plus its parameter) as last set.
type EvaluationRule struct {
	ID        string          `json:"id"`
	Rule      json.RawMessage `json:"rule"`
	IsEnabled bool            `json:"is_enabled"`
}

// Store is the storage backend of the Security Control mock.
type Store interface {
	CreateActivation(servicePrincipalID string) (Activation, error)
	ReadActivation() (Activation, error)
	UpdateActivation(a Activation) (Activation, error)

	ListAutomatedActions() []AutomatedAction
	CreateAutomatedAction(in AutomatedActionInput) (AutomatedAction, error)
	ReadAutomatedAction(id string) (AutomatedAction, error)
	UpdateAutomatedAction(id string, in AutomatedActionInput) (AutomatedAction, error)
	DeleteAutomatedAction(id string) bool

	ListEvaluationRules() []EvaluationRule
	ReadEvaluationRule(id string) (EvaluationRule, error)
	UpdateEvaluationRule(r EvaluationRule) (EvaluationRule, error)

	Close() error
}
