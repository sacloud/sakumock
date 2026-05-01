package simplenotification

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// MemoryStore is an in-memory Store for accepted notification messages.
type MemoryStore struct {
	mu       sync.Mutex
	messages []MessageRecord
	nextID   int64
}

// NewMemoryStore creates a new empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nextID: 1,
	}
}

// Send records a notification message and returns the stored record.
func (s *MemoryStore) Send(groupID, message string, now time.Time) (MessageRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec := MessageRecord{
		ID:        fmt.Sprintf("%d", s.nextID),
		GroupID:   groupID,
		Message:   message,
		CreatedAt: now,
	}
	s.nextID++
	s.messages = append(s.messages, rec)
	slog.Debug("message stored", "id", rec.ID, "group_id", groupID)
	return rec, nil
}

// List returns a snapshot of all accepted messages in send order.
func (s *MemoryStore) List() []MessageRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]MessageRecord, len(s.messages))
	copy(out, s.messages)
	return out
}

// Close releases resources held by the store.
func (s *MemoryStore) Close() error {
	return nil
}
