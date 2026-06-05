package simplemq

import (
	"errors"
	"time"
)

const (
	defaultVisibilityTimeout = 30 * time.Second
	defaultMessageExpiration = 4 * 24 * time.Hour

	// queueIDBase is the starting value for generated queue resource IDs.
	// It is a realistic 12-digit value (matching the spec example
	// "113700153028") so IDs never carry leading zeros, which would not
	// round-trip through clients that parse the oneOf string|int ID as an
	// integer.
	queueIDBase int64 = 113700000000
)

var (
	ErrQueueNotFound = errors.New("queue not found")
	ErrQueueConflict = errors.New("queue name already exists")
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

type storedQueue struct {
	ID                       string
	Name                     string
	Description              string
	Tags                     []string
	VisibilityTimeoutSeconds int
	ExpireSeconds            int
	APIKey                   string
	CreatedAt                time.Time
	ModifiedAt               time.Time
}

// Store is the interface for message storage backends.
type Store interface {
	// Data plane operations
	Send(queueName, content string, now time.Time) (storedMessage, error)
	Receive(queueName string, now time.Time) (storedMessage, bool, error)
	ExtendTimeout(queueName, id string, now time.Time) (storedMessage, error)
	Delete(queueName, id string) error

	// Control plane operations
	CreateQueue(name, description string, tags []string, visibilityTimeoutSeconds, expireSeconds int, now time.Time) (storedQueue, error)
	ListQueues() ([]storedQueue, error)
	GetQueueByID(id string) (storedQueue, error)
	GetQueueByName(name string) (storedQueue, error)
	UpdateQueue(id, description string, tags []string, visibilityTimeoutSeconds, expireSeconds int, now time.Time) (storedQueue, error)
	DeleteQueueByID(id string) error
	CountMessages(id string, now time.Time) (int, error)
	RotateAPIKey(id, newKey string, now time.Time) (storedQueue, error)
	ClearMessages(id string) error

	Close() error
}
