package core

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// StandardError is the SAKURA Cloud standard error envelope returned by
// commonserviceitem-based endpoints.
type StandardError struct {
	IsFatal   bool   `json:"is_fatal"`
	Serial    string `json:"serial"`
	Status    string `json:"status"`
	ErrorCode string `json:"error_code"`
	ErrorMsg  string `json:"error_msg"`
}

// WriteStandardError writes a fatal StandardError with the given HTTP status.
// code is the machine-readable error_code; when empty it is derived from the
// status text (e.g. 404 -> "not_found"). msg is the human-readable error_msg.
func WriteStandardError(w http.ResponseWriter, status int, code, msg string) {
	if code == "" {
		code = strings.ReplaceAll(strings.ToLower(http.StatusText(status)), " ", "_")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(StandardError{
		IsFatal:   true,
		Serial:    "00000000000000000000000000000000",
		Status:    fmt.Sprintf("%d %s", status, http.StatusText(status)),
		ErrorCode: code,
		ErrorMsg:  msg,
	}); err != nil {
		slog.Error("failed to write error response", "error", err)
	}
}
