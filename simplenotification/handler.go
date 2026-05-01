package simplenotification

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"time"
	"unicode/utf8"
)

const (
	groupIDPattern   = `^[0-9]{12}$`
	maxMessageLength = 2048
)

var groupIDRe = regexp.MustCompile(groupIDPattern)

// JSON types matching the Simple Notification OpenAPI spec.

type sendMessageRequest struct {
	Message string `json:"Message"`
}

type sendMessageResponse struct {
	IsOk bool `json:"is_ok"`
}

// Inspection JSON types for the /_sakumock/messages endpoint.
// This namespace is sakumock-specific and not part of the SAKURA Cloud API.

type inspectMessage struct {
	ID        string `json:"id"`
	GroupID   string `json:"group_id"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

type inspectMessageList struct {
	Messages []inspectMessage `json:"messages"`
}

func (s *Server) buildMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /commonserviceitem/{id}/simplenotification/message", s.handleSendMessage)
	mux.HandleFunc("GET /_sakumock/messages", s.handleInspectMessages)
	mux.HandleFunc("DELETE /_sakumock/messages", s.handleResetMessages)
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

func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !groupIDRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, "id must match "+groupIDPattern)
		return
	}
	var req sendMessageRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Message == "" {
		writeError(w, http.StatusBadRequest, "Message is required")
		return
	}
	if utf8.RuneCountInString(req.Message) > maxMessageLength {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Message must be %d characters or less", maxMessageLength))
		return
	}
	rec, err := s.store.Send(id, req.Message, time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	slog.Debug("notification message accepted", "id", id, "message_id", rec.ID)
	writeJSON(w, http.StatusAccepted, sendMessageResponse{IsOk: true})
}

func (s *Server) handleInspectMessages(w http.ResponseWriter, r *http.Request) {
	records := s.store.List()
	out := make([]inspectMessage, len(records))
	for i, rec := range records {
		out[i] = inspectMessage{
			ID:        rec.ID,
			GroupID:   rec.GroupID,
			Message:   rec.Message,
			CreatedAt: rec.CreatedAt.Format(time.RFC3339Nano),
		}
	}
	writeJSON(w, http.StatusOK, inspectMessageList{Messages: out})
}

func (s *Server) handleResetMessages(w http.ResponseWriter, r *http.Request) {
	s.store.Reset()
	w.WriteHeader(http.StatusNoContent)
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
