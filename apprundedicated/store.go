package apprundedicated

type Cluster struct {
	ClusterID          string
	Name               string
	ServicePrincipalID string
	LetsEncryptEmail   *string
	Ports              []ClusterPort
	Created            int64
}

type ClusterPort struct {
	Port     uint16
	Protocol string
}

type Application struct {
	ApplicationID          string
	Name                   string
	ClusterID              string
	ActiveVersion          *int32
	DesiredCount           *int32
	ScalingCooldownSeconds int32
}

type ApplicationVersion struct {
	Version           int32
	ApplicationID     string
	CPU               int64
	Memory            int64
	ScalingMode       string
	FixedScale        *int32
	MinScale          *int32
	MaxScale          *int32
	ScaleInThreshold  *int32
	ScaleOutThreshold *int32
	Image             string
	Cmd               []string
	RegistryUsername  *string
	RegistryPassword  *string
	ExposedPorts      []ExposedPort
	Env               []EnvironmentVariable
	ActiveNodeCount   int64
	Created           int64
}

type ExposedPort struct {
	TargetPort       uint16
	LoadBalancerPort *uint16
	UseLetsEncrypt   bool
	Host             []string
	HealthCheck      *HealthCheck
}

type HealthCheck struct {
	Path            string
	IntervalSeconds int32
	TimeoutSeconds  int32
}

type EnvironmentVariable struct {
	Key    string
	Value  *string
	Secret bool
}

type AutoScalingGroup struct {
	AutoScalingGroupID     string
	ClusterID              string
	Name                   string
	Zone                   string
	NameServers            []string
	WorkerServiceClassPath string
	MinNodes               int32
	MaxNodes               int32
	WorkerNodeCount        int32
	Deleting               bool
	Interfaces             []ASGNodeInterface
}

type ASGNodeInterface struct {
	InterfaceIndex int16
	Upstream       string
	IpPool         []IpRange
	NetmaskLen     *int16
	DefaultGateway *string
	PacketFilterID *string
	ConnectsToLB   bool
}

type IpRange struct {
	Start string
	End   string
}

type LoadBalancer struct {
	LoadBalancerID     string
	ClusterID          string
	AutoScalingGroupID string
	Name               string
	ServiceClassPath   string
	NameServers        []string
	Interfaces         []LBInterface
	Created            int64
	Deleting           bool
}

type LBInterface struct {
	InterfaceIndex  int16
	Upstream        string
	IpPool          []IpRange
	NetmaskLen      *int16
	DefaultGateway  *string
	Vip             *string
	VirtualRouterID *int16
	PacketFilterID  *string
}

type LoadBalancerNode struct {
	LoadBalancerNodeID string
	LoadBalancerID     string
	ResourceID         *string
	Status             string
	Interfaces         []LBNodeInterface
	ArchiveVersion     *string
	CreateErrorMessage *string
	Created            int64
}

type LBNodeInterface struct {
	InterfaceIndex int16
	Addresses      []LBNodeAddress
}

type LBNodeAddress struct {
	Address string
	Vip     bool
}

type WorkerNode struct {
	WorkerNodeID       string
	ClusterID          string
	AutoScalingGroupID string
	ResourceID         *string
	Draining           bool
	Status             string
	Healthy            bool
	Creating           bool
	NetworkInterfaces  []WorkerNodeNIC
	ArchiveVersion     *string
	CreateErrorMessage *string
	Created            int64
}

type WorkerNodeNIC struct {
	InterfaceIndex int16
	Addresses      []string
}

type Certificate struct {
	CertificateID              string
	ClusterID                  string
	Name                       string
	CommonName                 string
	SubjectAlternativeNames    []string
	NotBeforeSec               int64
	NotAfterSec                int64
	Created                    int64
	Updated                    int64
	CertificatePem             string
	PrivateKeyPem              string
	IntermediateCertificatePem *string
}

type Store interface {
	CreateCluster(c *Cluster) error
	ListClusters(cursor string, maxItems int) ([]*Cluster, *string)
	ReadCluster(id string) (*Cluster, bool)
	UpdateCluster(id string, servicePrincipalID string, letsEncryptEmail *string) error
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
