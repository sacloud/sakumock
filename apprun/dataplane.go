package apprun

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sacloud/sakumock/core"
)

type DockerManager struct {
	mu         sync.RWMutex
	containers map[string]*containerInfo
	logger     *slog.Logger
}

type containerInfo struct {
	containerID string
	hostPort    string
}

func NewDockerManager(logger *slog.Logger) *DockerManager {
	return &DockerManager{
		containers: make(map[string]*containerInfo),
		logger:     logger,
	}
}

func (dm *DockerManager) StartContainer(appID, image string, containerPort string, env []EnvVar) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	name := "sakumock-apprun-" + appID
	args := []string{"run", "-d", "--name", name, "-p", "0:" + containerPort}
	for _, e := range env {
		args = append(args, "-e", e.Key+"="+e.Value)
	}
	args = append(args, image)

	dm.logger.Info("starting container", "name", name, "image", image, "port", containerPort)
	out, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		dm.logger.Error("failed to start container", "name", name, "error", err, "output", string(out))
		return
	}
	containerID := strings.TrimSpace(string(out))

	portOut, err := exec.Command("docker", "port", containerID, containerPort).CombinedOutput()
	if err != nil {
		dm.logger.Error("failed to get container port", "name", name, "error", err, "output", string(portOut))
		return
	}
	hostPort := parseDockerPort(strings.TrimSpace(string(portOut)))
	dm.logger.Info("container started", "name", name, "container_id", containerID[:12], "host_port", hostPort)

	dm.containers[appID] = &containerInfo{
		containerID: containerID,
		hostPort:    hostPort,
	}
}

func (dm *DockerManager) StopContainer(appID string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	info, ok := dm.containers[appID]
	if !ok {
		return
	}

	name := "sakumock-apprun-" + appID
	dm.logger.Info("stopping container", "name", name)
	out, err := exec.Command("docker", "rm", "-f", info.containerID).CombinedOutput()
	if err != nil {
		dm.logger.Error("failed to stop container", "name", name, "error", err, "output", string(out))
	}
	delete(dm.containers, appID)
}

func (dm *DockerManager) GetContainerPort(appID string) (string, bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	info, ok := dm.containers[appID]
	if !ok {
		return "", false
	}
	return info.hostPort, true
}

func (dm *DockerManager) Close() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	for appID, info := range dm.containers {
		dm.logger.Info("cleaning up container", "name", "sakumock-apprun-"+appID)
		exec.Command("docker", "rm", "-f", info.containerID).Run()
	}
	dm.containers = make(map[string]*containerInfo)
}

// parseDockerPort extracts the host port from `docker port` output.
// The output format is like "0.0.0.0:32768" or "0.0.0.0:32768\n:::32768".
func parseDockerPort(output string) string {
	lines := strings.SplitSeq(output, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		_, port, err := net.SplitHostPort(line)
		if err == nil {
			return port
		}
	}
	return ""
}

type dataPlane struct {
	listener net.Listener
	docker   *DockerManager
	store    *MemoryStore
	logger   *slog.Logger
}

func startDataPlane(cfg Config, docker *DockerManager, store *MemoryStore, logger *slog.Logger) (*dataPlane, error) {
	ln, err := net.Listen("tcp", cfg.DataPlaneAddr)
	if err != nil {
		return nil, fmt.Errorf("data plane listen: %w", err)
	}

	dp := &dataPlane{
		listener: ln,
		docker:   docker,
		store:    store,
		logger:   logger,
	}

	srv := &http.Server{Handler: dp.handler()}
	go func() {
		if err := core.ServeListener(srv, ln, cfg.tls); err != nil && err != http.ErrServerClosed {
			logger.Error("data plane serve error", "error", err)
		}
	}()

	logger.Info("data plane started", "addr", ln.Addr().String())
	return dp, nil
}

func (dp *dataPlane) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appID := extractAppID(r.Host)
		if appID == "" {
			http.Error(w, "missing app ID in Host header", http.StatusBadRequest)
			return
		}

		hostPort, ok := dp.docker.GetContainerPort(appID)
		if !ok {
			http.Error(w, "application not found or container not running", http.StatusBadGateway)
			return
		}

		app, ok := dp.store.ReadApplication(appID)
		if !ok {
			http.Error(w, "application not found", http.StatusBadGateway)
			return
		}

		if app.TimeoutSeconds > 0 {
			ctx, cancel := context.WithTimeout(r.Context(), time.Duration(app.TimeoutSeconds)*time.Second)
			defer cancel()
			r = r.WithContext(ctx)
		}

		clientIP, _, _ := net.SplitHostPort(r.RemoteAddr)
		if clientIP == "" {
			clientIP = r.RemoteAddr
		}

		target, _ := url.Parse("http://127.0.0.1:" + hostPort)
		proxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = target.Scheme
				req.URL.Host = target.Host
				req.Host = target.Host
				req.Header.Set("X-Real-Ip", clientIP)
				req.Header.Set("X-Request-Id", uuid.NewString())
			},
			ErrorHandler: func(rw http.ResponseWriter, req *http.Request, err error) {
				if req.Context().Err() == context.DeadlineExceeded {
					http.Error(rw, "request timeout", http.StatusGatewayTimeout)
					return
				}
				http.Error(rw, err.Error(), http.StatusBadGateway)
			},
		}
		proxy.ServeHTTP(w, r)
	})
}

func (dp *dataPlane) Addr() string {
	if dp == nil || dp.listener == nil {
		return ""
	}
	return dp.listener.Addr().String()
}

func (dp *dataPlane) Close() {
	if dp == nil {
		return
	}
	if dp.listener != nil {
		dp.listener.Close()
	}
	if dp.docker != nil {
		dp.docker.Close()
	}
}

// extractAppID extracts the app ID from a Host header like "<app-id>.localhost:28088".
func extractAppID(host string) string {
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	if !strings.HasSuffix(h, ".localhost") {
		return ""
	}
	return strings.TrimSuffix(h, ".localhost")
}
