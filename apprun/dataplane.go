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
	store      *MemoryStore
	cancel     context.CancelFunc
	done       chan struct{}
}

type containerInfo struct {
	containerID string
	hostPort    string
}

func NewDockerManager(logger *slog.Logger, store *MemoryStore) *DockerManager {
	ctx, cancel := context.WithCancel(context.Background())
	dm := &DockerManager{
		containers: make(map[string]*containerInfo),
		logger:     logger,
		store:      store,
		cancel:     cancel,
		done:       make(chan struct{}),
	}
	go dm.monitor(ctx)
	return dm
}

func (dm *DockerManager) StartContainer(appID, image string, containerPort string, env []EnvVar) error {
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
		return fmt.Errorf("failed to start container: %w", err)
	}
	containerID := strings.TrimSpace(string(out))

	portOut, err := exec.Command("docker", "port", containerID, containerPort).CombinedOutput()
	if err != nil {
		dm.logger.Error("failed to get container port", "name", name, "error", err, "output", string(portOut))
		return fmt.Errorf("failed to get container port: %w", err)
	}
	hostPort := parseDockerPort(strings.TrimSpace(string(portOut)))
	dm.logger.Info("container started", "name", name, "container_id", containerID[:12], "host_port", hostPort)

	dm.containers[appID] = &containerInfo{
		containerID: containerID,
		hostPort:    hostPort,
	}

	if !inspectRunning(containerID) {
		dm.logger.Error("container exited immediately after start", "name", name)
		delete(dm.containers, appID)
		dm.store.SetApplicationStatus(appID, "Unhealthy")
		return fmt.Errorf("container exited immediately")
	}
	dm.store.SetApplicationStatus(appID, "Healthy")
	return nil
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
	dm.cancel()
	<-dm.done

	dm.mu.Lock()
	defer dm.mu.Unlock()

	for appID, info := range dm.containers {
		dm.logger.Info("cleaning up container", "name", "sakumock-apprun-"+appID)
		exec.Command("docker", "rm", "-f", info.containerID).Run()
	}
	dm.containers = make(map[string]*containerInfo)
}

func (dm *DockerManager) monitor(ctx context.Context) {
	defer close(dm.done)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dm.checkContainers()
		}
	}
}

func (dm *DockerManager) checkContainers() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	for appID, info := range dm.containers {
		if !inspectRunning(info.containerID) {
			dm.logger.Info("container exited", "name", "sakumock-apprun-"+appID, "container_id", info.containerID[:12])
			delete(dm.containers, appID)
			dm.store.SetApplicationStatus(appID, "Unhealthy")
		}
	}
}

// inspectRunning returns true if the container is running.
func inspectRunning(containerID string) bool {
	out, err := exec.Command("docker", "inspect", "--format", "{{.State.Running}}", containerID).CombinedOutput()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
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
	server   *http.Server
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

	dp.server = &http.Server{Handler: dp.handler()}
	go func() {
		if err := core.ServeListener(dp.server, ln, cfg.tls); err != nil && err != http.ErrServerClosed {
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
	if dp.server != nil {
		dp.server.Close()
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
