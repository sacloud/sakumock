package eventbus

import (
	"testing"
	"time"
)

// The valid cases are the examples published in the EventBus manual
// (指定可能なCrontab形式の記法); the invalid cases are its documented
// restrictions.
func TestParseCrontab(t *testing.T) {
	valid := []string{
		"* * * * *",                // 毎分実行
		"0 0 * * *",                // 毎日 0:00
		"30 14 * * 1-5",            // 月曜日から金曜日の 14:30
		"*/15 * * * *",             // 15 分間隔
		"5,20,35 9-17/2 * * *",     // 9〜17 時を 2 時間刻み、分は 5・20・35
		"0 0 1 * *",                // 毎月 1 日 0:00
		"0 0 1 */2 *",              // 偶数月 1 日 0:00
		"0 7-19/3 * * 6",           // 土曜日の 7:00〜19:00 を 3 時間刻み
		"10-50/10 8,12,18 * * 0,3", // 日・水曜日の 8・12・18 時
	}
	for _, expr := range valid {
		if _, err := ParseCrontab(expr); err != nil {
			t.Errorf("ParseCrontab(%q): unexpected error: %v", expr, err)
		}
	}

	invalid := []string{
		"",             // empty
		"* * * *",      // 4 fields
		"* * * * * *",  // 6 fields
		"* * * * 7",    // day-of-week 7 is explicitly invalid (Sunday is 0)
		"0 0 * JAN *",  // month names are not supported
		"0 0 * * MON",  // day names are not supported
		"@yearly",      // aliases are not supported
		"0 0 L * *",    // extended operator L
		"0 0 ? * *",    // extended operator ?
		"H * * * *",    // extended operator H
		"0 0 1W * *",   // extended operator W
		"0 0 * * 1#2",  // extended operator #
		"60 * * * *",   // minute out of range
		"* 24 * * *",   // hour out of range
		"* * 0 * *",    // day out of range
		"* * 32 * *",   // day out of range
		"* * * 0 *",    // month out of range
		"* * * 13 *",   // month out of range
		"17-9 * * * *", // reversed range
		"5/2 * * * *",  // step after a single value (spec shows steps only after * or a range)
		"*/0 * * * *",  // zero step
		"-5 * * * *",   // negative / malformed
		"1- * * * *",   // malformed range
		"1,, * * * *",  // empty list element
	}
	for _, expr := range invalid {
		if _, err := ParseCrontab(expr); err == nil {
			t.Errorf("ParseCrontab(%q): expected error, got none", expr)
		}
	}
}

// jst mirrors crontabLocation for building expected times in tests.
var jst = time.FixedZone("JST", 9*60*60)

func TestCrontabNext(t *testing.T) {
	// A fixed reference point: Friday 2026-06-12 10:30:45 JST.
	base := time.Date(2026, 6, 12, 10, 30, 45, 0, jst)

	tests := []struct {
		expr string
		want time.Time
	}{
		{"* * * * *", time.Date(2026, 6, 12, 10, 31, 0, 0, jst)},
		{"0 0 * * *", time.Date(2026, 6, 13, 0, 0, 0, 0, jst)},
		{"30 14 * * 1-5", time.Date(2026, 6, 12, 14, 30, 0, 0, jst)},
		{"*/15 * * * *", time.Date(2026, 6, 12, 10, 45, 0, 0, jst)},
		{"5,20,35 9-17/2 * * *", time.Date(2026, 6, 12, 11, 5, 0, 0, jst)}, // 9-17/2 → 9,11,13,15,17; hour 10 is skipped
		{"0 0 1 * *", time.Date(2026, 7, 1, 0, 0, 0, 0, jst)},
		// Standard cron semantics: "*/2" in the month field steps from the
		// range start (1), giving the odd months 1,3,5,7,9,11. The manual's
		// example table describes this expression as 偶数月 (even months),
		// which conflicts with standard cron; we follow standard cron until
		// the real API's behavior can be verified.
		{"0 0 1 */2 *", time.Date(2026, 7, 1, 0, 0, 0, 0, jst)},
		{"0 7-19/3 * * 6", time.Date(2026, 6, 13, 7, 0, 0, 0, jst)},            // next Saturday
		{"10-50/10 8,12,18 * * 0,3", time.Date(2026, 6, 14, 8, 10, 0, 0, jst)}, // next Sunday
	}
	for _, tt := range tests {
		c, err := ParseCrontab(tt.expr)
		if err != nil {
			t.Fatalf("ParseCrontab(%q): %v", tt.expr, err)
		}
		if got := c.Next(base); !got.Equal(tt.want) {
			t.Errorf("Next(%q) = %v, want %v", tt.expr, got, tt.want)
		}
	}

	// "9-17/2" must not fire at 10:35 even though 10 is inside 9-17.
	c, _ := ParseCrontab("5,20,35 9-17/2 * * *")
	if c.Matches(time.Date(2026, 6, 12, 10, 35, 0, 0, jst)) {
		t.Error("9-17/2 should not match hour 10")
	}
	if !c.Matches(time.Date(2026, 6, 12, 11, 35, 0, 0, jst)) {
		t.Error("9-17/2 should match hour 11")
	}
}

