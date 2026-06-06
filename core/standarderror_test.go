package core

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWriteStandardError(t *testing.T) {
	t.Run("explicit code", func(t *testing.T) {
		rec := httptest.NewRecorder()
		WriteStandardError(rec, 409, "conflict", "same name found")
		if rec.Code != 409 {
			t.Fatalf("status = %d, want 409", rec.Code)
		}
		var got StandardError
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		want := StandardError{
			IsFatal:   true,
			Serial:    "00000000000000000000000000000000",
			Status:    "409 Conflict",
			ErrorCode: "conflict",
			ErrorMsg:  "same name found",
		}
		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("code derived from status", func(t *testing.T) {
		rec := httptest.NewRecorder()
		WriteStandardError(rec, 404, "", "missing")
		var got StandardError
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.ErrorCode != "not_found" {
			t.Errorf("error_code = %q, want not_found", got.ErrorCode)
		}
	})
}
