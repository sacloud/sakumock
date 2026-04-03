package secretmanager

// SecretMeta represents secret metadata returned by List.
type SecretMeta struct {
	Name          string
	LatestVersion int
}

// Store is the interface for secret storage backends.
type Store interface {
	List(vaultID string) []SecretMeta
	Create(vaultID, name, value string) (int, error)
	Unveil(vaultID, name string, version int) (value string, actualVersion int, err error)
	Delete(vaultID, name string) error
	Close() error
}
