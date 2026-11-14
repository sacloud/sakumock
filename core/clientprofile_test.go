package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestWriteClientProfile(t *testing.T) {
	dir := t.TempDir()
	env, err := WriteClientProfile(dir, []EnvVar{
		{Key: "SAKURA_ENDPOINTS_KMS", Value: "http://127.0.0.1:18000/kms"},
		{Key: "SAKURA_ENDPOINTS_SIMPLE_MQ_QUEUE", Value: "http://127.0.0.1:18000/simplemq"},
		{Key: "AWS_ACCESS_KEY_ID", Value: "sakumock"},
		{Key: "SAKURA_ACCESS_TOKEN", Value: "dummy-token"},
		{Key: "SAKURA_ACCESS_TOKEN_SECRET", Value: "dummy-secret"},
	})
	if err != nil {
		t.Fatalf("WriteClientProfile: %v", err)
	}
	// Only the profile selection and what a profile cannot hold stay in the env.
	wantEnv := []EnvVar{
		{Key: "SAKURA_PROFILE", Value: "sakumock"},
		{Key: "SAKURA_PROFILE_DIR", Value: dir},
		{Key: "AWS_ACCESS_KEY_ID", Value: "sakumock"},
	}
	if !slices.Equal(env, wantEnv) {
		t.Errorf("env = %+v, want %+v", env, wantEnv)
	}

	path := filepath.Join(dir, "sakumock", "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got clientProfile
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("profile is not JSON: %v\n%s", err, data)
	}
	if got.AccessToken != "dummy-token" || got.AccessTokenSecret != "dummy-secret" {
		t.Errorf("credentials = %q / %q", got.AccessToken, got.AccessTokenSecret)
	}
	wantEndpoints := map[string]string{
		"kms":             "http://127.0.0.1:18000/kms",
		"simple_mq_queue": "http://127.0.0.1:18000/simplemq",
	}
	if len(got.Endpoints) != len(wantEndpoints) {
		t.Errorf("endpoints = %v, want %v", got.Endpoints, wantEndpoints)
	}
	for k, v := range wantEndpoints {
		if got.Endpoints[k] != v {
			t.Errorf("endpoint %q = %q, want %q", k, got.Endpoints[k], v)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("profile mode = %o, want 600", perm)
	}
}
