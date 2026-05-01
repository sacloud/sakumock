package simplenotification

// NewStore creates a Store based on configuration.
// Currently only in-memory storage is supported.
func NewStore() *MemoryStore {
	return NewMemoryStore()
}
