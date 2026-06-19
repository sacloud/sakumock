package iam

import (
	"net/http"
	"time"

	"github.com/sacloud/sakumock/core"
)

type servicePrincipalJSON struct {
	ID          int    `json:"id"`
	ProjectID   int    `json:"project_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type createServicePrincipalRequest struct {
	ProjectID   int    `json:"project_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type updateServicePrincipalRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type spKeyJSON struct {
	ID           string  `json:"id"`
	Kid          string  `json:"kid"`
	Status       string  `json:"status"`
	KeyOrigin    string  `json:"key_origin"`
	PublicKey    string  `json:"public_key"`
	CreatedAt    string  `json:"created_at"`
	KeyExpiresAt *string `json:"key_expires_at"`
}

type uploadKeyRequest struct {
	PublicKey string `json:"public_key"`
}

type oauthTokenResponse struct {
	AccessToken    string `json:"access_token"`
	TokenType      string `json:"token_type"`
	TokenExpiredAt string `json:"token_expired_at"`
	ExpiresIn      int    `json:"expires_in"`
}

func spToJSON(r *ServicePrincipalRecord) servicePrincipalJSON {
	return servicePrincipalJSON{
		ID:          r.ID,
		ProjectID:   r.ProjectID,
		Name:        r.Name,
		Description: r.Description,
		CreatedAt:   core.FormatRFC3339(r.CreatedAt),
		UpdatedAt:   core.FormatRFC3339(r.UpdatedAt),
	}
}

func spKeyToJSON(r *ServicePrincipalKeyRecord) spKeyJSON {
	j := spKeyJSON{
		ID:        r.ID,
		Kid:       r.Kid,
		Status:    r.Status,
		KeyOrigin: r.KeyOrigin,
		PublicKey: r.PublicKey,
		CreatedAt: core.FormatRFC3339(r.CreatedAt),
	}
	if r.KeyExpiresAt != "" {
		j.KeyExpiresAt = &r.KeyExpiresAt
	}
	return j
}

func (s *Server) handleListServicePrincipals(w http.ResponseWriter, r *http.Request) {
	records := s.store.servicePrincipals.all()
	items := make([]servicePrincipalJSON, 0, len(records))
	for _, rec := range records {
		items = append(items, spToJSON(rec))
	}
	writePage(w, items)
}

func (s *Server) handleCreateServicePrincipal(w http.ResponseWriter, r *http.Request) {
	var req createServicePrincipalRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	now := time.Now()
	rec := &ServicePrincipalRecord{
		ID:          s.store.nextID(),
		ProjectID:   req.ProjectID,
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.store.servicePrincipals.set(idKey(rec.ID), rec)
	s.logger.Debug("service principal created", "id", rec.ID, "name", rec.Name)
	core.WriteJSON(w, http.StatusCreated, spToJSON(rec))
}

func (s *Server) handleReadServicePrincipal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("service_principal_id")
	rec, ok := s.store.servicePrincipals.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "service principal not found")
		return
	}
	core.WriteJSON(w, http.StatusOK, spToJSON(rec))
}

func (s *Server) handleUpdateServicePrincipal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("service_principal_id")
	rec, ok := s.store.servicePrincipals.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "service principal not found")
		return
	}
	var req updateServicePrincipalRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rec.Name = req.Name
	rec.Description = req.Description
	rec.UpdatedAt = time.Now()
	s.store.servicePrincipals.set(id, rec)
	s.logger.Debug("service principal updated", "id", id)
	core.WriteJSON(w, http.StatusOK, spToJSON(rec))
}

