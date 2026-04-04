package kms

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// JSON types matching the KMS OpenAPI spec.

type keyResponse struct {
	ID            string   `json:"ID"`
	CreatedAt     string   `json:"CreatedAt"`
	ModifiedAt    string   `json:"ModifiedAt"`
	ServiceClass  string   `json:"ServiceClass"`
	Name          string   `json:"Name"`
	Description   string   `json:"Description"`
	KeyOrigin     string   `json:"KeyOrigin"`
	LatestVersion int      `json:"LatestVersion"`
	Status        string   `json:"Status"`
	Tags          []string `json:"Tags"`
}

type createKeyResponse struct {
	ID          string   `json:"ID"`
	CreatedAt   string   `json:"CreatedAt"`
	ModifiedAt  string   `json:"ModifiedAt"`
	Name        string   `json:"Name"`
	Description string   `json:"Description,omitempty"`
	KeyOrigin   string   `json:"KeyOrigin"`
	Tags        []string `json:"Tags"`
	PlainKey    string   `json:"PlainKey,omitempty"`
}

type wrappedKey struct {
	Key keyResponse `json:"Key"`
}

type wrappedCreateKey struct {
	Key createKeyResponse `json:"Key"`
}

type paginatedKeyList struct {
	Count int           `json:"Count"`
	From  int           `json:"From"`
	Total int           `json:"Total"`
	Keys  []keyResponse `json:"Keys"`
}

type createKeyRequest struct {
	Key struct {
		Name        string   `json:"Name"`
		Description string   `json:"Description"`
		KeyOrigin   string   `json:"KeyOrigin"`
		Tags        []string `json:"Tags"`
		PlainKey    string   `json:"PlainKey"`
	} `json:"Key"`
}

type updateKeyRequest struct {
	Key struct {
		Name        string   `json:"Name"`
		Description string   `json:"Description"`
		KeyOrigin   string   `json:"KeyOrigin"`
		Tags        []string `json:"Tags"`
	} `json:"Key"`
}

type changeStatusRequest struct {
	Key struct {
		Status string `json:"Status"`
	} `json:"Key"`
}

type scheduleDestructionRequest struct {
	Key struct {
		PendingDays int `json:"PendingDays"`
	} `json:"Key"`
}

type keyCipherResponse struct {
	Cipher string `json:"Cipher"`
}

type wrappedKeyCipher struct {
	Key keyCipherResponse `json:"Key"`
}

type keyPlainResponse struct {
	Plain string `json:"Plain"`
}

type wrappedKeyPlain struct {
	Key keyPlainResponse `json:"Key"`
}

type encryptRequest struct {
	Key struct {
		Plain string `json:"Plain"`
		Algo  string `json:"Algo"`
	} `json:"Key"`
}

type decryptRequest struct {
	Key struct {
		Cipher string `json:"Cipher"`
	} `json:"Key"`
}

func (s *Server) buildMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /kms/keys", s.handleListKeys)
	mux.HandleFunc("POST /kms/keys", s.handleCreateKey)
	mux.HandleFunc("GET /kms/keys/{resource_id}", s.handleReadKey)
	mux.HandleFunc("PUT /kms/keys/{resource_id}", s.handleUpdateKey)
	mux.HandleFunc("DELETE /kms/keys/{resource_id}", s.handleDeleteKey)
	mux.HandleFunc("POST /kms/keys/{resource_id}/rotate", s.handleRotateKey)
	mux.HandleFunc("POST /kms/keys/{resource_id}/status", s.handleChangeStatus)
	mux.HandleFunc("POST /kms/keys/{resource_id}/schedule-destruction", s.handleScheduleDestruction)
	mux.HandleFunc("POST /kms/keys/{resource_id}/encrypt", s.handleEncrypt)
	mux.HandleFunc("POST /kms/keys/{resource_id}/decrypt", s.handleDecrypt)
	return mux
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.latency > 0 {
		time.Sleep(s.latency)
	}
	rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
	s.mux.ServeHTTP(rw, r)
	slog.Info("request",
		"method", r.Method,
		"path", r.URL.Path,
		"status", rw.statusCode,
	)
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func formatTime(t time.Time) string {
	return t.Format(time.RFC3339Nano)
}

