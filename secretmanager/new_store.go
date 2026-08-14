package secretmanager

import "log/slog"

// NewStore creates the store backing the mock.
func NewStore(logger *slog.Logger) Store {
	return NewMemoryStore(logger)
}
