package core

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
)

// WriteJSON responds with v encoded as JSON and the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to write response", "error", err)
	}
}

// ReadJSON decodes the request body into v, rejecting an empty or malformed
// body with an error suitable for a 400 response.
func ReadJSON(r *http.Request, v any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return errors.New("empty request body")
	}
	return json.Unmarshal(body, v)
}
