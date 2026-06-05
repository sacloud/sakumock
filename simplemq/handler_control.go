package simplemq

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Control plane error response (SAKURA Cloud standard format).
type cpError struct {
	IsFatal   bool   `json:"is_fatal"`
	Serial    string `json:"serial"`
	Status    string `json:"status"`
	ErrorCode string `json:"error_code"`
	ErrorMsg  string `json:"error_msg"`
}

func writeCPError(w http.ResponseWriter, status int, code, msg string) {
	statusText := fmt.Sprintf("%d %s", status, http.StatusText(status))
	writeJSON(w, status, cpError{
		IsFatal:   true,
		Serial:    "00000000000000000000000000000000",
		Status:    statusText,
		ErrorCode: code,
		ErrorMsg:  msg,
	})
}

// Control plane JSON response types.

type cpSettings struct {
	VisibilityTimeoutSeconds int `json:"VisibilityTimeoutSeconds"`
	ExpireSeconds            int `json:"ExpireSeconds"`
}

type cpStatus struct {
	QueueName string `json:"QueueName"`
}

type cpProvider struct {
	ID           int    `json:"ID"`
	Class        string `json:"Class"`
	Name         string `json:"Name"`
	ServiceClass string `json:"ServiceClass"`
}

type cpCommonServiceItem struct {
	ID           string     `json:"ID"`
	Name         string     `json:"Name"`
	Description  *string    `json:"Description"`
	Settings     cpSettings `json:"Settings"`
	SettingsHash string     `json:"SettingsHash"`
	Status       cpStatus   `json:"Status"`
	ServiceClass string     `json:"ServiceClass"`
	Availability string     `json:"Availability"`
	CreatedAt    time.Time  `json:"CreatedAt"`
	ModifiedAt   time.Time  `json:"ModifiedAt"`
	Provider     cpProvider `json:"Provider"`
	Icon         any        `json:"Icon"`
	Tags         []string   `json:"Tags"`
}

func cpSettingsHash(vtSecs, expSecs int) string {
	h := md5.Sum(fmt.Appendf(nil, "%d:%d", vtSecs, expSecs))
	return fmt.Sprintf("%x", h)
}

func queueToCSI(q storedQueue) cpCommonServiceItem {
	var desc *string
	if q.Description != "" {
		d := q.Description
		desc = &d
	}
	tags := q.Tags
	if tags == nil {
		tags = []string{}
	}
	return cpCommonServiceItem{
		ID:           q.ID,
		Name:         q.Name,
		Description:  desc,
		Settings:     cpSettings{VisibilityTimeoutSeconds: q.VisibilityTimeoutSeconds, ExpireSeconds: q.ExpireSeconds},
		SettingsHash: cpSettingsHash(q.VisibilityTimeoutSeconds, q.ExpireSeconds),
		Status:       cpStatus{QueueName: q.Name},
		ServiceClass: "cloud/simplemq/1",
		Availability: "available",
		CreatedAt:    q.CreatedAt,
		ModifiedAt:   q.ModifiedAt,
		Provider:     cpProvider{ID: 5200001, Class: "simplemq", Name: "SimpleMQ", ServiceClass: "cloud/simplemq"},
		Icon:         nil,
		Tags:         tags,
	}
}

// Request decode types.

type cpCreateQueueRequest struct {
	CommonServiceItem struct {
		Name        string `json:"Name"`
		Description string `json:"Description"`
		Provider    struct {
			Class string `json:"Class"`
		} `json:"Provider"`
		Tags []string `json:"Tags"`
	} `json:"CommonServiceItem"`
}

type cpConfigQueueRequest struct {
	CommonServiceItem struct {
		Description string `json:"Description"`
		Settings    struct {
			VisibilityTimeoutSeconds int `json:"VisibilityTimeoutSeconds"`
			ExpireSeconds            int `json:"ExpireSeconds"`
		} `json:"Settings"`
		Tags []string `json:"Tags"`
	} `json:"CommonServiceItem"`
}

// Handlers.

func (s *Server) handleCreateQueue(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeCPError(w, http.StatusBadRequest, "bad_request", "failed to read body")
		return
	}
	var req cpCreateQueueRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeCPError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	csi := req.CommonServiceItem
	if err := validateQueueName(csi.Name); err != nil {
		writeCPError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if csi.Provider.Class != "simplemq" {
		writeCPError(w, http.StatusBadRequest, "bad_request", "Provider.Class must be simplemq")
		return
	}

	now := time.Now()
	q, err := s.store.CreateQueue(csi.Name, csi.Description, csi.Tags, 0, 0, now)
	if err != nil {
		if errors.Is(err, ErrQueueConflict) {
			writeCPError(w, http.StatusConflict, "conflict", "same queue name found")
			return
		}
		writeCPError(w, http.StatusInternalServerError, "internal_server_error", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"CommonServiceItem": queueToCSI(q),
		"Success":           true,
		"is_ok":             true,
	})
}

