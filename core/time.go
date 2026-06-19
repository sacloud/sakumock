package core

import "time"

func FormatRFC3339(t time.Time) string     { return t.Format(time.RFC3339) }
func FormatRFC3339Nano(t time.Time) string { return t.Format(time.RFC3339Nano) }
