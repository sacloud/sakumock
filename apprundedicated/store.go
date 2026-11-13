package apprundedicated

// Cluster is the isolated environment applications and their nodes belong to.
// The json tags (here and on the other record types) are the form in the state
// file (--state-dir), not in API responses.
type Cluster struct {
	ClusterID          string        `json:"clusterID"`
	Name               string        `json:"name"`
	ServicePrincipalID string        `json:"servicePrincipalID"`
	LetsEncryptEmail   *string       `json:"letsEncryptEmail"`
	Ports              []ClusterPort `json:"ports"`
	Created            int64         `json:"created"`
}

// ClusterPort is a port the cluster exposes.
type ClusterPort struct {
	Port     uint16 `json:"port"`
	Protocol string `json:"protocol"`
}

// Application is a deployable unit running on a cluster.
type Application struct {
	ApplicationID          string `json:"applicationID"`
	Name                   string `json:"name"`
	ClusterID              string `json:"clusterID"`
	ActiveVersion          *int32 `json:"activeVersion"`
	DesiredCount           *int32 `json:"desiredCount"`
	ScalingCooldownSeconds int32  `json:"scalingCooldownSeconds"`
}

// ApplicationVersion is an immutable, deployable revision of an application.
type ApplicationVersion struct {
	Version           int32                 `json:"version"`
	ApplicationID     string                `json:"applicationID"`
	CPU               int64                 `json:"cpu"`
	Memory            int64                 `json:"memory"`
	ScalingMode       string                `json:"scalingMode"`
	FixedScale        *int32                `json:"fixedScale"`
	MinScale          *int32                `json:"minScale"`
	MaxScale          *int32                `json:"maxScale"`
	ScaleInThreshold  *int32                `json:"scaleInThreshold"`
	ScaleOutThreshold *int32                `json:"scaleOutThreshold"`
	Image             string                `json:"image"`
	Cmd               []string              `json:"cmd"`
	RegistryUsername  *string               `json:"registryUsername"`
	RegistryPassword  *string               `json:"registryPassword"`
	ExposedPorts      []ExposedPort         `json:"exposedPorts"`
	Env               []EnvironmentVariable `json:"env"`
	ActiveNodeCount   int64                 `json:"activeNodeCount"`
	Created           int64                 `json:"created"`
}

// ExposedPort is a port an application version listens on.
type ExposedPort struct {
	TargetPort       uint16       `json:"targetPort"`
	LoadBalancerPort *uint16      `json:"loadBalancerPort"`
	UseLetsEncrypt   bool         `json:"useLetsEncrypt"`
	Host             []string     `json:"host"`
	HealthCheck      *HealthCheck `json:"healthCheck"`
}

// HealthCheck is how the platform decides an application version is healthy.
type HealthCheck struct {
	Path            string `json:"path"`
	IntervalSeconds int32  `json:"intervalSeconds"`
	TimeoutSeconds  int32  `json:"timeoutSeconds"`
}

// EnvironmentVariable is one variable passed to an application version.
type EnvironmentVariable struct {
	Key    string  `json:"key"`
	Value  *string `json:"value"`
	Secret bool    `json:"secret"`
}

// AutoScalingGroup is the pool of worker nodes an application runs on.
type AutoScalingGroup struct {
	AutoScalingGroupID     string             `json:"autoScalingGroupID"`
	ClusterID              string             `json:"clusterID"`
	Name                   string             `json:"name"`
	Zone                   string             `json:"zone"`
	NameServers            []string           `json:"nameServers"`
	WorkerServiceClassPath string             `json:"workerServiceClassPath"`
	MinNodes               int32              `json:"minNodes"`
	MaxNodes               int32              `json:"maxNodes"`
	WorkerNodeCount        int32              `json:"workerNodeCount"`
	Deleting               bool               `json:"deleting"`
	Interfaces             []ASGNodeInterface `json:"interfaces"`
}

// ASGNodeInterface is a network interface attached to the group's nodes.
type ASGNodeInterface struct {
	InterfaceIndex int16     `json:"interfaceIndex"`
	Upstream       string    `json:"upstream"`
	IpPool         []IpRange `json:"ipPool"`
	NetmaskLen     *int16    `json:"netmaskLen"`
	DefaultGateway *string   `json:"defaultGateway"`
	PacketFilterID *string   `json:"packetFilterID"`
	ConnectsToLB   bool      `json:"connectsToLB"`
}

// IpRange is the address range assigned to an interface.
type IpRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// LoadBalancer fronts an auto scaling group.
type LoadBalancer struct {
	LoadBalancerID     string        `json:"loadBalancerID"`
	ClusterID          string        `json:"clusterID"`
	AutoScalingGroupID string        `json:"autoScalingGroupID"`
	Name               string        `json:"name"`
	ServiceClassPath   string        `json:"serviceClassPath"`
	NameServers        []string      `json:"nameServers"`
	Interfaces         []LBInterface `json:"interfaces"`
	Created            int64         `json:"created"`
	Deleting           bool          `json:"deleting"`
}

// LBInterface is a network interface of a load balancer.
type LBInterface struct {
	InterfaceIndex  int16     `json:"interfaceIndex"`
	Upstream        string    `json:"upstream"`
	IpPool          []IpRange `json:"ipPool"`
	NetmaskLen      *int16    `json:"netmaskLen"`
	DefaultGateway  *string   `json:"defaultGateway"`
	Vip             *string   `json:"vip"`
	VirtualRouterID *int16    `json:"virtualRouterID"`
	PacketFilterID  *string   `json:"packetFilterID"`
}

