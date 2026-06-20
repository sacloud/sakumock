package apprundedicated

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

type MemoryStore struct {
	mu sync.RWMutex

	clusters     map[string]*Cluster
	clusterOrder []string

	applications map[string]*Application
	appOrder     []string

	versions    map[string][]*ApplicationVersion // applicationID -> sorted versions
	versionSeqs map[string]int32                 // applicationID -> next version number

	autoScalingGroups map[string]*AutoScalingGroup
	asgOrder          []string

	loadBalancers map[string]*LoadBalancer
	lbOrder       []string

	loadBalancerNodes map[string][]*LoadBalancerNode // lbID -> nodes

	workerNodes     map[string]*WorkerNode
	workerNodeOrder []string

	certificates map[string]*Certificate
	certOrder    []string
}

func NewStore() *MemoryStore {
	return &MemoryStore{
		clusters:          make(map[string]*Cluster),
		applications:      make(map[string]*Application),
		versions:          make(map[string][]*ApplicationVersion),
		versionSeqs:       make(map[string]int32),
		autoScalingGroups: make(map[string]*AutoScalingGroup),
		loadBalancers:     make(map[string]*LoadBalancer),
		loadBalancerNodes: make(map[string][]*LoadBalancerNode),
		workerNodes:       make(map[string]*WorkerNode),
		certificates:      make(map[string]*Certificate),
	}
}

func nowUnix() int64 { return time.Now().Unix() }

func cursorPage[T any](items []T, idFunc func(T) string, cursor string, maxItems int) ([]T, *string) {
	if maxItems <= 0 {
		maxItems = 20
	}
	start := 0
	if cursor != "" {
		for i, item := range items {
			if idFunc(item) == cursor {
				start = i + 1
				break
			}
		}
	}
	if start >= len(items) {
		return nil, nil
	}
	end := start + maxItems
	if end >= len(items) {
		return items[start:], nil
	}
	next := idFunc(items[end-1])
	return items[start:end], &next
}

// Clusters

func (s *MemoryStore) CreateCluster(c *Cluster) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c.ClusterID = uuid.NewString()
	c.Created = nowUnix()
	s.clusters[c.ClusterID] = c
	s.clusterOrder = append(s.clusterOrder, c.ClusterID)
	return nil
}

func (s *MemoryStore) ListClusters(cursor string, maxItems int) ([]*Cluster, *string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var items []*Cluster
	for _, id := range s.clusterOrder {
		if c, ok := s.clusters[id]; ok {
			items = append(items, c)
		}
	}
	return cursorPage(items, func(c *Cluster) string { return c.ClusterID }, cursor, maxItems)
}

func (s *MemoryStore) ReadCluster(id string) (*Cluster, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.clusters[id]
	return c, ok
}

func (s *MemoryStore) UpdateCluster(id string, servicePrincipalID string, letsEncryptEmail *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.clusters[id]
	if !ok {
		return fmt.Errorf("cluster not found")
	}
	c.ServicePrincipalID = servicePrincipalID
	c.LetsEncryptEmail = letsEncryptEmail
	return nil
}

func (s *MemoryStore) DeleteCluster(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.clusters[id]; !ok {
		return fmt.Errorf("cluster not found")
	}
	delete(s.clusters, id)
	s.clusterOrder = removeFromOrder(s.clusterOrder, id)
	return nil
}

// Applications

func (s *MemoryStore) CountApplications() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.applications)
}

func (s *MemoryStore) CreateApplication(app *Application) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	app.ApplicationID = uuid.NewString()
	s.applications[app.ApplicationID] = app
	s.appOrder = append(s.appOrder, app.ApplicationID)
	s.versionSeqs[app.ApplicationID] = 0
	return nil
}

func (s *MemoryStore) ListApplications(clusterID *string, cursor string, maxItems int) ([]*Application, *string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var items []*Application
	for _, id := range s.appOrder {
		app, ok := s.applications[id]
		if !ok {
			continue
		}
		if clusterID != nil && app.ClusterID != *clusterID {
			continue
		}
		items = append(items, app)
	}
	return cursorPage(items, func(a *Application) string { return a.ApplicationID }, cursor, maxItems)
}

func (s *MemoryStore) ReadApplication(id string) (*Application, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	app, ok := s.applications[id]
	return app, ok
}

func (s *MemoryStore) UpdateApplication(id string, activeVersion *int32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.applications[id]
	if !ok {
		return fmt.Errorf("application not found")
	}
	app.ActiveVersion = activeVersion
	if activeVersion == nil {
		app.DesiredCount = nil
	} else {
		for _, v := range s.versions[id] {
			if v.Version == *activeVersion {
				switch v.ScalingMode {
				case "manual":
					app.DesiredCount = v.FixedScale
				case "cpu":
					app.DesiredCount = v.MinScale
				}
				break
			}
		}
	}
	return nil
}

