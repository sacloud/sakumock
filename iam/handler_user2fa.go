package iam

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/sacloud/sakumock/core"
)

type trustedDeviceJSON struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type securityKeyJSON struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	SignCount    int     `json:"sign_count"`
	AAGUID       string  `json:"aaguid"`
	RegisteredAt string  `json:"registered_at"`
	LastUsedAt   *string `json:"last_used_at"`
}

type updateSecurityKeyRequest struct {
	Name string `json:"name"`
}

// seedTrustedDeviceRequest and seedSecurityKeyRequest are the bodies of the
// mock-only seeding endpoints; every field is optional.
type seedTrustedDeviceRequest struct {
	Name string `json:"name"`
}

type seedSecurityKeyRequest struct {
	Name      string `json:"name"`
	AAGUID    string `json:"aaguid"`
	SignCount int    `json:"sign_count"`
}

func trustedDeviceToJSON(r *UserTrustedDeviceRecord) trustedDeviceJSON {
	return trustedDeviceJSON{
		ID:        r.ID,
		Name:      r.Name,
		CreatedAt: core.FormatRFC3339(r.CreatedAt),
	}
}

func securityKeyToJSON(r *UserSecurityKeyRecord) securityKeyJSON {
	var lastUsedAt *string
	if r.LastUsedAt != nil {
		v := core.FormatRFC3339(*r.LastUsedAt)
		lastUsedAt = &v
	}
	return securityKeyJSON{
		ID:           r.ID,
		Name:         r.Name,
		SignCount:    r.SignCount,
		AAGUID:       r.AAGUID,
		RegisteredAt: core.FormatRFC3339(r.RegisteredAt),
		LastUsedAt:   lastUsedAt,
	}
}

// readSeedRequest decodes the body of a seeding endpoint. Every field there is
// optional, so an empty body is valid — unlike core.ReadJSON, which rejects it.
func readSeedRequest(r *http.Request, v any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	return json.Unmarshal(body, v)
}

// user2faUser resolves the {user_id} path value, writing 404 when no such user
// exists. Every 2FA endpoint is scoped to a user.
func (s *Server) user2faUser(w http.ResponseWriter, r *http.Request) (*UserRecord, bool) {
	rec, ok := s.store.users.get(r.PathValue("user_id"))
	if !ok {
		writeError(w, http.StatusNotFound, "user not found")
		return nil, false
	}
	return rec, true
}

