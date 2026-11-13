package apprun

import (
	"encoding/json"
	"fmt"
	"sort"
)

// storeSnapshot is the store's form in the state file (--state-dir).
type storeSnapshot struct {
	UserCreated bool `json:"user_created"`
	AppSeq      int  `json:"app_seq"`
	VersionSeq  int  `json:"version_seq"`
	// Applications is keyed by application ID; each holds the retained
	// versions, oldest first.
	Applications  map[string][]appVersionSnapshot `json:"applications"`
	Traffic       map[string][]TrafficItem        `json:"traffic"`
	PacketFilters map[string]*PacketFilter        `json:"packet_filters"`
}

// appVersionSnapshot is one version of an application together with the
// application as it was at that version.
type appVersionSnapshot struct {
	App        *Application `json:"app"`
	AppSeq     int          `json:"app_seq"`
	Version    *Version     `json:"version"`
	VersionSeq int          `json:"version_seq"`
}

// Snapshot returns the user, every application with its versions, traffic
// and packet filter, and the sequence counters as JSON, for core.StateFile.
func (s *MemoryStore) Snapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := storeSnapshot{
		UserCreated:   s.userCreated,
		AppSeq:        s.appSeq,
		VersionSeq:    s.versionSeq,
		Applications:  make(map[string][]appVersionSnapshot, len(s.appVersions)),
		Traffic:       s.traffic,
		PacketFilters: s.packetFilters,
	}
	for id, entries := range s.appVersions {
		out := make([]appVersionSnapshot, 0, len(entries))
		for _, e := range entries {
			out = append(out, appVersionSnapshot{
				App:        e.app,
				AppSeq:     e.app.seq,
				Version:    e.version,
				VersionSeq: e.version.seq,
			})
		}
		snap.Applications[id] = out
	}
	return json.MarshalIndent(snap, "", "  ")
}

// Restore replaces the store's state with a snapshot, and advances the ID
// generator past the restored resource IDs so new applications never reuse
// one. Every application comes back "Healthy" with the public URL of the
// current data plane address: its container did not survive the restart, so
// the status is the server's to set again once it has restarted the container.
func (s *MemoryStore) Restore(data []byte) error {
	var snap storeSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	appVersions := make(map[string][]*appVersionEntry, len(snap.Applications))
	for id, entries := range snap.Applications {
		if len(entries) == 0 {
			return fmt.Errorf("application %q: no version", id)
		}
		out := make([]*appVersionEntry, 0, len(entries))
		for i, e := range entries {
			if e.App == nil || e.Version == nil || e.App.ID != id || e.Version.AppID != id {
				return fmt.Errorf("application %q: version entry %d is incomplete or belongs to another application", id, i)
			}
			e.App.seq = e.AppSeq
			e.App.Status = "Healthy"
			e.App.PublicURL = s.publicURLFunc(id)
			e.Version.seq = e.VersionSeq
			out = append(out, &appVersionEntry{app: e.App, version: e.Version})
		}
		appVersions[id] = out
	}
	if snap.Traffic == nil {
		snap.Traffic = make(map[string][]TrafficItem)
	}
	for id := range snap.Traffic {
		if _, ok := appVersions[id]; !ok {
			return fmt.Errorf("traffic of unknown application %q", id)
		}
	}
	if snap.PacketFilters == nil {
		snap.PacketFilters = make(map[string]*PacketFilter)
	}
	for id, pf := range snap.PacketFilters {
		if _, ok := appVersions[id]; !ok || pf == nil {
			return fmt.Errorf("packet filter %q: record is missing or its application is unknown", id)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.userCreated = snap.UserCreated
	s.appSeq = snap.AppSeq
	s.versionSeq = snap.VersionSeq
	s.appVersions = appVersions
	s.traffic = snap.Traffic
	s.packetFilters = snap.PacketFilters
	for _, entries := range appVersions {
		s.ids.Observe(entries[len(entries)-1].app.ResourceID)
	}
	return nil
}

// applicationsByCreation returns every application in creation order.
func (s *MemoryStore) applicationsByCreation() []*Application {
	s.mu.RLock()
	defer s.mu.RUnlock()
	apps := make([]*Application, 0, len(s.appVersions))
	for id := range s.appVersions {
		if app := s.latestApp(id); app != nil {
			apps = append(apps, app)
		}
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].seq < apps[j].seq })
	return apps
}
