package cloudhsm

import (
	"fmt"
	"log/slog"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/sacloud/sakumock/core"
)

// firstUsableAddress returns the first host address in ipv4Net (network
// address + 1), mirroring how the real API assigns the CloudHSM's own
// address within its network. Returns "" if ipv4Net is not a valid IPv4
// address.
func firstUsableAddress(ipv4Net string) string {
	ip := net.ParseIP(ipv4Net).To4()
	if ip == nil {
		return ""
	}
	addr := make(net.IP, len(ip))
	copy(addr, ip)
	addr[3]++
	return addr.String()
}

// MemoryStore is the in-memory Store implementation.
type MemoryStore struct {
	mu       sync.RWMutex
	hsms     map[string]*CloudHSMRecord
	clients  map[string]map[string]*ClientRecord // hsmID -> clientID -> record
	peers    map[string]map[string]*PeerRecord   // hsmID -> peerID -> record
	licenses map[string]*LicenseRecord
	ids      *core.IDGenerator
	logger   *slog.Logger
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore(logger *slog.Logger) *MemoryStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &MemoryStore{
		hsms:     make(map[string]*CloudHSMRecord),
		clients:  make(map[string]map[string]*ClientRecord),
		peers:    make(map[string]map[string]*PeerRecord),
		licenses: make(map[string]*LicenseRecord),
		ids:      core.NewIDGenerator(core.DefaultIDBase()),
		logger:   logger,
	}
}

func (s *MemoryStore) generateID() string {
	return s.ids.Next()
}

