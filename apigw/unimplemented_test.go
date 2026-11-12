package apigw_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/sacloud/sakumock/apigw"
)

// TestUnimplementedOperation verifies a spec operation listed in skipPaths
// answers 501 in the service's error envelope instead of 404.
func TestUnimplementedOperation(t *testing.T) {
	srv := apigw.NewTestServer(apigw.Config{})
	defer closeAndCheck(t, srv)

	req, err := http.NewRequest("POST", srv.TestURL()+"/certificates/acme", strings.NewReader("{}"))
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
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Message != "POST /certificates/acme is not implemented by sakumock" {
		t.Errorf("message = %q", body.Message)
	}
}
