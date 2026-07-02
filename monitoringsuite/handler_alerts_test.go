package monitoringsuite_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	mssdk "github.com/sacloud/sacloud-sdk-go/api/monitoring-suite"

	"github.com/sacloud/sakumock/monitoringsuite"
)

// TestAlertRuleValidation exercises the request constraints declared in the
// OpenAPI spec (AlertRuleRequest / PatchedAlertRuleRequest): required fields,
// minLength and maxLength. Raw HTTP is used so status codes and invalid
// bodies the SDK would not produce can be asserted directly.
func TestAlertRuleValidation(t *testing.T) {
	srv := monitoringsuite.NewTestServer(monitoringsuite.Config{})
	defer srv.Close()
	client := newClient(t, srv.TestURL())
	ctx := t.Context()

	storage, err := mssdk.NewMetricsStorageOp(client).Create(ctx, mssdk.MetricsStorageCreateParams{Name: "metrics"})
	if err != nil {
		t.Fatal(err)
	}
	sid := ridOf(t, storage.GetResourceID())

	project, err := mssdk.NewAlertProjectOp(client).Create(ctx, mssdk.AlertProjectCreateParams{Name: "proj"})
	if err != nil {
		t.Fatal(err)
	}
	pid := ridOf(t, project.GetResourceID())

	rulesURL := srv.TestURL() + "/alerts/projects/" + pid + "/rules/"

	do := func(t *testing.T, method, url, body string) (int, []byte) {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		out, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, out
	}

	long := func(n int) string { return strings.Repeat("a", n) }
	valid := `{"metrics_storage_id": ` + sid + `, "query": "up == 0"`

	t.Run("create", func(t *testing.T) {
		cases := []struct {
			name string
			body string
			want int
		}{
			{"missing metrics_storage_id", `{"query": "up == 0"}`, http.StatusBadRequest},
			{"missing query", `{"metrics_storage_id": ` + sid + `}`, http.StatusBadRequest},
			{"empty query", `{"metrics_storage_id": ` + sid + `, "query": ""}`, http.StatusBadRequest},
			{"query too long", `{"metrics_storage_id": ` + sid + `, "query": "` + long(4097) + `"}`, http.StatusBadRequest},
			{"name too long", valid + `, "name": "` + long(257) + `"}`, http.StatusBadRequest},
			{"format too long", valid + `, "format": "` + long(257) + `"}`, http.StatusBadRequest},
			{"template too long", valid + `, "template": "` + long(257) + `"}`, http.StatusBadRequest},
			{"empty threshold_warning", valid + `, "threshold_warning": ""}`, http.StatusBadRequest},
			{"threshold_warning too long", valid + `, "threshold_warning": "` + long(257) + `"}`, http.StatusBadRequest},
			{"empty threshold_critical", valid + `, "threshold_critical": ""}`, http.StatusBadRequest},
			{"null thresholds accepted", valid + `, "threshold_warning": null, "threshold_critical": null}`, http.StatusCreated},
			// metrics_storage_id is required but nullable in the spec: the key
			// must be present, but an explicit null is valid.
			{"null metrics_storage_id accepted", `{"metrics_storage_id": null, "query": "up == 0"}`, http.StatusCreated},
			{"valid with thresholds", valid + `, "threshold_warning": ">= 10", "threshold_critical": ">= 100"}`, http.StatusCreated},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				status, body := do(t, http.MethodPost, rulesURL, c.body)
				if status != c.want {
					t.Fatalf("expected %d, got %d: %s", c.want, status, body)
				}
			})
		}
	})

	// A known-good rule for update tests.
	status, body := do(t, http.MethodPost, rulesURL, valid+`}`)
	if status != http.StatusCreated {
		t.Fatalf("setup create failed: %d: %s", status, body)
	}
	var created struct {
		UID string `json:"uid"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatal(err)
	}
	ruleURL := rulesURL + created.UID + "/"

	t.Run("update", func(t *testing.T) {
		cases := []struct {
			name   string
			method string
			body   string
			want   int
		}{
			// PUT bodies are AlertRuleRequest: required fields enforced.
			{"put missing required", http.MethodPut, `{"name": "renamed"}`, http.StatusBadRequest},
			{"put valid", http.MethodPut, valid + `, "threshold_warning": ">= 10"}`, http.StatusOK},
			// PATCH bodies are PatchedAlertRuleRequest: all fields optional,
			// but present fields still honor min/max length.
			{"patch empty body", http.MethodPatch, `{}`, http.StatusOK},
			{"patch empty threshold_warning", http.MethodPatch, `{"threshold_warning": ""}`, http.StatusBadRequest},
			{"patch threshold_critical too long", http.MethodPatch, `{"threshold_critical": "` + long(257) + `"}`, http.StatusBadRequest},
			{"patch empty query", http.MethodPatch, `{"query": ""}`, http.StatusBadRequest},
			{"patch valid threshold", http.MethodPatch, `{"threshold_warning": "< 5"}`, http.StatusOK},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				status, body := do(t, c.method, ruleURL, c.body)
				if status != c.want {
					t.Fatalf("expected %d, got %d: %s", c.want, status, body)
				}
			})
		}
	})
}
