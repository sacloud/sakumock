package objectstorage

import (
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sacloud/sakumock/core"
)

// MemoryStore is an in-memory Store for object storage control-plane resources.
type MemoryStore struct {
	mu sync.Mutex
	// buckets are federation-global, keyed by bucket name.
	buckets map[string]*Bucket
	// accounts are per-site, keyed by site ID.
	accounts map[string]*Account
	// permissions are per-site: site ID -> permission ID -> permission.
	permissions map[string]map[int64]*Permission
	ids         *core.IDGenerator
	logger      *slog.Logger
}

// NewMemoryStore creates a new empty MemoryStore. logger is the service-tagged
// logger used for operation logs; nil falls back to the default.
func NewMemoryStore(logger *slog.Logger) *MemoryStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &MemoryStore{
		buckets:     make(map[string]*Bucket),
		accounts:    make(map[string]*Account),
		permissions: make(map[string]map[int64]*Permission),
		ids:         core.NewIDGenerator(core.DefaultIDBase()),
		logger:      logger,
	}
}

// newKeyID returns a fresh access key ID. It satisfies the API's AccessKeyID /
// PermissionKeyID patterns (word characters, <= 40 chars).
func newKeyID() string {
	return uuid.NewString()[:8] + uuid.NewString()[:8]
}

// newSecret returns a fresh access key secret matching the SecretAccessKey /
// PermissionSecret patterns ([\w\d/+=], <= 40 chars). UUID hyphens are stripped
// because the pattern's \w does not allow them.
func newSecret() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}

func cloneBucket(b *Bucket) Bucket {
	c := *b
	if b.Encryption != nil {
		e := *b.Encryption
		c.Encryption = &e
	}
	if b.Replication != nil {
		r := *b.Replication
		c.Replication = &r
	}
	return c
}

// CreateBucket stores a new bucket. ok is false when a bucket with the same name
// already exists (the real API rejects duplicate names with 409).
func (s *MemoryStore) CreateBucket(name, clusterID, planType, serviceClassPath string) (Bucket, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.buckets[name]; exists {
		return Bucket{}, false
	}
	b := &Bucket{
		Name:             name,
		ClusterID:        clusterID,
		ResourceID:       s.ids.Next(),
		PlanType:         planType,
		ServiceClassPath: serviceClassPath,
		CreatedAt:        time.Now(),
	}
	s.buckets[name] = b
	s.logger.Info("bucket created", "name", name, "cluster", clusterID, "resource_id", b.ResourceID)
	return cloneBucket(b), true
}

func (s *MemoryStore) GetBucket(name string) (Bucket, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.buckets[name]
	if !ok {
		return Bucket{}, false
	}
	return cloneBucket(b), true
}

// ListBuckets returns the buckets belonging to the given site (cluster),
// ordered by name. An empty siteID returns every bucket.
func (s *MemoryStore) ListBuckets(siteID string) []Bucket {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Bucket, 0, len(s.buckets))
	for _, b := range s.buckets {
		if siteID != "" && b.ClusterID != siteID {
			continue
		}
		out = append(out, cloneBucket(b))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *MemoryStore) DeleteBucket(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.buckets[name]; !ok {
		return false
	}
	delete(s.buckets, name)
	s.logger.Info("bucket deleted", "name", name)
	return true
}

func (s *MemoryStore) SetBucketEncryption(name, kmsKeyID string) (Bucket, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.buckets[name]
	if !ok {
		return Bucket{}, false
	}
	b.Encryption = &Encryption{KMSKeyID: kmsKeyID, ConfiguredAt: time.Now()}
	return cloneBucket(b), true
}

func (s *MemoryStore) DeleteBucketEncryption(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.buckets[name]
	if !ok {
		return false
	}
	b.Encryption = nil
	return true
}

func (s *MemoryStore) SetBucketReplication(name, destBucket string) (Bucket, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.buckets[name]
	if !ok {
		return Bucket{}, false
	}
	b.Replication = &Replication{DestBucket: destBucket, ConfigStatus: "created", CreatedAt: time.Now()}
	return cloneBucket(b), true
}

func (s *MemoryStore) DeleteBucketReplication(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.buckets[name]
	if !ok {
		return false
	}
	b.Replication = nil
	return true
}

func cloneAccount(a *Account) Account {
	c := *a
	c.Keys = append([]AccountKey(nil), a.Keys...)
	return c
}

func (s *MemoryStore) CreateAccount(siteID string) (Account, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.accounts[siteID]; exists {
		return Account{}, false
	}
	id := s.ids.Next()
	a := &Account{
		ResourceID: id,
		Code:       id + "@sakumock@" + siteID,
		CreatedAt:  time.Now(),
	}
	s.accounts[siteID] = a
	s.logger.Info("account created", "site", siteID, "resource_id", id)
	return cloneAccount(a), true
}

