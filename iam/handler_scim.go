package iam

import (
	"net/http"
	"time"
)

type scimConfigJSON struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type scimConfigWithTokenJSON struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	BaseURL     string `json:"base_url"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	SecretToken string `json:"secret_token"`
}

type createScimRequest struct {
	Name string `json:"name"`
}

type updateScimRequest struct {
	Name string `json:"name"`
}

type regenerateTokenResponse struct {
	SecretToken string `json:"secret_token"`
}

func scimToJSON(r *ScimConfigurationRecord) scimConfigJSON {
	return scimConfigJSON{
		ID:        r.ID,
		Name:      r.Name,
		BaseURL:   r.BaseURL,
		CreatedAt: formatTime(r.CreatedAt),
		UpdatedAt: formatTime(r.UpdatedAt),
	}
}

func (s *Server) handleListScimConfigs(w http.ResponseWriter, _ *http.Request) {
	records := s.store.scimConfigs.all()
	items := make([]scimConfigJSON, 0, len(records))
	for _, rec := range records {
		items = append(items, scimToJSON(rec))
	}
	writePage(w, items)
}

func (s *Server) handleCreateScimConfig(w http.ResponseWriter, r *http.Request) {
	var req createScimRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	now := time.Now()
	id := newUUID()
	rec := &ScimConfigurationRecord{
		ID:          id,
		Name:        req.Name,
		BaseURL:     "https://secure.sakura.ad.jp/cloud/api/iam/1.0/scim/" + id,
		SecretToken: randomToken(32),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.store.scimConfigs.set(id, rec)
	s.logger.Debug("SCIM config created", "id", id, "name", rec.Name)
	writeJSON(w, http.StatusCreated, scimConfigWithTokenJSON{
		ID:          rec.ID,
		Name:        rec.Name,
		BaseURL:     rec.BaseURL,
		CreatedAt:   formatTime(rec.CreatedAt),
		UpdatedAt:   formatTime(rec.UpdatedAt),
		SecretToken: rec.SecretToken,
	})
}

func (s *Server) handleReadScimConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rec, ok := s.store.scimConfigs.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "SCIM configuration not found")
		return
	}
	writeJSON(w, http.StatusOK, scimToJSON(rec))
}

func (s *Server) handleUpdateScimConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rec, ok := s.store.scimConfigs.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "SCIM configuration not found")
		return
	}
	var req updateScimRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rec.Name = req.Name
	rec.UpdatedAt = time.Now()
	s.store.scimConfigs.set(id, rec)
	s.logger.Debug("SCIM config updated", "id", id)
	writeJSON(w, http.StatusOK, scimToJSON(rec))
}

func (s *Server) handleDeleteScimConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.store.scimConfigs.delete(id) {
		writeError(w, http.StatusNotFound, "SCIM configuration not found")
		return
	}
	s.logger.Debug("SCIM config deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRegenerateScimToken(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rec, ok := s.store.scimConfigs.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "SCIM configuration not found")
		return
	}
	rec.SecretToken = randomToken(32)
	rec.UpdatedAt = time.Now()
	s.store.scimConfigs.set(id, rec)
	writeJSON(w, http.StatusOK, regenerateTokenResponse{SecretToken: rec.SecretToken})
}
