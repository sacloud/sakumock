package eventbus

import (
	"encoding/json"
	"log/slog"
	"slices"
	"sync"
	"time"
)

// The data plane turns stored schedules and triggers into firings. There are
// two kinds of event source:
//
//   - schedules are time-driven: a Crontab expression or a RecurringStep /
//     RecurringUnit interval measured from StartsAt. They are self-contained —
//     evaluated against the wall clock — so the mock can fire them on its own.
//   - triggers are event-driven: they react to an external event (a
//     monitoringsuite alert, an eventlog entry). The mock cannot observe those
//     events, so it accepts injected ones on POST /_sakumock/events and matches
//     them against every trigger's Source / Types / Conditions.
//
// Both kinds resolve to the same action: fire the referenced process
// configuration, recording the outcome as a Delivery (inspectable via
// GET /_sakumock/deliveries) and on the source resource's Status. Actually
// sending the job to its Destination service (simplemq / simplenotification)
// over HTTP is a separate layer built on top of these recorded deliveries.

// maxFiresPerTick caps how many boundaries a single schedule may fire in one
// tick, so a fast recurring schedule evaluated over a long window (e.g. a first
// tick far after StartsAt) cannot produce an unbounded burst.
const maxFiresPerTick = 1000

// Delivery is one firing of a process configuration. It records what the mock
// would deliver — the resolved Destination and Parameters — so tests can assert
// firing without a live destination service.
type Delivery struct {
	FiredAt                time.Time       `json:"FiredAt"`
	SourceID               string          `json:"SourceID"`    // the schedule or trigger resource ID
	SourceClass            string          `json:"SourceClass"` // eventbusschedule | eventbustrigger
	ProcessConfigurationID string          `json:"ProcessConfigurationID"`
	Destination            string          `json:"Destination"`
	Parameters             string          `json:"Parameters"`
	Event                  json.RawMessage `json:"Event,omitempty"` // the triggering event, for triggers
	Error                  string          `json:"Error,omitempty"` // non-empty when the firing could not be resolved
}

// event is the body of POST /_sakumock/events: a mock-injected event evaluated
// against every trigger. Source and Type drive Source/Types matching; the
// Attributes drive Conditions. Data is the passthrough payload (the API
// reserves the "data" key from Conditions) and is not used for matching.
type event struct {
	Source     string          `json:"Source"`
	Type       string          `json:"Type"`
	Attributes map[string]any  `json:"Attributes"`
	Data       json.RawMessage `json:"Data,omitempty"`
}

// dataPlane evaluates schedules and triggers and records the resulting
// deliveries. It is always present so the inspection endpoints work in tests;
// the autonomous wall-clock scheduler (see run) is started only when the data
// plane is enabled.
type dataPlane struct {
	store  Store
	logger *slog.Logger
	now    func() time.Time

	mu         sync.Mutex
	deliveries []Delivery
	lastTick   time.Time

	stop chan struct{}
	done chan struct{}
}

func newDataPlane(store Store, logger *slog.Logger, now func() time.Time) *dataPlane {
	if now == nil {
		now = time.Now
	}
	return &dataPlane{
		store:    store,
		logger:   logger,
		now:      now,
		lastTick: now(),
	}
}

// injectEvent matches an injected event against every trigger and fires the
// matched ones, returning the deliveries produced.
func (dp *dataPlane) injectEvent(ev event) []Delivery {
	raw, _ := json.Marshal(ev)
	// One event fires all matched triggers at a single instant, so every
	// resulting delivery shares the same FiredAt.
	firedAt := dp.now()
	var fired []Delivery
	for _, it := range dp.store.ListItems(classTrigger) {
		st, err := parseTriggerSettings(it.Settings)
		if err != nil {
			dp.logger.Warn("skipping trigger with invalid settings", "id", it.ID, "error", err)
			continue
		}
		if !triggerMatches(st, ev) {
			continue
		}
		fired = append(fired, dp.fire(it, st.ProcessConfigurationID, raw, firedAt))
	}
	dp.logger.Debug("event injected", "source", ev.Source, "type", ev.Type, "matched", len(fired))
	return fired
}

// triggerMatches reports whether the event satisfies the trigger: exact Source,
// Type membership when Types is restricted, and every Condition.
func triggerMatches(st triggerSettings, ev event) bool {
	if st.Source != ev.Source {
		return false
	}
	if len(st.Types) > 0 && !slices.Contains(st.Types, ev.Type) {
		return false
	}
	for _, c := range st.Conditions {
		if !c.matches(ev.Attributes) {
			return false
		}
	}
	return true
}

// tick evaluates every schedule for the half-open window (lastTick, at] and
// fires each due boundary, returning the deliveries produced. A manual tick via
// POST /_sakumock/tick and the autonomous scheduler both call this; advancing
// lastTick makes a boundary fire exactly once across successive ticks.
func (dp *dataPlane) tick(at time.Time) []Delivery {
	dp.mu.Lock()
	windowStart := dp.lastTick
	if at.After(dp.lastTick) {
		dp.lastTick = at
	}
	dp.mu.Unlock()

	if !at.After(windowStart) {
		return nil
	}

	var fired []Delivery
	for _, it := range dp.store.ListItems(classSchedule) {
		st, err := parseScheduleSettings(it.Settings)
		if err != nil {
			dp.logger.Warn("skipping schedule with invalid settings", "id", it.ID, "error", err)
			continue
		}
		for _, boundary := range dueBoundaries(st, windowStart, at) {
			fired = append(fired, dp.fire(it, st.ProcessConfigurationID, nil, boundary))
		}
	}
	return fired
}

