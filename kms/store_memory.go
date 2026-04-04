package kms

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// MemoryStore is an in-memory Store for KMS keys.
type MemoryStore struct {
	mu     sync.RWMutex
	keys   map[string]*KeyRecord
	nextID int64
	// encryption key material per key ID per version (version -> 32-byte AES key)
	keyMaterial map[string]map[int][]byte
}

// NewMemoryStore creates a new empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		keys:        make(map[string]*KeyRecord),
		nextID:      110000000001,
		keyMaterial: make(map[string]map[int][]byte),
	}
}

func (s *MemoryStore) generateID() string {
	id := fmt.Sprintf("%012d", s.nextID)
	s.nextID++
	return id
}

func (s *MemoryStore) generateKeyMaterial(id string, version int) {
	if _, ok := s.keyMaterial[id]; !ok {
		s.keyMaterial[id] = make(map[int][]byte)
	}
	key := make([]byte, 32) // AES-256
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		panic(fmt.Sprintf("failed to generate key material: %v", err))
	}
	s.keyMaterial[id][version] = key
}

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

func (s *MemoryStore) Read(id string) (KeyRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	k, ok := s.keys[id]
	if !ok {
		return KeyRecord{}, fmt.Errorf("key %q not found", id)
	}
	return *k, nil
}

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
	s.generateKeyMaterial(id, 1)
	slog.Debug("key created", "id", id, "name", name)
	return *k, nil
}

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
	slog.Debug("key updated", "id", id, "name", name)
	return *k, nil
}

func (s *MemoryStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.keys[id]; !ok {
		return fmt.Errorf("key %q not found", id)
	}
	delete(s.keys, id)
	delete(s.keyMaterial, id)
	slog.Debug("key deleted", "id", id)
	return nil
}

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
	s.generateKeyMaterial(id, k.LatestVersion)
	slog.Debug("key rotated", "id", id, "version", k.LatestVersion)
	return *k, nil
}

func (s *MemoryStore) ChangeStatus(id, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	k, ok := s.keys[id]
	if !ok {
		return fmt.Errorf("key %q not found", id)
	}
	k.Status = status
	k.ModifiedAt = time.Now()
	slog.Debug("key status changed", "id", id, "status", status)
	return nil
}

// Encrypt encrypts plaintext using the latest version of the key material.
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
	material := s.keyMaterial[id][k.LatestVersion]
	block, err := aes.NewCipher(material)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts ciphertext using the key material (tries all versions).
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

	versions := s.keyMaterial[id]
	for _, material := range versions {
		block, err := aes.NewCipher(material)
		if err != nil {
			continue
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			continue
		}
		if len(ciphertext) < gcm.NonceSize() {
			continue
		}
		nonce, ct := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
		plaintext, err := gcm.Open(nil, nonce, ct, nil)
		if err != nil {
			continue
		}
		return plaintext, nil
	}
	return nil, fmt.Errorf("failed to decrypt: no matching key version")
}

func (s *MemoryStore) Close() error {
	return nil
}
