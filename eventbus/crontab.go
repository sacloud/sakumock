package eventbus

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// crontabLocation is the timezone schedules are evaluated in.
//
// The published EventBus documentation does not state which timezone the real
// service uses to evaluate crontab expressions; JST (UTC+9, no DST) is an
// assumption based on the service being operated for Japan. If the real
// service turns out to evaluate in a different zone, only this variable needs
// to change.
var crontabLocation = time.FixedZone("JST", 9*60*60)

// Crontab is a parsed crontab expression as accepted by the EventBus API.
//
// The accepted grammar follows the published spec
// (https://manual.sakura.ad.jp/cloud/appliance/eventbus/control_panel.html,
// 指定可能なCrontab形式の記法):
//
//   - exactly five space-separated fields: minute hour day-of-month month day-of-week
//   - values are numeric only; "*", ",", "-", "/" are the only symbols allowed
//   - minute 0-59, hour 0-23, day 1-31, month 1-12, day-of-week 0-6 (0=Sunday;
//     7 is explicitly invalid)
//   - month/day names (JAN, MON), aliases (@yearly), and extended operators
//     (L W # ? H) are all rejected
//
// A step ("/N") is accepted after "*" or a range ("a-b"), the only forms the
// spec's examples show; a step after a single value ("5/2") is rejected.
type Crontab struct {
	minute, hour, dom, month, dow uint64 // bitmasks of allowed values

	// domStar/dowStar record whether the day-of-month/day-of-week field was
	// the unrestricted "*", which changes the day-matching rule (see Matches).
	domStar, dowStar bool
}

// crontabFields describes the five fields in order, with their allowed ranges.
var crontabFields = []struct {
	name     string
	min, max int
}{
	{"minute", 0, 59},
	{"hour", 0, 23},
	{"day", 1, 31},
	{"month", 1, 12},
	{"day-of-week", 0, 6},
}

// ParseCrontab parses a crontab expression per the EventBus spec, returning an
// error describing the first violation found.
func ParseCrontab(expr string) (*Crontab, error) {
	fields := strings.Fields(expr)
	if len(fields) != len(crontabFields) {
		return nil, fmt.Errorf("crontab must have 5 space-separated fields (minute hour day month day-of-week), got %d", len(fields))
	}
	var masks [5]uint64
	var stars [5]bool
	for i, f := range fields {
		mask, star, err := parseCrontabField(f, crontabFields[i].min, crontabFields[i].max)
		if err != nil {
			return nil, fmt.Errorf("%s field %q: %w", crontabFields[i].name, f, err)
		}
		masks[i] = mask
		stars[i] = star
	}
	return &Crontab{
		minute:  masks[0],
		hour:    masks[1],
		dom:     masks[2],
		month:   masks[3],
		dow:     masks[4],
		domStar: stars[2],
		dowStar: stars[4],
	}, nil
}

// parseCrontabField parses one field into a bitmask of allowed values. star
// reports whether the field is the single unrestricted "*" (a stepped "*/n"
// is a restriction, not a star).
func parseCrontabField(field string, min, max int) (mask uint64, star bool, err error) {
	if field == "*" {
		return rangeMask(min, max, 1), true, nil
	}
	for part := range strings.SplitSeq(field, ",") {
		base, stepStr, hasStep := strings.Cut(part, "/")
		step := 1
		if hasStep {
			step, err = parseCrontabNumber(stepStr)
			if err != nil || step < 1 {
				return 0, false, fmt.Errorf("invalid step %q", stepStr)
			}
		}
		var lo, hi int
		switch {
		case base == "*":
			lo, hi = min, max
		case strings.Contains(base, "-"):
			loStr, hiStr, _ := strings.Cut(base, "-")
			lo, err = parseCrontabNumber(loStr)
			if err == nil {
				hi, err = parseCrontabNumber(hiStr)
			}
			if err != nil {
				return 0, false, fmt.Errorf("invalid range %q: values must be numeric (names like JAN/MON and symbols other than * , - / are not supported)", base)
			}
			if lo > hi {
				return 0, false, fmt.Errorf("invalid range %q: start is after end", base)
			}
		default:
			lo, err = parseCrontabNumber(base)
			if err != nil {
				return 0, false, fmt.Errorf("invalid value %q: values must be numeric (names like JAN/MON, aliases like @yearly, and symbols other than * , - / are not supported)", base)
			}
			if hasStep {
				// The spec's examples only show steps after "*" or a range.
				return 0, false, fmt.Errorf("step is only allowed after * or a range, not after a single value %q", base)
			}
			hi = lo
		}
		if lo < min || hi > max {
			return 0, false, fmt.Errorf("value out of range: %s (allowed %d-%d)", base, min, max)
		}
		mask |= rangeMask(lo, hi, step)
	}
	return mask, false, nil
}

// parseCrontabNumber parses a plain decimal number; anything else (names,
// signs, extended operators) is an error.
func parseCrontabNumber(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty number")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not a number")
		}
	}
	return strconv.Atoi(s)
}

func rangeMask(lo, hi, step int) uint64 {
	var mask uint64
	for v := lo; v <= hi; v += step {
		mask |= 1 << uint(v)
	}
	return mask
}

func hasBit(mask uint64, v int) bool {
	return mask&(1<<uint(v)) != 0
}

// Matches reports whether the schedule fires at the wall-clock minute
// containing t, evaluated in JST (see crontabLocation).
//
// Day matching follows the classic Vixie cron rule, which the published spec
// does not address: when both the day-of-month and day-of-week fields are
// restricted (neither is "*"), the schedule fires when EITHER matches; when
// one of them is "*", the other alone decides.
func (c *Crontab) Matches(t time.Time) bool {
	t = t.In(crontabLocation)
	if !hasBit(c.minute, t.Minute()) || !hasBit(c.hour, t.Hour()) || !hasBit(c.month, int(t.Month())) {
		return false
	}
	return c.dayMatches(t)
}

func (c *Crontab) dayMatches(t time.Time) bool {
	domOK := hasBit(c.dom, t.Day())
	dowOK := hasBit(c.dow, int(t.Weekday()))
	switch {
	case c.domStar:
		return dowOK
	case c.dowStar:
		return domOK
	default:
		return domOK || dowOK
	}
}

// Next returns the first time strictly after t at which the schedule fires,
// in JST (see crontabLocation). It returns the zero time when no occurrence
// exists within five years (e.g. "0 0 31 2 *").
func (c *Crontab) Next(t time.Time) time.Time {
	cur := t.In(crontabLocation).Truncate(time.Minute).Add(time.Minute)
	limit := cur.AddDate(5, 0, 0)
	for cur.Before(limit) {
		switch {
		case !hasBit(c.month, int(cur.Month())):
			// First minute of the next month.
			cur = time.Date(cur.Year(), cur.Month(), 1, 0, 0, 0, 0, crontabLocation).AddDate(0, 1, 0)
		case !c.dayMatches(cur):
			// First minute of the next day.
			cur = time.Date(cur.Year(), cur.Month(), cur.Day(), 0, 0, 0, 0, crontabLocation).AddDate(0, 0, 1)
		case !hasBit(c.hour, cur.Hour()):
			// First minute of the next hour.
			cur = time.Date(cur.Year(), cur.Month(), cur.Day(), cur.Hour(), 0, 0, 0, crontabLocation).Add(time.Hour)
		case !hasBit(c.minute, cur.Minute()):
			cur = cur.Add(time.Minute)
		default:
			return cur
		}
	}
	return time.Time{}
}
