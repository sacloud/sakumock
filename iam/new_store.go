package iam

import "log/slog"

// NewStore creates the store backing the mock.
func NewStore(logger *slog.Logger) *MemoryStore {
	return NewMemoryStore(logger)
}
