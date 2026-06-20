package apprundedicated

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/sacloud/sakumock/core"
)

// Error response matching the SDK's Error type.
type apiError struct {
	Status int    `json:"status"`
	Title  string `json:"title"`
}

func writeError(w http.ResponseWriter, status int, title string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(apiError{Status: status, Title: title})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func parsePagination(r *http.Request, defaultMax int) (cursor string, maxItems int) {
	cursor = r.URL.Query().Get("cursor")
	maxItems = defaultMax
	if v := r.URL.Query().Get("maxItems"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxItems = n
		}
	}
	return
}

// --- JSON types matching SDK schemas ---

// Cluster

type createClusterReq struct {
	Name               string            `json:"name"`
	LetsEncryptEmail   *string           `json:"letsEncryptEmail,omitempty"`
	Ports              []clusterPortJSON `json:"ports"`
	ServicePrincipalID string            `json:"servicePrincipalID"`
}

type clusterPortJSON struct {
	Port     uint16 `json:"port"`
	Protocol string `json:"protocol"`
}

type updateClusterReq struct {
	LetsEncryptEmail   *string `json:"letsEncryptEmail,omitempty"`
	ServicePrincipalID string  `json:"servicePrincipalID"`
}

type clusterDetailJSON struct {
	ClusterID           string            `json:"clusterID"`
	Name                string            `json:"name"`
	Ports               []clusterPortJSON `json:"ports"`
	ServicePrincipalID  string            `json:"servicePrincipalID"`
	HasLetsEncryptEmail bool              `json:"hasLetsEncryptEmail"`
	Created             int64             `json:"created"`
}

type clusterSummaryJSON struct {
	ClusterID string `json:"clusterID"`
	Name      string `json:"name"`
	Created   int64  `json:"created"`
}

func toClusterDetail(c *Cluster) clusterDetailJSON {
	ports := make([]clusterPortJSON, len(c.Ports))
	for i, p := range c.Ports {
		ports[i] = clusterPortJSON{Port: p.Port, Protocol: p.Protocol}
	}
	return clusterDetailJSON{
		ClusterID:           c.ClusterID,
		Name:                c.Name,
		Ports:               ports,
		ServicePrincipalID:  c.ServicePrincipalID,
		HasLetsEncryptEmail: c.LetsEncryptEmail != nil,
		Created:             c.Created,
	}
}

func toClusterSummary(c *Cluster) clusterSummaryJSON {
	return clusterSummaryJSON{
		ClusterID: c.ClusterID,
		Name:      c.Name,
		Created:   c.Created,
	}
}

// Application

type createApplicationReq struct {
	Name      string `json:"name"`
	ClusterID string `json:"clusterID"`
}

type updateApplicationReq struct {
	ActiveVersion *int32 `json:"activeVersion"`
}

type applicationDetailJSON struct {
	ApplicationID          string `json:"applicationID"`
	Name                   string `json:"name"`
	ClusterID              string `json:"clusterID"`
	ClusterName            string `json:"clusterName"`
	ActiveVersion          *int32 `json:"activeVersion"`
	DesiredCount           *int32 `json:"desiredCount"`
	ScalingCooldownSeconds int32  `json:"scalingCooldownSeconds"`
}

func (s *Server) toApplicationDetail(app *Application) applicationDetailJSON {
	clusterName := ""
	if c, ok := s.store.ReadCluster(app.ClusterID); ok {
		clusterName = c.Name
	}
	return applicationDetailJSON{
		ApplicationID:          app.ApplicationID,
		Name:                   app.Name,
		ClusterID:              app.ClusterID,
		ClusterName:            clusterName,
		ActiveVersion:          app.ActiveVersion,
		DesiredCount:           app.DesiredCount,
		ScalingCooldownSeconds: app.ScalingCooldownSeconds,
	}
}

// Application Version

type createVersionReq struct {
	CPU                    int64             `json:"cpu"`
	Memory                 int64             `json:"memory"`
	ScalingMode            string            `json:"scalingMode"`
	FixedScale             *int32            `json:"fixedScale,omitempty"`
	MinScale               *int32            `json:"minScale,omitempty"`
	MaxScale               *int32            `json:"maxScale,omitempty"`
	ScaleInThreshold       *int32            `json:"scaleInThreshold,omitempty"`
	ScaleOutThreshold      *int32            `json:"scaleOutThreshold,omitempty"`
	Image                  string            `json:"image"`
	Cmd                    []string          `json:"cmd"`
	RegistryUsername       *string           `json:"registryUsername"`
	RegistryPassword       *string           `json:"registryPassword"`
	RegistryPasswordAction string            `json:"registryPasswordAction"`
	ExposedPorts           []exposedPortJSON `json:"exposedPorts"`
	Env                    []createEnvJSON   `json:"env"`
}

type exposedPortJSON struct {
	TargetPort       uint16           `json:"targetPort"`
	LoadBalancerPort *uint16          `json:"loadBalancerPort"`
	UseLetsEncrypt   bool             `json:"useLetsEncrypt"`
	Host             []string         `json:"host"`
	HealthCheck      *healthCheckJSON `json:"healthCheck"`
}

type healthCheckJSON struct {
	Path            string `json:"path"`
	IntervalSeconds int32  `json:"intervalSeconds"`
	TimeoutSeconds  int32  `json:"timeoutSeconds"`
}

type createEnvJSON struct {
	Key    string  `json:"key"`
	Value  *string `json:"value,omitempty"`
	Secret bool    `json:"secret"`
}

type readEnvJSON struct {
	Key    string  `json:"key"`
	Value  *string `json:"value"`
	Secret bool    `json:"secret"`
}

