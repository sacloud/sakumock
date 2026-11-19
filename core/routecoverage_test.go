package core_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sacloud/sakumock/core"
)

func TestAnsweredRoutes(t *testing.T) {
	schemas := map[string]map[int]*core.BodySchema{
		"GET /ok":      {http.StatusOK: nil},
		"GET /missing": {http.StatusOK: nil},
		"GET /unused":  {http.StatusOK: nil},
	}
	rv := core.NewResponseValidator(schemas, nil)
	call := func(key string, status int) {
		h := rv.Middleware("GET", key, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) })
		h(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, key, nil))
	}
	call("/ok", http.StatusOK)
	call("/missing", http.StatusNotFound)

	got := core.AnsweredRoutes(schemas)
	if len(got) != 1 || !got["GET /ok"] {
		t.Errorf("AnsweredRoutes = %v, want only GET /ok (an error response does not count)", got)
	}
}
