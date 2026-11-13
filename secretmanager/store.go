package secretmanager

import "time"

// Vault is a SecretManager vault: the control-plane resource that holds
// secrets. The json tags are its form in the state file (--state-dir), not in
// API responses.
type Vault struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	KmsKeyID    string    `json:"kms_key_id"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	ModifiedAt  time.Time `json:"modified_at"`
}

// SecretMeta is the metadata List reports for a secret; only Unveil returns
// the value itself.
type SecretMeta struct {
	Name          string
	LatestVersion int
}

// Store is the storage backend for vaults and their versioned secrets.
type Store interface {
	CreateVault(name, kmsKeyID, description string, tags []string) *Vault
	GetVault(id string) (*Vault, bool)
	ListVaults() []*Vault
	UpdateVault(id, name, description string, tags []string) (*Vault, bool)
	DeleteVault(id string) bool

	List(vaultID string) []SecretMeta
	Create(vaultID, name, value string) (int, error)
	Unveil(vaultID, name string, version int) (value string, actualVersion int, err error)
	Delete(vaultID, name string) error

	Close() error
}