type versionDetailJSON struct {
	Version           int32             `json:"version"`
	CPU               int64             `json:"cpu"`
	Memory            int64             `json:"memory"`
	ScalingMode       string            `json:"scalingMode"`
	FixedScale        *int32            `json:"fixedScale,omitempty"`
	MinScale          *int32            `json:"minScale,omitempty"`
	MaxScale          *int32            `json:"maxScale,omitempty"`
	ScaleInThreshold  *int32            `json:"scaleInThreshold,omitempty"`
	ScaleOutThreshold *int32            `json:"scaleOutThreshold,omitempty"`
	Image             string            `json:"image"`
	Cmd               []string          `json:"cmd"`
	RegistryUsername  *string           `json:"registryUsername"`
	RegistryPassword  *string           `json:"registryPassword"`
	ActiveNodeCount   int64             `json:"activeNodeCount"`
	Created           int64             `json:"created"`
	ExposedPorts      []exposedPortJSON `json:"exposedPorts"`
	Env               []readEnvJSON     `json:"env"`
}

type versionDeploymentJSON struct {
	Version         int32  `json:"version"`
	Image           string `json:"image"`
	ActiveNodeCount int64  `json:"activeNodeCount"`
	Created         int64  `json:"created"`
}

func toVersionDetail(v *ApplicationVersion) versionDetailJSON {
	ports := make([]exposedPortJSON, len(v.ExposedPorts))
	for i, p := range v.ExposedPorts {
		host := p.Host
		if host == nil {
			host = []string{}
		}
		ports[i] = exposedPortJSON{
			TargetPort:       p.TargetPort,
			LoadBalancerPort: p.LoadBalancerPort,
			UseLetsEncrypt:   p.UseLetsEncrypt,
			Host:             host,
		}
		if p.HealthCheck != nil {
			ports[i].HealthCheck = &healthCheckJSON{
				Path:            p.HealthCheck.Path,
				IntervalSeconds: p.HealthCheck.IntervalSeconds,
				TimeoutSeconds:  p.HealthCheck.TimeoutSeconds,
			}
		}
	}
	env := make([]readEnvJSON, len(v.Env))
	for i, e := range v.Env {
		re := readEnvJSON{Key: e.Key, Secret: e.Secret}
		if !e.Secret {
			re.Value = e.Value
		}
		env[i] = re
	}
	cmd := v.Cmd
	if cmd == nil {
		cmd = []string{}
	}
	return versionDetailJSON{
		Version:           v.Version,
		CPU:               v.CPU,
		Memory:            v.Memory,
		ScalingMode:       v.ScalingMode,
		FixedScale:        v.FixedScale,
		MinScale:          v.MinScale,
		MaxScale:          v.MaxScale,
		ScaleInThreshold:  v.ScaleInThreshold,
		ScaleOutThreshold: v.ScaleOutThreshold,
		Image:             v.Image,
		Cmd:               cmd,
		RegistryUsername:  v.RegistryUsername,
		RegistryPassword:  nil,
		ActiveNodeCount:   v.ActiveNodeCount,
		Created:           v.Created,
		ExposedPorts:      ports,
		Env:               env,
	}
}

func toVersionDeployment(v *ApplicationVersion) versionDeploymentJSON {
	return versionDeploymentJSON{
		Version:         v.Version,
		Image:           v.Image,
		ActiveNodeCount: v.ActiveNodeCount,
		Created:         v.Created,
	}
}

// Auto Scaling Group

type createASGReq struct {
	Name                   string             `json:"name"`
	Zone                   string             `json:"zone"`
	NameServers            []string           `json:"nameServers"`
	WorkerServiceClassPath string             `json:"workerServiceClassPath"`
	MinNodes               int32              `json:"minNodes"`
	MaxNodes               int32              `json:"maxNodes"`
	Interfaces             []asgInterfaceJSON `json:"interfaces"`
}

type asgInterfaceJSON struct {
	InterfaceIndex int16         `json:"interfaceIndex"`
	Upstream       string        `json:"upstream"`
	IpPool         []ipRangeJSON `json:"ipPool"`
	NetmaskLen     *int16        `json:"netmaskLen,omitempty"`
	DefaultGateway *string       `json:"defaultGateway,omitempty"`
	PacketFilterID *string       `json:"packetFilterID,omitempty"`
	ConnectsToLB   bool          `json:"connectsToLB"`
}

type ipRangeJSON struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type asgDetailJSON struct {
	AutoScalingGroupID     string             `json:"autoScalingGroupID"`
	Name                   string             `json:"name"`
	Zone                   string             `json:"zone"`
	NameServers            []string           `json:"nameServers"`
	WorkerServiceClassPath string             `json:"workerServiceClassPath"`
	MinNodes               int32              `json:"minNodes"`
	MaxNodes               int32              `json:"maxNodes"`
	WorkerNodeCount        int32              `json:"workerNodeCount"`
	Deleting               bool               `json:"deleting"`
	Interfaces             []asgInterfaceJSON `json:"interfaces"`
}

func toASGDetail(asg *AutoScalingGroup) asgDetailJSON {
	ifaces := make([]asgInterfaceJSON, len(asg.Interfaces))
	for i, iface := range asg.Interfaces {
		ipPool := make([]ipRangeJSON, len(iface.IpPool))
		for j, r := range iface.IpPool {
			ipPool[j] = ipRangeJSON{Start: r.Start, End: r.End}
		}
		ifaces[i] = asgInterfaceJSON{
			InterfaceIndex: iface.InterfaceIndex,
			Upstream:       iface.Upstream,
			IpPool:         ipPool,
			NetmaskLen:     iface.NetmaskLen,
			DefaultGateway: iface.DefaultGateway,
			PacketFilterID: iface.PacketFilterID,
			ConnectsToLB:   iface.ConnectsToLB,
		}
	}
	return asgDetailJSON{
		AutoScalingGroupID:     asg.AutoScalingGroupID,
		Name:                   asg.Name,
		Zone:                   asg.Zone,
		NameServers:            asg.NameServers,
		WorkerServiceClassPath: asg.WorkerServiceClassPath,
		MinNodes:               asg.MinNodes,
		MaxNodes:               asg.MaxNodes,
		WorkerNodeCount:        asg.WorkerNodeCount,
		Deleting:               asg.Deleting,
		Interfaces:             ifaces,
	}
}

