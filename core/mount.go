package core

import (
	"fmt"
	"net/http"
	"net/url"
)

// MountPath is the path prefix a service's control plane is served under when
// every service shares one listener (the `sakumock all` default).
func MountPath(name string) string {
	return "/" + name
}

// MountedClientEnv rewrites cfg's ClientEnv to reach the service mounted under
// MountPath(cfg.Name()) on addr. The original endpoint path is kept after the
// prefix (e.g. eventbus's trailing slash).
func MountedClientEnv(cfg ServiceConfig, addr string) ([]EnvVar, error) {
	vars := cfg.ClientEnv()
	out := make([]EnvVar, len(vars))
	for i, v := range vars {
		u, err := url.Parse(v.Value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", v.Key, err)
		}
		u.Host = addr
		u.Path = MountPath(cfg.Name()) + u.Path
		v.Value = u.String()
		out[i] = v
	}
	return out, nil
}

// MountedHandler pairs a service name with its control-plane handler for
// NewMountHandler.
type MountedHandler struct {
	Name    string
	Handler http.Handler
}

// NewMountHandler serves every handler under its MountPath, stripping the
// prefix so each service sees the same paths as on its own listener.
func NewMountHandler(handlers []MountedHandler) http.Handler {
	mux := http.NewServeMux()
	for _, h := range handlers {
		p := MountPath(h.Name)
		mux.Handle(p+"/", http.StripPrefix(p, h.Handler))
	}
	return mux
}