// handleDeactivateOTP turns off the user's one-time password. Activation
// happens in the browser, so there is nothing for the mock to undo.
func (s *Server) handleDeactivateOTP(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.user2faUser(w, r); !ok {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListUserTrustedDevices(w http.ResponseWriter, r *http.Request) {
	user, ok := s.user2faUser(w, r)
	if !ok {
		return
	}
	devices := s.store.userTrustedDevices(user.ID)
	items := make([]trustedDeviceJSON, 0, len(devices))
	for _, d := range devices {
		items = append(items, trustedDeviceToJSON(d))
	}
	writePage(w, items)
}

func (s *Server) handleDeleteTrustedDevice(w http.ResponseWriter, r *http.Request) {
	user, ok := s.user2faUser(w, r)
	if !ok {
		return
	}
	key := r.PathValue("trusted_device_id")
	device, ok := s.store.trustedDevices.get(key)
	if !ok || device.UserID != user.ID {
		writeError(w, http.StatusNotFound, "trusted device not found")
		return
	}
	s.store.trustedDevices.delete(key)
	s.logger.Debug("trusted device deleted", "user_id", user.ID, "id", device.ID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleClearTrustedDevices(w http.ResponseWriter, r *http.Request) {
	user, ok := s.user2faUser(w, r)
	if !ok {
		return
	}
	for _, d := range s.store.userTrustedDevices(user.ID) {
		s.store.trustedDevices.delete(idKey(d.ID))
	}
	s.logger.Debug("trusted devices cleared", "user_id", user.ID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListUserSecurityKeys(w http.ResponseWriter, r *http.Request) {
	user, ok := s.user2faUser(w, r)
	if !ok {
		return
	}
	keys := s.store.userSecurityKeys(user.ID)
	items := make([]securityKeyJSON, 0, len(keys))
	for _, k := range keys {
		items = append(items, securityKeyToJSON(k))
	}
	writePage(w, items)
}

// securityKey resolves the {security_key_id} path value within the user,
// writing 404 when the key does not belong to them.
func (s *Server) securityKey(w http.ResponseWriter, r *http.Request, user *UserRecord) (*UserSecurityKeyRecord, bool) {
	key, ok := s.store.securityKeys.get(r.PathValue("security_key_id"))
	if !ok || key.UserID != user.ID {
		writeError(w, http.StatusNotFound, "security key not found")
		return nil, false
	}
	return key, true
}

func (s *Server) handleReadSecurityKey(w http.ResponseWriter, r *http.Request) {
	user, ok := s.user2faUser(w, r)
	if !ok {
		return
	}
	key, ok := s.securityKey(w, r, user)
	if !ok {
		return
	}
	core.WriteJSON(w, http.StatusOK, securityKeyToJSON(key))
}

func (s *Server) handleUpdateSecurityKey(w http.ResponseWriter, r *http.Request) {
	user, ok := s.user2faUser(w, r)
	if !ok {
		return
	}
	key, ok := s.securityKey(w, r, user)
	if !ok {
		return
	}
	var req updateSecurityKeyRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	key.Name = req.Name
	s.store.securityKeys.set(idKey(key.ID), key)
	s.logger.Debug("security key updated", "user_id", user.ID, "id", key.ID)
	core.WriteJSON(w, http.StatusOK, securityKeyToJSON(key))
}

func (s *Server) handleDeleteSecurityKey(w http.ResponseWriter, r *http.Request) {
	user, ok := s.user2faUser(w, r)
	if !ok {
		return
	}
	key, ok := s.securityKey(w, r, user)
	if !ok {
		return
	}
	s.store.securityKeys.delete(idKey(key.ID))
	s.logger.Debug("security key deleted", "user_id", user.ID, "id", key.ID)
	w.WriteHeader(http.StatusNoContent)
}

// handleSeedTrustedDevice registers a trusted device for the user. The real API
// creates one when the user checks "trust this device" during a two-factor
// login, which the mock cannot reproduce, so this endpoint stands in for it.
func (s *Server) handleSeedTrustedDevice(w http.ResponseWriter, r *http.Request) {
	user, ok := s.user2faUser(w, r)
	if !ok {
		return
	}
	var req seedTrustedDeviceRequest
	if err := readSeedRequest(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	device := &UserTrustedDeviceRecord{
		ID:        s.store.nextID(),
		UserID:    user.ID,
		Name:      req.Name,
		CreatedAt: time.Now(),
	}
	if device.Name == "" {
		device.Name = "mock-device"
	}
	s.store.trustedDevices.set(idKey(device.ID), device)
	s.logger.Debug("trusted device seeded", "user_id", user.ID, "id", device.ID)
	core.WriteJSON(w, http.StatusCreated, trustedDeviceToJSON(device))
}

// handleSeedSecurityKey registers a security key for the user. WebAuthn
// registration happens in a browser, so this endpoint stands in for it.
func (s *Server) handleSeedSecurityKey(w http.ResponseWriter, r *http.Request) {
	user, ok := s.user2faUser(w, r)
	if !ok {
		return
	}
	var req seedSecurityKeyRequest
	if err := readSeedRequest(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	key := &UserSecurityKeyRecord{
		ID:           s.store.nextID(),
		UserID:       user.ID,
		Name:         req.Name,
		SignCount:    req.SignCount,
		AAGUID:       req.AAGUID,
		RegisteredAt: time.Now(),
	}
	if key.Name == "" {
		key.Name = "mock-security-key"
	}
	if key.AAGUID == "" {
		key.AAGUID = newUUID()
	}
	s.store.securityKeys.set(idKey(key.ID), key)
	s.logger.Debug("security key seeded", "user_id", user.ID, "id", key.ID)
	core.WriteJSON(w, http.StatusCreated, securityKeyToJSON(key))
}