// Load Balancer

type createLBReq struct {
	Name             string            `json:"name"`
	ServiceClassPath string            `json:"serviceClassPath"`
	NameServers      []string          `json:"nameServers"`
	Interfaces       []lbInterfaceJSON `json:"interfaces"`
}

type lbInterfaceJSON struct {
	InterfaceIndex  int16         `json:"interfaceIndex"`
	Upstream        string        `json:"upstream"`
	IpPool          []ipRangeJSON `json:"ipPool"`
	NetmaskLen      *int16        `json:"netmaskLen,omitempty"`
	DefaultGateway  *string       `json:"defaultGateway,omitempty"`
	Vip             *string       `json:"vip,omitempty"`
	VirtualRouterID *int16        `json:"virtualRouterID,omitempty"`
	PacketFilterID  *string       `json:"packetFilterID,omitempty"`
}

type lbDetailJSON struct {
	LoadBalancerID   string            `json:"loadBalancerID"`
	Name             string            `json:"name"`
	ServiceClassPath string            `json:"serviceClassPath"`
	NameServers      []string          `json:"nameServers"`
	Interfaces       []lbInterfaceJSON `json:"interfaces"`
	Created          int64             `json:"created"`
	Deleting         bool              `json:"deleting"`
}

type lbSummaryJSON struct {
	LoadBalancerID   string   `json:"loadBalancerID"`
	Name             string   `json:"name"`
	ServiceClassPath string   `json:"serviceClassPath"`
	NameServers      []string `json:"nameServers"`
	Created          int64    `json:"created"`
	Deleting         bool     `json:"deleting"`
}

func toLBInterfaces(ifaces []LBInterface) []lbInterfaceJSON {
	result := make([]lbInterfaceJSON, len(ifaces))
	for i, iface := range ifaces {
		ipPool := make([]ipRangeJSON, len(iface.IpPool))
		for j, r := range iface.IpPool {
			ipPool[j] = ipRangeJSON{Start: r.Start, End: r.End}
		}
		result[i] = lbInterfaceJSON{
			InterfaceIndex:  iface.InterfaceIndex,
			Upstream:        iface.Upstream,
			IpPool:          ipPool,
			NetmaskLen:      iface.NetmaskLen,
			DefaultGateway:  iface.DefaultGateway,
			Vip:             iface.Vip,
			VirtualRouterID: iface.VirtualRouterID,
			PacketFilterID:  iface.PacketFilterID,
		}
	}
	return result
}

func toLBDetail(lb *LoadBalancer) lbDetailJSON {
	return lbDetailJSON{
		LoadBalancerID:   lb.LoadBalancerID,
		Name:             lb.Name,
		ServiceClassPath: lb.ServiceClassPath,
		NameServers:      lb.NameServers,
		Interfaces:       toLBInterfaces(lb.Interfaces),
		Created:          lb.Created,
		Deleting:         lb.Deleting,
	}
}

func toLBSummary(lb *LoadBalancer) lbSummaryJSON {
	return lbSummaryJSON{
		LoadBalancerID:   lb.LoadBalancerID,
		Name:             lb.Name,
		ServiceClassPath: lb.ServiceClassPath,
		NameServers:      lb.NameServers,
		Created:          lb.Created,
		Deleting:         lb.Deleting,
	}
}

// Load Balancer Node

type lbNodeJSON struct {
	LoadBalancerNodeID string            `json:"loadBalancerNodeID"`
	ResourceID         *string           `json:"resourceID"`
	Status             string            `json:"status"`
	Interfaces         []lbNodeIfaceJSON `json:"interfaces"`
	ArchiveVersion     *string           `json:"archiveVersion,omitempty"`
	CreateErrorMessage *string           `json:"createErrorMessage,omitempty"`
	Created            int64             `json:"created"`
}

type lbNodeIfaceJSON struct {
	InterfaceIndex int16            `json:"interfaceIndex"`
	Addresses      []lbNodeAddrJSON `json:"addresses"`
}

type lbNodeAddrJSON struct {
	Address string `json:"address"`
	Vip     bool   `json:"vip"`
}

func toLBNode(n *LoadBalancerNode) lbNodeJSON {
	ifaces := make([]lbNodeIfaceJSON, len(n.Interfaces))
	for i, iface := range n.Interfaces {
		addrs := make([]lbNodeAddrJSON, len(iface.Addresses))
		for j, addr := range iface.Addresses {
			addrs[j] = lbNodeAddrJSON{Address: addr.Address, Vip: addr.Vip}
		}
		ifaces[i] = lbNodeIfaceJSON{InterfaceIndex: iface.InterfaceIndex, Addresses: addrs}
	}
	return lbNodeJSON{
		LoadBalancerNodeID: n.LoadBalancerNodeID,
		ResourceID:         n.ResourceID,
		Status:             n.Status,
		Interfaces:         ifaces,
		ArchiveVersion:     n.ArchiveVersion,
		CreateErrorMessage: n.CreateErrorMessage,
		Created:            n.Created,
	}
}

// Worker Node

