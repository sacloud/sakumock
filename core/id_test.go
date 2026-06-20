package core

import (
	"strconv"
	"sync"
	"testing"
)

func TestIDGeneratorSequential(t *testing.T) {
	g := NewIDGenerator(990000000000)
	want := []string{"990000000000", "990000000001", "990000000002"}
	for i, w := range want {
		if got := g.Next(); got != w {
			t.Errorf("Next() #%d = %q, want %q", i, got, w)
		}
	}
}

func TestIDGeneratorDefaultBase(t *testing.T) {
	for _, base := range []int64{0, -1} {
		g := NewIDGenerator(base)
		got := g.Next()
		n, err := strconv.ParseInt(got, 10, 64)
		if err != nil {
			t.Fatalf("base %d: first ID %q not numeric: %v", base, got, err)
		}
		if n < 900_000_000_000 || n > 999_999_999_999 {
			t.Errorf("base %d: first ID = %d, want 12-digit number starting with 9", base, n)
		}
	}
}

func TestIDGeneratorNoLeadingZeros(t *testing.T) {
	// Generated IDs must round-trip through an integer parse/format cycle, which
	// only holds when there are no leading zeros.
	g := NewIDGenerator(990000000000)
	for range 5 {
		id := g.Next()
		n, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			t.Fatalf("ID %q not numeric: %v", id, err)
		}
		if strconv.FormatInt(n, 10) != id {
			t.Errorf("ID %q does not round-trip as integer", id)
		}
	}
}

func TestIDGeneratorObserve(t *testing.T) {
	g := NewIDGenerator(100)

	// A larger observed value advances the sequence past it.
	g.Observe("500")
	if got := g.Next(); got != "501" {
		t.Errorf("after Observe(500), Next() = %q, want 501", got)
	}

	// Smaller and non-numeric values are ignored.
	g.Observe("10")
	g.Observe("not-a-number")
	if got := g.Next(); got != "502" {
		t.Errorf("Next() = %q, want 502 (smaller/invalid observations ignored)", got)
	}
}

func TestIDGeneratorConcurrent(t *testing.T) {
	g := NewIDGenerator(1)
	const n = 1000
	var wg sync.WaitGroup
	results := make([]string, n)
	for i := range n {
		wg.Go(func() {
			results[i] = g.Next()
		})
	}
	wg.Wait()

	seen := make(map[string]bool, n)
	for _, id := range results {
		if seen[id] {
			t.Fatalf("duplicate ID generated: %q", id)
		}
		seen[id] = true
	}
	if len(seen) != n {
		t.Errorf("expected %d unique IDs, got %d", n, len(seen))
	}
}
