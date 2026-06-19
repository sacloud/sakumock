package iam

import (
	"encoding/json"
	"net/http"
)

type organizationJSON struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type updateOrgRequest struct {
	Name string `json:"name"`
}

type passwordPolicyJSON struct {
	MinLength        int  `json:"min_length"`
	RequireUppercase bool `json:"require_uppercase"`
	RequireLowercase bool `json:"require_lowercase"`
	RequireSymbols   bool `json:"require_symbols"`
}

type authContextJSON struct {
	ResourceID         int64  `json:"resource_id"`
	AuthType           string `json:"auth_type"`
	LimitedToProjectID *int   `json:"limited_to_project_id"`
}

func (s *Server) handleReadOrganization(w http.ResponseWriter, _ *http.Request) {
	s.store.mu.RLock()
	org := s.store.organization
	s.store.mu.RUnlock()
	writeJSON(w, http.StatusOK, organizationJSON{ID: org.ID, Name: org.Name})
}

func (s *Server) handleUpdateOrganization(w http.ResponseWriter, r *http.Request) {
	var req updateOrgRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.store.mu.Lock()
	s.store.organization.Name = req.Name
	org := s.store.organization
	s.store.mu.Unlock()
	s.logger.Debug("organization updated", "name", req.Name)
	writeJSON(w, http.StatusOK, organizationJSON{ID: org.ID, Name: org.Name})
}

func (s *Server) handleReadPasswordPolicy(w http.ResponseWriter, _ *http.Request) {
	s.store.mu.RLock()
	pp := s.store.passwordPolicy
	s.store.mu.RUnlock()
	writeJSON(w, http.StatusOK, passwordPolicyJSON{
		MinLength:        pp.MinLength,
		RequireUppercase: pp.RequireUppercase,
		RequireLowercase: pp.RequireLowercase,
		RequireSymbols:   pp.RequireSymbols,
	})
}

func (s *Server) handleUpdatePasswordPolicy(w http.ResponseWriter, r *http.Request) {
	var req passwordPolicyJSON
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.store.mu.Lock()
	s.store.passwordPolicy = passwordPolicyState{
		MinLength:        req.MinLength,
		RequireUppercase: req.RequireUppercase,
		RequireLowercase: req.RequireLowercase,
		RequireSymbols:   req.RequireSymbols,
	}
	s.store.mu.Unlock()
	s.logger.Debug("password policy updated")
	writeJSON(w, http.StatusOK, req)
}

func (s *Server) handleReadAuthConditions(w http.ResponseWriter, _ *http.Request) {
	s.store.mu.RLock()
	data := s.store.authConditions
	s.store.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data) //nolint:errcheck
}

func (s *Server) handleUpdateAuthConditions(w http.ResponseWriter, r *http.Request) {
	var raw json.RawMessage
	if err := readJSON(r, &raw); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.store.mu.Lock()
	s.store.authConditions = raw
	s.store.mu.Unlock()
	s.logger.Debug("auth conditions updated")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(raw) //nolint:errcheck
}

func (s *Server) handleAuthContext(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, authContextJSON{
		ResourceID:         1,
		AuthType:           "apikey",
		LimitedToProjectID: nil,
	})
}