type workerNodeDetailJSON struct {
	WorkerNodeID       string             `json:"workerNodeID"`
	ResourceID         *string            `json:"resourceID"`
	Draining           bool               `json:"draining"`
	Status             string             `json:"status"`
	Healthy            bool               `json:"healthy"`
	Creating           bool               `json:"creating"`
	Created            int64              `json:"created"`
	RunningContainers  []runContainerJSON `json:"runningContainers"`
	NetworkInterfaces  []workerNICJSON    `json:"networkInterfaces"`
	ArchiveVersion     *string            `json:"archiveVersion,omitempty"`
	CreateErrorMessage *string            `json:"createErrorMessage,omitempty"`
}

type workerNodeSummaryJSON struct {
	WorkerNodeID       string          `json:"workerNodeID"`
	ResourceID         *string         `json:"resourceID"`
	Draining           bool            `json:"draining"`
	Status             string          `json:"status"`
	NetworkInterfaces  []workerNICJSON `json:"networkInterfaces"`
	ArchiveVersion     *string         `json:"archiveVersion,omitempty"`
	Created            int64           `json:"created"`
	CreateErrorMessage *string         `json:"createErrorMessage,omitempty"`
}

type runContainerJSON struct {
	ContainerID        string `json:"containerID"`
	Name               string `json:"name"`
	State              string `json:"state"`
	Status             string `json:"status"`
	Image              string `json:"image"`
	StartedAt          int64  `json:"startedAt"`
	ApplicationID      string `json:"applicationID"`
	ApplicationVersion int32  `json:"applicationVersion"`
}

type workerNICJSON struct {
	InterfaceIndex int16            `json:"interfaceIndex"`
	Addresses      []workerAddrJSON `json:"addresses"`
}

type workerAddrJSON struct {
	Address string `json:"address"`
}

func toWorkerNodeDetail(wn *WorkerNode) workerNodeDetailJSON {
	nics := make([]workerNICJSON, len(wn.NetworkInterfaces))
	for i, nic := range wn.NetworkInterfaces {
		addrs := make([]workerAddrJSON, len(nic.Addresses))
		for j, a := range nic.Addresses {
			addrs[j] = workerAddrJSON{Address: a}
		}
		nics[i] = workerNICJSON{InterfaceIndex: nic.InterfaceIndex, Addresses: addrs}
	}
	return workerNodeDetailJSON{
		WorkerNodeID:       wn.WorkerNodeID,
		ResourceID:         wn.ResourceID,
		Draining:           wn.Draining,
		Status:             wn.Status,
		Healthy:            wn.Healthy,
		Creating:           wn.Creating,
		Created:            wn.Created,
		RunningContainers:  []runContainerJSON{},
		NetworkInterfaces:  nics,
		ArchiveVersion:     wn.ArchiveVersion,
		CreateErrorMessage: wn.CreateErrorMessage,
	}
}

func toWorkerNodeSummary(wn *WorkerNode) workerNodeSummaryJSON {
	nics := make([]workerNICJSON, len(wn.NetworkInterfaces))
	for i, nic := range wn.NetworkInterfaces {
		addrs := make([]workerAddrJSON, len(nic.Addresses))
		for j, a := range nic.Addresses {
			addrs[j] = workerAddrJSON{Address: a}
		}
		nics[i] = workerNICJSON{InterfaceIndex: nic.InterfaceIndex, Addresses: addrs}
	}
	return workerNodeSummaryJSON{
		WorkerNodeID:       wn.WorkerNodeID,
		ResourceID:         wn.ResourceID,
		Draining:           wn.Draining,
		Status:             wn.Status,
		NetworkInterfaces:  nics,
		ArchiveVersion:     wn.ArchiveVersion,
		Created:            wn.Created,
		CreateErrorMessage: wn.CreateErrorMessage,
	}
}

// Certificate

type createCertificateReq struct {
	Name                       string  `json:"name"`
	CertificatePem             string  `json:"certificatePem"`
	PrivatekeyPem              string  `json:"privatekeyPem"`
	IntermediateCertificatePem *string `json:"intermediateCertificatePem,omitempty"`
}

type certificateJSON struct {
	CertificateID           string   `json:"certificateID"`
	Name                    string   `json:"name"`
	CommonName              string   `json:"commonName"`
	SubjectAlternativeNames []string `json:"subjectAlternativeNames"`
	NotBeforeSec            int64    `json:"notBeforeSec"`
	NotAfterSec             int64    `json:"notAfterSec"`
	Created                 int64    `json:"created"`
	Updated                 int64    `json:"updated"`
}

func toCertificateJSON(cert *Certificate) certificateJSON {
	return certificateJSON{
		CertificateID:           cert.CertificateID,
		Name:                    cert.Name,
		CommonName:              cert.CommonName,
		SubjectAlternativeNames: cert.SubjectAlternativeNames,
		NotBeforeSec:            cert.NotBeforeSec,
		NotAfterSec:             cert.NotAfterSec,
		Created:                 cert.Created,
		Updated:                 cert.Updated,
	}
}

// Service Classes

type lbServiceClassJSON struct {
	Path      string `json:"path"`
	NodeCount int16  `json:"nodeCount"`
	Name      string `json:"name"`
}

type workerServiceClassJSON struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// List response types — nextCursor must be omitted (not null) when absent.

type listClustersResp struct {
	Clusters   []clusterSummaryJSON `json:"clusters"`
	NextCursor *string              `json:"nextCursor,omitempty"`
}

type listApplicationsResp struct {
	Applications []applicationDetailJSON `json:"applications"`
	NextCursor   *string                 `json:"nextCursor,omitempty"`
}

type listVersionsResp struct {
	Versions   []versionDeploymentJSON `json:"versions"`
	NextCursor *int32                  `json:"nextCursor,omitempty"`
}

