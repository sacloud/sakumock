package simplemq

import (
	"log/slog"
	"time"
)

// NewStore creates a Store based on configuration. The logger is the
// service-tagged logger used for operation logs (nil falls back to the default).
// If database is non-empty, a SQLite-backed store is created; otherwise in-memory.
func NewStore(visibilityTimeout, messageExpiration time.Duration, database string, logger *slog.Logger) (Store, error) {
	if database != "" {
		return NewSQLiteStore(database, visibilityTimeout, messageExpiration, logger)
	}
	return NewMemoryStore(visibilityTimeout, messageExpiration, logger), nil
}
