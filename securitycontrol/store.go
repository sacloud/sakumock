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

// Activation is the project's security-control activation state.
type Activation struct {
	ServicePrincipalID string
	IsActive           bool
}

// AutomatedAction is a stored automated action. Action keeps the request's
// action definition (a discriminated union) as sent.
type AutomatedAction struct {
	ID                 string
	Name               string
	Description        *string
	Action             json.RawMessage
	ExecutionCondition string
	IsEnabled          bool
	CreatedAt          time.Time
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
	ID        string
	Rule      json.RawMessage
	IsEnabled bool
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