// ListCloudHSMs returns every CloudHSM, oldest first.
func (s *MemoryStore) ListCloudHSMs() []CloudHSMRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]CloudHSMRecord, 0, len(s.hsms))
	for _, h := range s.hsms {
		result = append(result, *h)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// ReadCloudHSM returns the CloudHSM with the given ID.
func (s *MemoryStore) ReadCloudHSM(id string) (CloudHSMRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	h, ok := s.hsms[id]
	if !ok {
		return CloudHSMRecord{}, fmt.Errorf("cloudhsm %q not found", id)
	}
	return *h, nil
}

// CreateCloudHSM stores a new CloudHSM and returns it.
func (s *MemoryStore) CreateCloudHSM(name, description string, tags []string, ipv4Net string, ipv4Prefix int) (CloudHSMRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	id := s.generateID()
	if tags == nil {
		tags = []string{}
	}
	h := &CloudHSMRecord{
		ID:                 id,
		Name:               name,
		Description:        description,
		Availability:       "available",
		Tags:               tags,
		Ipv4NetworkAddress: ipv4Net,
		Ipv4PrefixLength:   ipv4Prefix,
		Ipv4Address:        firstUsableAddress(ipv4Net),
		CreatedAt:          now,
		ModifiedAt:         now,
	}
	s.hsms[id] = h
	s.clients[id] = make(map[string]*ClientRecord)
	s.peers[id] = make(map[string]*PeerRecord)
	s.logger.Debug("cloudhsm created", "id", id, "name", name)
	return *h, nil
}

// UpdateCloudHSM applies the given values to an existing CloudHSM.
func (s *MemoryStore) UpdateCloudHSM(id, name, description string, tags []string, ipv4Net string, ipv4Prefix int) (CloudHSMRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	h, ok := s.hsms[id]
	if !ok {
		return CloudHSMRecord{}, fmt.Errorf("cloudhsm %q not found", id)
	}
	h.Name = name
	h.Description = description
	if tags == nil {
		tags = []string{}
	}
	h.Tags = tags
	h.Ipv4NetworkAddress = ipv4Net
	h.Ipv4PrefixLength = ipv4Prefix
	h.Ipv4Address = firstUsableAddress(ipv4Net)
	h.ModifiedAt = time.Now()
	s.logger.Debug("cloudhsm updated", "id", id, "name", name)
	return *h, nil
}

// DeleteCloudHSM removes the CloudHSM together with its clients and peers.
func (s *MemoryStore) DeleteCloudHSM(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.hsms[id]; !ok {
		return fmt.Errorf("cloudhsm %q not found", id)
	}
	delete(s.hsms, id)
	delete(s.clients, id)
	delete(s.peers, id)
	s.logger.Debug("cloudhsm deleted", "id", id)
	return nil
}

// ListClients returns the clients registered against the CloudHSM.
func (s *MemoryStore) ListClients(hsmID string) ([]ClientRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clients, ok := s.clients[hsmID]
	if !ok {
		return nil, fmt.Errorf("cloudhsm %q not found", hsmID)
	}
	result := make([]ClientRecord, 0, len(clients))
	for _, c := range clients {
		result = append(result, *c)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

// CreateClient registers a client certificate against the CloudHSM.
func (s *MemoryStore) CreateClient(hsmID, name, certificate string) (ClientRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	clients, ok := s.clients[hsmID]
	if !ok {
		return ClientRecord{}, fmt.Errorf("cloudhsm %q not found", hsmID)
	}
	now := time.Now()
	id := s.generateID()
	c := &ClientRecord{
		ID:           id,
		CloudHSMID:   hsmID,
		Name:         name,
		Availability: "available",
		Certificate:  certificate,
		CreatedAt:    now,
		ModifiedAt:   now,
	}
	clients[id] = c
	s.logger.Debug("cloudhsm client created", "hsm_id", hsmID, "id", id, "name", name)
	return *c, nil
}

// ReadClient returns one client of the CloudHSM.
func (s *MemoryStore) ReadClient(hsmID, id string) (ClientRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clients, ok := s.clients[hsmID]
	if !ok {
		return ClientRecord{}, fmt.Errorf("cloudhsm %q not found", hsmID)
	}
	c, ok := clients[id]
	if !ok {
		return ClientRecord{}, fmt.Errorf("cloudhsm client %q not found", id)
	}
	return *c, nil
}

// UpdateClient applies the given values to an existing client.
func (s *MemoryStore) UpdateClient(hsmID, id, name, certificate string) (ClientRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	clients, ok := s.clients[hsmID]
	if !ok {
		return ClientRecord{}, fmt.Errorf("cloudhsm %q not found", hsmID)
	}
	c, ok := clients[id]
	if !ok {
		return ClientRecord{}, fmt.Errorf("cloudhsm client %q not found", id)
	}
	c.Name = name
	c.Certificate = certificate
	c.ModifiedAt = time.Now()
	s.logger.Debug("cloudhsm client updated", "hsm_id", hsmID, "id", id, "name", name)
	return *c, nil
}

// DeleteClient removes the client from the CloudHSM.
func (s *MemoryStore) DeleteClient(hsmID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	clients, ok := s.clients[hsmID]
	if !ok {
		return fmt.Errorf("cloudhsm %q not found", hsmID)
	}
	if _, ok := clients[id]; !ok {
		return fmt.Errorf("cloudhsm client %q not found", id)
	}
	delete(clients, id)
	s.logger.Debug("cloudhsm client deleted", "hsm_id", hsmID, "id", id)
	return nil
}

// ListPeers returns the IPsec peers registered against the CloudHSM.
func (s *MemoryStore) ListPeers(hsmID string) ([]PeerRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	peers, ok := s.peers[hsmID]
	if !ok {
		return nil, fmt.Errorf("cloudhsm %q not found", hsmID)
	}
	result := make([]PeerRecord, 0, len(peers))
	for _, p := range peers {
		result = append(result, *p)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Index < result[j].Index })
	return result, nil
}

// CreatePeer registers an IPsec peer against the CloudHSM.
func (s *MemoryStore) CreatePeer(hsmID, peerID string) (PeerRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	peers, ok := s.peers[hsmID]
	if !ok {
		return PeerRecord{}, fmt.Errorf("cloudhsm %q not found", hsmID)
	}
	if _, exists := peers[peerID]; exists {
		return PeerRecord{}, fmt.Errorf("cloudhsm peer %q already exists", peerID)
	}
	p := &PeerRecord{
		ID:         peerID,
		CloudHSMID: hsmID,
		Index:      len(peers) + 1,
		Status:     "UP",
		Routes:     []string{},
	}
	peers[peerID] = p
	s.logger.Debug("cloudhsm peer created", "hsm_id", hsmID, "id", peerID)
	return *p, nil
}

// DeletePeer removes the IPsec peer from the CloudHSM.
func (s *MemoryStore) DeletePeer(hsmID, peerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	peers, ok := s.peers[hsmID]
	if !ok {
		return fmt.Errorf("cloudhsm %q not found", hsmID)
	}
	if _, ok := peers[peerID]; !ok {
		return fmt.Errorf("cloudhsm peer %q not found", peerID)
	}
	delete(peers, peerID)
	s.logger.Debug("cloudhsm peer deleted", "hsm_id", hsmID, "id", peerID)
	return nil
}

// ListLicenses returns every software license, oldest first.
func (s *MemoryStore) ListLicenses() []LicenseRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]LicenseRecord, 0, len(s.licenses))
	for _, l := range s.licenses {
		result = append(result, *l)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// CreateLicense stores a new software license and returns it.
func (s *MemoryStore) CreateLicense(name, description string, tags []string) (LicenseRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	id := s.generateID()
	if tags == nil {
		tags = []string{}
	}
	l := &LicenseRecord{
		ID:          id,
		Name:        name,
		Description: description,
		Tags:        tags,
		CreatedAt:   now,
		ModifiedAt:  now,
	}
	s.licenses[id] = l
	s.logger.Debug("cloudhsm license created", "id", id, "name", name)
	return *l, nil
}

// ReadLicense returns the software license with the given ID.
func (s *MemoryStore) ReadLicense(id string) (LicenseRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	l, ok := s.licenses[id]
	if !ok {
		return LicenseRecord{}, fmt.Errorf("cloudhsm license %q not found", id)
	}
	return *l, nil
}

// UpdateLicense applies the given values to an existing license.
func (s *MemoryStore) UpdateLicense(id, name, description string, tags []string) (LicenseRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	l, ok := s.licenses[id]
	if !ok {
		return LicenseRecord{}, fmt.Errorf("cloudhsm license %q not found", id)
	}
	l.Name = name
	l.Description = description
	if tags == nil {
		tags = []string{}
	}
	l.Tags = tags
	l.ModifiedAt = time.Now()
	s.logger.Debug("cloudhsm license updated", "id", id, "name", name)
	return *l, nil
}

// DeleteLicense removes the software license.
func (s *MemoryStore) DeleteLicense(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.licenses[id]; !ok {
		return fmt.Errorf("cloudhsm license %q not found", id)
	}
	delete(s.licenses, id)
	s.logger.Debug("cloudhsm license deleted", "id", id)
	return nil
}

// Close releases the resources held by the store.
func (s *MemoryStore) Close() error {
	return nil
}
