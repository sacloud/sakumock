package iam

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/sacloud/sakumock/core"
)

type projectAPIKeyJSON struct {
	ID               int      `json:"id"`
	ProjectID        int      `json:"project_id"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	AccessToken      string   `json:"access_token"`
	ServerResourceID *string  `json:"server_resource_id"`
	IAMRoles         []string `json:"iam_roles"`
	ZoneID           *string  `json:"zone_id"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
}

type projectAPIKeyWithSecretJSON struct {
	projectAPIKeyJSON
	AccessTokenSecret string `json:"access_token_secret"`
}

type createAPIKeyRequest struct {
	ProjectID        int      `json:"project_id"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	ServerResourceID string   `json:"server_resource_id,omitempty"`
	IAMRoles         []string `json:"iam_roles"`
	ZoneID           string   `json:"zone_id,omitempty"`
}

type updateAPIKeyRequest struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	ServerResourceID string   `json:"server_resource_id,omitempty"`
	IAMRoles         []string `json:"iam_roles"`
	ZoneID           string   `json:"zone_id,omitempty"`
}

func apiKeyToJSON(r *ProjectAPIKeyRecord) projectAPIKeyJSON {
	j := projectAPIKeyJSON{
		ID:          r.ID,
		ProjectID:   r.ProjectID,
		Name:        r.Name,
		Description: r.Description,
		AccessToken: r.AccessToken,
		IAMRoles:    r.IAMRoles,
		CreatedAt:   core.FormatRFC3339(r.CreatedAt),
		UpdatedAt:   core.FormatRFC3339(r.UpdatedAt),
	}
	if r.ServerResourceID != "" {
		j.ServerResourceID = &r.ServerResourceID
	}
	if r.ZoneID != "" {
		j.ZoneID = &r.ZoneID
	}
	return j
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	records := s.store.apiKeys.all()
	items := make([]projectAPIKeyJSON, 0, len(records))
	for _, rec := range records {
		items = append(items, apiKeyToJSON(rec))
	}
	writePage(w, items)
}

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req createAPIKeyRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.IAMRoles == nil {
		req.IAMRoles = []string{}
	}
	now := time.Now()
	rec := &ProjectAPIKeyRecord{
		ID:                s.store.nextID(),
		ProjectID:         req.ProjectID,
		Name:              req.Name,
		Description:       req.Description,
		AccessToken:       randomToken(32),
		AccessTokenSecret: randomToken(32),
		ServerResourceID:  req.ServerResourceID,
		IAMRoles:          req.IAMRoles,
		ZoneID:            req.ZoneID,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	s.store.apiKeys.set(idKey(rec.ID), rec)
	s.logger.Debug("API key created", "id", rec.ID, "name", rec.Name)
	resp := projectAPIKeyWithSecretJSON{
		projectAPIKeyJSON: apiKeyToJSON(rec),
		AccessTokenSecret: rec.AccessTokenSecret,
	}
	core.WriteJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleReadAPIKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("apikey_id")
	rec, ok := s.store.apiKeys.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "API key not found")
		return
	}
	core.WriteJSON(w, http.StatusOK, apiKeyToJSON(rec))
}

func (s *Server) handleUpdateAPIKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("apikey_id")
	rec, ok := s.store.apiKeys.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "API key not found")
		return
	}
	var req updateAPIKeyRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rec.Name = req.Name
	rec.Description = req.Description
	rec.ServerResourceID = req.ServerResourceID
	if req.IAMRoles != nil {
		rec.IAMRoles = req.IAMRoles
	}
	rec.ZoneID = req.ZoneID
	rec.UpdatedAt = time.Now()
	s.store.apiKeys.set(id, rec)
	s.logger.Debug("API key updated", "id", id)
	core.WriteJSON(w, http.StatusOK, apiKeyToJSON(rec))
}

func (s *Server) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("apikey_id")
	if !s.store.apiKeys.delete(id) {
		writeError(w, http.StatusNotFound, "API key not found")
		return
	}
	s.logger.Debug("API key deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}
