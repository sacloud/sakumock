package iam

import (
	"net/http"
	"time"

	"github.com/sacloud/sakumock/core"
)

type ssoProfileJSON struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	SpEntityID     string `json:"sp_entity_id"`
	SpAcsURL       string `json:"sp_acs_url"`
	IdpEntityID    string `json:"idp_entity_id"`
	IdpLoginURL    string `json:"idp_login_url"`
	IdpLogoutURL   string `json:"idp_logout_url"`
	IdpCertificate string `json:"idp_certificate"`
	Assigned       bool   `json:"assigned"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type createSSOProfileRequest struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	IdpEntityID    string `json:"idp_entity_id"`
	IdpLoginURL    string `json:"idp_login_url"`
	IdpLogoutURL   string `json:"idp_logout_url"`
	IdpCertificate string `json:"idp_certificate"`
}

type updateSSOProfileRequest struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	IdpEntityID    string `json:"idp_entity_id"`
	IdpLoginURL    string `json:"idp_login_url"`
	IdpLogoutURL   string `json:"idp_logout_url"`
	IdpCertificate string `json:"idp_certificate"`
}

func ssoToJSON(r *SSOProfileRecord) ssoProfileJSON {
	return ssoProfileJSON{
		ID:             r.ID,
		Name:           r.Name,
		Description:    r.Description,
		SpEntityID:     r.SpEntityID,
		SpAcsURL:       r.SpAcsURL,
		IdpEntityID:    r.IdpEntityID,
		IdpLoginURL:    r.IdpLoginURL,
		IdpLogoutURL:   r.IdpLogoutURL,
		IdpCertificate: r.IdpCertificate,
		Assigned:       r.Assigned,
		CreatedAt:      core.FormatRFC3339(r.CreatedAt),
		UpdatedAt:      core.FormatRFC3339(r.UpdatedAt),
	}
}

func (s *Server) handleListSSOProfiles(w http.ResponseWriter, _ *http.Request) {
	records := s.store.ssoProfiles.all()
	items := make([]ssoProfileJSON, 0, len(records))
	for _, rec := range records {
		items = append(items, ssoToJSON(rec))
	}
	writePage(w, items)
}

func (s *Server) handleCreateSSOProfile(w http.ResponseWriter, r *http.Request) {
	var req createSSOProfileRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	now := time.Now()
	rec := &SSOProfileRecord{
		ID:             s.store.nextID(),
		Name:           req.Name,
		Description:    req.Description,
		SpEntityID:     "https://secure.sakura.ad.jp/cloud/sso/saml/metadata",
		SpAcsURL:       "https://secure.sakura.ad.jp/cloud/sso/saml/acs",
		IdpEntityID:    req.IdpEntityID,
		IdpLoginURL:    req.IdpLoginURL,
		IdpLogoutURL:   req.IdpLogoutURL,
		IdpCertificate: req.IdpCertificate,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.store.ssoProfiles.set(idKey(rec.ID), rec)
	s.logger.Debug("SSO profile created", "id", rec.ID, "name", rec.Name)
	core.WriteJSON(w, http.StatusCreated, ssoToJSON(rec))
}

func (s *Server) handleReadSSOProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sso_profile_id")
	rec, ok := s.store.ssoProfiles.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "SSO profile not found")
		return
	}
	core.WriteJSON(w, http.StatusOK, ssoToJSON(rec))
}

func (s *Server) handleUpdateSSOProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sso_profile_id")
	rec, ok := s.store.ssoProfiles.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "SSO profile not found")
		return
	}
	var req updateSSOProfileRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rec.Name = req.Name
	rec.Description = req.Description
	rec.IdpEntityID = req.IdpEntityID
	rec.IdpLoginURL = req.IdpLoginURL
	rec.IdpLogoutURL = req.IdpLogoutURL
	rec.IdpCertificate = req.IdpCertificate
	rec.UpdatedAt = time.Now()
	s.store.ssoProfiles.set(id, rec)
	s.logger.Debug("SSO profile updated", "id", id)
	core.WriteJSON(w, http.StatusOK, ssoToJSON(rec))
}

func (s *Server) handleDeleteSSOProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sso_profile_id")
	if !s.store.ssoProfiles.delete(id) {
		writeError(w, http.StatusNotFound, "SSO profile not found")
		return
	}
	s.logger.Debug("SSO profile deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAssignSSOProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sso_profile_id")
	rec, ok := s.store.ssoProfiles.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "SSO profile not found")
		return
	}
	rec.Assigned = true
	rec.UpdatedAt = time.Now()
	s.store.ssoProfiles.set(id, rec)
	core.WriteJSON(w, http.StatusOK, ssoToJSON(rec))
}

func (s *Server) handleUnassignSSOProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sso_profile_id")
	rec, ok := s.store.ssoProfiles.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "SSO profile not found")
		return
	}
	rec.Assigned = false
	rec.UpdatedAt = time.Now()
	s.store.ssoProfiles.set(id, rec)
	core.WriteJSON(w, http.StatusOK, ssoToJSON(rec))
}
