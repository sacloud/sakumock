package cloudhsm

import "time"

// CloudHSMRecord represents a CloudHSM partition stored in the backend. The
// json tags of the record types are their form in the state file
// (--state-dir), not in API responses.
type CloudHSMRecord struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	Availability       string    `json:"availability"` // "precreate", "available", "discontinued"
	Tags               []string  `json:"tags"`
	IPv4NetworkAddress string    `json:"ipv4_network_address"`
	IPv4PrefixLength   int       `json:"ipv4_prefix_length"`
	IPv4Address        string    `json:"ipv4_address"`
	CreatedAt          time.Time `json:"created_at"`
	ModifiedAt         time.Time `json:"modified_at"`
}

// ClientRecord represents a client certificate registered against a CloudHSM.
type ClientRecord struct {
	ID           string    `json:"id"`
	CloudHSMID   string    `json:"cloudhsm_id"`
	Name         string    `json:"name"`
	Availability string    `json:"availability"`
	Certificate  string    `json:"certificate"`
	CreatedAt    time.Time `json:"created_at"`
	ModifiedAt   time.Time `json:"modified_at"`
}

// PeerRecord represents an IPsec peer registered against a CloudHSM.
type PeerRecord struct {
	ID          string `json:"id"`
	CloudHSMID  string `json:"cloudhsm_id"`
	SecretKey   string `json:"secret_key"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
}

// LicenseRecord represents a CloudHSM software license, a top-level resource
// independent of any specific CloudHSM partition.
type LicenseRecord struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	ModifiedAt  time.Time `json:"modified_at"`
}

// DocumentRecord represents a downloadable document attached to a software
// license.
type DocumentRecord struct {
	ID         string    `json:"id"`
	LicenseID  string    `json:"license_id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	ModifiedAt time.Time `json:"modified_at"`
}

// Store is the storage backend for CloudHSM partitions, their clients and
// peers, and software licenses with their documents.
type Store interface {
	ListCloudHSMs() []CloudHSMRecord
	ReadCloudHSM(id string) (CloudHSMRecord, error)
	CreateCloudHSM(name, description string, tags []string, ipv4Net string, ipv4Prefix int) (CloudHSMRecord, error)
	UpdateCloudHSM(id, name, description string, tags []string, ipv4Net string, ipv4Prefix int) (CloudHSMRecord, error)
	DeleteCloudHSM(id string) error

	ListClients(hsmID string) ([]ClientRecord, error)
	CreateClient(hsmID, name, certificate string) (ClientRecord, error)
	ReadClient(hsmID, id string) (ClientRecord, error)
	UpdateClient(hsmID, id, name string) (ClientRecord, error)
	DeleteClient(hsmID, id string) error

	ListPeers(hsmID string) ([]PeerRecord, error)
	CreatePeer(hsmID, peerID, secretKey string) (PeerRecord, error)
	DeletePeer(hsmID, peerID string) error

	ListLicenses() []LicenseRecord
	CreateLicense(name, description string, tags []string) (LicenseRecord, error)
	ReadLicense(id string) (LicenseRecord, error)
	UpdateLicense(id, name, description string, tags []string) (LicenseRecord, error)
	DeleteLicense(id string) error

	ListDocuments(licenseID string) ([]DocumentRecord, error)
	ReadDocument(licenseID, id string) (DocumentRecord, error)

	Close() error
}