// TestCrontabJST pins the JST evaluation assumption: the input time's own
// location must not matter, only its instant.
func TestCrontabJST(t *testing.T) {
	c, err := ParseCrontab("0 9 * * *") // 09:00 JST == 00:00 UTC
	if err != nil {
		t.Fatal(err)
	}
	utcMidnight := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	if !c.Matches(utcMidnight) {
		t.Errorf("0 9 * * * should match %v (= 09:00 JST)", utcMidnight)
	}
	next := c.Next(time.Date(2026, 6, 12, 0, 30, 0, 0, time.UTC))
	if want := time.Date(2026, 6, 13, 9, 0, 0, 0, jst); !next.Equal(want) {
		t.Errorf("Next = %v, want %v", next, want)
	}
}

// TestCrontabDayOfMonthOrDayOfWeek pins the Vixie-cron day rule: when both
// day fields are restricted, either matching fires the schedule.
func TestCrontabDayOfMonthOrDayOfWeek(t *testing.T) {
	c, err := ParseCrontab("0 0 1 * 1") // the 1st of the month OR Mondays
	if err != nil {
		t.Fatal(err)
	}
	// 2026-06-01 is a Monday, but 2026-07-01 is a Wednesday: still fires (dom).
	if !c.Matches(time.Date(2026, 7, 1, 0, 0, 0, 0, jst)) {
		t.Error("should fire on the 1st even when it is not a Monday")
	}
	// 2026-06-08 is a Monday, not the 1st: still fires (dow).
	if !c.Matches(time.Date(2026, 6, 8, 0, 0, 0, 0, jst)) {
		t.Error("should fire on Mondays even when not the 1st")
	}
	// 2026-06-09 is a Tuesday and not the 1st: must not fire.
	if c.Matches(time.Date(2026, 6, 9, 0, 0, 0, 0, jst)) {
		t.Error("should not fire on a day matching neither field")
	}

	// With day-of-month "*", only the day-of-week restricts.
	c, _ = ParseCrontab("0 0 * * 1")
	if c.Matches(time.Date(2026, 7, 1, 0, 0, 0, 0, jst)) {
		t.Error("with dom=*, a non-Monday must not fire")
	}
}

func TestCrontabNextNoOccurrence(t *testing.T) {
	c, err := ParseCrontab("0 0 31 2 *") // February 31st never exists
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Next(time.Date(2026, 6, 12, 0, 0, 0, 0, jst)); !got.IsZero() {
		t.Errorf("expected zero time for an impossible schedule, got %v", got)
	}
}

func TestCrontabNextLeapDay(t *testing.T) {
	c, err := ParseCrontab("0 0 29 2 *")
	if err != nil {
		t.Fatal(err)
	}
	// The next Feb 29 after 2026-06-12 is in 2028.
	if got, want := c.Next(time.Date(2026, 6, 12, 0, 0, 0, 0, jst)), time.Date(2028, 2, 29, 0, 0, 0, 0, jst); !got.Equal(want) {
		t.Errorf("Next = %v, want %v", got, want)
	}
}
