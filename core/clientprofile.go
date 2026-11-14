package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ClientProfileName is the name of the SDK profile WriteClientProfile writes,
// i.e. the value a client sets in SAKURA_PROFILE.
const ClientProfileName = "sakumock"

const endpointEnvPrefix = "SAKURA_ENDPOINTS_"

// clientProfile is the SDK profile file (the JSON format every SDK and
// Terraform provider release reads).
type clientProfile struct {
	AccessToken       string            `json:"AccessToken"`
	AccessTokenSecret string            `json:"AccessTokenSecret"`
	Endpoints         map[string]string `json:"Endpoints"`
}

// WriteClientProfile writes vars as the SDK profile ClientProfileName under
// dir (<dir>/sakumock/config.json) and returns the env a client sets instead
// of vars: SAKURA_PROFILE and SAKURA_PROFILE_DIR, followed by the vars a
// profile cannot hold (e.g. AWS_* for a data plane). However many services
// there are, the SDK settings stay two variables.
func WriteClientProfile(dir string, vars []EnvVar) ([]EnvVar, error) {
	profile := clientProfile{Endpoints: map[string]string{}}
	env := []EnvVar{
		{Key: "SAKURA_PROFILE", Value: ClientProfileName},
		{Key: "SAKURA_PROFILE_DIR", Value: dir},
	}
	for _, v := range vars {
		// The SDK looks an endpoint up by the lowercased env var suffix.
		if key, ok := strings.CutPrefix(v.Key, endpointEnvPrefix); ok {
			profile.Endpoints[strings.ToLower(key)] = v.Value
			continue
		}
		switch v.Key {
		case "SAKURA_ACCESS_TOKEN":
			profile.AccessToken = v.Value
		case "SAKURA_ACCESS_TOKEN_SECRET":
			profile.AccessTokenSecret = v.Value
		default:
			env = append(env, v)
		}
	}
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return nil, err
	}
	profileDir := filepath.Join(dir, ClientProfileName)
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(profileDir, "config.json"), append(data, '\n'), 0o600); err != nil {
		return nil, err
	}
	return env, nil
}
