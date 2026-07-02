package core_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sacloud/sakumock/core"
)

func testErrWriter(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":%q}`, message)
}

func TestBodyValidatorMiddleware(t *testing.T) {
	schemas := map[string]*core.BodySchema{
		"POST /things": {
			Type:     "object",
			Required: []string{"name"},
			Properties: map[string]*core.BodySchema{
				"name": {Type: "string", MinLength: core.IntPtr(1)},
			},
		},
	}
	bv := core.NewBodyValidator(schemas, testErrWriter)

	echo := func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}

	do := func(t *testing.T, h http.HandlerFunc, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/things", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h(rec, req)
		return rec
	}

	t.Run("valid body passes and is restored for the handler", func(t *testing.T) {
		h := bv.Middleware("POST", "/things", echo)
		body := `{"name": "ok"}`
		rec := do(t, h, body)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if rec.Body.String() != body {
			t.Fatalf("handler read %q, want %q", rec.Body.String(), body)
		}
	})

	t.Run("violation returns 400 via the error writer", func(t *testing.T) {
		h := bv.Middleware("POST", "/things", echo)
		rec := do(t, h, `{"name": ""}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		want := `{"error":"name must not be empty"}`
		if rec.Body.String() != want {
			t.Fatalf("body = %q, want %q", rec.Body.String(), want)
		}
	})

	t.Run("missing required returns 400", func(t *testing.T) {
		h := bv.Middleware("POST", "/things", echo)
		rec := do(t, h, `{}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("empty body passes through to the handler", func(t *testing.T) {
		h := bv.Middleware("POST", "/things", echo)
		rec := do(t, h, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (handler decides)", rec.Code)
		}
	})

	t.Run("malformed JSON passes through to the handler", func(t *testing.T) {
		h := bv.Middleware("POST", "/things", echo)
		rec := do(t, h, `{"name": `)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (handler decides)", rec.Code)
		}
		if rec.Body.String() != `{"name": ` {
			t.Fatalf("body not restored: %q", rec.Body.String())
		}
	})

	t.Run("route without schema returns next unchanged", func(t *testing.T) {
		called := false
		next := func(w http.ResponseWriter, r *http.Request) { called = true }
		h := bv.Middleware("GET", "/things", next)
		do(t, h, `{"name": ""}`)
		if !called {
			t.Fatal("next was not called")
		}
	})

	t.Run("nil validator returns next unchanged", func(t *testing.T) {
		var nilBV *core.BodyValidator
		called := false
		next := func(w http.ResponseWriter, r *http.Request) { called = true }
		h := nilBV.Middleware("POST", "/things", next)
		do(t, h, `{"name": ""}`)
		if !called {
			t.Fatal("next was not called")
		}
	})
}

func TestWithNonEmpty(t *testing.T) {
	newSchemas := func() map[string]*core.BodySchema {
		return map[string]*core.BodySchema{
			"POST /keys": {
				Type:     "object",
				Required: []string{"Key"},
				Properties: map[string]*core.BodySchema{
					"Key": {
						Type:     "object",
						Required: []string{"Name"},
						Properties: map[string]*core.BodySchema{
							"Name":  {Type: "string", MaxLength: core.IntPtr(255)},
							"Count": {Type: "integer"},
						},
					},
				},
			},
		}
	}

	do := func(t *testing.T, bv *core.BodyValidator, body string) int {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/keys", strings.NewReader(body))
		rec := httptest.NewRecorder()
		bv.Middleware("POST", "/keys", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})(rec, req)
		return rec.Code
	}

	t.Run("rejects empty string at the overlaid path", func(t *testing.T) {
		bv := core.NewBodyValidator(newSchemas(), testErrWriter,
			core.WithNonEmpty(map[string][]string{"POST /keys": {"Key.Name"}}))
		if got := do(t, bv, `{"Key": {"Name": ""}}`); got != http.StatusBadRequest {
			t.Fatalf("empty Name: status = %d, want 400", got)
		}
		if got := do(t, bv, `{"Key": {"Name": "k"}}`); got != http.StatusOK {
			t.Fatalf("valid Name: status = %d, want 200", got)
		}
	})

	t.Run("does not mutate the passed schemas", func(t *testing.T) {
		schemas := newSchemas()
		core.NewBodyValidator(schemas, testErrWriter,
			core.WithNonEmpty(map[string][]string{"POST /keys": {"Key.Name"}}))
		if schemas["POST /keys"].Properties["Key"].Properties["Name"].MinLength != nil {
			t.Fatal("original schema gained a MinLength")
		}
	})

	t.Run("keeps an existing spec minLength", func(t *testing.T) {
		schemas := newSchemas()
		schemas["POST /keys"].Properties["Key"].Properties["Name"].MinLength = core.IntPtr(5)
		bv := core.NewBodyValidator(schemas, testErrWriter,
			core.WithNonEmpty(map[string][]string{"POST /keys": {"Key.Name"}}))
		if got := do(t, bv, `{"Key": {"Name": "abc"}}`); got != http.StatusBadRequest {
			t.Fatalf("3-char Name with minLength 5: status = %d, want 400", got)
		}
	})

	mustPanic := func(t *testing.T, fields map[string][]string) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic")
			}
		}()
		core.NewBodyValidator(newSchemas(), testErrWriter, core.WithNonEmpty(fields))
	}

	t.Run("panics on unknown route", func(t *testing.T) {
		mustPanic(t, map[string][]string{"POST /nope": {"Key.Name"}})
	})
	t.Run("panics on unknown property", func(t *testing.T) {
		mustPanic(t, map[string][]string{"POST /keys": {"Key.Nope"}})
	})
	t.Run("panics on non-string property", func(t *testing.T) {
		mustPanic(t, map[string][]string{"POST /keys": {"Key.Count"}})
	})
}
