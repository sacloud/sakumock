package apprun

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sacloud/sakumock/core"
)

type MemoryStore struct {
	mu          sync.RWMutex
	userCreated bool
	ids         *core.IDGenerator
	versionSeq  int

	appVersions   map[string][]*appVersionEntry
	traffic       map[string][]TrafficItem
	packetFilters map[string]*PacketFilter

	publicURLFunc func(appID string) string
}

type appVersionEntry struct {
	app     *Application
	version *Version
}

func NewStore(publicURLFunc func(string) string) *MemoryStore {
	return &MemoryStore{
		ids:           core.NewIDGenerator(core.DefaultIDBase),
		appVersions:   make(map[string][]*appVersionEntry),
		traffic:       make(map[string][]TrafficItem),
		packetFilters: make(map[string]*PacketFilter),
		publicURLFunc: publicURLFunc,
	}
}

func (s *MemoryStore) UserCreated() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.userCreated
}

func (s *MemoryStore) CreateUser() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.userCreated = true
}

func (s *MemoryStore) ListApplications(params ListParams) ([]*Application, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var apps []*Application
	for id := range s.appVersions {
		if app := s.latestApp(id); app != nil {
			apps = append(apps, app)
		}
	}

	sortField := params.SortField
	if sortField == "" {
		sortField = "created_at"
	}
	sortOrder := params.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}

	sort.Slice(apps, func(i, j int) bool {
		switch sortField {
		case "created_at":
			if sortOrder == "desc" {
				return apps[i].CreatedAt.After(apps[j].CreatedAt)
			}
			return apps[i].CreatedAt.Before(apps[j].CreatedAt)
		default:
			return false
		}
	})

	total := len(apps)
	pageNum := max(params.PageNum, 1)
	pageSize := params.PageSize
	if pageSize < 1 {
		pageSize = 50
	}

	start := (pageNum - 1) * pageSize
	if start > total {
		return nil, total
	}
	end := min(start+pageSize, total)
	return apps[start:end], total
}

func (s *MemoryStore) CreateApplication(app *Application) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	app.ID = uuid.NewString()
	app.ResourceID = s.ids.Next()
	app.Status = "Healthy"
	app.CreatedAt = time.Now().UTC().Truncate(time.Second)
	app.PublicURL = s.publicURLFunc(app.ID)

	if app.ScaleTargetConcurrency == 0 {
		app.ScaleTargetConcurrency = 100
	}

	version := s.createVersionLocked(app)
	s.appVersions[app.ID] = []*appVersionEntry{{app: app, version: version}}
	s.traffic[app.ID] = []TrafficItem{{
		VersionName:     version.Name,
		IsLatestVersion: true,
		Percent:         100,
	}}

	return nil
}

func (s *MemoryStore) ReadApplication(id string) (*Application, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	app := s.latestApp(id)
	return app, app != nil
}

func (s *MemoryStore) UpdateApplication(id string, patch *Application) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	current := s.latestApp(id)
	if current == nil {
		return fmt.Errorf("application not found")
	}

	updated := *current
	if patch.TimeoutSeconds != 0 {
		updated.TimeoutSeconds = patch.TimeoutSeconds
	}
	if patch.Port != 0 {
		updated.Port = patch.Port
	}
	if patch.MinScale >= 0 {
		updated.MinScale = patch.MinScale
	}
	if patch.MaxScale != 0 {
		updated.MaxScale = patch.MaxScale
	}
	if patch.ScaleTargetConcurrency != 0 {
		updated.ScaleTargetConcurrency = patch.ScaleTargetConcurrency
	}
	if len(patch.Components) > 0 {
		updated.Components = patch.Components
	}
	updated.UpdatedAt = time.Now().UTC().Truncate(time.Second)

	version := s.createVersionLocked(&updated)
	s.appVersions[id] = append(s.appVersions[id], &appVersionEntry{app: &updated, version: version})

	if len(s.appVersions[id]) > maxVersionsPerApp {
		s.appVersions[id] = s.appVersions[id][len(s.appVersions[id])-maxVersionsPerApp:]
	}

	return nil
}

func (s *MemoryStore) DeleteApplication(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.appVersions[id]; !ok {
		return fmt.Errorf("application not found")
	}
	delete(s.appVersions, id)
	delete(s.traffic, id)
	delete(s.packetFilters, id)
	return nil
}

func (s *MemoryStore) ListVersions(appID string, params ListParams) ([]*Version, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, ok := s.appVersions[appID]
	if !ok {
		return nil, 0
	}

	var versions []*Version
	for _, e := range entries {
		versions = append(versions, e.version)
	}

	sortOrder := params.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}
	sort.Slice(versions, func(i, j int) bool {
		if sortOrder == "desc" {
			return versions[i].CreatedAt.After(versions[j].CreatedAt)
		}
		return versions[i].CreatedAt.Before(versions[j].CreatedAt)
	})

	total := len(versions)
	pageNum := max(params.PageNum, 1)
	pageSize := params.PageSize
	if pageSize < 1 {
		pageSize = 50
	}

	start := (pageNum - 1) * pageSize
	if start > total {
		return nil, total
	}
	end := min(start+pageSize, total)
	return versions[start:end], total
}

func (s *MemoryStore) ReadVersion(appID, versionID string) (*Version, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, ok := s.appVersions[appID]
	if !ok {
		return nil, false
	}
	for _, e := range entries {
		if e.version.ID == versionID {
			return e.version, true
		}
	}
	return nil, false
}

func (s *MemoryStore) DeleteVersion(appID, versionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, ok := s.appVersions[appID]
	if !ok {
		return fmt.Errorf("application not found")
	}
	for i, e := range entries {
		if e.version.ID == versionID {
			s.appVersions[appID] = append(entries[:i], entries[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("version not found")
}

func (s *MemoryStore) GetTraffic(appID string) ([]TrafficItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items, ok := s.traffic[appID]
	return items, ok
}

func (s *MemoryStore) PutTraffic(appID string, items []TrafficItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.appVersions[appID]; !ok {
		return fmt.Errorf("application not found")
	}
	s.traffic[appID] = items
	return nil
}

func (s *MemoryStore) GetPacketFilter(appID string) (*PacketFilter, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pf, ok := s.packetFilters[appID]
	if !ok {
		if _, exists := s.appVersions[appID]; !exists {
			return nil, false
		}
		return &PacketFilter{IsEnabled: false, Settings: []PacketFilterSetting{}}, true
	}
	return pf, true
}

func (s *MemoryStore) PatchPacketFilter(appID string, pf *PacketFilter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.appVersions[appID]; !ok {
		return fmt.Errorf("application not found")
	}
	s.packetFilters[appID] = pf
	return nil
}

func (s *MemoryStore) Close() {}

func (s *MemoryStore) latestApp(id string) *Application {
	entries, ok := s.appVersions[id]
	if !ok || len(entries) == 0 {
		return nil
	}
	return entries[len(entries)-1].app
}

func (s *MemoryStore) createVersionLocked(app *Application) *Version {
	s.versionSeq++
	now := time.Now().UTC().Truncate(time.Second)
	return &Version{
		ID:                     uuid.NewString(),
		AppID:                  app.ID,
		Name:                   fmt.Sprintf("%s-%s-%d", app.Name, app.ID, s.versionSeq),
		Status:                 "Healthy",
		TimeoutSeconds:         app.TimeoutSeconds,
		Port:                   app.Port,
		MinScale:               app.MinScale,
		MaxScale:               app.MaxScale,
		ScaleTargetConcurrency: app.ScaleTargetConcurrency,
		Components:             app.Components,
		CreatedAt:              now,
	}
}
