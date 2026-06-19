package iam

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type userJSON struct {
	ID                      int        `json:"id"`
	Member                  userMember `json:"member"`
	Name                    string     `json:"name"`
	Code                    string     `json:"code"`
	Status                  string     `json:"status"`
	Description             string     `json:"description"`
	Otp                     userOtp    `json:"otp"`
	IsSecurityKeyRegistered bool       `json:"is_security_key_registered"`
	Email                   string     `json:"email"`
	IsPasswordless          bool       `json:"is_passwordless"`
	CreatedAt               string     `json:"created_at"`
	UpdatedAt               string     `json:"updated_at"`
}

type userMember struct {
	ID   int    `json:"id"`
	Code string `json:"code"`
}

type userOtp struct {
	Status          string `json:"status"`
	HasRecoveryCode bool   `json:"has_recovery_code"`
}

type createUserRequest struct {
	Name        string `json:"name"`
	Password    string `json:"password"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Email       string `json:"email,omitempty"`
}

type updateUserRequest struct {
	Name        string `json:"name"`
	Password    string `json:"password,omitempty"`
	Description string `json:"description"`
}

type registerEmailRequest struct {
	Email string `json:"email"`
}

func userToJSON(r *UserRecord) userJSON {
	return userJSON{
		ID:          r.ID,
		Member:      userMember{ID: 1, Code: "mem00001"},
		Name:        r.Name,
		Code:        r.Code,
		Status:      r.Status,
		Description: r.Description,
		Otp:         userOtp{Status: "deactivated"},
		Email:       r.Email,
		CreatedAt:   formatTime(r.CreatedAt),
		UpdatedAt:   formatTime(r.UpdatedAt),
	}
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	records := s.store.users.all()
	items := make([]userJSON, 0, len(records))
	for _, rec := range records {
		items = append(items, userToJSON(rec))
	}
	writePage(w, items)
}

const (
	passwordMinLength      = 12
	passwordAllowedSymbols = `!"#$%&'()*+,-./:;<=>?@[\]^_` + "`{|}~"
)

func validatePassword(pw string, policy passwordPolicyState) string {
	if pw == "" {
		return "password is required"
	}
	minLen := max(policy.MinLength, passwordMinLength)
	if len(pw) < minLen {
		return fmt.Sprintf("password must be at least %d characters", minLen)
	}
	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSymbol := false
	for _, ch := range pw {
		switch {
		case ch >= 'A' && ch <= 'Z':
			hasUpper = true
		case ch >= 'a' && ch <= 'z':
			hasLower = true
		case ch >= '0' && ch <= '9':
			hasDigit = true
		case strings.ContainsRune(passwordAllowedSymbols, ch):
			hasSymbol = true
		default:
			return "password contains invalid character"
		}
	}
	if !hasUpper && !hasLower {
		return "password must contain at least one letter"
	}
	if !hasDigit {
		return "password must contain at least one digit"
	}
	if policy.RequireUppercase && !hasUpper {
		return "password must contain at least one uppercase letter"
	}
	if policy.RequireLowercase && !hasLower {
		return "password must contain at least one lowercase letter"
	}
	if policy.RequireSymbols && !hasSymbol {
		return "password must contain at least one symbol"
	}
	return ""
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" || req.Code == "" {
		writeError(w, http.StatusBadRequest, "name and code are required")
		return
	}
	if msg := validatePassword(req.Password, s.store.getPasswordPolicy()); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	now := time.Now()
	rec := &UserRecord{
		ID:          s.store.nextID(),
		Name:        req.Name,
		Code:        req.Code,
		Password:    req.Password,
		Status:      "available",
		Description: req.Description,
		Email:       req.Email,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.store.users.set(idKey(rec.ID), rec)
	s.logger.Debug("user created", "id", rec.ID, "name", rec.Name)
	writeJSON(w, http.StatusCreated, userToJSON(rec))
}

func (s *Server) handleReadUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("user_id")
	rec, ok := s.store.users.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, userToJSON(rec))
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("user_id")
	rec, ok := s.store.users.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	var req updateUserRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Password != "" {
		if msg := validatePassword(req.Password, s.store.getPasswordPolicy()); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
	}
	rec.Name = req.Name
	rec.Description = req.Description
	if req.Password != "" {
		rec.Password = req.Password
	}
	rec.UpdatedAt = time.Now()
	s.store.users.set(id, rec)
	s.logger.Debug("user updated", "id", id)
	writeJSON(w, http.StatusOK, userToJSON(rec))
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("user_id")
	if !s.store.users.delete(id) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	s.logger.Debug("user deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRegisterEmail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("user_id")
	rec, ok := s.store.users.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	var req registerEmailRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rec.Email = req.Email
	rec.UpdatedAt = time.Now()
	s.store.users.set(id, rec)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUnregisterEmail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("user_id")
	rec, ok := s.store.users.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	rec.Email = ""
	rec.UpdatedAt = time.Now()
	s.store.users.set(id, rec)
	w.WriteHeader(http.StatusNoContent)
}

// handleListUserTrustedDevices returns an empty list of trusted devices.
func (s *Server) handleListUserTrustedDevices(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, []any{})
}

// handleListUserSecurityKeys returns an empty list of security keys.
func (s *Server) handleListUserSecurityKeys(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, []any{})
}

// handleDeactivateOTP is a no-op that returns 204.
func (s *Server) handleDeactivateOTP(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("user_id")
	if _, ok := s.store.users.get(id); !ok {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleClearTrustedDevices is a no-op that returns 204.
func (s *Server) handleClearTrustedDevices(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("user_id")
	if _, ok := s.store.users.get(id); !ok {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleDeleteTrustedDevice is a no-op that returns 204.
func (s *Server) handleDeleteTrustedDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("user_id")
	if _, ok := s.store.users.get(id); !ok {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleUpdateSecurityKey is a no-op that returns 200 with empty JSON.
func (s *Server) handleUpdateSecurityKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("user_id")
	if _, ok := s.store.users.get(id); !ok {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

// handleDeleteSecurityKey is a no-op that returns 204.
func (s *Server) handleDeleteSecurityKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("user_id")
	if _, ok := s.store.users.get(id); !ok {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