func (s *MemoryStore) DeleteApplication(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.applications[id]; !ok {
		return fmt.Errorf("application not found")
	}
	delete(s.applications, id)
	delete(s.versions, id)
	delete(s.versionSeqs, id)
	s.appOrder = removeFromOrder(s.appOrder, id)
	return nil
}

// Application Versions

func (s *MemoryStore) CreateVersion(appID string, v *ApplicationVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.applications[appID]; !ok {
		return fmt.Errorf("application not found")
	}
	s.versionSeqs[appID]++
	v.Version = s.versionSeqs[appID]
	v.ApplicationID = appID
	v.Created = nowUnix()
	s.versions[appID] = append(s.versions[appID], v)
	return nil
}

func (s *MemoryStore) ListVersions(appID string, cursor string, maxItems int) ([]*ApplicationVersion, *int32) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions := s.versions[appID]
	if len(versions) == 0 {
		return nil, nil
	}

	sorted := make([]*ApplicationVersion, len(versions))
	copy(sorted, versions)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Version > sorted[j].Version
	})

	if maxItems <= 0 {
		maxItems = 20
	}
	start := 0
	if cursor != "" {
		for i, v := range sorted {
			if fmt.Sprintf("%d", v.Version) == cursor {
				start = i + 1
				break
			}
		}
	}
	if start >= len(sorted) {
		return nil, nil
	}
	end := start + maxItems
	if end >= len(sorted) {
		return sorted[start:], nil
	}
	next := sorted[end-1].Version
	return sorted[start:end], &next
}

func (s *MemoryStore) ReadVersion(appID string, version int32) (*ApplicationVersion, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, v := range s.versions[appID] {
		if v.Version == version {
			return v, true
		}
	}
	return nil, false
}

