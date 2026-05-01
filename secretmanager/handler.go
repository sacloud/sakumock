package secretmanager

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// JSON request/response types matching the SecretManager OpenAPI spec.

type secretResponse struct {
	Name          string `json:"Name"`
	LatestVersion int    `json:"LatestVersion"`
}

type wrappedSecret struct {
	Secret secretResponse `json:"Secret"`
}

type createSecretRequest struct {
	Name  string `json:"Name"`
	Value string `json:"Value"`
}

type wrappedCreateSecret struct {
	Secret createSecretRequest `json:"Secret"`
}

type deleteSecretRequest struct {
	Name string `json:"Name"`
}

type wrappedDeleteSecret struct {
	Secret deleteSecretRequest `json:"Secret"`
}

type unveilRequest struct {
	Name    string `json:"Name"`
	Version *int   `json:"Version"`
}

type wrappedUnveilRequest struct {
	Secret unveilRequest `json:"Secret"`
}

type unveilResponse struct {
	Name    string `json:"Name"`
	Version *int   `json:"Version"`
	Value   string `json:"Value"`
}

type wrappedUnveilResponse struct {
	Secret unveilResponse `json:"Secret"`
}

type paginatedSecretList struct {
	Count   int              `json:"Count"`
	From    int              `json:"From"`
	Total   int              `json:"Total"`
	Secrets []secretResponse `json:"Secrets"`
}

func (s *Server) buildMux() *http.ServeMux {
	mux := http.NewServeMux()
	for _, r := range s.routeTable() {
		mux.HandleFunc(r.Method+" "+r.Path, r.handler)
	}
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

func (s *Server) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	vaultID := r.PathValue("vault_resource_id")
	secrets := s.store.List(vaultID)
	items := make([]secretResponse, len(secrets))
	for i, sec := range secrets {
		items[i] = secretResponse{Name: sec.Name, LatestVersion: sec.LatestVersion}
	}
	slog.Debug("secrets listed", "vault_id", vaultID, "count", len(items))
	writeJSON(w, http.StatusOK, paginatedSecretList{
		Count:   len(items),
		From:    0,
		Total:   len(items),
		Secrets: items,
	})
}

func (s *Server) handleCreateSecret(w http.ResponseWriter, r *http.Request) {
	vaultID := r.PathValue("vault_resource_id")
	var req wrappedCreateSecret
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Secret.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}
	latestVersion, err := s.store.Create(vaultID, req.Secret.Name, req.Secret.Value)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	slog.Debug("secret created", "vault_id", vaultID, "name", req.Secret.Name, "version", latestVersion)
	writeJSON(w, http.StatusCreated, wrappedSecret{
		Secret: secretResponse{Name: req.Secret.Name, LatestVersion: latestVersion},
	})
}

func (s *Server) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	vaultID := r.PathValue("vault_resource_id")
	var req wrappedDeleteSecret
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.Delete(vaultID, req.Secret.Name); err != nil {
		slog.Debug("delete failed", "vault_id", vaultID, "name", req.Secret.Name, "error", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	slog.Debug("secret deleted", "vault_id", vaultID, "name", req.Secret.Name)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUnveil(w http.ResponseWriter, r *http.Request) {
	vaultID := r.PathValue("vault_resource_id")
	var req wrappedUnveilRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	version := 0
	if req.Secret.Version != nil {
		version = *req.Secret.Version
	}
	value, actualVersion, err := s.store.Unveil(vaultID, req.Secret.Name, version)
	if err != nil {
		slog.Debug("unveil failed", "vault_id", vaultID, "name", req.Secret.Name, "error", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	slog.Debug("secret unveiled", "vault_id", vaultID, "name", req.Secret.Name, "version", actualVersion)
	writeJSON(w, http.StatusOK, wrappedUnveilResponse{
		Secret: unveilResponse{
			Name:    req.Secret.Name,
			Version: &actualVersion,
			Value:   value,
		},
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
