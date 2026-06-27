package workflows

import "log/slog"

func NewStore(logger *slog.Logger) *MemoryStore {
	return NewMemoryStore(logger)
}
