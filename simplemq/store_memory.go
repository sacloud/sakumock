package simplemq

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sacloud/sakumock/core"
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
		ID:        uuid.Must(uuid.NewV7()).String(),
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

func (q *memoryQueue) clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.messages = nil
}

func (q *memoryQueue) count(now time.Time) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	for _, msg := range q.messages {
		if now.Before(msg.ExpiresAt) {
			n++
		}
	}
	return n
}

// MemoryStore manages named queues in memory.
type MemoryStore struct {
	mu                sync.Mutex
	queues            map[string]*memoryQueue // name → message queue
	queueResources    map[string]*storedQueue // id → queue resource
	queueByName       map[string]string       // name → id
	ids               *core.IDGenerator
	visibilityTimeout time.Duration
	messageExpiration time.Duration
	done              chan struct{}
	closeOnce         sync.Once
	logger            *slog.Logger
}

// NewMemoryStore creates a new in-memory Store. logger is the service-tagged
// logger used for operation logs; nil falls back to the default.
func NewMemoryStore(visibilityTimeout, messageExpiration time.Duration, logger *slog.Logger) *MemoryStore {
	if visibilityTimeout <= 0 {
		visibilityTimeout = defaultVisibilityTimeout
	}
	if messageExpiration <= 0 {
		messageExpiration = defaultMessageExpiration
	}
	if logger == nil {
		logger = slog.Default()
	}
	s := &MemoryStore{
		queues:            make(map[string]*memoryQueue),
		queueResources:    make(map[string]*storedQueue),
		queueByName:       make(map[string]string),
		ids:               core.NewIDGenerator(core.DefaultIDBase),
		visibilityTimeout: visibilityTimeout,
		messageExpiration: messageExpiration,
		done:              make(chan struct{}),
		logger:            logger,
	}
	go s.compactLoop()
	return s
}

func (s *MemoryStore) getQueue(name string) *memoryQueue {
	s.mu.Lock()
	defer s.mu.Unlock()

	q, ok := s.queues[name]
	if !ok {
		vt := s.visibilityTimeout
		me := s.messageExpiration
		if id, hasCP := s.queueByName[name]; hasCP {
			if res := s.queueResources[id]; res != nil {
				vt = time.Duration(res.VisibilityTimeoutSeconds) * time.Second
				me = time.Duration(res.ExpireSeconds) * time.Second
			}
		}
		q = newMemoryQueue(vt, me)
		s.queues[name] = q
		s.logger.Info("queue created", "queue", name)
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

// Control plane operations

func (s *MemoryStore) CreateQueue(name, description string, tags []string, vtSecs, expSecs int, now time.Time) (storedQueue, error) {
	if tags == nil {
		tags = []string{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.queueByName[name]; exists {
		return storedQueue{}, ErrQueueConflict
	}

	vt := s.visibilityTimeout
	me := s.messageExpiration
	if vtSecs > 0 {
		vt = time.Duration(vtSecs) * time.Second
	} else {
		vtSecs = int(vt.Seconds())
	}
	if expSecs > 0 {
		me = time.Duration(expSecs) * time.Second
	} else {
		expSecs = int(me.Seconds())
	}

	id := s.ids.Next()
	res := &storedQueue{
		ID:                       id,
		Name:                     name,
		Description:              description,
		Tags:                     tags,
		VisibilityTimeoutSeconds: vtSecs,
		ExpireSeconds:            expSecs,
		CreatedAt:                now,
		ModifiedAt:               now,
	}
	s.queueResources[id] = res
	s.queueByName[name] = id

	// Update existing message queue settings if already created via data plane
	if q, ok := s.queues[name]; ok {
		q.mu.Lock()
		q.visibilityTimeout = vt
		q.messageExpiration = me
		q.mu.Unlock()
	}

	return *res, nil
}

func (s *MemoryStore) ListQueues() ([]storedQueue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]storedQueue, 0, len(s.queueResources))
	for _, res := range s.queueResources {
		result = append(result, *res)
	}
	return result, nil
}

func (s *MemoryStore) GetQueueByID(id string) (storedQueue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, ok := s.queueResources[id]
	if !ok {
		return storedQueue{}, ErrQueueNotFound
	}
	return *res, nil
}

func (s *MemoryStore) GetQueueByName(name string) (storedQueue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.queueByName[name]
	if !ok {
		return storedQueue{}, ErrQueueNotFound
	}
	res, ok := s.queueResources[id]
	if !ok {
		return storedQueue{}, ErrQueueNotFound
	}
	return *res, nil
}

func (s *MemoryStore) UpdateQueue(id, description string, tags []string, vtSecs, expSecs int, now time.Time) (storedQueue, error) {
	if tags == nil {
		tags = []string{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	res, ok := s.queueResources[id]
	if !ok {
		return storedQueue{}, ErrQueueNotFound
	}

	res.Description = description
	res.Tags = tags
	res.VisibilityTimeoutSeconds = vtSecs
	res.ExpireSeconds = expSecs
	res.ModifiedAt = now

	vt := time.Duration(vtSecs) * time.Second
	me := time.Duration(expSecs) * time.Second
	if q, ok := s.queues[res.Name]; ok {
		q.mu.Lock()
		q.visibilityTimeout = vt
		q.messageExpiration = me
		q.mu.Unlock()
	}

	return *res, nil
}

func (s *MemoryStore) DeleteQueueByID(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, ok := s.queueResources[id]
	if !ok {
		return ErrQueueNotFound
	}

	name := res.Name
	delete(s.queueResources, id)
	delete(s.queueByName, name)
	delete(s.queues, name)
	return nil
}

func (s *MemoryStore) CountMessages(id string, now time.Time) (int, error) {
	s.mu.Lock()
	res, ok := s.queueResources[id]
	if !ok {
		s.mu.Unlock()
		return 0, ErrQueueNotFound
	}
	q := s.queues[res.Name]
	s.mu.Unlock()

	if q == nil {
		return 0, nil
	}
	return q.count(now), nil
}

func (s *MemoryStore) RotateAPIKey(id, newKey string, now time.Time) (storedQueue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, ok := s.queueResources[id]
	if !ok {
		return storedQueue{}, ErrQueueNotFound
	}
	res.APIKey = newKey
	res.ModifiedAt = now
	return *res, nil
}

func (s *MemoryStore) ClearMessages(id string) error {
	s.mu.Lock()
	res, ok := s.queueResources[id]
	if !ok {
		s.mu.Unlock()
		return ErrQueueNotFound
	}
	q := s.queues[res.Name]
	s.mu.Unlock()

	if q != nil {
		q.clear()
	}
	return nil
}
