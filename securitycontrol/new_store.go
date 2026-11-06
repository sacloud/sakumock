package securitycontrol

import "log/slog"

// NewStore creates the store backing the mock. actionLimit caps the number of
// automated actions.
func NewStore(logger *slog.Logger, actionLimit int) *MemoryStore {
	return NewMemoryStore(logger, actionLimit)
}
