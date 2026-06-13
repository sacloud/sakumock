package objectstorage

import "time"

// Bucket is a control-plane object storage bucket. Buckets are globally unique
// across the federation, so the store keys them by name; ClusterID records the
// site (cluster) the bucket belongs to, which is how per-site listing filters.
type Bucket struct {
	Name             string
	ClusterID        string
	ResourceID       string
	PlanType         string // "standard" or "archive"
	ServiceClassPath string
	CreatedAt        time.Time
	Encryption       *Encryption
	Replication      *Replication
}

// Encryption is a bucket's server-side encryption configuration.
type Encryption struct {
	KMSKeyID     string
	ConfiguredAt time.Time
}

// Replication is a bucket's replication configuration.
type Replication struct {
	DestBucket   string
	ConfigStatus string // "creating" or "created"
	CreatedAt    time.Time
}

// Account is the per-site root account. The real API creates it automatically
// when the control panel first accesses a site; the mock creates it on demand.
type Account struct {
	ResourceID string
	Code       string
	CreatedAt  time.Time
	Keys       []AccountKey
}

// AccountKey is a root-user access key. Secret is only returned when the key is
// created; subsequent reads return it empty, mirroring the real API.
type AccountKey struct {
	ID        string
	Secret    string
	CreatedAt time.Time
}

// Permission is a per-site permission (a scoped IAM-like principal) with its own
// bucket controls and access keys.
type Permission struct {
	ID             int64
	DisplayName    string
	BucketControls []BucketControl
	CreatedAt      time.Time
	Keys           []PermissionKey
}

// BucketControl grants a permission read/write access to a bucket.
type BucketControl struct {
	BucketName string
	CanRead    bool
	CanWrite   bool
	CreatedAt  time.Time
}

// PermissionKey is an access key issued for a permission. As with AccountKey,
// Secret is only present on creation.
type PermissionKey struct {
	ID        string
	Secret    string
	CreatedAt time.Time
}

// Store is the interface for object storage control-plane storage backends.
//
// Buckets are federation-global (keyed by name); accounts and permissions are
// per-site (keyed by site ID), as in the real API where each cluster owns its
// own account and permission set.
type Store interface {
	// Buckets (federation-global).
	CreateBucket(name, clusterID string, planType, serviceClassPath string) (Bucket, bool) // ok=false when the name already exists
	GetBucket(name string) (Bucket, bool)
	ListBuckets(siteID string) []Bucket
	DeleteBucket(name string) bool
	SetBucketEncryption(name, kmsKeyID string) (Bucket, bool)
	DeleteBucketEncryption(name string) bool
	SetBucketReplication(name, destBucket string) (Bucket, bool)
	DeleteBucketReplication(name string) bool

	// Accounts (per-site).
	CreateAccount(siteID string) (Account, bool) // ok=false when an account already exists
	GetAccount(siteID string) (Account, bool)
	DeleteAccount(siteID string) bool
	CreateAccountKey(siteID string) (AccountKey, bool) // ok=false when no account exists
	ListAccountKeys(siteID string) ([]AccountKey, bool)
	GetAccountKey(siteID, keyID string) (AccountKey, bool)
	DeleteAccountKey(siteID, keyID string) bool

	// Permissions (per-site).
	CreatePermission(siteID, displayName string, controls []BucketControl) Permission
	ListPermissions(siteID string) []Permission
	GetPermission(siteID string, id int64) (Permission, bool)
	UpdatePermission(siteID string, id int64, displayName string, controls []BucketControl) (Permission, bool)
	DeletePermission(siteID string, id int64) bool
	CreatePermissionKey(siteID string, id int64) (PermissionKey, bool) // ok=false when the permission does not exist
	ListPermissionKeys(siteID string, id int64) ([]PermissionKey, bool)
	GetPermissionKey(siteID string, id int64, keyID string) (PermissionKey, bool)
	DeletePermissionKey(siteID string, id int64, keyID string) bool

	Close() error
}
