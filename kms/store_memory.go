package kms

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/sacloud/sakumock/core"
)

// MemoryStore is the in-memory Store implementation.
type MemoryStore struct {
	mu   sync.RWMutex
	keys map[string]*KeyRecord
	ids  *core.IDGenerator
	// encryption key material per key ID per version (version -> 32-byte AES key)
	keyMaterial map[string]map[int][]byte
	logger      *slog.Logger
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore(logger *slog.Logger) *MemoryStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &MemoryStore{
		keys:        make(map[string]*KeyRecord),
		ids:         core.NewIDGenerator(core.DefaultIDBase()),
		keyMaterial: make(map[string]map[int][]byte),
		logger:      logger,
	}
}

func (s *MemoryStore) generateID() string {
	return s.ids.Next()
}

// generateKeyMaterial creates random version-1 material for a new key.
func (s *MemoryStore) generateKeyMaterial(id string) {
	key := make([]byte, 32) // AES-256
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		panic(fmt.Sprintf("failed to generate key material: %v", err))
	}
	s.keyMaterial[id] = map[int][]byte{1: key}
}

// rotateKeyMaterial derives the material for version "version" from the
// previous version (see nextKeyMaterial). Every key, preset or generated,
// rotates by the same chain, so rotation never draws fresh randomness.
func (s *MemoryStore) rotateKeyMaterial(id string, version int) {
	s.keyMaterial[id][version] = nextKeyMaterial(s.keyMaterial[id][version-1])
}

// Preset registers a key with a fixed ID and version-1 key material, already
// rotated to the given version (>= 1), as configured by --key, so
// encrypt/decrypt results survive a restart. Versions 2..version are derived
// by the rotation chain. The ID is reserved on the ID generator, so generated
// IDs never collide with it and, under the unified binary, another service
// presetting the same ID fails at startup.
func (s *MemoryStore) Preset(id string, material []byte, version int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ids.Reserve(id, "kms"); err != nil {
		return err
	}
	now := time.Now()
	s.keys[id] = &KeyRecord{
		ID:            id,
		Name:          "preset-" + id,
		Description:   "preset by --key",
		KeyOrigin:     "imported",
		Status:        "active",
		LatestVersion: version,
		Tags:          []string{},
		CreatedAt:     now,
		ModifiedAt:    now,
	}
	s.keyMaterial[id] = map[int][]byte{1: material}
	for v := 2; v <= version; v++ {
		s.rotateKeyMaterial(id, v)
	}
	s.logger.Debug("key preset", "id", id, "version", version)
	return nil
}

