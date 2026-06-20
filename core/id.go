package core

import (
	"strconv"
	"sync"
	"time"
)

// DefaultIDBase returns a time-derived starting value for generated resource IDs.
//
// The generated base is a 12-digit number of the form 9TTTTTTTTTCC, where T is
// derived from the current Unix timestamp and CC is a 2-digit counter space
// (00–99), giving 100 unique IDs per second before overlapping into the next
// second's range (within a single process, IDs remain unique regardless because
// the counter is monotonic).
//
// The 9xx band stays clear of real SAKURA Cloud IDs (currently in the 11xx–12xx
// band), so a mock ID that leaks to the real API hits nothing (404).
func DefaultIDBase() int64 {
	return 900_000_000_000 + (time.Now().Unix()%1_000_000_000)*100
}

// IDGenerator hands out sequential numeric resource IDs as decimal strings,
// resembling the IDs SAKURA Cloud assigns when a resource is created via the
// control plane (e.g. a SimpleMQ queue or a KMS key). It is meant for those
// control-plane resource IDs, not data-plane identifiers such as message IDs.
// It is safe for concurrent use.
type IDGenerator struct {
	mu   sync.Mutex
	next int64
}

// NewIDGenerator returns a generator whose first ID is base. A base <= 0 falls
// back to DefaultIDBase().
func NewIDGenerator(base int64) *IDGenerator {
	if base <= 0 {
		base = DefaultIDBase()
	}
	return &IDGenerator{next: base}
}

// Next returns the next ID as a decimal string with no leading zeros.
func (g *IDGenerator) Next() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	id := g.next
	g.next++
	return strconv.FormatInt(id, 10)
}

// Observe advances the generator so future IDs exceed an existing one, letting a
// generator resume after reloading IDs from persistent storage. Non-numeric or
// smaller values are ignored.
func (g *IDGenerator) Observe(existing string) {
	n, err := strconv.ParseInt(existing, 10, 64)
	if err != nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if n >= g.next {
		g.next = n + 1
	}
}
