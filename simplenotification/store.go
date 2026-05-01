package simplenotification

import "time"

// MessageRecord represents a notification message accepted by the mock.
type MessageRecord struct {
	ID        string
	GroupID   string
	Message   string
	CreatedAt time.Time
}

// Store is the interface for notification message storage backends.
type Store interface {
	Send(groupID, message string, now time.Time) (MessageRecord, error)
	List() []MessageRecord
	Reset()
	Close() error
}
