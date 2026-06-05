package core

import (
	"strconv"
	"sync"
)

// DefaultIDBase is the starting value for generated resource IDs.
//
// Real SAKURA Cloud resource IDs are 12-digit numbers whose allocation counter
// currently sits in the 11xxxxxxxxxx–12xxxxxxxxxx range. Generating from the top
// of the 12-digit space (9xxxxxxxxxxx) keeps mock IDs realistic in length while
// staying clear of real IDs: the counter would have to grow ~7x to reach the
// 9xx band, by which point the 12-digit space would be near exhaustion and real
// IDs would almost certainly have grown more digits — so real allocation never
// reaches here. A mock ID that leaks to the real API (e.g. via a misconfigured
// endpoint) therefore hits nothing (404) instead of a live resource. The value
// also has no leading zeros, so it round-trips through clients that parse the ID
// as an integer.
const DefaultIDBase int64 = 990000000000

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
// back to DefaultIDBase.
func NewIDGenerator(base int64) *IDGenerator {
	if base <= 0 {
		base = DefaultIDBase
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
