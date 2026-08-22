package core

import (
	"fmt"
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
	// reserved maps a user-specified ID to the service that claimed it.
	reserved map[string]string
}

// NewIDGenerator returns a generator whose first ID is base. A base <= 0 uses
// DefaultIDBase.
func NewIDGenerator(base int64) *IDGenerator {
	if base <= 0 {
		base = DefaultIDBase()
	}
	return &IDGenerator{next: base}
}

// Next returns the next ID as a decimal string.
func (g *IDGenerator) Next() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	id := g.next
	g.next++
	return strconv.FormatInt(id, 10)
}

// Reserve claims a user-specified ID (e.g. a preset KMS key) for owner, so it
// is never handed out by Next and cannot be claimed again by a different
// owner. Under the unified binary the generator is shared by every service,
// so two services configured with the same fixed ID fail at startup instead
// of serving two resources under one ID. Reserving an ID already held by the
// same owner is a no-op.
func (g *IDGenerator) Reserve(id, owner string) error {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 {
		return fmt.Errorf("invalid resource ID %q: must be a positive integer", id)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if prev, ok := g.reserved[id]; ok && prev != owner {
		return fmt.Errorf("resource ID %s is already reserved by %s", id, prev)
	}
	if g.reserved == nil {
		g.reserved = make(map[string]string)
	}
	g.reserved[id] = owner
	if n >= g.next {
		g.next = n + 1
	}
	return nil
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
