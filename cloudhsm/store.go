package cloudhsm

import "time"

// CloudHSMRecord represents a CloudHSM partition stored in the backend.
type CloudHSMRecord struct {
	ID                 string
	Name               string
	Description        string
	Availability       string // "precreate", "available", "discontinued"
	Tags               []string
	IPv4NetworkAddress string
	IPv4PrefixLength   int
	IPv4Address        string
	CreatedAt          time.Time
	ModifiedAt         time.Time
}

// ClientRecord represents a client certificate registered against a CloudHSM.
type ClientRecord struct {
	ID           string
	CloudHSMID   string
	Name         string
	Availability string
	Certificate  string
	CreatedAt    time.Time
	ModifiedAt   time.Time
}

// PeerRecord represents an IPsec peer registered against a CloudHSM.
type PeerRecord struct {
	ID          string
	CloudHSMID  string
	SecretKey   string
	Enabled     bool
	Description string
}

// LicenseRecord represents a CloudHSM software license, a top-level resource
// independent of any specific CloudHSM partition.
type LicenseRecord struct {
	ID          string
	Name        string
	Description string
	Tags        []string
	CreatedAt   time.Time
	ModifiedAt  time.Time
}

// DocumentRecord represents a downloadable document attached to a software
// license.
type DocumentRecord struct {
	ID         string
	LicenseID  string
	Name       string
	CreatedAt  time.Time
	ModifiedAt time.Time
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
