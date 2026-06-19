package iam

import (
	"encoding/json"
	"net/http"
)

type servicePolicyStatusJSON struct {
	IsActive bool `json:"is_active"`
}

type servicePolicyRulesResponse struct {
	Rules json.RawMessage `json:"rules"`
}

func (s *Server) handleEnableServicePolicy(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDisableServicePolicy(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleServicePolicyStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, servicePolicyStatusJSON{IsActive: false})
}

func (s *Server) handleReadOrgServicePolicy(w http.ResponseWriter, _ *http.Request) {
	s.store.mu.RLock()
	data := s.store.servicePolicyRules
	s.store.mu.RUnlock()
	writeJSON(w, http.StatusOK, servicePolicyRulesResponse{Rules: data})
}

func (s *Server) handleUpdateOrgServicePolicy(w http.ResponseWriter, r *http.Request) {
	var req servicePolicyRulesResponse
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.store.mu.Lock()
	s.store.servicePolicyRules = req.Rules
	s.store.mu.Unlock()
	s.logger.Debug("service policy rules updated")
	writeJSON(w, http.StatusOK, req)
}

func (s *Server) handleServicePolicyRuleTemplates(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, []any{})
}
