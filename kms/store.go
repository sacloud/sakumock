package kms

import "time"

// KeyRecord represents a KMS key stored in the backend.
type KeyRecord struct {
	ID            string
	Name          string
	Description   string
	KeyOrigin     string // "generated" or "imported"
	Status        string // "active", "restricted", "suspended", "pending_destruction"
	LatestVersion int
	Tags          []string
	CreatedAt     time.Time
	ModifiedAt    time.Time
	// DeletionScheduledAfter is set while Status is "pending_destruction".
	DeletionScheduledAfter *time.Time
}

// Store is the storage backend for KMS keys and their key material.
type Store interface {
	List() []KeyRecord
	Read(id string) (KeyRecord, error)
	Create(name, description, keyOrigin string, tags []string) (KeyRecord, error)
	Update(id, name, description string, tags []string) (KeyRecord, error)
	Delete(id string) error
	// Rotate returns the unchanged key along with the error when the key
	// exists but is not active, and a zero KeyRecord when it does not exist.
	Rotate(id string) (KeyRecord, error)
	ChangeStatus(id, status string) error
	ScheduleDestruction(id string, after time.Time) (KeyRecord, error)
	Close() error
}
