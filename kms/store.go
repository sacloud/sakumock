package kms

import "time"

// KeyRecord represents a KMS key stored in the backend. The json tags are its
// form in the state file (--state-dir), not in API responses.
type KeyRecord struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	KeyOrigin     string    `json:"key_origin"` // "generated" or "imported"
	Status        string    `json:"status"`     // "active", "restricted", "suspended", "pending_destruction"
	LatestVersion int       `json:"latest_version"`
	Tags          []string  `json:"tags"`
	CreatedAt     time.Time `json:"created_at"`
	ModifiedAt    time.Time `json:"modified_at"`
	// DeletionScheduledAfter is set while Status is "pending_destruction".
	DeletionScheduledAfter *time.Time `json:"deletion_scheduled_after,omitempty"`
}

// Store is the storage backend for KMS keys and their key material.
type Store interface {
	List() []KeyRecord
	Read(id string) (KeyRecord, error)
	Create(name, description, keyOrigin string, tags []string) (KeyRecord, error)
	Update(id, name, description string, tags []string) (KeyRecord, error)
	// Rotate returns the unchanged key along with the error when the key
	// exists but is not active, and a zero KeyRecord when it does not exist.
	Rotate(id string) (KeyRecord, error)
	ChangeStatus(id, status string) error
	ScheduleDestruction(id string, after time.Time) (KeyRecord, error)
	Close() error
}
