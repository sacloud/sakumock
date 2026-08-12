package iam

import (
	"encoding/json"
	"net/http"

	"github.com/sacloud/sakumock/core"
)

type servicePolicyStatusJSON struct {
	Enabled bool `json:"enabled"`
}

type servicePolicyRulesResponse struct {
	Rules json.RawMessage `json:"rules"`
}

func (s *Server) handleEnableServicePolicy(w http.ResponseWriter, _ *http.Request) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	if s.store.servicePolicyEnabled {
		writeError(w, http.StatusConflict, "service policy is already enabled")
		return
	}
	s.store.servicePolicyEnabled = true
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDisableServicePolicy(w http.ResponseWriter, _ *http.Request) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	if !s.store.servicePolicyEnabled {
		writeError(w, http.StatusConflict, "service policy is already disabled")
		return
	}
	s.store.servicePolicyEnabled = false
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleServicePolicyStatus(w http.ResponseWriter, _ *http.Request) {
	s.store.mu.RLock()
	enabled := s.store.servicePolicyEnabled
	s.store.mu.RUnlock()
	core.WriteJSON(w, http.StatusOK, servicePolicyStatusJSON{Enabled: enabled})
}

func (s *Server) handleReadOrgServicePolicy(w http.ResponseWriter, _ *http.Request) {
	s.store.mu.RLock()
	data := s.store.servicePolicyRules
	s.store.mu.RUnlock()
	core.WriteJSON(w, http.StatusOK, servicePolicyRulesResponse{Rules: data})
}

func (s *Server) handleUpdateOrgServicePolicy(w http.ResponseWriter, r *http.Request) {
	var req servicePolicyRulesResponse
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.store.mu.Lock()
	s.store.servicePolicyRules = req.Rules
	s.store.mu.Unlock()
	s.logger.Debug("service policy rules updated")
	core.WriteJSON(w, http.StatusOK, req)
}

// ruleTemplateJSON mirrors the spec's RuleTemplate; the mock ships no
// built-in templates, so the list is always an empty page.
type ruleTemplateJSON struct {
	Code           string   `json:"code"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Type           string   `json:"type"`
	SupportsDryRun bool     `json:"supports_dry_run"`
	Prefixes       []string `json:"prefixes"`
}

func (s *Server) handleServicePolicyRuleTemplates(w http.ResponseWriter, _ *http.Request) {
	writePage(w, []ruleTemplateJSON{})
}