func (s *Server) handleDeleteServicePrincipal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("service_principal_id")
	if !s.store.servicePrincipals.delete(id) {
		writeError(w, http.StatusNotFound, "service principal not found")
		return
	}
	// Clean up associated keys
	for _, key := range s.store.spKeys.all() {
		if idKey(key.ServicePrincipalID) == id {
			s.store.spKeys.delete(key.ID)
		}
	}
	s.logger.Debug("service principal deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListSPKeys(w http.ResponseWriter, r *http.Request) {
	spID := r.PathValue("service_principal_id")
	if _, ok := s.store.servicePrincipals.get(spID); !ok {
		writeError(w, http.StatusNotFound, "service principal not found")
		return
	}
	allKeys := s.store.spKeys.all()
	items := make([]spKeyJSON, 0)
	for _, key := range allKeys {
		if idKey(key.ServicePrincipalID) == spID {
			items = append(items, spKeyToJSON(key))
		}
	}
	writePage(w, items)
}

func (s *Server) handleUploadSPKey(w http.ResponseWriter, r *http.Request) {
	spID := r.PathValue("service_principal_id")
	sp, ok := s.store.servicePrincipals.get(spID)
	if !ok {
		writeError(w, http.StatusNotFound, "service principal not found")
		return
	}
	var req uploadKeyRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	keyID := newUUID()
	rec := &ServicePrincipalKeyRecord{
		ID:                 keyID,
		ServicePrincipalID: sp.ID,
		Kid:                newUUID(),
		PublicKey:          req.PublicKey,
		Status:             "enabled",
		KeyOrigin:          "user",
		CreatedAt:          time.Now(),
	}
	s.store.spKeys.set(keyID, rec)
	s.logger.Debug("SP key uploaded", "sp_id", spID, "key_id", keyID)
	core.WriteJSON(w, http.StatusCreated, spKeyToJSON(rec))
}

func (s *Server) handleEnableSPKey(w http.ResponseWriter, r *http.Request) {
	spID := r.PathValue("service_principal_id")
	if _, ok := s.store.servicePrincipals.get(spID); !ok {
		writeError(w, http.StatusNotFound, "service principal not found")
		return
	}
	keyID := r.PathValue("service_principal_key_id")
	key, ok := s.store.spKeys.get(keyID)
	if !ok || idKey(key.ServicePrincipalID) != spID {
		writeError(w, http.StatusNotFound, "key not found")
		return
	}
	key.Status = "enabled"
	s.store.spKeys.set(keyID, key)
	core.WriteJSON(w, http.StatusOK, spKeyToJSON(key))
}

func (s *Server) handleDisableSPKey(w http.ResponseWriter, r *http.Request) {
	spID := r.PathValue("service_principal_id")
	if _, ok := s.store.servicePrincipals.get(spID); !ok {
		writeError(w, http.StatusNotFound, "service principal not found")
		return
	}
	keyID := r.PathValue("service_principal_key_id")
	key, ok := s.store.spKeys.get(keyID)
	if !ok || idKey(key.ServicePrincipalID) != spID {
		writeError(w, http.StatusNotFound, "key not found")
		return
	}
	key.Status = "disabled"
	s.store.spKeys.set(keyID, key)
	core.WriteJSON(w, http.StatusOK, spKeyToJSON(key))
}

func (s *Server) handleDeleteSPKey(w http.ResponseWriter, r *http.Request) {
	spID := r.PathValue("service_principal_id")
	if _, ok := s.store.servicePrincipals.get(spID); !ok {
		writeError(w, http.StatusNotFound, "service principal not found")
		return
	}
	keyID := r.PathValue("service_principal_key_id")
	key, ok := s.store.spKeys.get(keyID)
	if !ok || idKey(key.ServicePrincipalID) != spID {
		writeError(w, http.StatusNotFound, "key not found")
		return
	}
	s.store.spKeys.delete(keyID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleOAuth2Token(w http.ResponseWriter, _ *http.Request) {
	exp := time.Now().Add(1 * time.Hour)
	core.WriteJSON(w, http.StatusOK, oauthTokenResponse{
		AccessToken:    "mock-access-token-" + newUUID(),
		TokenType:      "Bearer",
		TokenExpiredAt: core.FormatRFC3339(exp),
		ExpiresIn:      3600,
	})
}
