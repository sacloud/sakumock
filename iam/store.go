package iam

import "time"

// UserRecord is a user of the organization.
type UserRecord struct {
	ID          int
	Name        string
	Code        string
	Password    string
	Status      string
	Description string
	Email       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// UserTrustedDeviceRecord is a device the user marked as trusted during a
// two-factor login. The real API has no endpoint that creates one (it happens
// in the browser), so the mock seeds them through /_sakumock/.
type UserTrustedDeviceRecord struct {
	ID        int
	UserID    int
	Name      string
	CreatedAt time.Time
}

// UserSecurityKeyRecord is a WebAuthn authenticator registered by the user.
// Like trusted devices, registration happens in the browser, so the mock seeds
// them through /_sakumock/.
type UserSecurityKeyRecord struct {
	ID           int
	UserID       int
	Name         string
	SignCount    int
	AAGUID       string
	RegisteredAt time.Time
	LastUsedAt   *time.Time
}

// GroupRecord is a group users can belong to.
type GroupRecord struct {
	ID          int
	Name        string
	Description string
	Members     []int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ProjectRecord is a project that resources are scoped to.
type ProjectRecord struct {
	ID             int
	Code           string
	Name           string
	Description    string
	Status         string
	ParentFolderID *int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// FolderRecord is a folder that groups projects.
type FolderRecord struct {
	ID          int
	Name        string
	ParentID    *int
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ServicePrincipalRecord is a non-human identity belonging to a project.
type ServicePrincipalRecord struct {
	ID          int
	ProjectID   int
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ServicePrincipalKeyRecord is a public key uploaded for a service principal.
type ServicePrincipalKeyRecord struct {
	ID                 string
	ServicePrincipalID int
	Kid                string
	PublicKey          string
	Status             string
	KeyOrigin          string
	CreatedAt          time.Time
	KeyExpiresAt       string
}

// ProjectAPIKeyRecord is an API key issued for a project.
type ProjectAPIKeyRecord struct {
	ID                int
	ProjectID         int
	Name              string
	Description       string
	AccessToken       string
	AccessTokenSecret string
	ServerResourceID  string
	IAMRoles          []string
	ZoneID            string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// IAMRoleRecord is a built-in IAM role that a policy binding can grant.
type IAMRoleRecord struct {
	ID                      string
	Name                    string
	Description             string
	Category                string
	LowestGrantableResource string
}

// IDRoleRecord is a built-in ID role that a policy binding can grant.
type IDRoleRecord struct {
	ID          string
	Name        string
	Description string
}

// PolicyBinding grants one role to a set of principals.
type PolicyBinding struct {
	Role       PolicyRole        `json:"role"`
	Principals []PolicyPrincipal `json:"principals"`
}

// PolicyRole identifies the role granted by a PolicyBinding.
type PolicyRole struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// PolicyPrincipal identifies one holder of a PolicyBinding.
type PolicyPrincipal struct {
	Type string `json:"type"`
	ID   int    `json:"id"`
}

// SSOProfileRecord is a single sign-on profile of the organization.
type SSOProfileRecord struct {
	ID             int
	Name           string
	Description    string
	SpEntityID     string
	SpAcsURL       string
	IdpEntityID    string
	IdpLoginURL    string
	IdpLogoutURL   string
	IdpCertificate string
	Assigned       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ScimConfigurationRecord is a SCIM provisioning configuration and its token.
type ScimConfigurationRecord struct {
	ID          string
	Name        string
	BaseURL     string
	SecretToken string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
