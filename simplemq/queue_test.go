package simplemq

import (
	"testing"
	"time"
)

func TestVisibilityTimeoutRevisibility(t *testing.T) {
	q := newMemoryQueue(2*time.Second, time.Hour)
	now := time.Now()

	q.send("hello", now)

	// First receive succeeds
	msg, ok := q.receive(now)
	if !ok {
		t.Fatal("expected to receive a message")
	}

	// Within timeout: message should be invisible
	_, ok = q.receive(now.Add(1 * time.Second))
	if ok {
		t.Error("expected no message within visibility timeout")
	}

	// After timeout: message should be visible again
	msg2, ok := q.receive(now.Add(3 * time.Second))
	if !ok {
		t.Fatal("expected message to be visible again after timeout")
	}
	if msg2.ID != msg.ID {
		t.Errorf("expected same message ID %s, got %s", msg.ID, msg2.ID)
	}
}

func TestMessageExpiration(t *testing.T) {
	q := newMemoryQueue(time.Second, 5*time.Second)
	now := time.Now()

	q.send("expires soon", now)

	// Before expiration: message should be receivable
	msg, ok := q.receive(now.Add(4 * time.Second))
	if !ok {
		t.Fatal("expected to receive message before expiration")
	}

	// Return message to queue by waiting past visibility timeout
	// Then check after expiration: receive should skip expired messages
	_, ok = q.receive(now.Add(6 * time.Second))
	if ok {
		t.Error("expected message to be expired and not receivable")
	}

	// Verify compact removes expired messages
	q.mu.Lock()
	q.compact(now.Add(6 * time.Second))
	q.mu.Unlock()
	if len(q.messages) != 0 {
		t.Errorf("expected 0 messages after compact, got %d", len(q.messages))
	}

	_ = msg // use msg
}

func TestConfigVisibilityTimeoutPropagation(t *testing.T) {
	cfg := Config{VisibilityTimeout: 5 * time.Second}
	s := NewTestServer(cfg)
	defer s.Close()

	ms := s.store.(*MemoryStore)
	q := ms.getQueue("test-queue")
	if q.visibilityTimeout != 5*time.Second {
		t.Errorf("expected visibility timeout 5s, got %s", q.visibilityTimeout)
	}
}

func TestConfigMessageExpirePropagation(t *testing.T) {
	cfg := Config{MessageExpire: time.Hour}
	s := NewTestServer(cfg)
	defer s.Close()

	ms := s.store.(*MemoryStore)
	q := ms.getQueue("test-queue")
	if q.messageExpiration != time.Hour {
		t.Errorf("expected message expiration 1h, got %s", q.messageExpiration)
	}
}

func TestConfigDefaultValues(t *testing.T) {
	cfg := Config{}
	s := NewTestServer(cfg)
	defer s.Close()

	ms := s.store.(*MemoryStore)
	q := ms.getQueue("test-queue")
	if q.visibilityTimeout != defaultVisibilityTimeout {
		t.Errorf("expected default visibility timeout %s, got %s", defaultVisibilityTimeout, q.visibilityTimeout)
	}
	if q.messageExpiration != defaultMessageExpiration {
		t.Errorf("expected default message expiration %s, got %s", defaultMessageExpiration, q.messageExpiration)
	}
}
