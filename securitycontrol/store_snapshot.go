package securitycontrol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// storeSnapshot is the store's form in the state file (--state-dir).
type storeSnapshot struct {
	// Activation is null until the project is activated.
	Activation *Activation                 `json:"activation"`
	Actions    map[string]*AutomatedAction `json:"automated_actions"`
	// Rules holds every catalog rule, including the ones never changed.
	Rules map[string]*EvaluationRule `json:"evaluation_rules"`
}

// Snapshot returns the activation, every automated action and every
// evaluation rule as JSON, for core.StateFile.
func (s *MemoryStore) Snapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.MarshalIndent(storeSnapshot{Activation: s.activation, Actions: s.actions, Rules: s.rules}, "", "  ")
}

// Restore replaces the store's state with a snapshot, and advances the ID
// generator past the restored automated-action IDs so new actions never reuse
// one. The evaluation rules stay the catalog's: a catalog rule the snapshot
// lacks keeps its default, and a snapshot rule the catalog no longer has is
// dropped.
func (s *MemoryStore) Restore(data []byte) error {
	var snap storeSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	if snap.Actions == nil {
		snap.Actions = make(map[string]*AutomatedAction)
	}
	for id, a := range snap.Actions {
		if a == nil || a.ID != id {
			return fmt.Errorf("automated action %q: record is missing or has a different id", id)
		}
		action, err := compactJSON(a.Action)
		if err != nil {
			return fmt.Errorf("automated action %q: action: %w", id, err)
		}
		a.Action = action
	}
	rules := make(map[string]*EvaluationRule, len(ruleCatalog))
	for _, c := range ruleCatalog {
		r, ok := snap.Rules[c.ID]
		if !ok {
			rules[c.ID] = defaultRule(c.ID)
			continue
		}
		if r == nil || r.ID != c.ID {
			return fmt.Errorf("evaluation rule %q: record is missing or has a different id", c.ID)
		}
		rule, err := compactJSON(r.Rule)
		if err != nil {
			return fmt.Errorf("evaluation rule %q: rule: %w", c.ID, err)
		}
		r.Rule = rule
		rules[c.ID] = r
	}
	for id := range snap.Rules {
		if _, ok := rules[id]; !ok {
			s.logger.Warn("dropping a restored evaluation rule that is not in the catalog", "id", id)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.activation = snap.Activation
	s.actions = snap.Actions
	s.rules = rules
	for id := range s.actions {
		s.ids.Observe(id)
	}
	return nil
}

// compactJSON strips the state file's indentation from a stored union, so it
// is held as compact as the request body it came from. A missing or null
// value is an error: the union is required in every response.
func compactJSON(raw json.RawMessage) (json.RawMessage, error) {
	var out bytes.Buffer
	if err := json.Compact(&out, raw); err != nil {
		return nil, err
	}
	if out.String() == "null" {
		return nil, errors.New("is null")
	}
	return out.Bytes(), nil
}
