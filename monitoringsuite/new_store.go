package monitoringsuite

// NewStore creates the store backing the mock.
func NewStore() *MemoryStore {
	return NewMemoryStore()
}
