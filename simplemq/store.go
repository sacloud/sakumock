package simplemq

import "time"

const (
	defaultVisibilityTimeout = 30 * time.Second
	defaultMessageExpiration = 4 * 24 * time.Hour
)

type storedMessage struct {
	ID                  string
	Content             string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	ExpiresAt           time.Time
	AcquiredAt          time.Time
	VisibilityTimeoutAt time.Time
}

// Store is the interface for message storage backends.
type Store interface {
	Send(queueName, content string, now time.Time) (storedMessage, error)
	Receive(queueName string, now time.Time) (storedMessage, bool, error)
	ExtendTimeout(queueName, id string, now time.Time) (storedMessage, error)
	Delete(queueName, id string) error
	Close() error
}
