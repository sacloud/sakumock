package securitycontrol

import (
	"cmp"
	"encoding/json"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/sacloud/sakumock/core"
)

var _ Store = (*MemoryStore)(nil)

// MemoryStore is the in-memory Store implementation.
type MemoryStore struct {
	mu          sync.RWMutex
	activation  *Activation
	actions     map[string]*AutomatedAction
	rules       map[string]*EvaluationRule
	actionLimit int
	ids         *core.IDGenerator
	logger      *slog.Logger
}

// NewMemoryStore returns a store holding no activation and no automated
// action, with every catalog evaluation rule disabled. actionLimit caps the
// number of automated actions.
func NewMemoryStore(logger *slog.Logger, actionLimit int) *MemoryStore {
	if logger == nil {
		logger = slog.Default()
	}
	s := &MemoryStore{
		actions:     make(map[string]*AutomatedAction),
		rules:       make(map[string]*EvaluationRule, len(ruleCatalog)),
		actionLimit: actionLimit,
		ids:         core.NewIDGenerator(core.DefaultIDBase()),
		logger:      logger,
	}
	for _, r := range ruleCatalog {
		s.rules[r.ID] = defaultRule(r.ID)
	}
	return s
}

// defaultRule returns a catalog rule as every project starts with it:
// disabled and without parameter.
func defaultRule(id string) *EvaluationRule {
	rule, _ := json.Marshal(struct {
		EvaluationRuleID string `json:"evaluationRuleId"`
	}{id})
	return &EvaluationRule{ID: id, Rule: rule}
}

// CreateActivation activates security control for the project.
func (s *MemoryStore) CreateActivation(servicePrincipalID string) (Activation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activation != nil {
		return Activation{}, ErrAlreadyActivated
	}
	s.activation = &Activation{ServicePrincipalID: servicePrincipalID, IsActive: true}
	return *s.activation, nil
}

// ReadActivation returns the activation, or ErrNotFound before the first one.
func (s *MemoryStore) ReadActivation() (Activation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.activation == nil {
		return Activation{}, ErrNotFound
	}
	return *s.activation, nil
}

// UpdateActivation replaces the activation state.
func (s *MemoryStore) UpdateActivation(a Activation) (Activation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activation == nil {
		return Activation{}, ErrNotFound
	}
	*s.activation = a
	return a, nil
}

// ListAutomatedActions returns every automated action, oldest first.
func (s *MemoryStore) ListAutomatedActions() []AutomatedAction {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AutomatedAction, 0, len(s.actions))
	for _, a := range s.actions {
		out = append(out, *a)
	}
	// Generated IDs are sequential decimal strings of equal width.
	slices.SortFunc(out, func(a, b AutomatedAction) int { return cmp.Compare(a.ID, b.ID) })
	return out
}

// CreateAutomatedAction stores a new automated action, failing with
// ErrActionLimit once the limit is reached.
func (s *MemoryStore) CreateAutomatedAction(in AutomatedActionInput) (AutomatedAction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.actions) >= s.actionLimit {
		return AutomatedAction{}, ErrActionLimit
	}
	a := &AutomatedAction{ID: s.ids.Next(), CreatedAt: time.Now()}
	a.apply(in)
	s.actions[a.ID] = a
	return *a, nil
}

// ReadAutomatedAction returns the automated action with the given ID.
func (s *MemoryStore) ReadAutomatedAction(id string) (AutomatedAction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.actions[id]
	if !ok {
		return AutomatedAction{}, ErrNotFound
	}
	return *a, nil
}

// UpdateAutomatedAction replaces the client-settable fields of an automated
// action.
func (s *MemoryStore) UpdateAutomatedAction(id string, in AutomatedActionInput) (AutomatedAction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.actions[id]
	if !ok {
		return AutomatedAction{}, ErrNotFound
	}
	a.apply(in)
	return *a, nil
}

// DeleteAutomatedAction removes an automated action and reports whether it
// existed.
func (s *MemoryStore) DeleteAutomatedAction(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.actions[id]
	delete(s.actions, id)
	return ok
}

func (a *AutomatedAction) apply(in AutomatedActionInput) {
	a.Name = in.Name
	a.Description = in.Description
	a.Action = in.Action
	a.ExecutionCondition = in.ExecutionCondition
	a.IsEnabled = in.IsEnabled
}

// ListEvaluationRules returns every evaluation rule in catalog order.
func (s *MemoryStore) ListEvaluationRules() []EvaluationRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]EvaluationRule, 0, len(ruleCatalog))
	for _, r := range ruleCatalog {
		out = append(out, *s.rules[r.ID])
	}
	return out
}

// ReadEvaluationRule returns the evaluation rule with the given ID.
func (s *MemoryStore) ReadEvaluationRule(id string) (EvaluationRule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.rules[id]
	if !ok {
		return EvaluationRule{}, ErrNotFound
	}
	return *r, nil
}

// UpdateEvaluationRule replaces the rule settings of an existing rule.
func (s *MemoryStore) UpdateEvaluationRule(r EvaluationRule) (EvaluationRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.rules[r.ID]
	if !ok {
		return EvaluationRule{}, ErrNotFound
	}
	*cur = r
	return r, nil
}

// Close releases the resources held by the store.
func (s *MemoryStore) Close() error { return nil }
