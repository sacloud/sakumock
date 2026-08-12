package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func respSchemas() map[string]map[int]*BodySchema {
	return map[string]map[int]*BodySchema{
		"GET /status": {
			200: {
				Type:     "object",
				Required: []string{"enabled"},
				Properties: map[string]*BodySchema{
					"enabled": {Type: "boolean"},
				},
			},
			401: nil,
		},
		"POST /enable": {
			204: nil,
		},
	}
}

func serve(t *testing.T, rv *ResponseValidator, method, path string, h http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	rv.Middleware(method, path, h)(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestResponseValidatorValid(t *testing.T) {
	rv := NewResponseValidator(respSchemas(), nil)
	serve(t, rv, "GET", "/status", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"enabled": true}`))
	})
	serve(t, rv, "GET", "/status", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized) // declared, body unchecked
		w.Write([]byte(`whatever`))
	})
	serve(t, rv, "POST", "/enable", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if got := rv.Violations(); len(got) != 0 {
		t.Fatalf("expected no violations, got %+v", got)
	}
}

func TestResponseValidatorSchemaViolation(t *testing.T) {
	rv := NewResponseValidator(respSchemas(), nil)
	for range 3 {
		serve(t, rv, "GET", "/status", func(w http.ResponseWriter, _ *http.Request) {
			w.Write([]byte(`{"is_active": false}`))
		})
	}
	got := rv.Violations()
	if len(got) != 1 {
		t.Fatalf("expected 1 deduped violation, got %+v", got)
	}
	v := got[0]
	if v.Method != "GET" || v.Path != "/status" || v.Status != 200 || v.Count != 3 {
		t.Errorf("unexpected violation %+v", v)
	}
	if v.Message == "" {
		t.Error("violation message is empty")
	}
}

func TestResponseValidatorUndeclaredStatus(t *testing.T) {
	rv := NewResponseValidator(respSchemas(), nil)
	serve(t, rv, "POST", "/enable", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated) // spec declares only 204
		w.Write([]byte(`{}`))
	})
	got := rv.Violations()
	if len(got) != 1 || got[0].Status != 201 {
		t.Fatalf("expected undeclared-status violation, got %+v", got)
	}
}

func TestResponseValidatorUndeclaredErrorStatusAllowed(t *testing.T) {
	// Specs routinely omit error statuses the real API returns; an undeclared
	// 4xx/5xx is a spec gap, not mock drift.
	rv := NewResponseValidator(respSchemas(), nil)
	serve(t, rv, "POST", "/enable", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"detail": "not found"}`))
	})
	if got := rv.Violations(); len(got) != 0 {
		t.Fatalf("expected no violations for undeclared error status, got %+v", got)
	}
}

func TestResponseValidatorBadBody(t *testing.T) {
	rv := NewResponseValidator(respSchemas(), nil)
	serve(t, rv, "GET", "/status", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`[`)) // not valid JSON
	})
	serve(t, rv, "GET", "/status", func(http.ResponseWriter, *http.Request) {
		// nothing written: implicit 200 with an empty body
	})
	got := rv.Violations()
	if len(got) != 2 {
		t.Fatalf("expected 2 violations, got %+v", got)
	}
}

func TestResponseValidatorUnknownRouteAndReset(t *testing.T) {
	rv := NewResponseValidator(respSchemas(), nil)
	// No responseSchemas entry: handler passes through unchecked.
	serve(t, rv, "GET", "/_sakumock/spec-violations", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	if got := rv.Violations(); len(got) != 0 {
		t.Fatalf("expected no violations for unmapped route, got %+v", got)
	}

	serve(t, rv, "GET", "/status", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{}`))
	})
	if got := rv.Violations(); len(got) != 1 {
		t.Fatalf("expected 1 violation before reset, got %+v", got)
	}
	rv.Reset()
	if got := rv.Violations(); len(got) != 0 {
		t.Fatalf("expected no violations after reset, got %+v", got)
	}
}

func TestResponseValidatorNil(t *testing.T) {
	var rv *ResponseValidator
	rec := serve(t, rv, "GET", "/status", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"is_active": false}`))
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("nil validator altered the response: %d", rec.Code)
	}
	if got := rv.Violations(); got != nil {
		t.Fatalf("nil validator reported violations: %+v", got)
	}
	rv.Reset() // must not panic
}