type listASGsResp struct {
	AutoScalingGroups []asgDetailJSON `json:"autoScalingGroups"`
	NextCursor        *string         `json:"nextCursor,omitempty"`
}

type listLBsResp struct {
	LoadBalancers []lbSummaryJSON `json:"loadBalancers"`
	NextCursor    *string         `json:"nextCursor,omitempty"`
}

type listLBNodesResp struct {
	LoadBalancerNodes []lbNodeJSON `json:"loadBalancerNodes"`
	NextCursor        *string      `json:"nextCursor,omitempty"`
}

type listWorkerNodesResp struct {
	WorkerNodes []workerNodeSummaryJSON `json:"workerNodes"`
	NextCursor  *string                 `json:"nextCursor,omitempty"`
}

type listCertificatesResp struct {
	Certificates []certificateJSON `json:"certificates"`
	NextCursor   *string           `json:"nextCursor,omitempty"`
}

// Container Placement

type containerPlacementJSON struct {
	NodeID          string `json:"nodeID"`
	ContainersStats any    `json:"containersStats"`
	Desired         any    `json:"desired"`
}

// --- Handlers ---

func (s *Server) handleCreateCluster(w http.ResponseWriter, r *http.Request) {
	var req createClusterReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg := validateCreateCluster(&req); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	ports := make([]ClusterPort, len(req.Ports))
	for i, p := range req.Ports {
		ports[i] = ClusterPort{Port: p.Port, Protocol: p.Protocol}
	}
	cluster := &Cluster{
		Name:               req.Name,
		ServicePrincipalID: req.ServicePrincipalID,
		LetsEncryptEmail:   req.LetsEncryptEmail,
		Ports:              ports,
	}
	if err := s.store.CreateCluster(cluster); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("cluster created", "id", cluster.ClusterID, "name", cluster.Name)
	writeJSON(w, http.StatusOK, map[string]any{
		"cluster": map[string]any{"clusterID": cluster.ClusterID},
	})
}

func (s *Server) handleListClusters(w http.ResponseWriter, r *http.Request) {
	cursor, maxItems := parsePagination(r, 20)
	clusters, next := s.store.ListClusters(cursor, maxItems)

	items := make([]clusterSummaryJSON, len(clusters))
	for i, c := range clusters {
		items[i] = toClusterSummary(c)
	}
	writeJSON(w, http.StatusOK, listClustersResp{Clusters: items, NextCursor: next})
}

func (s *Server) handleGetCluster(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("clusterID")
	cluster, ok := s.store.ReadCluster(id)
	if !ok {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"cluster": toClusterDetail(cluster),
	})
}

func (s *Server) handleUpdateCluster(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("clusterID")
	var req updateClusterReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg := validateUpdateCluster(&req); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.store.UpdateCluster(id, req.ServicePrincipalID, req.LetsEncryptEmail); err != nil {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}
	s.logger.Debug("cluster updated", "id", id)
	writeNoContent(w)
}

func (s *Server) handleDeleteCluster(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("clusterID")
	if err := s.store.DeleteCluster(id); err != nil {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}
	s.logger.Info("cluster deleted", "id", id)
	writeNoContent(w)
}

// Application handlers

func (s *Server) handleCreateApplication(w http.ResponseWriter, r *http.Request) {
	var req createApplicationReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg := validateCreateApplication(&req); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if _, ok := s.store.ReadCluster(req.ClusterID); !ok {
		writeError(w, http.StatusBadRequest, "cluster not found")
		return
	}
	if count := s.store.CountApplications(); count >= 10 {
		writeError(w, http.StatusBadRequest, "maximum 10 applications per account")
		return
	}
	app := &Application{
		Name:      req.Name,
		ClusterID: req.ClusterID,
	}
	if err := s.store.CreateApplication(app); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("application created", "id", app.ApplicationID, "name", app.Name)
	writeJSON(w, http.StatusOK, map[string]any{
		"application": map[string]any{"applicationID": app.ApplicationID},
	})
}

func (s *Server) handleListApplications(w http.ResponseWriter, r *http.Request) {
	cursor, maxItems := parsePagination(r, 20)
	var clusterID *string
	if v := r.URL.Query().Get("clusterID"); v != "" {
		clusterID = &v
	}
	apps, next := s.store.ListApplications(clusterID, cursor, maxItems)

	items := make([]applicationDetailJSON, len(apps))
	for i, app := range apps {
		items[i] = s.toApplicationDetail(app)
	}
	writeJSON(w, http.StatusOK, listApplicationsResp{Applications: items, NextCursor: next})
}

func (s *Server) handleGetApplication(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("applicationID")
	app, ok := s.store.ReadApplication(id)
	if !ok {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"application": s.toApplicationDetail(app),
	})
}

func (s *Server) handleUpdateApplication(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("applicationID")
	var req updateApplicationReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := s.store.UpdateApplication(id, req.ActiveVersion); err != nil {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}

	if s.docker != nil {
		if req.ActiveVersion != nil {
			v, ok := s.store.ReadVersion(id, *req.ActiveVersion)
			if ok && v.Image != "" {
				port := "8080"
				if len(v.ExposedPorts) > 0 {
					port = strconv.Itoa(int(v.ExposedPorts[0].TargetPort))
				}
				var env []core.EnvVar
				for _, e := range v.Env {
					val := ""
					if e.Value != nil {
						val = *e.Value
					}
					env = append(env, core.EnvVar{Key: e.Key, Value: val})
				}
				s.docker.StartContainer(id, v.Image, port, env)
			}
		} else {
			s.docker.StopContainer(id)
		}
	}

	s.logger.Debug("application updated", "id", id, "activeVersion", req.ActiveVersion)
	writeNoContent(w)
}

