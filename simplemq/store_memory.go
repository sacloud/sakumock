package simplemq

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

type memoryQueue struct {
	mu                sync.Mutex
	messages          []*storedMessage
	visibilityTimeout time.Duration
	messageExpiration time.Duration
}

func newMemoryQueue(visibilityTimeout, messageExpiration time.Duration) *memoryQueue {
	return &memoryQueue{
		visibilityTimeout: visibilityTimeout,
		messageExpiration: messageExpiration,
	}
}

func (q *memoryQueue) send(content string, now time.Time) storedMessage {
	q.mu.Lock()
	defer q.mu.Unlock()

	msg := &storedMessage{
		ID:        uuid.New().String(),
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(q.messageExpiration),
	}
	q.messages = append(q.messages, msg)
	return *msg
}

func (q *memoryQueue) receive(now time.Time) (storedMessage, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, msg := range q.messages {
		if now.After(msg.ExpiresAt) {
			continue
		}
		if !msg.VisibilityTimeoutAt.IsZero() && now.Before(msg.VisibilityTimeoutAt) {
			continue
		}
		msg.AcquiredAt = now
		msg.UpdatedAt = now
		msg.VisibilityTimeoutAt = now.Add(q.visibilityTimeout)
		return *msg, true
	}
	return storedMessage{}, false
}

// compact removes expired messages from the slice. Must be called with mu held.
func (q *memoryQueue) compact(now time.Time) {
	n := 0
	for _, msg := range q.messages {
		if now.Before(msg.ExpiresAt) {
			q.messages[n] = msg
			n++
		}
	}
	clear(q.messages[n:])
	q.messages = q.messages[:n]
}

func (q *memoryQueue) extendTimeout(id string, now time.Time) (storedMessage, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, msg := range q.messages {
		if msg.ID == id {
			msg.VisibilityTimeoutAt = now.Add(q.visibilityTimeout)
			msg.UpdatedAt = now
			return *msg, nil
		}
	}
	return storedMessage{}, fmt.Errorf("message not found: %s", id)
}

func (q *memoryQueue) delete(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, msg := range q.messages {
		if msg.ID == id {
			q.messages = append(q.messages[:i], q.messages[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("message not found: %s", id)
}

// MemoryStore manages named queues in memory.
type MemoryStore struct {
	mu                sync.Mutex
	queues            map[string]*memoryQueue
	visibilityTimeout time.Duration
	messageExpiration time.Duration
	done              chan struct{}
	closeOnce         sync.Once
}

// NewMemoryStore creates a new in-memory Store.
func NewMemoryStore(visibilityTimeout, messageExpiration time.Duration) *MemoryStore {
	if visibilityTimeout <= 0 {
		visibilityTimeout = defaultVisibilityTimeout
	}
	if messageExpiration <= 0 {
		messageExpiration = defaultMessageExpiration
	}
	s := &MemoryStore{
		queues:            make(map[string]*memoryQueue),
		visibilityTimeout: visibilityTimeout,
		messageExpiration: messageExpiration,
		done:              make(chan struct{}),
	}
	go s.compactLoop()
	return s
}

func (s *MemoryStore) getQueue(name string) *memoryQueue {
	s.mu.Lock()
	defer s.mu.Unlock()

	q, ok := s.queues[name]
	if !ok {
		q = newMemoryQueue(s.visibilityTimeout, s.messageExpiration)
		s.queues[name] = q
		slog.Info("queue created", "queue", name)
	}
	return q
}

func (s *MemoryStore) compactLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case now := <-ticker.C:
			s.mu.Lock()
			for _, q := range s.queues {
				q.mu.Lock()
				q.compact(now)
				q.mu.Unlock()
			}
			s.mu.Unlock()
		}
	}
}

func (s *MemoryStore) Send(queueName, content string, now time.Time) (storedMessage, error) {
	q := s.getQueue(queueName)
	return q.send(content, now), nil
}

func (s *MemoryStore) Receive(queueName string, now time.Time) (storedMessage, bool, error) {
	q := s.getQueue(queueName)
	msg, ok := q.receive(now)
	return msg, ok, nil
}

func (s *MemoryStore) ExtendTimeout(queueName, id string, now time.Time) (storedMessage, error) {
	q := s.getQueue(queueName)
	return q.extendTimeout(id, now)
}

func (s *MemoryStore) Delete(queueName, id string) error {
	q := s.getQueue(queueName)
	return q.delete(id)
}

func (s *MemoryStore) Close() error {
	s.closeOnce.Do(func() {
		close(s.done)
	})
	return nil
}
