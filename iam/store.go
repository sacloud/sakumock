package iam

import "time"

// UserRecord is a user of the organization.
type UserRecord struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Code        string    `json:"code"`
	Password    string    `json:"password"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
	Email       string    `json:"email"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UserTrustedDeviceRecord is a device the user marked as trusted during a
// two-factor login. The real API has no endpoint that creates one (it happens
// in the browser), so the mock seeds them through /_sakumock/.
type UserTrustedDeviceRecord struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// UserSecurityKeyRecord is a WebAuthn authenticator registered by the user.
// Like trusted devices, registration happens in the browser, so the mock seeds
// them through /_sakumock/.
type UserSecurityKeyRecord struct {
	ID           int        `json:"id"`
	UserID       int        `json:"user_id"`
	Name         string     `json:"name"`
	SignCount    int        `json:"sign_count"`
	AAGUID       string     `json:"aaguid"`
	RegisteredAt time.Time  `json:"registered_at"`
	LastUsedAt   *time.Time `json:"last_used_at"`
}

// GroupRecord is a group users can belong to.
type GroupRecord struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Members     []int     `json:"members"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ProjectRecord is a project that resources are scoped to.
type ProjectRecord struct {
	ID             int       `json:"id"`
	Code           string    `json:"code"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Status         string    `json:"status"`
	ParentFolderID *int      `json:"parent_folder_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// FolderRecord is a folder that groups projects.
type FolderRecord struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	ParentID    *int      `json:"parent_id"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ServicePrincipalRecord is a non-human identity belonging to a project.
type ServicePrincipalRecord struct {
	ID          int       `json:"id"`
	ProjectID   int       `json:"project_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ServicePrincipalKeyRecord is a public key uploaded for a service principal.
type ServicePrincipalKeyRecord struct {
	ID                 string    `json:"id"`
	ServicePrincipalID int       `json:"service_principal_id"`
	Kid                string    `json:"kid"`
	PublicKey          string    `json:"public_key"`
	Status             string    `json:"status"`
	KeyOrigin          string    `json:"key_origin"`
	CreatedAt          time.Time `json:"created_at"`
	KeyExpiresAt       string    `json:"key_expires_at"`
}

// ProjectAPIKeyRecord is an API key issued for a project.
type ProjectAPIKeyRecord struct {
	ID                int       `json:"id"`
	ProjectID         int       `json:"project_id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	AccessToken       string    `json:"access_token"`
	AccessTokenSecret string    `json:"access_token_secret"`
	ServerResourceID  string    `json:"server_resource_id"`
	IAMRoles          []string  `json:"iam_roles"`
	ZoneID            string    `json:"zone_id"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// IAMRoleRecord is a built-in IAM role that a policy binding can grant.
type IAMRoleRecord struct {
	ID                      string `json:"id"`
	Name                    string `json:"name"`
	Description             string `json:"description"`
	Category                string `json:"category"`
	LowestGrantableResource string `json:"lowest_grantable_resource"`
}

// IDRoleRecord is a built-in ID role that a policy binding can grant.
type IDRoleRecord struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
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
	ID             int       `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	SpEntityID     string    `json:"sp_entity_id"`
	SpAcsURL       string    `json:"sp_acs_url"`
	IdpEntityID    string    `json:"idp_entity_id"`
	IdpLoginURL    string    `json:"idp_login_url"`
	IdpLogoutURL   string    `json:"idp_logout_url"`
	IdpCertificate string    `json:"idp_certificate"`
	Assigned       bool      `json:"assigned"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ScimConfigurationRecord is a SCIM provisioning configuration and its token.
type ScimConfigurationRecord struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	BaseURL     string    `json:"base_url"`
	SecretToken string    `json:"secret_token"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
