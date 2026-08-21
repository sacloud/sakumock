package addon

import (
	"log/slog"
	"time"
)

// NewStore creates the store backing the mock. provisioningDelay is how long
// a created resource stays in the "Running" deployment state before it
// becomes visible to list/get.
func NewStore(logger *slog.Logger, provisioningDelay time.Duration) *MemoryStore {
	return NewMemoryStore(logger, provisioningDelay)
}
