package core

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

// BodyValidationErrorWriter writes the 400 response in the service's own
// error envelope. Every service's writeError already has this signature.
type BodyValidationErrorWriter func(w http.ResponseWriter, status int, message string)

// BodyValidator validates JSON request bodies against per-route schemas
// (typically the generated bodySchemas table of a service). A nil
// *BodyValidator is valid and disables validation, so services can build
// their route table before the validator is configured.
type BodyValidator struct {
	schemas  map[string]*BodySchema
	errWrite BodyValidationErrorWriter
}

// NewBodyValidator creates a validator over schemas keyed by
// "METHOD /path" (the mux pattern registered in the route table).
func NewBodyValidator(schemas map[string]*BodySchema, errWrite BodyValidationErrorWriter) *BodyValidator {
	return &BodyValidator{schemas: schemas, errWrite: errWrite}
}

// Middleware wraps next with request-body validation for the route registered
// as method+" "+path. The schema lookup happens once at wrap time; routes
// without a schema (e.g. /_sakumock/ helpers) get next back unchanged.
//
// Empty and non-JSON bodies pass through so the handler's own ReadJSON error
// handling stays authoritative for those; the validator only rejects bodies
// that parse but violate the schema. The body is restored before next runs.
func (bv *BodyValidator) Middleware(method, path string, next http.HandlerFunc) http.HandlerFunc {
	if bv == nil {
		return next
	}
	schema, ok := bv.schemas[method+" "+path]
	if !ok || schema == nil {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			bv.errWrite(w, http.StatusBadRequest, "failed to read request body")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		if len(body) == 0 {
			next(w, r)
			return
		}
		var v any
		if err := json.Unmarshal(body, &v); err != nil {
			next(w, r)
			return
		}
		if msg := schema.Validate(v); msg != "" {
			bv.errWrite(w, http.StatusBadRequest, msg)
			return
		}
		next(w, r)
	}
}
