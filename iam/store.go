package iam

import "time"

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

type GroupRecord struct {
	ID          int
	Name        string
	Description string
	Members     []int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

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

type FolderRecord struct {
	ID          int
	Name        string
	ParentID    *int
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ServicePrincipalRecord struct {
	ID          int
	ProjectID   int
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

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

type IAMRoleRecord struct {
	ID                      string
	Name                    string
	Description             string
	Category                string
	LowestGrantableResource string
}

type IDRoleRecord struct {
	ID          string
	Name        string
	Description string
}

type PolicyBinding struct {
	Role       PolicyRole        `json:"role"`
	Principals []PolicyPrincipal `json:"principals"`
}

type PolicyRole struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type PolicyPrincipal struct {
	Type string `json:"type"`
	ID   int    `json:"id"`
}

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

type ScimConfigurationRecord struct {
	ID          string
	Name        string
	BaseURL     string
	SecretToken string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