func (s *Server) handleDeleteApplication(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("applicationID")
	if s.docker != nil {
		s.docker.StopContainer(id)
	}
	if err := s.store.DeleteApplication(id); err != nil {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}
	s.logger.Info("application deleted", "id", id)
	writeNoContent(w)
}

func (s *Server) handleGetApplicationContainers(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("applicationID")
	if _, ok := s.store.ReadApplication(id); !ok {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"nodes": []containerPlacementJSON{},
	})
}

// Version handlers

func (s *Server) handleCreateVersion(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("applicationID")
	if _, ok := s.store.ReadApplication(appID); !ok {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}

	var req createVersionReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg := validateCreateVersion(&req); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	exposedPorts := make([]ExposedPort, len(req.ExposedPorts))
	for i, p := range req.ExposedPorts {
		ep := ExposedPort{
			TargetPort:       p.TargetPort,
			LoadBalancerPort: p.LoadBalancerPort,
			UseLetsEncrypt:   p.UseLetsEncrypt,
			Host:             p.Host,
		}
		if p.HealthCheck != nil {
			ep.HealthCheck = &HealthCheck{
				Path:            p.HealthCheck.Path,
				IntervalSeconds: p.HealthCheck.IntervalSeconds,
				TimeoutSeconds:  p.HealthCheck.TimeoutSeconds,
			}
		}
		exposedPorts[i] = ep
	}

	envVars := make([]EnvironmentVariable, len(req.Env))
	for i, e := range req.Env {
		envVars[i] = EnvironmentVariable{Key: e.Key, Value: e.Value, Secret: e.Secret}
	}

	version := &ApplicationVersion{
		CPU:               req.CPU,
		Memory:            req.Memory,
		ScalingMode:       req.ScalingMode,
		FixedScale:        req.FixedScale,
		MinScale:          req.MinScale,
		MaxScale:          req.MaxScale,
		ScaleInThreshold:  req.ScaleInThreshold,
		ScaleOutThreshold: req.ScaleOutThreshold,
		Image:             req.Image,
		Cmd:               req.Cmd,
		RegistryUsername:  req.RegistryUsername,
		RegistryPassword:  req.RegistryPassword,
		ExposedPorts:      exposedPorts,
		Env:               envVars,
	}

	if err := s.store.CreateVersion(appID, version); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("version created", "app_id", appID, "version", version.Version)
	writeJSON(w, http.StatusOK, map[string]any{
		"applicationVersion": map[string]any{"version": version.Version},
	})
}

func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("applicationID")
	cursor, maxItems := parsePagination(r, 20)
	versions, next := s.store.ListVersions(appID, cursor, maxItems)

	items := make([]versionDeploymentJSON, len(versions))
	for i, v := range versions {
		items[i] = toVersionDeployment(v)
	}
	writeJSON(w, http.StatusOK, listVersionsResp{Versions: items, NextCursor: next})
}

func (s *Server) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("applicationID")
	versionStr := r.PathValue("version")
	vnum, err := strconv.Atoi(versionStr)
	if err != nil || vnum < 1 || vnum > math.MaxInt32 {
		writeError(w, http.StatusBadRequest, "invalid version number")
		return
	}
	v, ok := s.store.ReadVersion(appID, int32(vnum))
	if !ok {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"applicationVersion": toVersionDetail(v),
	})
}

func (s *Server) handleDeleteVersion(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("applicationID")
	versionStr := r.PathValue("version")
	vnum, err := strconv.Atoi(versionStr)
	if err != nil || vnum < 1 || vnum > math.MaxInt32 {
		writeError(w, http.StatusBadRequest, "invalid version number")
		return
	}
	if err := s.store.DeleteVersion(appID, int32(vnum)); err != nil {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}
	s.logger.Info("version deleted", "app_id", appID, "version", vnum)
	writeNoContent(w)
}

// ASG handlers

func (s *Server) handleCreateASG(w http.ResponseWriter, r *http.Request) {
	clusterID := r.PathValue("clusterID")
	if _, ok := s.store.ReadCluster(clusterID); !ok {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}

	var req createASGReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg := validateCreateASG(&req); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	ifaces := make([]ASGNodeInterface, len(req.Interfaces))
	for i, iface := range req.Interfaces {
		ipPool := make([]IpRange, len(iface.IpPool))
		for j, r := range iface.IpPool {
			ipPool[j] = IpRange{Start: r.Start, End: r.End}
		}
		ifaces[i] = ASGNodeInterface{
			InterfaceIndex: iface.InterfaceIndex,
			Upstream:       iface.Upstream,
			IpPool:         ipPool,
			NetmaskLen:     iface.NetmaskLen,
			DefaultGateway: iface.DefaultGateway,
			PacketFilterID: iface.PacketFilterID,
			ConnectsToLB:   iface.ConnectsToLB,
		}
	}

	asg := &AutoScalingGroup{
		ClusterID:              clusterID,
		Name:                   req.Name,
		Zone:                   req.Zone,
		NameServers:            req.NameServers,
		WorkerServiceClassPath: req.WorkerServiceClassPath,
		MinNodes:               req.MinNodes,
		MaxNodes:               req.MaxNodes,
		Interfaces:             ifaces,
	}
	if err := s.store.CreateAutoScalingGroup(asg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("asg created", "id", asg.AutoScalingGroupID, "name", asg.Name)
	writeJSON(w, http.StatusOK, map[string]any{
		"autoScalingGroup": map[string]any{"autoScalingGroupID": asg.AutoScalingGroupID},
	})
}

func (s *Server) handleListASGs(w http.ResponseWriter, r *http.Request) {
	clusterID := r.PathValue("clusterID")
	cursor, maxItems := parsePagination(r, 20)
	asgs, next := s.store.ListAutoScalingGroups(clusterID, cursor, maxItems)

	items := make([]asgDetailJSON, len(asgs))
	for i, asg := range asgs {
		items[i] = toASGDetail(asg)
	}
	writeJSON(w, http.StatusOK, listASGsResp{AutoScalingGroups: items, NextCursor: next})
}

func (s *Server) handleGetASG(w http.ResponseWriter, r *http.Request) {
	clusterID := r.PathValue("clusterID")
	asgID := r.PathValue("autoScalingGroupID")
	asg, ok := s.store.ReadAutoScalingGroup(clusterID, asgID)
	if !ok {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"autoScalingGroup": toASGDetail(asg),
	})
}

