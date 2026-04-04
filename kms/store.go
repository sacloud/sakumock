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
}

// Store is the interface for KMS key storage backends.
type Store interface {
	List() []KeyRecord
	Read(id string) (KeyRecord, error)
	Create(name, description, keyOrigin string, tags []string) (KeyRecord, error)
	Update(id, name, description string, tags []string) (KeyRecord, error)
	Delete(id string) error
	Rotate(id string) (KeyRecord, error)
	ChangeStatus(id, status string) error
	Close() error
}
