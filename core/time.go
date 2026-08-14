package core

import "time"

// FormatRFC3339 renders t in the timestamp format the mock APIs return.
func FormatRFC3339(t time.Time) string { return t.Format(time.RFC3339) }

// FormatRFC3339Nano renders t in the timestamp format the mock APIs return
// where sub-second precision is expected.
func FormatRFC3339Nano(t time.Time) string { return t.Format(time.RFC3339Nano) }