func (s *Server) handleDeleteASG(w http.ResponseWriter, r *http.Request) {
	clusterID := r.PathValue("clusterID")
	asgID := r.PathValue("autoScalingGroupID")
	if err := s.store.DeleteAutoScalingGroup(clusterID, asgID); err != nil {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}
	s.logger.Info("asg deleted", "id", asgID)
	writeNoContent(w)
}

// Load Balancer handlers

func (s *Server) handleCreateLB(w http.ResponseWriter, r *http.Request) {
	clusterID := r.PathValue("clusterID")
	asgID := r.PathValue("autoScalingGroupID")
	if _, ok := s.store.ReadAutoScalingGroup(clusterID, asgID); !ok {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}

	var req createLBReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg := validateCreateLB(&req); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	ifaces := make([]LBInterface, len(req.Interfaces))
	for i, iface := range req.Interfaces {
		ipPool := make([]IpRange, len(iface.IpPool))
		for j, rng := range iface.IpPool {
			ipPool[j] = IpRange{Start: rng.Start, End: rng.End}
		}
		ifaces[i] = LBInterface{
			InterfaceIndex:  iface.InterfaceIndex,
			Upstream:        iface.Upstream,
			IpPool:          ipPool,
			NetmaskLen:      iface.NetmaskLen,
			DefaultGateway:  iface.DefaultGateway,
			Vip:             iface.Vip,
			VirtualRouterID: iface.VirtualRouterID,
			PacketFilterID:  iface.PacketFilterID,
		}
	}

	lb := &LoadBalancer{
		ClusterID:          clusterID,
		AutoScalingGroupID: asgID,
		Name:               req.Name,
		ServiceClassPath:   req.ServiceClassPath,
		NameServers:        req.NameServers,
		Interfaces:         ifaces,
	}
	if err := s.store.CreateLoadBalancer(lb); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("lb created", "id", lb.LoadBalancerID, "name", lb.Name)
	writeJSON(w, http.StatusOK, map[string]any{
		"loadBalancer": map[string]any{"loadBalancerID": lb.LoadBalancerID},
	})
}

func (s *Server) handleListLBs(w http.ResponseWriter, r *http.Request) {
	clusterID := r.PathValue("clusterID")
	asgID := r.PathValue("autoScalingGroupID")
	cursor, maxItems := parsePagination(r, 20)
	lbs, next := s.store.ListLoadBalancers(clusterID, asgID, cursor, maxItems)

	items := make([]lbSummaryJSON, len(lbs))
	for i, lb := range lbs {
		items[i] = toLBSummary(lb)
	}
	writeJSON(w, http.StatusOK, listLBsResp{LoadBalancers: items, NextCursor: next})
}

func (s *Server) handleGetLB(w http.ResponseWriter, r *http.Request) {
	clusterID := r.PathValue("clusterID")
	asgID := r.PathValue("autoScalingGroupID")
	lbID := r.PathValue("loadBalancerID")
	lb, ok := s.store.ReadLoadBalancer(clusterID, asgID, lbID)
	if !ok {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"loadBalancer": toLBDetail(lb),
	})
}

func (s *Server) handleDeleteLB(w http.ResponseWriter, r *http.Request) {
	clusterID := r.PathValue("clusterID")
	asgID := r.PathValue("autoScalingGroupID")
	lbID := r.PathValue("loadBalancerID")
	if err := s.store.DeleteLoadBalancer(clusterID, asgID, lbID); err != nil {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}
	s.logger.Info("lb deleted", "id", lbID)
	writeNoContent(w)
}

// LB Node handlers

func (s *Server) handleListLBNodes(w http.ResponseWriter, r *http.Request) {
	clusterID := r.PathValue("clusterID")
	asgID := r.PathValue("autoScalingGroupID")
	lbID := r.PathValue("loadBalancerID")
	cursor, maxItems := parsePagination(r, 20)
	nodes, next := s.store.ListLoadBalancerNodes(clusterID, asgID, lbID, cursor, maxItems)

	items := make([]lbNodeJSON, len(nodes))
	for i, n := range nodes {
		items[i] = toLBNode(n)
	}
	writeJSON(w, http.StatusOK, listLBNodesResp{LoadBalancerNodes: items, NextCursor: next})
}

func (s *Server) handleGetLBNode(w http.ResponseWriter, r *http.Request) {
	clusterID := r.PathValue("clusterID")
	asgID := r.PathValue("autoScalingGroupID")
	lbID := r.PathValue("loadBalancerID")
	nodeID := r.PathValue("loadBalancerNodeID")
	node, ok := s.store.ReadLoadBalancerNode(clusterID, asgID, lbID, nodeID)
	if !ok {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"loadBalancerNode": toLBNode(node),
	})
}

// Worker Node handlers

