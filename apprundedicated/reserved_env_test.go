package apprundedicated

import (
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/sacloud/sakumock/core"
)

func TestContainerEnv(t *testing.T) {
	ptr := func(s string) *string { return &s }
	v := &ApplicationVersion{
		Version: 3,
		Env: []EnvironmentVariable{
			{Key: "FOO", Value: ptr("bar")},
			{Key: "SECRET", Secret: true},
			{Key: EnvContainerID, Value: ptr("user-defined")},
		},
	}
	ref := ContainerRef{ID: "c-id", NodeID: "n-id"}

	got := containerEnv("app-id", v, ref)
	want := []core.EnvVar{
		{Key: "FOO", Value: "bar"},
		{Key: "SECRET", Value: ""},
		{Key: EnvApplicationID, Value: "app-id"},
		{Key: EnvApplicationVersion, Value: "3"},
		{Key: EnvContainerID, Value: "c-id"},
		{Key: EnvNodeID, Value: "n-id"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("containerEnv() = %v, want %v", got, want)
	}
}

func TestNewContainerRef(t *testing.T) {
	ref := newContainerRef("node-1")
	if ref.NodeID != "node-1" {
		t.Errorf("NodeID = %q, want node-1", ref.NodeID)
	}
	if id, err := uuid.Parse(ref.ID); err != nil || id.Version() != 7 {
		t.Errorf("ID = %q, want a UUID v7", ref.ID)
	}

	ref = newContainerRef("")
	if id, err := uuid.Parse(ref.NodeID); err != nil || id.Version() != 7 {
		t.Errorf("NodeID without worker nodes = %q, want a UUID v7", ref.NodeID)
	}
}
