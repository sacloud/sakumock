package simplenotification_test

import (
	"io"
	"net/http"
	"testing"

	"github.com/sacloud/sakumock/simplenotification"
)

func getStatus(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("drain: %v", err)
	}
	return resp.StatusCode
}

func TestFaultInjected(t *testing.T) {
	srv := simplenotification.NewTestServer(simplenotification.Config{Fault: []string{"500:1"}})
	defer srv.Close()

	if status := getStatus(t, srv.TestURL()+"/commonserviceitem"); status != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", status)
	}
}

func TestFaultInspectionBypassed(t *testing.T) {
	// Inspection endpoints (/_sakumock/...) are exempt from fault injection.
	srv := simplenotification.NewTestServer(simplenotification.Config{Fault: []string{"500:1"}})
	defer srv.Close()

	if status := getStatus(t, srv.TestURL()+"/_sakumock/messages"); status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
}
