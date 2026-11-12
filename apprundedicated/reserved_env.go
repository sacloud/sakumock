package apprundedicated

import (
	"strconv"

	"github.com/google/uuid"

	"github.com/sacloud/sakumock/core"
)

// Reserved environment variables AppRun Dedicated provides to every application
// container (https://manual.sakura.ad.jp/cloud/apprun-dedicated/glossary.html#apprun-dedicated-glossary-env-vars).
const (
	EnvApplicationID      = "APPRUN_DEDICATED_APPLICATION_ID"
	EnvApplicationVersion = "APPRUN_DEDICATED_APPLICATION_VERSION"
	EnvContainerID        = "APPRUN_DEDICATED_CONTAINER_ID"
	EnvNodeID             = "APPRUN_DEDICATED_NODE_ID"
)

// ContainerRef is the AppRun-level identity of a running container: its
// container ID and the worker node it is placed on.
type ContainerRef struct {
	ID     string
	NodeID string
}

func newContainerRef(nodeID string) ContainerRef {
	if nodeID == "" {
		nodeID = uuid.Must(uuid.NewV7()).String()
	}
	return ContainerRef{ID: uuid.Must(uuid.NewV7()).String(), NodeID: nodeID}
}

// containerEnv returns the version's user-defined variables followed by the
// reserved ones. User variables named like a reserved one are dropped so the
// platform-provided value always wins.
func containerEnv(appID string, v *ApplicationVersion, ref ContainerRef) []core.EnvVar {
	reserved := []core.EnvVar{
		{Key: EnvApplicationID, Value: appID},
		{Key: EnvApplicationVersion, Value: strconv.Itoa(int(v.Version))},
		{Key: EnvContainerID, Value: ref.ID},
		{Key: EnvNodeID, Value: ref.NodeID},
	}
	isReserved := make(map[string]bool, len(reserved))
	for _, e := range reserved {
		isReserved[e.Key] = true
	}
	env := make([]core.EnvVar, 0, len(v.Env)+len(reserved))
	for _, e := range v.Env {
		if isReserved[e.Key] {
			continue
		}
		val := ""
		if e.Value != nil {
			val = *e.Value
		}
		env = append(env, core.EnvVar{Key: e.Key, Value: val})
	}
	return append(env, reserved...)
}

// placementNodeID returns the worker node the mock places the application's
// single container on: the first worker node of the cluster, or "" when the
// cluster has none.
func (s *Server) placementNodeID(clusterID string) string {
	asgs, _ := s.store.ListAutoScalingGroups(clusterID, "", 1000)
	for _, asg := range asgs {
		wns, _ := s.store.ListWorkerNodes(clusterID, asg.AutoScalingGroupID, "", 1000)
		if len(wns) > 0 {
			return wns[0].WorkerNodeID
		}
	}
	return ""
}

// containerIDOn returns the container ID the API reports for the application on
// the worker node: the running container's ID on the node it is placed on, a
// fresh ID elsewhere. running is false when the data plane is enabled but the
// application has no running container.
func (s *Server) containerIDOn(appID, nodeID string) (id string, running bool) {
	if s.docker == nil {
		return uuid.NewString(), true
	}
	ref, ok := s.docker.Container(appID)
	if !ok {
		return uuid.NewString(), false
	}
	if ref.NodeID == nodeID {
		return ref.ID, true
	}
	return uuid.NewString(), true
}
