package eventbus

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/sacloud/sakumock/core"
)

// Mock-only data-plane endpoints under /_sakumock/ (Kind "inspection"). They do
// not exist in the real EventBus API; they let tests and local runs drive and
// observe firing without a live alert source or destination service.

// handleInjectEvent (POST /_sakumock/events) injects an event and fires every
// trigger it matches, returning the resulting deliveries.
func (s *Server) handleInjectEvent(w http.ResponseWriter, r *http.Request) {
	var ev event
	if err := core.ReadJSON(r, &ev); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if ev.Source == "" {
		writeError(w, http.StatusBadRequest, "Source is required")
		return
	}
	fired := s.dataPlane.injectEvent(ev)
	core.WriteJSON(w, http.StatusOK, deliveriesResponse(fired))
}

// handleTick (POST /_sakumock/tick) forces a scheduler evaluation. The optional
// ?at=<time> query sets the evaluation time (defaulting to now), so tests can
// fire time-driven schedules deterministically without waiting on the wall
// clock. The time is an RFC3339 timestamp (e.g. 2024-01-01T09:00:00+09:00) or,
// for convenience, bare epoch seconds — second resolution is enough since
// schedules fire at minute granularity. Schedules fire for every boundary in
// (last tick, at].
func (s *Server) handleTick(w http.ResponseWriter, r *http.Request) {
	at := s.dataPlane.now()
	if v := r.URL.Query().Get("at"); v != "" {
		parsed, err := parseTickTime(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		at = parsed
	}
	fired := s.dataPlane.tick(at)
	core.WriteJSON(w, http.StatusOK, deliveriesResponse(fired))
}

// parseTickTime accepts an RFC3339 timestamp or bare epoch seconds.
func parseTickTime(v string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, nil
	}
	if sec, err := strconv.ParseInt(v, 10, 64); err == nil {
		return time.Unix(sec, 0), nil
	}
	return time.Time{}, fmt.Errorf("at must be an RFC3339 timestamp or epoch seconds, got %q", v)
}

// handleListDeliveries (GET /_sakumock/deliveries) returns every firing the data
// plane has recorded, oldest first.
func (s *Server) handleListDeliveries(w http.ResponseWriter, r *http.Request) {
	core.WriteJSON(w, http.StatusOK, deliveriesResponse(s.dataPlane.recordedDeliveries()))
}

// handleClearDeliveries (DELETE /_sakumock/deliveries) discards recorded firings.
func (s *Server) handleClearDeliveries(w http.ResponseWriter, r *http.Request) {
	s.dataPlane.clearDeliveries()
	w.WriteHeader(http.StatusNoContent)
}

func deliveriesResponse(ds []Delivery) map[string]any {
	if ds == nil {
		ds = []Delivery{}
	}
	return map[string]any{
		"Deliveries": ds,
		"Count":      len(ds),
	}
}