func (s *MemoryStore) DeleteVersion(appID string, version int32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	versions := s.versions[appID]
	for i, v := range versions {
		if v.Version == version {
			s.versions[appID] = append(versions[:i], versions[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("version not found")
}

// Auto Scaling Groups

func (s *MemoryStore) CreateAutoScalingGroup(asg *AutoScalingGroup) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.clusters[asg.ClusterID]; !ok {
		return fmt.Errorf("cluster not found")
	}
	asg.AutoScalingGroupID = uuid.NewString()
	asg.WorkerNodeCount = asg.MinNodes
	s.autoScalingGroups[asg.AutoScalingGroupID] = asg
	s.asgOrder = append(s.asgOrder, asg.AutoScalingGroupID)

	for i := range asg.MinNodes {
		wn := &WorkerNode{
			WorkerNodeID:       uuid.NewString(),
			ClusterID:          asg.ClusterID,
			AutoScalingGroupID: asg.AutoScalingGroupID,
			Status:             "healthy",
			Healthy:            true,
			Created:            nowUnix(),
			NetworkInterfaces:  s.generateWorkerNICs(asg.Interfaces, i),
		}
		s.workerNodes[wn.WorkerNodeID] = wn
		s.workerNodeOrder = append(s.workerNodeOrder, wn.WorkerNodeID)
	}
	return nil
}

func (s *MemoryStore) generateWorkerNICs(ifaces []ASGNodeInterface, nodeIdx int32) []WorkerNodeNIC {
	var nics []WorkerNodeNIC
	for _, iface := range ifaces {
		nic := WorkerNodeNIC{InterfaceIndex: iface.InterfaceIndex}
		if len(iface.IpPool) > 0 && int(nodeIdx) < len(iface.IpPool) {
			nic.Addresses = []string{iface.IpPool[nodeIdx].Start}
		}
		nics = append(nics, nic)
	}
	return nics
}

func (s *MemoryStore) ListAutoScalingGroups(clusterID string, cursor string, maxItems int) ([]*AutoScalingGroup, *string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var items []*AutoScalingGroup
	for _, id := range s.asgOrder {
		asg, ok := s.autoScalingGroups[id]
		if !ok || asg.ClusterID != clusterID {
			continue
		}
		items = append(items, asg)
	}
	return cursorPage(items, func(a *AutoScalingGroup) string { return a.AutoScalingGroupID }, cursor, maxItems)
}

func (s *MemoryStore) ReadAutoScalingGroup(clusterID, asgID string) (*AutoScalingGroup, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	asg, ok := s.autoScalingGroups[asgID]
	if !ok || asg.ClusterID != clusterID {
		return nil, false
	}
	return asg, true
}

func (s *MemoryStore) DeleteAutoScalingGroup(clusterID, asgID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	asg, ok := s.autoScalingGroups[asgID]
	if !ok || asg.ClusterID != clusterID {
		return fmt.Errorf("auto scaling group not found")
	}

	// Remove worker nodes belonging to this ASG
	var remaining []string
	for _, wnID := range s.workerNodeOrder {
		wn := s.workerNodes[wnID]
		if wn.AutoScalingGroupID == asgID {
			delete(s.workerNodes, wnID)
		} else {
			remaining = append(remaining, wnID)
		}
	}
	s.workerNodeOrder = remaining

	// Remove load balancers belonging to this ASG
	var remainingLBs []string
	for _, lbID := range s.lbOrder {
		lb := s.loadBalancers[lbID]
		if lb.AutoScalingGroupID == asgID {
			delete(s.loadBalancerNodes, lbID)
			delete(s.loadBalancers, lbID)
		} else {
			remainingLBs = append(remainingLBs, lbID)
		}
	}
	s.lbOrder = remainingLBs

	delete(s.autoScalingGroups, asgID)
	s.asgOrder = removeFromOrder(s.asgOrder, asgID)
	return nil
}

// Load Balancers

func (s *MemoryStore) CreateLoadBalancer(lb *LoadBalancer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.autoScalingGroups[lb.AutoScalingGroupID]; !ok {
		return fmt.Errorf("auto scaling group not found")
	}
	lb.LoadBalancerID = uuid.NewString()
	lb.Created = nowUnix()
	s.loadBalancers[lb.LoadBalancerID] = lb
	s.lbOrder = append(s.lbOrder, lb.LoadBalancerID)

	node := &LoadBalancerNode{
		LoadBalancerNodeID: uuid.NewString(),
		LoadBalancerID:     lb.LoadBalancerID,
		Status:             "healthy",
		Created:            nowUnix(),
		Interfaces:         s.generateLBNodeInterfaces(lb.Interfaces),
	}
	s.loadBalancerNodes[lb.LoadBalancerID] = []*LoadBalancerNode{node}
	return nil
}

func (s *MemoryStore) generateLBNodeInterfaces(ifaces []LBInterface) []LBNodeInterface {
	var nifs []LBNodeInterface
	for _, iface := range ifaces {
		nif := LBNodeInterface{InterfaceIndex: iface.InterfaceIndex}
		if len(iface.IpPool) > 0 {
			addr := LBNodeAddress{Address: iface.IpPool[0].Start}
			nif.Addresses = append(nif.Addresses, addr)
		}
		if iface.Vip != nil {
			nif.Addresses = append(nif.Addresses, LBNodeAddress{Address: *iface.Vip, Vip: true})
		}
		nifs = append(nifs, nif)
	}
	return nifs
}

func (s *MemoryStore) ListLoadBalancers(clusterID, asgID string, cursor string, maxItems int) ([]*LoadBalancer, *string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var items []*LoadBalancer
	for _, id := range s.lbOrder {
		lb, ok := s.loadBalancers[id]
		if !ok || lb.ClusterID != clusterID || lb.AutoScalingGroupID != asgID {
			continue
		}
		items = append(items, lb)
	}
	return cursorPage(items, func(lb *LoadBalancer) string { return lb.LoadBalancerID }, cursor, maxItems)
}

func (s *MemoryStore) ReadLoadBalancer(clusterID, asgID, lbID string) (*LoadBalancer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lb, ok := s.loadBalancers[lbID]
	if !ok || lb.ClusterID != clusterID || lb.AutoScalingGroupID != asgID {
		return nil, false
	}
	return lb, true
}

func (s *MemoryStore) DeleteLoadBalancer(clusterID, asgID, lbID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lb, ok := s.loadBalancers[lbID]
	if !ok || lb.ClusterID != clusterID || lb.AutoScalingGroupID != asgID {
		return fmt.Errorf("load balancer not found")
	}
	delete(s.loadBalancers, lbID)
	delete(s.loadBalancerNodes, lbID)
	s.lbOrder = removeFromOrder(s.lbOrder, lbID)
	return nil
}

// Load Balancer Nodes

func (s *MemoryStore) ListLoadBalancerNodes(clusterID, asgID, lbID string, cursor string, maxItems int) ([]*LoadBalancerNode, *string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lb, ok := s.loadBalancers[lbID]
	if !ok || lb.ClusterID != clusterID || lb.AutoScalingGroupID != asgID {
		return nil, nil
	}
	nodes := s.loadBalancerNodes[lbID]
	return cursorPage(nodes, func(n *LoadBalancerNode) string { return n.LoadBalancerNodeID }, cursor, maxItems)
}

func (s *MemoryStore) ReadLoadBalancerNode(clusterID, asgID, lbID, nodeID string) (*LoadBalancerNode, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lb, ok := s.loadBalancers[lbID]
	if !ok || lb.ClusterID != clusterID || lb.AutoScalingGroupID != asgID {
		return nil, false
	}
	for _, n := range s.loadBalancerNodes[lbID] {
		if n.LoadBalancerNodeID == nodeID {
			return n, true
		}
	}
	return nil, false
}

// Worker Nodes

func (s *MemoryStore) ListWorkerNodes(clusterID, asgID string, cursor string, maxItems int) ([]*WorkerNode, *string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	asg, ok := s.autoScalingGroups[asgID]
	if !ok || asg.ClusterID != clusterID {
		return nil, nil
	}
	var items []*WorkerNode
	for _, id := range s.workerNodeOrder {
		wn := s.workerNodes[id]
		if wn.AutoScalingGroupID == asgID {
			items = append(items, wn)
		}
	}
	return cursorPage(items, func(wn *WorkerNode) string { return wn.WorkerNodeID }, cursor, maxItems)
}

func (s *MemoryStore) ReadWorkerNode(clusterID, asgID, nodeID string) (*WorkerNode, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	wn, ok := s.workerNodes[nodeID]
	if !ok || wn.ClusterID != clusterID || wn.AutoScalingGroupID != asgID {
		return nil, false
	}
	return wn, true
}

func (s *MemoryStore) UpdateWorkerNodeDraining(clusterID, asgID, nodeID string, draining bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	wn, ok := s.workerNodes[nodeID]
	if !ok || wn.ClusterID != clusterID || wn.AutoScalingGroupID != asgID {
		return fmt.Errorf("worker node not found")
	}
	wn.Draining = draining
	return nil
}

// Certificates

func (s *MemoryStore) CreateCertificate(cert *Certificate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.clusters[cert.ClusterID]; !ok {
		return fmt.Errorf("cluster not found")
	}
	cert.CertificateID = uuid.NewString()
	now := nowUnix()
	cert.Created = now
	cert.Updated = now
	if cert.CommonName == "" {
		cert.CommonName = "mock.example.com"
	}
	if cert.SubjectAlternativeNames == nil {
		cert.SubjectAlternativeNames = []string{cert.CommonName}
	}
	if cert.NotBeforeSec == 0 {
		cert.NotBeforeSec = now
	}
	if cert.NotAfterSec == 0 {
		cert.NotAfterSec = now + 365*24*3600
	}
	s.certificates[cert.CertificateID] = cert
	s.certOrder = append(s.certOrder, cert.CertificateID)
	return nil
}

func (s *MemoryStore) ListCertificates(clusterID string, cursor string, maxItems int) ([]*Certificate, *string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var items []*Certificate
	for _, id := range s.certOrder {
		cert, ok := s.certificates[id]
		if !ok || cert.ClusterID != clusterID {
			continue
		}
		items = append(items, cert)
	}
	return cursorPage(items, func(c *Certificate) string { return c.CertificateID }, cursor, maxItems)
}

func (s *MemoryStore) ReadCertificate(clusterID, certID string) (*Certificate, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cert, ok := s.certificates[certID]
	if !ok || cert.ClusterID != clusterID {
		return nil, false
	}
	return cert, true
}

func (s *MemoryStore) UpdateCertificate(clusterID, certID string, updated *Certificate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cert, ok := s.certificates[certID]
	if !ok || cert.ClusterID != clusterID {
		return fmt.Errorf("certificate not found")
	}
	cert.Name = updated.Name
	cert.CertificatePem = updated.CertificatePem
	cert.PrivateKeyPem = updated.PrivateKeyPem
	cert.IntermediateCertificatePem = updated.IntermediateCertificatePem
	cert.Updated = nowUnix()
	return nil
}

func (s *MemoryStore) DeleteCertificate(clusterID, certID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cert, ok := s.certificates[certID]
	if !ok || cert.ClusterID != clusterID {
		return fmt.Errorf("certificate not found")
	}
	delete(s.certificates, certID)
	s.certOrder = removeFromOrder(s.certOrder, certID)
	return nil
}

func (s *MemoryStore) Close() {}

func removeFromOrder(order []string, id string) []string {
	for i, v := range order {
		if v == id {
			return append(order[:i], order[i+1:]...)
		}
	}
	return order
}
