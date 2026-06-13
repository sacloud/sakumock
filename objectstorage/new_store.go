package objectstorage

import "log/slog"

// NewStore creates a Store based on configuration. logger is the service-tagged
// logger used for operation logs (nil falls back to the default).
// Currently only in-memory storage is supported.
func NewStore(logger *slog.Logger) *MemoryStore {
	return NewMemoryStore(logger)
}
