package simplemq

import (
	"log/slog"
	"time"
)

// NewStore creates the store backing the mock: SQLite when database names a
// file, otherwise in-memory.
func NewStore(visibilityTimeout, messageExpiration time.Duration, database string, logger *slog.Logger) (Store, error) {
	if database != "" {
		return NewSQLiteStore(database, visibilityTimeout, messageExpiration, logger)
	}
	return NewMemoryStore(visibilityTimeout, messageExpiration, logger), nil
}