func (s *Server) handleListQueues(w http.ResponseWriter, r *http.Request) {
	queues, err := s.store.ListQueues()
	if err != nil {
		writeCPError(w, http.StatusInternalServerError, "internal_server_error", err.Error())
		return
	}
	items := make([]cpCommonServiceItem, len(queues))
	for i, q := range queues {
		items[i] = queueToCSI(q)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"From":               0,
		"Count":              len(items),
		"Total":              len(items),
		"CommonServiceItems": items,
		"is_ok":              true,
	})
}

func (s *Server) handleGetQueue(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	q, err := s.store.GetQueueByID(id)
	if err != nil {
		if errors.Is(err, ErrQueueNotFound) {
			writeCPError(w, http.StatusNotFound, "not_found", "対象が見つかりません。")
			return
		}
		writeCPError(w, http.StatusInternalServerError, "internal_server_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"CommonServiceItem": queueToCSI(q),
		"is_ok":             true,
	})
}

func (s *Server) handleConfigQueue(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeCPError(w, http.StatusBadRequest, "bad_request", "failed to read body")
		return
	}
	var req cpConfigQueueRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeCPError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	settings := req.CommonServiceItem.Settings
	if settings.VisibilityTimeoutSeconds < 5 || settings.VisibilityTimeoutSeconds > 900 {
		writeCPError(w, http.StatusBadRequest, "bad_request", "VisibilityTimeoutSeconds must be between 5 and 900")
		return
	}
	if settings.ExpireSeconds < 60 || settings.ExpireSeconds > 1209600 {
		writeCPError(w, http.StatusBadRequest, "bad_request", "ExpireSeconds must be between 60 and 1209600")
		return
	}

	now := time.Now()
	q, err := s.store.UpdateQueue(id, req.CommonServiceItem.Description, req.CommonServiceItem.Tags,
		settings.VisibilityTimeoutSeconds, settings.ExpireSeconds, now)
	if err != nil {
		if errors.Is(err, ErrQueueNotFound) {
			writeCPError(w, http.StatusNotFound, "not_found", "対象が見つかりません。")
			return
		}
		writeCPError(w, http.StatusInternalServerError, "internal_server_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"CommonServiceItem": queueToCSI(q),
		"Success":           true,
		"is_ok":             true,
	})
}

func (s *Server) handleDeleteQueue(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	q, err := s.store.GetQueueByID(id)
	if err != nil {
		if errors.Is(err, ErrQueueNotFound) {
			writeCPError(w, http.StatusNotFound, "not_found", "対象が見つかりません。")
			return
		}
		writeCPError(w, http.StatusInternalServerError, "internal_server_error", err.Error())
		return
	}

	if err := s.store.DeleteQueueByID(id); err != nil {
		if errors.Is(err, ErrQueueNotFound) {
			writeCPError(w, http.StatusNotFound, "not_found", "対象が見つかりません。")
			return
		}
		writeCPError(w, http.StatusInternalServerError, "internal_server_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"CommonServiceItem": queueToCSI(q),
		"Success":           true,
		"is_ok":             true,
	})
}

func (s *Server) handleGetMessageCount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	now := time.Now()

	count, err := s.store.CountMessages(id, now)
	if err != nil {
		if errors.Is(err, ErrQueueNotFound) {
			writeCPError(w, http.StatusNotFound, "not_found", "対象が見つかりません。")
			return
		}
		writeCPError(w, http.StatusInternalServerError, "internal_server_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"SimpleMQ": map[string]any{
			"result": "success",
			"count":  count,
		},
		"is_ok": true,
	})
}

func (s *Server) handleRotateAPIKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	newKey := uuid.New().String()
	now := time.Now()

	_, err := s.store.RotateAPIKey(id, newKey, now)
	if err != nil {
		if errors.Is(err, ErrQueueNotFound) {
			writeCPError(w, http.StatusNotFound, "not_found", "対象が見つかりません。")
			return
		}
		writeCPError(w, http.StatusInternalServerError, "internal_server_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"SimpleMQ": map[string]any{
			"result": "success",
			"apikey": newKey,
		},
		"is_ok": true,
	})
}

func (s *Server) handleClearMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.store.ClearMessages(id); err != nil {
		if errors.Is(err, ErrQueueNotFound) {
			writeCPError(w, http.StatusNotFound, "not_found", "対象が見つかりません。")
			return
		}
		writeCPError(w, http.StatusInternalServerError, "internal_server_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"SimpleMQ": map[string]any{
			"result": "success",
		},
		"is_ok": true,
	})
}
