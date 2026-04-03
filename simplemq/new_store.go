package simplemq

import (
	"time"
)

// NewStore creates a Store based on configuration.
// If database is non-empty, a SQLite-backed store is created; otherwise in-memory.
func NewStore(visibilityTimeout, messageExpiration time.Duration, database string) (Store, error) {
	if database != "" {
		return NewSQLiteStore(database, visibilityTimeout, messageExpiration)
	}
	return NewMemoryStore(visibilityTimeout, messageExpiration), nil
}
