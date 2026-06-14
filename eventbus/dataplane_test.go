package eventbus

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"
)

func startsAtMs(t time.Time) json.RawMessage {
	b, _ := json.Marshal(strconv.FormatInt(t.UnixMilli(), 10))
	return b
}

func TestStartsAtAcceptsStringAndInteger(t *testing.T) {
	want := time.UnixMilli(1700000000000)
	for _, raw := range []json.RawMessage{
		json.RawMessage(`"1700000000000"`),
		json.RawMessage(`1700000000000`),
	} {
		got, ok := scheduleSettings{StartsAt: raw}.startsAt()
		if !ok || !got.Equal(want) {
			t.Errorf("startsAt(%s) = %v, %v; want %v", raw, got, ok, want)
		}
	}
	if _, ok := (scheduleSettings{}).startsAt(); ok {
		t.Error("empty StartsAt should not parse")
	}
}

func TestParseTickTime(t *testing.T) {
	want := time.Unix(1700000000, 0)
	// RFC3339 (the human-friendly form) and bare epoch seconds both parse.
	for _, v := range []string{want.UTC().Format(time.RFC3339), "1700000000"} {
		got, err := parseTickTime(v)
		if err != nil || !got.Equal(want) {
			t.Errorf("parseTickTime(%q) = %v, %v; want %v", v, got, err, want)
		}
	}
	if _, err := parseTickTime("not-a-time"); err == nil {
		t.Error("expected error for an unparseable time")
	}
}

func TestDueBoundariesRecurring(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	st := scheduleSettings{
		StartsAt:      startsAtMs(start),
		RecurringStep: 1,
		RecurringUnit: "min",
	}

	// First evaluation window starting before StartsAt fires at StartsAt and at
	// each minute up to and including `at`.
	got := dueBoundaries(st, start.Add(-time.Second), start.Add(3*time.Minute))
	if len(got) != 4 {
		t.Fatalf("expected 4 boundaries, got %d: %v", len(got), got)
	}
	if !got[0].Equal(start) {
		t.Errorf("first boundary = %v, want %v", got[0], start)
	}

	// A boundary exactly at windowStart was already fired and must not repeat.
	got = dueBoundaries(st, start, start.Add(2*time.Minute))
	if len(got) != 2 || !got[0].Equal(start.Add(time.Minute)) {
		t.Errorf("expected boundaries at +1m,+2m, got %v", got)
	}
}

func TestDueBoundariesNeitherCronNorRecurring(t *testing.T) {
	// A schedule with neither Crontab nor a recurring interval is rejected at
	// create time (validateSettings); should it reach the engine, it never fires.
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	st := scheduleSettings{StartsAt: startsAtMs(start)}
	if got := dueBoundaries(st, start.Add(-time.Minute), start.Add(time.Hour)); len(got) != 0 {
		t.Errorf("expected no boundaries, got %v", got)
	}
}

func TestDueBoundariesCron(t *testing.T) {
	// Every 15 minutes. Evaluate a one-hour window in JST (crontab is JST).
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, crontabLocation)
	st := scheduleSettings{StartsAt: startsAtMs(start), Crontab: "*/15 * * * *"}

	got := dueBoundaries(st, start, start.Add(time.Hour))
	// (00:00, 01:00] contains :15, :30, :45, and 01:00 -> 4 boundaries.
	if len(got) != 4 {
		t.Fatalf("expected 4 cron boundaries, got %d: %v", len(got), got)
	}
	if got[0].In(crontabLocation).Minute() != 15 {
		t.Errorf("first cron boundary = %v, want minute 15", got[0])
	}

	// Boundaries before StartsAt are skipped: with StartsAt 00:40, only 00:45
	// and 01:00 remain in (00:00, 01:00].
	later := start.Add(40 * time.Minute) // 00:40
	got = dueBoundaries(scheduleSettings{StartsAt: startsAtMs(later), Crontab: "*/15 * * * *"}, start, start.Add(time.Hour))
	if len(got) != 2 || got[0].In(crontabLocation).Minute() != 45 {
		t.Errorf("expected the 00:45 and 01:00 boundaries after StartsAt 00:40, got %v", got)
	}
}

func TestTriggerMatches(t *testing.T) {
	st := triggerSettings{
		Source: "src",
		Types:  []string{"a", "b"},
		Conditions: []triggerCondition{
			{Key: "status", Op: "eq", Values: []string{"critical"}},
			{Key: "region", Op: "in", Values: []string{"is1a", "is1b"}},
		},
	}
	base := event{Source: "src", Type: "a", Attributes: map[string]any{"status": "critical", "region": "is1b"}}

	if !triggerMatches(st, base) {
		t.Error("expected match")
	}
	// Wrong source.
	if triggerMatches(st, event{Source: "other", Type: "a", Attributes: base.Attributes}) {
		t.Error("source mismatch should not match")
	}
	// Type not in Types.
	if triggerMatches(st, event{Source: "src", Type: "z", Attributes: base.Attributes}) {
		t.Error("type mismatch should not match")
	}
	// Failing condition.
	if triggerMatches(st, event{Source: "src", Type: "a", Attributes: map[string]any{"status": "ok", "region": "is1b"}}) {
		t.Error("eq condition mismatch should not match")
	}
	// Missing attribute.
	if triggerMatches(st, event{Source: "src", Type: "a", Attributes: map[string]any{"status": "critical"}}) {
		t.Error("missing attribute should not match")
	}
	// Empty Types matches any type.
	st.Types = nil
	if !triggerMatches(st, event{Source: "src", Type: "anything", Attributes: base.Attributes}) {
		t.Error("empty Types should match any type")
	}
}
