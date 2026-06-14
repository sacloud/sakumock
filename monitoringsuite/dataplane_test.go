package monitoringsuite_test

import (
	"bytes"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang/snappy"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"

	"github.com/sacloud/sakumock/monitoringsuite"
)

func post(t *testing.T, url, contentType string, body []byte) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// dumpContains reports whether any dump file whose name starts with prefix
// contains all the given substrings (across that file).
func dumpContains(t *testing.T, dir, prefix string, subs ...string) bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dump dir: %v", err)
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		ok := true
		for _, s := range subs {
			if !strings.Contains(string(data), s) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func TestDataPlaneIngestAndDump(t *testing.T) {
	dumpDir := t.TempDir()
	srv := monitoringsuite.NewTestServer(monitoringsuite.Config{
		EnableDataPlane:  true,
		DataPlaneAddr:    "127.0.0.1:0", // ephemeral; real address via srv.DataPlaneAddr()
		DataPlaneDumpDir: dumpDir,
	})
	defer srv.Close()

	base := "http://" + srv.DataPlaneAddr()

	// Metrics via Prometheus remote-write (snappy + protobuf), acknowledged 204.
	rw := snappy.Encode(nil, encodeRemoteWrite("up", 1, 1700000000000))
	if code := post(t, base+"/prometheus/api/v1/write", "application/x-protobuf", rw); code != http.StatusNoContent {
		t.Errorf("remote-write status = %d, want 204", code)
	}

	// Logs via OTLP/HTTP (protobuf), acknowledged 200. LogsData is wire-compatible
	// with ExportLogsServiceRequest, so the server decodes it.
	logs := &logspb.LogsData{ResourceLogs: []*logspb.ResourceLogs{{
		ScopeLogs: []*logspb.ScopeLogs{{
			LogRecords: []*logspb.LogRecord{{
				Body: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "hello-from-test"}},
			}},
		}},
	}}}
	logBody, err := proto.Marshal(logs)
	if err != nil {
		t.Fatal(err)
	}
	if code := post(t, base+"/v1/logs", "application/x-protobuf", logBody); code != http.StatusOK {
		t.Errorf("otlp logs status = %d, want 200", code)
	}

	if !dumpContains(t, dumpDir, "metrics-remotewrite-", "__name__", "up", "\"value\":1") {
		t.Error("remote-write payload not dumped as expected")
	}
	if !dumpContains(t, dumpDir, "otlp-logs-", "hello-from-test") {
		t.Error("otlp logs payload not dumped as expected")
	}
}

func TestDataPlaneValidatesWithoutDump(t *testing.T) {
	// No dump dir and no debug: the body must still be decoded so malformed
	// payloads are rejected (the mock validates the wire format, not just acks).
	srv := monitoringsuite.NewTestServer(monitoringsuite.Config{
		EnableDataPlane: true,
		DataPlaneAddr:   "127.0.0.1:0",
	})
	defer srv.Close()
	base := "http://" + srv.DataPlaneAddr()

	// Malformed bodies → 400. (Valid snappy wrapping non-protobuf bytes for
	// remote-write; an invalid OTLP protobuf wire type for logs.)
	if code := post(t, base+"/prometheus/api/v1/write", "application/x-protobuf", snappy.Encode(nil, []byte{0xff, 0xff, 0xff})); code != http.StatusBadRequest {
		t.Errorf("malformed remote-write status = %d, want 400", code)
	}
	if code := post(t, base+"/v1/logs", "application/x-protobuf", []byte{0xff, 0xff, 0xff}); code != http.StatusBadRequest {
		t.Errorf("malformed otlp logs status = %d, want 400", code)
	}

	// Well-formed bodies still succeed even without a dump dir.
	rw := snappy.Encode(nil, encodeRemoteWrite("up", 1, 1700000000000))
	if code := post(t, base+"/prometheus/api/v1/write", "application/x-protobuf", rw); code != http.StatusNoContent {
		t.Errorf("valid remote-write status = %d, want 204", code)
	}
}

// encodeRemoteWrite builds a minimal Prometheus WriteRequest with one series
// (__name__=<name>) and one sample, mirroring the wire format the handler parses.
func encodeRemoteWrite(name string, value float64, tsMs int64) []byte {
	label := func(n, v string) []byte {
		var b []byte
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendBytes(b, []byte(n))
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendBytes(b, []byte(v))
		return b
	}
	var sample []byte
	sample = protowire.AppendTag(sample, 1, protowire.Fixed64Type)
	sample = protowire.AppendFixed64(sample, math.Float64bits(value))
	sample = protowire.AppendTag(sample, 2, protowire.VarintType)
	sample = protowire.AppendVarint(sample, uint64(tsMs))

	var ts []byte
	ts = protowire.AppendTag(ts, 1, protowire.BytesType)
	ts = protowire.AppendBytes(ts, label("__name__", name))
	ts = protowire.AppendTag(ts, 2, protowire.BytesType)
	ts = protowire.AppendBytes(ts, sample)

	var req []byte
	req = protowire.AppendTag(req, 1, protowire.BytesType)
	req = protowire.AppendBytes(req, ts)
	return req
}
