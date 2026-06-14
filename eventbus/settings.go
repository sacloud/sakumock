package eventbus

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"time"
)

// Typed views over the per-class Settings raw JSON. The store keeps Settings as
// json.RawMessage and echoes it back verbatim (see store.go); the data plane
// parses it into these structs only when it needs to evaluate firing.

// processConfigSettings is the parsed Settings of an eventbusprocessconfiguration:
// what a fired job delivers and where.
type processConfigSettings struct {
	Destination string `json:"Destination"`
	Parameters  string `json:"Parameters"`
}

// scheduleSettings is the parsed Settings of an eventbusschedule: a time-driven
// event source. A schedule fires either on a Crontab expression or on a
// RecurringStep/RecurringUnit interval measured from StartsAt.
type scheduleSettings struct {
	ProcessConfigurationID string          `json:"ProcessConfigurationID"`
	StartsAt               json.RawMessage `json:"StartsAt"` // epoch ms; integer on request, string on response
	Crontab                string          `json:"Crontab"`
	RecurringStep          int             `json:"RecurringStep"`
	RecurringUnit          string          `json:"RecurringUnit"` // min|hour|day
}

// triggerSettings is the parsed Settings of an eventbustrigger: an event-driven
// source. A trigger fires when an injected event's Source matches, its Type is
// among Types (when Types is set), and every Condition holds.
type triggerSettings struct {
	Source                 string             `json:"Source"`
	Types                  []string           `json:"Types"`
	Conditions             []triggerCondition `json:"Conditions"`
	ProcessConfigurationID string             `json:"ProcessConfigurationID"`
}

// triggerCondition matches a single key of an event's attributes with eq or in.
type triggerCondition struct {
	Key    string   `json:"Key"`
	Op     string   `json:"Op"` // eq|in
	Values []string `json:"Values"`
}

func parseProcessConfigSettings(raw json.RawMessage) (processConfigSettings, error) {
	var st processConfigSettings
	if err := json.Unmarshal(raw, &st); err != nil {
		return processConfigSettings{}, fmt.Errorf("invalid process configuration settings: %w", err)
	}
	return st, nil
}

func parseScheduleSettings(raw json.RawMessage) (scheduleSettings, error) {
	var st scheduleSettings
	if err := json.Unmarshal(raw, &st); err != nil {
		return scheduleSettings{}, fmt.Errorf("invalid schedule settings: %w", err)
	}
	return st, nil
}

func parseTriggerSettings(raw json.RawMessage) (triggerSettings, error) {
	var st triggerSettings
	if err := json.Unmarshal(raw, &st); err != nil {
		return triggerSettings{}, fmt.Errorf("invalid trigger settings: %w", err)
	}
	return st, nil
}

// startsAt parses StartsAt, which the API accepts as integer epoch milliseconds
// and returns (and the store keeps) as a string. Both forms are handled so the
// data plane works whether or not the value passed through normalization.
func (s scheduleSettings) startsAt() (time.Time, bool) {
	if len(s.StartsAt) == 0 || string(s.StartsAt) == "null" {
		return time.Time{}, false
	}
	var str string
	if err := json.Unmarshal(s.StartsAt, &str); err == nil {
		ms, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return time.Time{}, false
		}
		return time.UnixMilli(ms), true
	}
	var ms int64
	if err := json.Unmarshal(s.StartsAt, &ms); err == nil {
		return time.UnixMilli(ms), true
	}
	return time.Time{}, false
}

// recurringInterval returns the schedule's recurring period, or false when the
// schedule is not recurring (no/zero RecurringStep or empty RecurringUnit).
func (s scheduleSettings) recurringInterval() (time.Duration, bool) {
	if s.RecurringStep <= 0 || s.RecurringUnit == "" {
		return 0, false
	}
	var unit time.Duration
	switch s.RecurringUnit {
	case "min":
		unit = time.Minute
	case "hour":
		unit = time.Hour
	case "day":
		unit = 24 * time.Hour
	default:
		return 0, false
	}
	return time.Duration(s.RecurringStep) * unit, true
}

// matches reports whether the condition holds for the event's attributes. A
// missing key never matches. Non-string attribute values are compared by their
// string form so an event may carry numbers or booleans.
func (c triggerCondition) matches(attrs map[string]any) bool {
	v, ok := attrs[c.Key]
	if !ok {
		return false
	}
	sv := stringifyAttr(v)
	switch c.Op {
	case "eq":
		return len(c.Values) == 1 && sv == c.Values[0]
	case "in":
		return slices.Contains(c.Values, sv)
	default:
		return false
	}
}

func stringifyAttr(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}
