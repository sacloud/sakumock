package cloudhsm

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

// API spec 1.2.0 renamed the CloudHSM fields Ipv4* to IPv4*. Clients built on
// the older SDK (terraform-provider-sakura <= v3.14.2) still send and expect
// the old names, so the mock accepts them in requests and returns both
// spellings in responses. Remove this once the provider has caught up.

// legacyIPv4Fields maps each pre-1.2.0 field name to its current name.
var legacyIPv4Fields = map[string]string{
	"Ipv4NetworkAddress": "IPv4NetworkAddress",
	"Ipv4PrefixLength":   "IPv4PrefixLength",
}

// legacyIPv4Names rewrites legacy Ipv4* keys of a {"CloudHSM": {...}} request
// body to their IPv4* names before the spec validator sees it. Bodies without
// legacy keys (or that are not JSON objects) pass through untouched.
func legacyIPv4Names(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil || (r.Method != http.MethodPost && r.Method != http.MethodPut) {
			next(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(renameLegacyIPv4(body)))
		next(w, r)
	}
}

func renameLegacyIPv4(body []byte) []byte {
	var top map[string]json.RawMessage
	if json.Unmarshal(body, &top) != nil || top["CloudHSM"] == nil {
		return body
	}
	var hsm map[string]json.RawMessage
	if json.Unmarshal(top["CloudHSM"], &hsm) != nil {
		return body
	}
	renamed := false
	for old, cur := range legacyIPv4Fields {
		v, ok := hsm[old]
		if !ok {
			continue
		}
		delete(hsm, old)
		if _, exists := hsm[cur]; !exists {
			hsm[cur] = v
		}
		renamed = true
	}
	if !renamed {
		return body
	}
	var err error
	if top["CloudHSM"], err = json.Marshal(hsm); err != nil {
		return body
	}
	out, err := json.Marshal(top)
	if err != nil {
		return body
	}
	return out
}

// legacyIPv4Response carries the pre-1.2.0 spellings of the IPv4* response
// fields; embed it in CloudHSM responses and fill it with newLegacyIPv4.
type legacyIPv4Response struct {
	LegacyIPv4NetworkAddress string `json:"Ipv4NetworkAddress"`
	LegacyIPv4PrefixLength   int    `json:"Ipv4PrefixLength"`
	LegacyIPv4Address        string `json:"Ipv4Address"`
}

func newLegacyIPv4(h CloudHSMRecord) legacyIPv4Response {
	return legacyIPv4Response{
		LegacyIPv4NetworkAddress: h.IPv4NetworkAddress,
		LegacyIPv4PrefixLength:   h.IPv4PrefixLength,
		LegacyIPv4Address:        h.IPv4Address,
	}
}