// LoadBalancerNode is one node making up a load balancer.
type LoadBalancerNode struct {
	LoadBalancerNodeID string            `json:"loadBalancerNodeID"`
	LoadBalancerID     string            `json:"loadBalancerID"`
	ResourceID         *string           `json:"resourceID"`
	Status             string            `json:"status"`
	Interfaces         []LBNodeInterface `json:"interfaces"`
	ArchiveVersion     *string           `json:"archiveVersion"`
	CreateErrorMessage *string           `json:"createErrorMessage"`
	Created            int64             `json:"created"`
}

// LBNodeInterface is a network interface of a load balancer node.
type LBNodeInterface struct {
	InterfaceIndex int16           `json:"interfaceIndex"`
	Addresses      []LBNodeAddress `json:"addresses"`
}

// LBNodeAddress is an address assigned to a load balancer node interface.
type LBNodeAddress struct {
	Address string `json:"address"`
	Vip     bool   `json:"vip"`
}

// WorkerNode is one node of an auto scaling group, where containers run.
type WorkerNode struct {
	WorkerNodeID       string          `json:"workerNodeID"`
	ClusterID          string          `json:"clusterID"`
	AutoScalingGroupID string          `json:"autoScalingGroupID"`
	ResourceID         *string         `json:"resourceID"`
	Draining           bool            `json:"draining"`
	Status             string          `json:"status"`
	Healthy            bool            `json:"healthy"`
	Creating           bool            `json:"creating"`
	NetworkInterfaces  []WorkerNodeNIC `json:"networkInterfaces"`
	ArchiveVersion     *string         `json:"archiveVersion"`
	CreateErrorMessage *string         `json:"createErrorMessage"`
	Created            int64           `json:"created"`
}

// WorkerNodeNIC is a network interface of a worker node.
type WorkerNodeNIC struct {
	InterfaceIndex int16    `json:"interfaceIndex"`
	Addresses      []string `json:"addresses"`
}

// Certificate is a TLS certificate a cluster's load balancers can serve.
type Certificate struct {
	CertificateID              string   `json:"certificateID"`
	ClusterID                  string   `json:"clusterID"`
	Name                       string   `json:"name"`
	CommonName                 string   `json:"commonName"`
	SubjectAlternativeNames    []string `json:"subjectAlternativeNames"`
	NotBeforeSec               int64    `json:"notBeforeSec"`
	NotAfterSec                int64    `json:"notAfterSec"`
	Created                    int64    `json:"created"`
	Updated                    int64    `json:"updated"`
	CertificatePem             string   `json:"certificatePem"`
	PrivateKeyPem              string   `json:"privateKeyPem"`
	IntermediateCertificatePem *string  `json:"intermediateCertificatePem"`
}

// Store is the storage backend for the AppRun Dedicated control plane.
type Store interface {
	CreateCluster(c *Cluster) error
	ListClusters(cursor string, maxItems int) ([]*Cluster, *string)
	ReadCluster(id string) (*Cluster, bool)
	UpdateCluster(id string, servicePrincipalID string, letsEncryptEmail *string, ports []ClusterPort) error
	DeleteCluster(id string) error

	CreateApplication(app *Application) error
	ListApplications(clusterID *string, cursor string, maxItems int) ([]*Application, *string)
	ReadApplication(id string) (*Application, bool)
	UpdateApplication(id string, activeVersion *int32) error
	DeleteApplication(id string) error

	CreateVersion(appID string, v *ApplicationVersion) error
	ListVersions(appID string, cursor string, maxItems int) ([]*ApplicationVersion, *int32)
	ReadVersion(appID string, version int32) (*ApplicationVersion, bool)
	DeleteVersion(appID string, version int32) error

	CreateAutoScalingGroup(asg *AutoScalingGroup) error
	ListAutoScalingGroups(clusterID string, cursor string, maxItems int) ([]*AutoScalingGroup, *string)
	ReadAutoScalingGroup(clusterID, asgID string) (*AutoScalingGroup, bool)
	DeleteAutoScalingGroup(clusterID, asgID string) error

	CreateLoadBalancer(lb *LoadBalancer) error
	ListLoadBalancers(clusterID, asgID string, cursor string, maxItems int) ([]*LoadBalancer, *string)
	ReadLoadBalancer(clusterID, asgID, lbID string) (*LoadBalancer, bool)
	DeleteLoadBalancer(clusterID, asgID, lbID string) error

	ListLoadBalancerNodes(clusterID, asgID, lbID string, cursor string, maxItems int) ([]*LoadBalancerNode, *string)
	ReadLoadBalancerNode(clusterID, asgID, lbID, nodeID string) (*LoadBalancerNode, bool)

	ListWorkerNodes(clusterID, asgID string, cursor string, maxItems int) ([]*WorkerNode, *string)
	ReadWorkerNode(clusterID, asgID, nodeID string) (*WorkerNode, bool)
	UpdateWorkerNodeDraining(clusterID, asgID, nodeID string, draining bool) error

	CreateCertificate(cert *Certificate) error
	ListCertificates(clusterID string, cursor string, maxItems int) ([]*Certificate, *string)
	ReadCertificate(clusterID, certID string) (*Certificate, bool)
	UpdateCertificate(clusterID, certID string, cert *Certificate) error
	DeleteCertificate(clusterID, certID string) error

	Close()
}
