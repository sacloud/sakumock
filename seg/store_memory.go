package seg

import (
	"crypto/md5" //nolint:gosec // used only to derive a plausible-looking opaque settings hash, not for security
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/sacloud/sakumock/core"
)

// settingsHash returns a stable opaque hash for the given settings, matching
// the shape (32 hex characters) of the spec's SettingsHash examples.
func settingsHash(settings SettingsRecord) string {
	b, _ := json.Marshal(settings)
	sum := md5.Sum(b) //nolint:gosec
	return hex.EncodeToString(sum[:])
}

// defaultZoneID is the zone ID reported on every appliance's Remark.Zone and
// Switch.Zone. The mock does not care which zone a client actually connects
// through, so a fixed value (31001 = is1a) is used for every appliance.
const defaultZoneID = "31001"

// MemoryStore is the in-memory Store implementation.
type MemoryStore struct {
	mu         sync.RWMutex
	appliances map[string]*ApplianceRecord
	ids        *core.IDGenerator
	sharedIPN  int // counter used to assign distinct synthetic shared-switch IPs
	logger     *slog.Logger
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore(logger *slog.Logger) *MemoryStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &MemoryStore{
		appliances: make(map[string]*ApplianceRecord),
		ids:        core.NewIDGenerator(core.DefaultIDBase()),
		logger:     logger,
	}
}

func (s *MemoryStore) generateID() string {
	return s.ids.Next()
}

// List returns every appliance, oldest first.
func (s *MemoryStore) List() []ApplianceRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ApplianceRecord, 0, len(s.appliances))
	for _, a := range s.appliances {
		result = append(result, *a)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// Create stores a new appliance, auto-powered-on, and returns it.
func (s *MemoryStore) Create(name, description string, tags []string, sw SwitchRemark, network NetworkRemark, servers []ServerRemark) (ApplianceRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	id := s.generateID()
	if tags == nil {
		tags = []string{}
	}
	if name == "" {
		name = fmt.Sprintf("Service Endpoint Gateway (%s)", id)
	}

	s.sharedIPN++
	sharedSwitchID := s.generateID()
	userIP := ""
	if len(servers) > 0 {
		userIP = servers[0].IPAddress
	}

	a := &ApplianceRecord{
		ID:              id,
		Name:            name,
		Description:     description,
		Tags:            tags,
		Switch:          sw,
		Network:         network,
		Servers:         servers,
		ZoneID:          defaultZoneID,
		Availability:    "available",
		PowerStatus:     "up",
		StatusChangedAt: now,
		Interfaces: []InterfaceRecord{
			{
				SwitchID:       sharedSwitchID,
				SwitchName:     "Switch",
				Scope:          "shared",
				IPAddress:      fmt.Sprintf("203.0.113.%d", 100+s.sharedIPN%150),
				HasSubnet:      true,
				NetworkAddress: "203.0.113.0",
				NetworkMaskLen: 24,
				DefaultRoute:   "203.0.113.1",
			},
			{
				SwitchID:       sw.ID,
				SwitchName:     "SwitchForSEG",
				Scope:          "user",
				UserIPAddress:  userIP,
				NetworkMaskLen: network.NetworkMaskLen,
			},
		},
		CreatedAt: now,
	}
	s.appliances[id] = a
	s.logger.Debug("seg appliance created", "id", id, "name", name)
	return *a, nil
}

// Read returns the appliance with the given ID.
func (s *MemoryStore) Read(id string) (ApplianceRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	a, ok := s.appliances[id]
	if !ok {
		return ApplianceRecord{}, fmt.Errorf("appliance %q not found", id)
	}
	return *a, nil
}

// Update stores the given settings on the appliance and recomputes its
// SettingsHash. The real API requires a separate "apply" call to reflect the
// settings; the mock keeps a single applied state and treats Apply as a
// no-op for simplicity (see seg/README.md).
func (s *MemoryStore) Update(id string, settings SettingsRecord) (ApplianceRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, ok := s.appliances[id]
	if !ok {
		return ApplianceRecord{}, fmt.Errorf("appliance %q not found", id)
	}
	a.Settings = &settings
	a.SettingsHash = settingsHash(settings)
	return *a, nil
}

// Delete removes the appliance.
func (s *MemoryStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.appliances[id]; !ok {
		return fmt.Errorf("appliance %q not found", id)
	}
	delete(s.appliances, id)
	s.logger.Debug("seg appliance deleted", "id", id)
	return nil
}

// Apply is a no-op returning the current state: the mock applies Update
// immediately rather than modeling a pending/applied config split.
func (s *MemoryStore) Apply(id string) (ApplianceRecord, error) {
	return s.Read(id)
}

// ReadInterface returns the interface on the appliance whose Switch ID
// matches interfaceID.
func (s *MemoryStore) ReadInterface(id, interfaceID string) (InterfaceRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	a, ok := s.appliances[id]
	if !ok {
		return InterfaceRecord{}, fmt.Errorf("appliance %q not found", id)
	}
	for _, iface := range a.Interfaces {
		if iface.SwitchID == interfaceID {
			return iface, nil
		}
	}
	return InterfaceRecord{}, fmt.Errorf("interface %q not found", interfaceID)
}

// PowerOn marks the appliance as up.
func (s *MemoryStore) PowerOn(id string) (ApplianceRecord, error) {
	return s.setPowerStatus(id, "up")
}

// PowerOff marks the appliance as down.
func (s *MemoryStore) PowerOff(id string) (ApplianceRecord, error) {
	return s.setPowerStatus(id, "down")
}

// Reset marks the appliance as up, simulating a completed reboot.
func (s *MemoryStore) Reset(id string) (ApplianceRecord, error) {
	return s.setPowerStatus(id, "up")
}

func (s *MemoryStore) setPowerStatus(id, status string) (ApplianceRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, ok := s.appliances[id]
	if !ok {
		return ApplianceRecord{}, fmt.Errorf("appliance %q not found", id)
	}
	a.PowerStatus = status
	a.StatusChangedAt = time.Now()
	s.logger.Debug("seg appliance power status changed", "id", id, "status", status)
	return *a, nil
}

// Close releases the resources held by the store.
func (s *MemoryStore) Close() error {
	return nil
}