func (s *Server) handleListWorkerNodes(w http.ResponseWriter, r *http.Request) {
	clusterID := r.PathValue("clusterID")
	asgID := r.PathValue("autoScalingGroupID")
	cursor, maxItems := parsePagination(r, 20)
	nodes, next := s.store.ListWorkerNodes(clusterID, asgID, cursor, maxItems)

	items := make([]workerNodeSummaryJSON, len(nodes))
	for i, wn := range nodes {
		items[i] = toWorkerNodeSummary(wn)
	}
	writeJSON(w, http.StatusOK, listWorkerNodesResp{WorkerNodes: items, NextCursor: next})
}

func (s *Server) handleGetWorkerNode(w http.ResponseWriter, r *http.Request) {
	clusterID := r.PathValue("clusterID")
	asgID := r.PathValue("autoScalingGroupID")
	nodeID := r.PathValue("workerNodeID")
	wn, ok := s.store.ReadWorkerNode(clusterID, asgID, nodeID)
	if !ok {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"workerNode": toWorkerNodeDetail(wn),
	})
}

func (s *Server) handleUpdateWorkerNodeDraining(w http.ResponseWriter, r *http.Request) {
	clusterID := r.PathValue("clusterID")
	asgID := r.PathValue("autoScalingGroupID")
	nodeID := r.PathValue("workerNodeID")

	var req struct {
		Draining bool `json:"draining"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := s.store.UpdateWorkerNodeDraining(clusterID, asgID, nodeID, req.Draining); err != nil {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}
	s.logger.Debug("worker node draining updated", "id", nodeID, "draining", req.Draining)
	writeNoContent(w)
}

// Certificate handlers

func (s *Server) handleCreateCertificate(w http.ResponseWriter, r *http.Request) {
	clusterID := r.PathValue("clusterID")
	if _, ok := s.store.ReadCluster(clusterID); !ok {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}

	var req createCertificateReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg := validateCreateCertificate(&req); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	cert := &Certificate{
		ClusterID:                  clusterID,
		Name:                       req.Name,
		CertificatePem:             req.CertificatePem,
		PrivateKeyPem:              req.PrivatekeyPem,
		IntermediateCertificatePem: req.IntermediateCertificatePem,
	}
	if err := s.store.CreateCertificate(cert); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("certificate created", "id", cert.CertificateID, "name", cert.Name)
	writeJSON(w, http.StatusOK, map[string]any{
		"certificate": map[string]any{"certificateID": cert.CertificateID},
	})
}

func (s *Server) handleListCertificates(w http.ResponseWriter, r *http.Request) {
	clusterID := r.PathValue("clusterID")
	cursor, maxItems := parsePagination(r, 20)
	certs, next := s.store.ListCertificates(clusterID, cursor, maxItems)

	items := make([]certificateJSON, len(certs))
	for i, cert := range certs {
		items[i] = toCertificateJSON(cert)
	}
	writeJSON(w, http.StatusOK, listCertificatesResp{Certificates: items, NextCursor: next})
}

func (s *Server) handleGetCertificate(w http.ResponseWriter, r *http.Request) {
	clusterID := r.PathValue("clusterID")
	certID := r.PathValue("certificateID")
	cert, ok := s.store.ReadCertificate(clusterID, certID)
	if !ok {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"certificate": toCertificateJSON(cert),
	})
}

func (s *Server) handleUpdateCertificate(w http.ResponseWriter, r *http.Request) {
	clusterID := r.PathValue("clusterID")
	certID := r.PathValue("certificateID")

	var req createCertificateReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg := validateCreateCertificate(&req); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	updated := &Certificate{
		Name:                       req.Name,
		CertificatePem:             req.CertificatePem,
		PrivateKeyPem:              req.PrivatekeyPem,
		IntermediateCertificatePem: req.IntermediateCertificatePem,
	}
	if err := s.store.UpdateCertificate(clusterID, certID, updated); err != nil {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}
	s.logger.Debug("certificate updated", "id", certID)
	writeNoContent(w)
}

func (s *Server) handleDeleteCertificate(w http.ResponseWriter, r *http.Request) {
	clusterID := r.PathValue("clusterID")
	certID := r.PathValue("certificateID")
	if err := s.store.DeleteCertificate(clusterID, certID); err != nil {
		writeError(w, http.StatusNotFound, "Not Found")
		return
	}
	s.logger.Info("certificate deleted", "id", certID)
	writeNoContent(w)
}

// Service Class handlers

func (s *Server) handleListLBServiceClasses(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"lbServiceClasses": []lbServiceClassJSON{
			{Path: "cloud/apprun/dedicated/lb/1vcpu_2gb", NodeCount: 1, Name: "1vCPU 2GB"},
			{Path: "cloud/apprun/dedicated/lb/2vcpu_2gb", NodeCount: 1, Name: "2vCPU 2GB"},
			{Path: "cloud/apprun/dedicated/lb-ha/1vcpu_2gb", NodeCount: 2, Name: "1vCPU 2GB (HA)"},
			{Path: "cloud/apprun/dedicated/lb-ha/2vcpu_2gb", NodeCount: 2, Name: "2vCPU 2GB (HA)"},
		},
	})
}

func (s *Server) handleListWorkerServiceClasses(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"workerServiceClasses": []workerServiceClassJSON{
			{Path: "cloud/apprun/dedicated/worker/1vcpu_2gb", Name: "1vCPU 2GB"},
			{Path: "cloud/apprun/dedicated/worker/2vcpu_2gb", Name: "2vCPU 2GB"},
			{Path: "cloud/apprun/dedicated/worker/4vcpu_4gb", Name: "4vCPU 4GB"},
			{Path: "cloud/apprun/dedicated/worker/8vcpu_8gb", Name: "8vCPU 8GB"},
		},
	})
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.latency > 0 {
		time.Sleep(s.latency)
	}
	s.logger.Info("request", "method", r.Method, "path", r.URL.Path)
	s.mux.ServeHTTP(w, r)
}