func (s *MemoryStore) GetAccount(siteID string) (Account, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[siteID]
	if !ok {
		return Account{}, false
	}
	return cloneAccount(a), true
}

func (s *MemoryStore) DeleteAccount(siteID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.accounts[siteID]; !ok {
		return false
	}
	delete(s.accounts, siteID)
	return true
}

func (s *MemoryStore) CreateAccountKey(siteID string) (AccountKey, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[siteID]
	if !ok {
		return AccountKey{}, false
	}
	k := AccountKey{ID: newKeyID(), Secret: newSecret(), CreatedAt: time.Now()}
	a.Keys = append(a.Keys, k)
	return k, true
}

func (s *MemoryStore) ListAccountKeys(siteID string) ([]AccountKey, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[siteID]
	if !ok {
		return nil, false
	}
	return append([]AccountKey(nil), a.Keys...), true
}

func (s *MemoryStore) GetAccountKey(siteID, keyID string) (AccountKey, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[siteID]
	if !ok {
		return AccountKey{}, false
	}
	for _, k := range a.Keys {
		if k.ID == keyID {
			return k, true
		}
	}
	return AccountKey{}, false
}

func (s *MemoryStore) DeleteAccountKey(siteID, keyID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[siteID]
	if !ok {
		return false
	}
	for i, k := range a.Keys {
		if k.ID == keyID {
			a.Keys = append(a.Keys[:i], a.Keys[i+1:]...)
			return true
		}
	}
	return false
}

func clonePermission(p *Permission) Permission {
	c := *p
	c.BucketControls = append([]BucketControl(nil), p.BucketControls...)
	c.Keys = append([]PermissionKey(nil), p.Keys...)
	return c
}

func (s *MemoryStore) sitePermissions(siteID string) map[int64]*Permission {
	m, ok := s.permissions[siteID]
	if !ok {
		m = make(map[int64]*Permission)
		s.permissions[siteID] = m
	}
	return m
}

func (s *MemoryStore) CreatePermission(siteID, displayName string, controls []BucketControl) Permission {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for i := range controls {
		controls[i].CreatedAt = now
	}
	id, _ := strconv.ParseInt(s.ids.Next(), 10, 64)
	p := &Permission{
		ID:             id,
		DisplayName:    displayName,
		BucketControls: controls,
		CreatedAt:      now,
	}
	s.sitePermissions(siteID)[id] = p
	s.logger.Info("permission created", "site", siteID, "id", id, "display_name", displayName)
	return clonePermission(p)
}

func (s *MemoryStore) ListPermissions(siteID string) []Permission {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.permissions[siteID]
	out := make([]Permission, 0, len(m))
	for _, p := range m {
		out = append(out, clonePermission(p))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *MemoryStore) GetPermission(siteID string, id int64) (Permission, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.permissions[siteID][id]
	if !ok {
		return Permission{}, false
	}
	return clonePermission(p), true
}

func (s *MemoryStore) UpdatePermission(siteID string, id int64, displayName string, controls []BucketControl) (Permission, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.permissions[siteID][id]
	if !ok {
		return Permission{}, false
	}
	now := time.Now()
	for i := range controls {
		controls[i].CreatedAt = now
	}
	p.DisplayName = displayName
	p.BucketControls = controls
	return clonePermission(p), true
}

func (s *MemoryStore) DeletePermission(siteID string, id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.permissions[siteID]
	if _, ok := m[id]; !ok {
		return false
	}
	delete(m, id)
	return true
}

func (s *MemoryStore) CreatePermissionKey(siteID string, id int64) (PermissionKey, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.permissions[siteID][id]
	if !ok {
		return PermissionKey{}, false
	}
	k := PermissionKey{ID: newKeyID(), Secret: newSecret(), CreatedAt: time.Now()}
	p.Keys = append(p.Keys, k)
	return k, true
}

func (s *MemoryStore) ListPermissionKeys(siteID string, id int64) ([]PermissionKey, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.permissions[siteID][id]
	if !ok {
		return nil, false
	}
	return append([]PermissionKey(nil), p.Keys...), true
}

func (s *MemoryStore) GetPermissionKey(siteID string, id int64, keyID string) (PermissionKey, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.permissions[siteID][id]
	if !ok {
		return PermissionKey{}, false
	}
	for _, k := range p.Keys {
		if k.ID == keyID {
			return k, true
		}
	}
	return PermissionKey{}, false
}

func (s *MemoryStore) DeletePermissionKey(siteID string, id int64, keyID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.permissions[siteID][id]
	if !ok {
		return false
	}
	for i, k := range p.Keys {
		if k.ID == keyID {
			p.Keys = append(p.Keys[:i], p.Keys[i+1:]...)
			return true
		}
	}
	return false
}

// Close releases resources held by the store.
func (s *MemoryStore) Close() error {
	return nil
}
