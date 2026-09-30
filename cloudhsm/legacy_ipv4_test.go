package cloudhsm_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/sacloud/sakumock/cloudhsm"
)

// TestLegacyIPv4Names covers clients built on the pre-1.2.0 SDK, which send
// and read Ipv4* instead of IPv4*.
func TestLegacyIPv4Names(t *testing.T) {
	srv := cloudhsm.NewTestServer(cloudhsm.Config{})
	defer closeAndCheck(t, srv)

	body := `{"CloudHSM":{"Name":"legacy","Description":"","Tags":[],"Ipv4NetworkAddress":"192.168.10.0","Ipv4PrefixLength":28}}`
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.TestURL()+"/is1b/api/cloud/1.1/cloudhsm/cloudhsms", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var got struct {
		CloudHSM map[string]any `json:"CloudHSM"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"IPv4NetworkAddress", "Ipv4NetworkAddress"} {
		if got.CloudHSM[name] != "192.168.10.0" {
			t.Errorf("%s = %v, want 192.168.10.0", name, got.CloudHSM[name])
		}
	}
	for _, name := range []string{"IPv4PrefixLength", "Ipv4PrefixLength"} {
		if got.CloudHSM[name] != float64(28) {
			t.Errorf("%s = %v, want 28", name, got.CloudHSM[name])
		}
	}
	for _, name := range []string{"IPv4Address", "Ipv4Address"} {
		if got.CloudHSM[name] == "" || got.CloudHSM[name] == nil {
			t.Errorf("%s is empty", name)
		}
	}
}