// List returns every key, oldest first.
func (s *MemoryStore) List() []KeyRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]KeyRecord, 0, len(s.keys))
	for _, k := range s.keys {
		result = append(result, *k)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// Read returns the key with the given ID.
func (s *MemoryStore) Read(id string) (KeyRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	k, ok := s.keys[id]
	if !ok {
		return KeyRecord{}, fmt.Errorf("key %q not found", id)
	}
	return *k, nil
}

// Create stores a new key and generates its first version of key material.
func (s *MemoryStore) Create(name, description, keyOrigin string, tags []string) (KeyRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	id := s.generateID()
	if tags == nil {
		tags = []string{}
	}
	k := &KeyRecord{
		ID:            id,
		Name:          name,
		Description:   description,
		KeyOrigin:     keyOrigin,
		Status:        "active",
		LatestVersion: 1,
		Tags:          tags,
		CreatedAt:     now,
		ModifiedAt:    now,
	}
	s.keys[id] = k
	s.generateKeyMaterial(id)
	s.logger.Debug("key created", "id", id, "name", name)
	return *k, nil
}

// Update applies the given values to an existing key.
func (s *MemoryStore) Update(id, name, description string, tags []string) (KeyRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	k, ok := s.keys[id]
	if !ok {
		return KeyRecord{}, fmt.Errorf("key %q not found", id)
	}
	k.Name = name
	k.Description = description
	if tags == nil {
		tags = []string{}
	}
	k.Tags = tags
	k.ModifiedAt = time.Now()
	s.logger.Debug("key updated", "id", id, "name", name)
	return *k, nil
}

// Delete removes the key and its key material.
func (s *MemoryStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.keys[id]; !ok {
		return fmt.Errorf("key %q not found", id)
	}
	delete(s.keys, id)
	delete(s.keyMaterial, id)
	s.logger.Debug("key deleted", "id", id)
	return nil
}

// Rotate adds a version of the key's material and makes it the latest.
func (s *MemoryStore) Rotate(id string) (KeyRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	k, ok := s.keys[id]
	if !ok {
		return KeyRecord{}, fmt.Errorf("key %q not found", id)
	}
	if k.Status != "active" {
		return KeyRecord{}, fmt.Errorf("key %q is not active", id)
	}
	k.LatestVersion++
	k.ModifiedAt = time.Now()
	s.rotateKeyMaterial(id, k.LatestVersion)
	s.logger.Debug("key rotated", "id", id, "version", k.LatestVersion)
	return *k, nil
}

// ChangeStatus sets the key's status, which controls whether it can be used.
func (s *MemoryStore) ChangeStatus(id, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	k, ok := s.keys[id]
	if !ok {
		return fmt.Errorf("key %q not found", id)
	}
	k.Status = status
	k.ModifiedAt = time.Now()
	s.logger.Debug("key status changed", "id", id, "status", status)
	return nil
}

// ciphertextVersionSize is the length of the key version prefix that starts
// every ciphertext, a big-endian uint32.
const ciphertextVersionSize = 4

// Encrypt encrypts plaintext using the latest version of the key material.
// The ciphertext is version (big-endian uint32) || GCM nonce || sealed data,
// base64-encoded, so Decrypt can select the key version directly.
func (s *MemoryStore) Encrypt(id string, plaintext []byte) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	k, ok := s.keys[id]
	if !ok {
		return "", fmt.Errorf("key %q not found", id)
	}
	if k.Status != "active" {
		return "", fmt.Errorf("key %q is not active", id)
	}
	gcm, err := newGCM(s.keyMaterial[id][k.LatestVersion])
	if err != nil {
		return "", err
	}
	out := make([]byte, ciphertextVersionSize+gcm.NonceSize())
	binary.BigEndian.PutUint32(out, uint32(k.LatestVersion))
	nonce := out[ciphertextVersionSize:]
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	out = gcm.Seal(out, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

// Decrypt decrypts ciphertext using the key version recorded in its prefix.
func (s *MemoryStore) Decrypt(id string, ciphertextB64 string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	k, ok := s.keys[id]
	if !ok {
		return nil, fmt.Errorf("key %q not found", id)
	}
	if k.Status == "suspended" {
		return nil, fmt.Errorf("key %q is suspended", id)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}
	if len(ciphertext) < ciphertextVersionSize {
		return nil, fmt.Errorf("invalid ciphertext: too short")
	}
	version := int(binary.BigEndian.Uint32(ciphertext))
	material, ok := s.keyMaterial[id][version]
	if !ok {
		return nil, fmt.Errorf("invalid ciphertext: key %q has no version %d (latest is %d)", id, version, k.LatestVersion)
	}
	gcm, err := newGCM(material)
	if err != nil {
		return nil, err
	}
	rest := ciphertext[ciphertextVersionSize:]
	if len(rest) < gcm.NonceSize() {
		return nil, fmt.Errorf("invalid ciphertext: too short")
	}
	nonce, ct := rest[:gcm.NonceSize()], rest[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}
	return plaintext, nil
}

// newGCM builds the AES-256-GCM AEAD for one version's key material.
func newGCM(material []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(material)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	return gcm, nil
}

// Close releases the resources held by the store.
func (s *MemoryStore) Close() error {
	return nil
}
