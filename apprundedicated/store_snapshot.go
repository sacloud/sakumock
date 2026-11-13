package apprundedicated

import (
	"encoding/json"
	"fmt"
)

// storeSnapshot is the store's form in the state file (--state-dir). The
// order slices keep the creation order the list endpoints report.
type storeSnapshot struct {
	Clusters     map[string]*Cluster `json:"clusters"`
	ClusterOrder []string            `json:"clusterOrder"`

	Applications map[string]*Application `json:"applications"`
	AppOrder     []string                `json:"appOrder"`

	// Versions and VersionSeqs are keyed by application ID; VersionSeqs is the
	// last version number handed out, which deleted versions never give back.
	Versions    map[string][]*ApplicationVersion `json:"versions"`
	VersionSeqs map[string]int32                 `json:"versionSeqs"`

	AutoScalingGroups map[string]*AutoScalingGroup `json:"autoScalingGroups"`
	ASGOrder          []string                     `json:"asgOrder"`

	LoadBalancers map[string]*LoadBalancer `json:"loadBalancers"`
	LBOrder       []string                 `json:"lbOrder"`

	// LoadBalancerNodes is keyed by load balancer ID.
	LoadBalancerNodes map[string][]*LoadBalancerNode `json:"loadBalancerNodes"`

	WorkerNodes     map[string]*WorkerNode `json:"workerNodes"`
	WorkerNodeOrder []string               `json:"workerNodeOrder"`

	Certificates map[string]*Certificate `json:"certificates"`
	CertOrder    []string                `json:"certOrder"`
}

// Snapshot returns every record of the store, with the list orders and the
// version counters, as JSON, for core.StateFile.
func (s *MemoryStore) Snapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.MarshalIndent(storeSnapshot{
		Clusters:          s.clusters,
		ClusterOrder:      s.clusterOrder,
		Applications:      s.applications,
		AppOrder:          s.appOrder,
		Versions:          s.versions,
		VersionSeqs:       s.versionSeqs,
		AutoScalingGroups: s.autoScalingGroups,
		ASGOrder:          s.asgOrder,
		LoadBalancers:     s.loadBalancers,
		LBOrder:           s.lbOrder,
		LoadBalancerNodes: s.loadBalancerNodes,
		WorkerNodes:       s.workerNodes,
		WorkerNodeOrder:   s.workerNodeOrder,
		Certificates:      s.certificates,
		CertOrder:         s.certOrder,
	}, "", "  ")
}

// Restore replaces the store's state with a snapshot. IDs are UUIDs, so there
// is no ID generator to advance; the per-application version counters come
// back with the snapshot. Containers are not part of it: the server restarts
// them for the applications restored with an active version.
func (s *MemoryStore) Restore(data []byte) error {
	var snap storeSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	if err := snap.validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.clusters = orEmpty(snap.Clusters)
	s.clusterOrder = snap.ClusterOrder
	s.applications = orEmpty(snap.Applications)
	s.appOrder = snap.AppOrder
	s.versions = orEmpty(snap.Versions)
	s.versionSeqs = orEmpty(snap.VersionSeqs)
	s.autoScalingGroups = orEmpty(snap.AutoScalingGroups)
	s.asgOrder = snap.ASGOrder
	s.loadBalancers = orEmpty(snap.LoadBalancers)
	s.lbOrder = snap.LBOrder
	s.loadBalancerNodes = orEmpty(snap.LoadBalancerNodes)
	s.workerNodes = orEmpty(snap.WorkerNodes)
	s.workerNodeOrder = snap.WorkerNodeOrder
	s.certificates = orEmpty(snap.Certificates)
	s.certOrder = snap.CertOrder
	return nil
}

// validate rejects a snapshot the store could not serve: an order slice and
// its records that do not match (a listed ID without a record panics at list
// time, an unlisted record is invisible), or versions and nodes without the
// record they hang on.
func (snap *storeSnapshot) validate() error {
	if err := checkOrder("cluster", snap.ClusterOrder, snap.Clusters, func(c *Cluster) string { return c.ClusterID }); err != nil {
		return err
	}
	if err := checkOrder("application", snap.AppOrder, snap.Applications, func(a *Application) string { return a.ApplicationID }); err != nil {
		return err
	}
	if err := checkOrder("auto scaling group", snap.ASGOrder, snap.AutoScalingGroups, func(a *AutoScalingGroup) string { return a.AutoScalingGroupID }); err != nil {
		return err
	}
	if err := checkOrder("load balancer", snap.LBOrder, snap.LoadBalancers, func(lb *LoadBalancer) string { return lb.LoadBalancerID }); err != nil {
		return err
	}
	if err := checkOrder("worker node", snap.WorkerNodeOrder, snap.WorkerNodes, func(wn *WorkerNode) string { return wn.WorkerNodeID }); err != nil {
		return err
	}
	if err := checkOrder("certificate", snap.CertOrder, snap.Certificates, func(c *Certificate) string { return c.CertificateID }); err != nil {
		return err
	}

	for appID, versions := range snap.Versions {
		if _, ok := snap.Applications[appID]; !ok {
			return fmt.Errorf("versions of unknown application %q", appID)
		}
		for _, v := range versions {
			if v == nil || v.ApplicationID != appID {
				return fmt.Errorf("application %q: a version record is missing or belongs to another application", appID)
			}
			if v.Version > snap.VersionSeqs[appID] {
				return fmt.Errorf("application %q: version %d is beyond the version counter %d", appID, v.Version, snap.VersionSeqs[appID])
			}
		}
	}
	for appID := range snap.VersionSeqs {
		if _, ok := snap.Applications[appID]; !ok {
			return fmt.Errorf("version counter of unknown application %q", appID)
		}
	}
	for lbID, nodes := range snap.LoadBalancerNodes {
		if _, ok := snap.LoadBalancers[lbID]; !ok {
			return fmt.Errorf("nodes of unknown load balancer %q", lbID)
		}
		for _, n := range nodes {
			if n == nil || n.LoadBalancerID != lbID {
				return fmt.Errorf("load balancer %q: a node record is missing or belongs to another load balancer", lbID)
			}
		}
	}
	return nil
}

// checkOrder verifies that order lists exactly the keys of records, once
// each, and that every record carries the ID it is stored under.
func checkOrder[T any](kind string, order []string, records map[string]*T, idOf func(*T) string) error {
	seen := make(map[string]bool, len(order))
	for _, id := range order {
		rec, ok := records[id]
		if !ok || rec == nil || idOf(rec) != id {
			return fmt.Errorf("%s %q: record is missing or has a different id", kind, id)
		}
		if seen[id] {
			return fmt.Errorf("%s %q: listed twice", kind, id)
		}
		seen[id] = true
	}
	for id := range records {
		if !seen[id] {
			return fmt.Errorf("%s %q: record is not in the list order", kind, id)
		}
	}
	return nil
}

// orEmpty returns m, or an empty map when the snapshot had none, so the store
// never writes to a nil map.
func orEmpty[K comparable, V any](m map[K]V) map[K]V {
	if m == nil {
		return make(map[K]V)
	}
	return m
}

// activeApplications returns the applications with an active version, in
// creation order.
func (s *MemoryStore) activeApplications() []*Application {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var apps []*Application
	for _, id := range s.appOrder {
		if app, ok := s.applications[id]; ok && app.ActiveVersion != nil {
			apps = append(apps, app)
		}
	}
	return apps
}
