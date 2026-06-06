package secretmanager

import (
	"log/slog"
	"time"
)

// Vault is a SecretManager vault: the control-plane resource that holds secrets.
type Vault struct {
	ID          string
	Name        string
	Description string
	KmsKeyID    string
	Tags        []string
	CreatedAt   time.Time
	ModifiedAt  time.Time
}

// SecretMeta represents secret metadata returned by List.
type SecretMeta struct {
	Name          string
	LatestVersion int
}

// Store is the interface for SecretManager storage backends.
type Store interface {
	// Vault lifecycle (control plane).
	CreateVault(name, kmsKeyID, description string, tags []string) *Vault
	GetVault(id string) (*Vault, bool)
	ListVaults() []*Vault
	UpdateVault(id, name, description string, tags []string) (*Vault, bool)
	DeleteVault(id string) bool

	// Secret operations within a vault.
	List(vaultID string) []SecretMeta
	Create(vaultID, name, value string) (int, error)
	Unveil(vaultID, name string, version int) (value string, actualVersion int, err error)
	Delete(vaultID, name string) error

	// setLogger sets the service-tagged logger the store uses for operation
	// logs; the unified binary injects it so store logs identify their service.
	setLogger(*slog.Logger)

	Close() error
}
