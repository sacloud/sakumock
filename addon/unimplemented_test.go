package addon_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/sacloud/sakumock/addon"
)

// TestUnimplementedOperation verifies a spec operation listed in skipPaths
// answers 501 in the service's error envelope instead of 404.
func TestUnimplementedOperation(t *testing.T) {
	srv := addon.NewTestServer(addon.Config{})
	defer closeAndCheck(t, srv)

	req, err := http.NewRequest("POST", srv.TestURL()+"/cdn/rg-test/purge", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", resp.StatusCode)
	}
	var body struct {
		Errors []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Errors) != 1 || body.Errors[0].Code != "NotImplemented" || !strings.Contains(body.Errors[0].Message, "not implemented") {
		t.Errorf("unexpected error body: %+v", body)
	}
}
