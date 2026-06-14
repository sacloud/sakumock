package monitoringsuite

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/golang/snappy"
	"github.com/sacloud/sakumock/core"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// dataPlane is the telemetry ingest listener: it accepts Prometheus remote-write
// (metrics) and OTLP/HTTP (logs, traces). Every request is decoded to validate
// its wire format — a malformed body is rejected with 400 — so the mock checks
// what is actually sent instead of blindly acking. There is no query side.
//
// This mirrors the sacloud-otel-collector `sacloud` exporter's data flow:
// metrics go out as remote-write, logs/traces as OTLP/HTTP. There is
// deliberately no OTLP /v1/metrics endpoint — metrics never travel that way.
//
// The decoded payload is additionally logged and/or written as JSON only when
// --debug or a dump dir is set.
type dataPlane struct {
	server  *http.Server
	ln      net.Listener
	dumpDir string
	debug   bool
	logger  *slog.Logger
	seq     atomic.Uint64
}

// startDataPlane binds the data-plane address and serves the ingest endpoints in
// a background goroutine until Close.
func startDataPlane(cfg Config, logger *slog.Logger) (*dataPlane, error) {
	if cfg.DataPlaneDumpDir != "" {
		if err := os.MkdirAll(cfg.DataPlaneDumpDir, 0o755); err != nil {
			return nil, fmt.Errorf("create data plane dump dir %q: %w", cfg.DataPlaneDumpDir, err)
		}
	}
	ln, err := net.Listen("tcp", cfg.DataPlaneAddr)
	if err != nil {
		return nil, fmt.Errorf("data plane listen on %s: %w", cfg.DataPlaneAddr, err)
	}
	dp := &dataPlane{
		ln:      ln,
		dumpDir: cfg.DataPlaneDumpDir,
		debug:   cfg.Debug,
		logger:  logger,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /prometheus/api/v1/write", dp.handleRemoteWrite)
	mux.HandleFunc("POST /v1/logs", dp.handleOTLP("logs", func() proto.Message { return &logspb.LogsData{} },
		func(m proto.Message) int { return len(m.(*logspb.LogsData).ResourceLogs) }))
	mux.HandleFunc("POST /v1/traces", dp.handleOTLP("traces", func() proto.Message { return &tracepb.TracesData{} },
		func(m proto.Message) int { return len(m.(*tracepb.TracesData).ResourceSpans) }))
	dp.server = &http.Server{Handler: mux}
	go func() {
		if err := core.ServeListener(dp.server, ln, cfg.tls); err != nil && err != http.ErrServerClosed {
			logger.Error("data plane server stopped", "error", err)
		}
	}()
	logger.Info("telemetry data plane listening (ingest only)",
		"addr", ln.Addr().String(),
		"scheme", cfg.tls.Scheme(),
		"remote_write", "POST /prometheus/api/v1/write",
		"otlp_http", "POST /v1/{logs,traces}",
		"dump_dir", cfg.DataPlaneDumpDir,
	)
	return dp, nil
}

// Addr returns the data plane's listen address (useful when a :0 port was used).
func (dp *dataPlane) Addr() string {
	if dp == nil {
		return ""
	}
	return dp.ln.Addr().String()
}

// Close stops the data plane listener. It is nil-safe.
func (dp *dataPlane) Close() {
	if dp == nil {
		return
	}
	_ = dp.server.Close()
}

// observing reports whether received payloads are logged or dumped, on top of
// the validation decoding that always happens.
func (dp *dataPlane) observing() bool { return dp.debug || dp.dumpDir != "" }

// handleRemoteWrite accepts a Prometheus remote-write request (snappy-compressed
// protobuf), validates it, and acknowledges it with 204 — or 400 if malformed.
func (dp *dataPlane) handleRemoteWrite(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	req, err := decodeRemoteWrite(body)
	if err != nil {
		dp.logger.Warn("data plane rejected", "signal", "metrics-remotewrite", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	dp.record("metrics-remotewrite", len(body), req, len(req.Timeseries))
}

// handleOTLP returns a handler for an OTLP/HTTP signal. The request body is an
// Export<Signal>ServiceRequest, which is wire-compatible with the otlp <Signal>Data
// message (both carry the resource list at field 1), so it is decoded into the
// latter — avoiding the collector service packages that would link gRPC. A
// malformed body is rejected with 400.
func (dp *dataPlane) handleOTLP(signal string, newMsg func() proto.Message, count func(proto.Message) int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isJSON := strings.Contains(r.Header.Get("Content-Type"), "json")
		gzipped := strings.Contains(r.Header.Get("Content-Encoding"), "gzip")
		body, err := io.ReadAll(r.Body)
		_ = r.Body.Close()
		var m proto.Message
		if err == nil {
			m, err = decodeOTLP(body, newMsg, isJSON, gzipped)
		}
		if err != nil {
			dp.logger.Warn("data plane rejected", "signal", "otlp-"+signal, "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// OTLP/HTTP success: 200 with an (empty) Export<Signal>ServiceResponse.
		if isJSON {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		} else {
			w.Header().Set("Content-Type", "application/x-protobuf")
			w.WriteHeader(http.StatusOK)
		}
		dp.record("otlp-"+signal, len(body), m, count(m))
	}
}

// decodeRemoteWrite snappy-decompresses and parses a Prometheus remote-write body.
func decodeRemoteWrite(body []byte) (*rwRequest, error) {
	raw, err := snappy.Decode(nil, body)
	if err != nil {
		return nil, fmt.Errorf("snappy decode: %w", err)
	}
	return parseRemoteWrite(raw)
}

// decodeOTLP gunzips (when needed) and unmarshals an OTLP/HTTP body into a fresh
// message from newMsg, as protobuf or JSON per isJSON.
func decodeOTLP(body []byte, newMsg func() proto.Message, isJSON, gzipped bool) (proto.Message, error) {
	raw := body
	if gzipped {
		zr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("gzip: %w", err)
		}
		defer func() { _ = zr.Close() }()
		if raw, err = io.ReadAll(zr); err != nil {
			return nil, fmt.Errorf("gzip: %w", err)
		}
	}
	m := newMsg()
	var err error
	if isJSON {
		err = protojson.Unmarshal(raw, m)
	} else {
		err = proto.Unmarshal(raw, m)
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

// record reports a received payload: an Info line per request (so a local user
// sees telemetry arriving) and, when --debug or a dump dir is set, the full
// payload logged and/or written as JSON. Validation already happened in the
// handler; this only reports what was received.
func (dp *dataPlane) record(signal string, nbytes int, v any, count int) {
	dp.logger.Info("data plane received", "signal", signal, "bytes", nbytes, "items", count)
	if !dp.observing() {
		return
	}
	js, err := marshalJSON(v)
	if err != nil {
		dp.logger.Warn("data plane marshal failed", "signal", signal, "error", err)
		return
	}
	if dp.debug {
		dp.logger.Debug("data plane payload", "signal", signal, "json", string(js))
	}
	if dp.dumpDir != "" {
		name := fmt.Sprintf("%s-%06d.json", signal, dp.seq.Add(1))
		if err := os.WriteFile(filepath.Join(dp.dumpDir, name), append(js, '\n'), 0o644); err != nil {
			dp.logger.Warn("data plane dump failed", "file", name, "error", err)
		}
	}
}

func marshalJSON(v any) ([]byte, error) {
	if m, ok := v.(proto.Message); ok {
		return protojson.Marshal(m)
	}
	return json.Marshal(v)
}

// ---- Prometheus remote-write decoding (protowire, no generated code) ----
//
// WriteRequest{ repeated TimeSeries timeseries = 1 }
// TimeSeries{ repeated Label labels = 1; repeated Sample samples = 2 }
// Label{ string name = 1; string value = 2 }
// Sample{ double value = 1; int64 timestamp = 2 }

type rwSample struct {
	Value       float64 `json:"value"`
	TimestampMs int64   `json:"timestamp_ms"`
}

type rwSeries struct {
	Labels  map[string]string `json:"labels"`
	Samples []rwSample        `json:"samples"`
}

type rwRequest struct {
	Timeseries []rwSeries `json:"timeseries"`
}

func parseRemoteWrite(b []byte) (*rwRequest, error) {
	req := &rwRequest{Timeseries: []rwSeries{}}
	err := eachField(b, func(num protowire.Number, typ protowire.Type, b []byte) (int, error) {
		if num == 1 && typ == protowire.BytesType {
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return 0, protowire.ParseError(n)
			}
			ts, err := parseTimeSeries(v)
			if err != nil {
				return 0, err
			}
			req.Timeseries = append(req.Timeseries, ts)
			return n, nil
		}
		return skipField(num, typ, b)
	})
	return req, err
}

func parseTimeSeries(b []byte) (rwSeries, error) {
	s := rwSeries{Labels: map[string]string{}, Samples: []rwSample{}}
	err := eachField(b, func(num protowire.Number, typ protowire.Type, b []byte) (int, error) {
		switch {
		case num == 1 && typ == protowire.BytesType: // Label
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return 0, protowire.ParseError(n)
			}
			name, val, err := parseLabel(v)
			if err != nil {
				return 0, err
			}
			s.Labels[name] = val
			return n, nil
		case num == 2 && typ == protowire.BytesType: // Sample
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return 0, protowire.ParseError(n)
			}
			sample, err := parseSample(v)
			if err != nil {
				return 0, err
			}
			s.Samples = append(s.Samples, sample)
			return n, nil
		default:
			return skipField(num, typ, b)
		}
	})
	return s, err
}

func parseLabel(b []byte) (name, value string, err error) {
	err = eachField(b, func(num protowire.Number, typ protowire.Type, b []byte) (int, error) {
		if typ == protowire.BytesType && (num == 1 || num == 2) {
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return 0, protowire.ParseError(n)
			}
			if num == 1 {
				name = string(v)
			} else {
				value = string(v)
			}
			return n, nil
		}
		return skipField(num, typ, b)
	})
	return name, value, err
}

func parseSample(b []byte) (rwSample, error) {
	var s rwSample
	err := eachField(b, func(num protowire.Number, typ protowire.Type, b []byte) (int, error) {
		switch {
		case num == 1 && typ == protowire.Fixed64Type: // double value
			v, n := protowire.ConsumeFixed64(b)
			if n < 0 {
				return 0, protowire.ParseError(n)
			}
			s.Value = math.Float64frombits(v)
			return n, nil
		case num == 2 && typ == protowire.VarintType: // int64 timestamp (ms)
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return 0, protowire.ParseError(n)
			}
			s.TimestampMs = int64(v)
			return n, nil
		default:
			return skipField(num, typ, b)
		}
	})
	return s, err
}

// eachField walks the protobuf fields in b, calling fn with the field number,
// wire type, and the bytes starting at the field value; fn returns how many
// bytes it consumed for the value.
func eachField(b []byte, fn func(num protowire.Number, typ protowire.Type, b []byte) (int, error)) error {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		consumed, err := fn(num, typ, b)
		if err != nil {
			return err
		}
		b = b[consumed:]
	}
	return nil
}

func skipField(num protowire.Number, typ protowire.Type, b []byte) (int, error) {
	n := protowire.ConsumeFieldValue(num, typ, b)
	if n < 0 {
		return 0, protowire.ParseError(n)
	}
	return n, nil
}