// dueBoundaries returns the schedule's fire times in the half-open window
// (windowStart, at], honoring StartsAt. A schedule fires on its Crontab when
// set, otherwise on its recurring interval. A schedule with neither is rejected
// at create time (see validateSettings), so it yields no boundaries here.
func dueBoundaries(st scheduleSettings, windowStart, at time.Time) []time.Time {
	start, ok := st.startsAt()
	if !ok {
		return nil
	}
	var out []time.Time
	switch {
	case st.Crontab != "":
		ct, err := ParseCrontab(st.Crontab)
		if err != nil {
			return nil
		}
		// Begin from just before the later of windowStart and StartsAt so the
		// first Next() yields a boundary at or after the schedule's start.
		from := windowStart
		if start.After(from) {
			from = start.Add(-time.Minute)
		}
		for b := ct.Next(from); !b.IsZero() && !b.After(at); b = ct.Next(b) {
			if b.Before(start) {
				continue
			}
			out = append(out, b)
			if len(out) >= maxFiresPerTick {
				break
			}
		}
	default:
		interval, recurring := st.recurringInterval()
		if !recurring {
			return nil
		}
		first := start
		if !windowStart.Before(start) {
			// windowStart >= start: skip to the first boundary strictly after
			// windowStart, so a boundary already fired by the previous tick (at
			// == windowStart) is not fired again.
			n := windowStart.Sub(start) / interval
			first = start.Add((n + 1) * interval)
		}
		for b := first; !b.After(at); b = b.Add(interval) {
			out = append(out, b)
			if len(out) >= maxFiresPerTick {
				break
			}
		}
	}
	return out
}

// fire resolves the process configuration referenced by a schedule or trigger,
// records a Delivery, and updates the source resource's Status. A missing or
// non-process-configuration reference yields a Delivery with Error set and a
// failed Status, matching what the real service would report for a broken job.
func (dp *dataPlane) fire(source ServiceItem, pcID string, ev json.RawMessage, firedAt time.Time) Delivery {
	d := Delivery{
		FiredAt:                firedAt,
		SourceID:               source.ID,
		SourceClass:            source.ProviderClass,
		ProcessConfigurationID: pcID,
		Event:                  ev,
	}

	pc, ok := dp.store.GetItem(pcID)
	if !ok || pc.ProviderClass != classProcessConfiguration {
		d.Error = "referenced process configuration not found: " + pcID
	} else if pcSt, err := parseProcessConfigSettings(pc.Settings); err != nil {
		d.Error = err.Error()
	} else {
		d.Destination = pcSt.Destination
		d.Parameters = pcSt.Parameters
	}

	dp.record(d)
	dp.store.SetStatus(source.ID, ItemStatus{
		Success:   d.Error == "",
		Message:   statusMessage(d),
		UpdatedAt: firedAt,
	})
	if d.Error != "" {
		dp.logger.Warn("firing failed", "source", source.ID, "class", source.ProviderClass, "error", d.Error)
	} else {
		dp.logger.Info("fired", "source", source.ID, "class", source.ProviderClass,
			"destination", d.Destination, "process_configuration", pcID)
	}
	return d
}

func statusMessage(d Delivery) string {
	if d.Error != "" {
		return d.Error
	}
	return "delivered to " + d.Destination
}

func (dp *dataPlane) record(d Delivery) {
	dp.mu.Lock()
	dp.deliveries = append(dp.deliveries, d)
	dp.mu.Unlock()
}

// recordedDeliveries returns a copy of every delivery recorded so far, oldest
// first.
func (dp *dataPlane) recordedDeliveries() []Delivery {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	out := make([]Delivery, len(dp.deliveries))
	copy(out, dp.deliveries)
	return out
}

func (dp *dataPlane) clearDeliveries() {
	dp.mu.Lock()
	dp.deliveries = nil
	dp.mu.Unlock()
}

// start launches the autonomous scheduler: it ticks once a minute on the wall
// clock until stop is called. The ticks are not aligned to the minute boundary,
// so a schedule may fire up to ~59s late; this is harmless because each tick
// fires every boundary in (last tick, now], never missing one. Manual injection
// and ticking work without it; only this loop is gated by --enable-data-plane.
func (dp *dataPlane) start() {
	dp.stop = make(chan struct{})
	dp.done = make(chan struct{})
	go dp.run()
}

func (dp *dataPlane) run() {
	defer close(dp.done)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	dp.logger.Info("data plane scheduler started")
	for {
		select {
		case <-dp.stop:
			return
		case <-ticker.C:
			dp.tick(dp.now())
		}
	}
}

func (dp *dataPlane) close() {
	if dp.stop != nil {
		close(dp.stop)
		<-dp.done
		dp.stop = nil
	}
}
