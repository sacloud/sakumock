package addon_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// knownSDKGaps lists incompatibilities between the portal spec, which the mock
// follows, and the SDK client the tests use, each with the sakumock issue
// tracking it. Tests hitting one call expectSDKGap, which fails once the SDK
// catches up, so the entry and its fallbacks get removed.
var knownSDKGaps = map[string]struct{ issue, marker string }{
	// Portal 1.2.0 adds GetResourceResponse.resourceGroup; the SDK's 1.1.0
	// schema has additionalProperties: false and rejects it.
	"get-resource-group": {
		issue:  "https://github.com/sacloud/sakumock/issues/220",
		marker: `unexpected field "resourceGroup"`,
	},
}

// expectSDKGap reports whether err is the known SDK gap, so the caller can
// fall back to raw HTTP. It fails the test when the call succeeded (the gap is
// resolved) and aborts it when the call failed for another reason.
func expectSDKGap(t *testing.T, gap string, err error) bool {
	t.Helper()
	g, ok := knownSDKGaps[gap]
	if !ok {
		t.Fatalf("unknown SDK gap %q", gap)
	}
	if err == nil {
		t.Errorf("SDK gap %q (%s) no longer reproduces: remove it from knownSDKGaps along with its fallbacks", gap, g.issue)
		return false
	}
	if !strings.Contains(err.Error(), g.marker) {
		t.Fatal(err)
	}
	t.Logf("known SDK gap %q (%s): %v", gap, g.issue, err)
	return true
}

// rawResourceData fetches a resource group the way the SDK's Read does and
// returns its free-form data, for when the SDK cannot decode the response.
func rawResourceData(t *testing.T, url string) []byte {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d", url, res.StatusCode)
	}
	var body struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body.Data
}