func keyRecordToResponse(k KeyRecord) keyResponse {
	return keyResponse{
		ID:            k.ID,
		CreatedAt:     formatTime(k.CreatedAt),
		ModifiedAt:    formatTime(k.ModifiedAt),
		ServiceClass:  "cloud/kms/key",
		Name:          k.Name,
		Description:   k.Description,
		KeyOrigin:     k.KeyOrigin,
		LatestVersion: k.LatestVersion,
		Status:        k.Status,
		Tags:          k.Tags,
	}
}

func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request) {
	keys := s.store.List()
	items := make([]keyResponse, len(keys))
	for i, k := range keys {
		items[i] = keyRecordToResponse(k)
	}
	slog.Debug("keys listed", "count", len(items))
	writeJSON(w, http.StatusOK, paginatedKeyList{
		Count: len(items),
		From:  0,
		Total: len(items),
		Keys:  items,
	})
}

func (s *Server) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	var req createKeyRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Key.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}
	origin := req.Key.KeyOrigin
	if origin == "" {
		origin = "generated"
	}
	if origin != "generated" && origin != "imported" {
		writeError(w, http.StatusBadRequest, "KeyOrigin must be 'generated' or 'imported'")
		return
	}
	k, err := s.store.Create(req.Key.Name, req.Key.Description, origin, req.Key.Tags)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	slog.Debug("key created", "id", k.ID, "name", k.Name)
	writeJSON(w, http.StatusCreated, wrappedCreateKey{
		Key: createKeyResponse{
			ID:          k.ID,
			CreatedAt:   formatTime(k.CreatedAt),
			ModifiedAt:  formatTime(k.ModifiedAt),
			Name:        k.Name,
			Description: k.Description,
			KeyOrigin:   k.KeyOrigin,
			Tags:        k.Tags,
		},
	})
}

func (s *Server) handleReadKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	k, err := s.store.Read(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, wrappedKey{Key: keyRecordToResponse(k)})
}

func (s *Server) handleUpdateKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	var req updateKeyRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	k, err := s.store.Update(id, req.Key.Name, req.Key.Description, req.Key.Tags)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, wrappedKey{Key: keyRecordToResponse(k)})
}

func (s *Server) handleDeleteKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	if err := s.store.Delete(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRotateKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	k, err := s.store.Rotate(id)
	if err != nil {
		if k.ID == "" {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			// Key exists but not active → 403 Forbidden
			w.WriteHeader(http.StatusForbidden)
		}
		return
	}
	writeJSON(w, http.StatusOK, wrappedKey{Key: keyRecordToResponse(k)})
}

func (s *Server) handleChangeStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	var req changeStatusRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	status := req.Key.Status
	if status != "active" && status != "restricted" && status != "suspended" {
		writeError(w, http.StatusBadRequest, "Status must be 'active', 'restricted', or 'suspended'")
		return
	}
	if err := s.store.ChangeStatus(id, status); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleScheduleDestruction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	var req scheduleDestructionRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Key.PendingDays < 7 || req.Key.PendingDays > 90 {
		writeError(w, http.StatusBadRequest, "PendingDays must be between 7 and 90")
		return
	}
	if err := s.store.ChangeStatus(id, "pending_destruction"); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleEncrypt(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	var req encryptRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.store.Read(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	ciphertext, err := s.store.Encrypt(id, []byte(req.Key.Plain))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, wrappedKeyCipher{
		Key: keyCipherResponse{Cipher: ciphertext},
	})
}

func (s *Server) handleDecrypt(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	var req decryptRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.store.Read(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	plaintext, err := s.store.Decrypt(id, req.Key.Cipher)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, wrappedKeyPlain{
		Key: keyPlainResponse{Plain: string(plaintext)},
	})
}

func readJSON(r *http.Request, v any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}
	defer r.Body.Close()
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to write response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
