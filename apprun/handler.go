package apprun

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/sacloud/sakumock/core"
)

// JSON request/response types matching the OpenAPI spec

type userResponse struct {
	Limit struct {
		ApplicationCount int `json:"application_count"`
	} `json:"limit"`
}

type listMeta struct {
	PageNum     int    `json:"page_num"`
	PageSize    int    `json:"page_size"`
	ObjectTotal int    `json:"object_total"`
	SortField   string `json:"sort_field"`
	SortOrder   string `json:"sort_order"`
}

type listApplicationsResponse struct {
	Meta listMeta                  `json:"meta"`
	Data []applicationListItemJSON `json:"data"`
}

type applicationListItemJSON struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	PublicURL string    `json:"public_url"`
	CreatedAt time.Time `json:"created_at"`
}

type applicationJSON struct {
	ID                     string          `json:"id"`
	Name                   string          `json:"name"`
	TimeoutSeconds         int             `json:"timeout_seconds"`
	Port                   int             `json:"port"`
	MinScale               int             `json:"min_scale"`
	MaxScale               int             `json:"max_scale"`
	ScaleTargetConcurrency int             `json:"scale_target_concurrency,omitempty"`
	Components             []componentJSON `json:"components"`
	Status                 string          `json:"status"`
	PublicURL              string          `json:"public_url"`
	ResourceID             string          `json:"resource_id"`
	CreatedAt              time.Time       `json:"created_at"`
}

type applicationPatchResponseJSON struct {
	ID                     string          `json:"id"`
	Name                   string          `json:"name"`
	TimeoutSeconds         int             `json:"timeout_seconds"`
	Port                   int             `json:"port"`
	MinScale               int             `json:"min_scale"`
	MaxScale               int             `json:"max_scale"`
	ScaleTargetConcurrency int             `json:"scale_target_concurrency,omitempty"`
	Components             []componentJSON `json:"components"`
	Status                 string          `json:"status"`
	PublicURL              string          `json:"public_url"`
	ResourceID             string          `json:"resource_id"`
	UpdatedAt              time.Time       `json:"updated_at"`
}

type componentJSON struct {
	Name         string           `json:"name"`
	MaxCPU       string           `json:"max_cpu"`
	MaxMemory    string           `json:"max_memory"`
	DeploySource deploySourceJSON `json:"deploy_source"`
	Env          []envVarJSON     `json:"env,omitempty"`
	Probe        *probeJSON       `json:"probe,omitempty"`
}

type deploySourceJSON struct {
	ContainerRegistry *containerRegistryJSON `json:"container_registry,omitempty"`
}

type containerRegistryJSON struct {
	Image    string `json:"image"`
	Server   string `json:"server,omitempty"`
	Username string `json:"username,omitempty"`
}

type envVarJSON struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type probeJSON struct {
	HTTPGet *httpGetProbeJSON `json:"http_get,omitempty"`
}

type httpGetProbeJSON struct {
	Path    string       `json:"path"`
	Port    int          `json:"port"`
	Headers []headerJSON `json:"headers,omitempty"`
}

type headerJSON struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type applicationStatusJSON struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type listVersionsResponse struct {
	Meta listMeta          `json:"meta"`
	Data []versionListJSON `json:"data"`
}

type versionListJSON struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type versionJSON struct {
	ID                     string          `json:"id"`
	Name                   string          `json:"name"`
	Status                 string          `json:"status"`
	TimeoutSeconds         int             `json:"timeout_seconds"`
	Port                   int             `json:"port"`
	MinScale               int             `json:"min_scale"`
	MaxScale               int             `json:"max_scale"`
	ScaleTargetConcurrency int             `json:"scale_target_concurrency,omitempty"`
	Components             []componentJSON `json:"components"`
	CreatedAt              time.Time       `json:"created_at"`
}

type trafficItemJSON struct {
	VersionName     string `json:"version_name"`
	IsLatestVersion bool   `json:"is_latest_version"`
	Percent         int    `json:"percent"`
}

type trafficResponse struct {
	Meta any               `json:"meta"`
	Data []trafficItemJSON `json:"data"`
}

type packetFilterJSON struct {
	IsEnabled bool                      `json:"is_enabled"`
	Settings  []packetFilterSettingJSON `json:"settings"`
}

type packetFilterSettingJSON struct {
	FromIP             string `json:"from_ip"`
	FromIPPrefixLength int    `json:"from_ip_prefix_length"`
}

type createApplicationRequest struct {
	Name                   string          `json:"name"`
	TimeoutSeconds         int             `json:"timeout_seconds"`
	Port                   int             `json:"port"`
	MinScale               int             `json:"min_scale"`
	MaxScale               int             `json:"max_scale"`
	ScaleTargetConcurrency int             `json:"scale_target_concurrency"`
	Components             []componentJSON `json:"components"`
}

type patchApplicationRequest struct {
	TimeoutSeconds         *int            `json:"timeout_seconds,omitempty"`
	Port                   *int            `json:"port,omitempty"`
	MinScale               *int            `json:"min_scale,omitempty"`
	MaxScale               *int            `json:"max_scale,omitempty"`
	ScaleTargetConcurrency *int            `json:"scale_target_concurrency,omitempty"`
	Components             []componentJSON `json:"components,omitempty"`
	AllTrafficAvailable    *bool           `json:"all_traffic_available,omitempty"`
}

// error response matching model.defaultError
type apiError struct {
	Error apiErrorDetail `json:"error"`
}

type apiErrorDetail struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Errors  []apiErrorEntry `json:"errors"`
}

type apiErrorEntry struct {
	Domain       *string `json:"domain"`
	Reason       *string `json:"reason"`
	Message      *string `json:"message"`
	LocationType *string `json:"location_type"`
	Location     *string `json:"location,omitempty"`
}

func writeAppError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(apiError{
		Error: apiErrorDetail{
			Code:    status,
			Message: msg,
			Errors:  []apiErrorEntry{},
		},
	})
}

// Handlers

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("GET /user")
	if !s.store.UserCreated() {
		writeAppError(w, http.StatusNotFound, "AppRun共用型にユーザーが存在しません。")
		return
	}
	resp := userResponse{}
	resp.Limit.ApplicationCount = 20
	core.WriteJSON(w, http.StatusOK, resp)
}

func (s *Server) handlePostUser(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("POST /user")
	if s.store.UserCreated() {
		writeAppError(w, http.StatusConflict, "AppRun共用型にユーザーがすでに存在します。")
		return
	}
	s.store.CreateUser()
	resp := userResponse{}
	resp.Limit.ApplicationCount = 20
	core.WriteJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleListApplications(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("GET /applications")
	params := listParamsFromQuery(r)
	apps, total := s.store.ListApplications(params)

	data := make([]applicationListItemJSON, 0, len(apps))
	for _, app := range apps {
		data = append(data, applicationListItemJSON{
			ID:        app.ID,
			Name:      app.Name,
			Status:    app.Status,
			PublicURL: app.PublicURL,
			CreatedAt: app.CreatedAt,
		})
	}

	core.WriteJSON(w, http.StatusOK, listApplicationsResponse{
		Meta: listMeta{
			PageNum:     params.PageNum,
			PageSize:    params.PageSize,
			ObjectTotal: total,
			SortField:   params.SortField,
			SortOrder:   params.SortOrder,
		},
		Data: data,
	})
}

func (s *Server) handlePostApplication(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("POST /applications")
	var req createApplicationRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeAppError(w, http.StatusBadRequest, err.Error())
		return
	}

	app := &Application{
		Name:                   req.Name,
		TimeoutSeconds:         req.TimeoutSeconds,
		Port:                   req.Port,
		MinScale:               req.MinScale,
		MaxScale:               req.MaxScale,
		ScaleTargetConcurrency: req.ScaleTargetConcurrency,
		Components:             componentsFromJSON(req.Components),
	}

	if err := s.store.CreateApplication(app); err != nil {
		writeAppError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if s.docker != nil && len(app.Components) > 0 {
		c := app.Components[0]
		if c.DeploySource.ContainerRegistry != nil {
			s.docker.StartContainer(app.ID, c.DeploySource.ContainerRegistry.Image, strconv.Itoa(app.Port), c.Env)
		}
	}

	core.WriteJSON(w, http.StatusCreated, appToJSON(app))
}

func (s *Server) handleGetApplication(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.logger.Debug("GET /applications/"+id, "id", id)
	app, ok := s.store.ReadApplication(id)
	if !ok {
		writeAppError(w, http.StatusNotFound, "アプリケーションが見つかりませんでした。")
		return
	}
	core.WriteJSON(w, http.StatusOK, appToJSON(app))
}

func (s *Server) handlePatchApplication(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.logger.Info("PATCH /applications/"+id, "id", id)

	var req patchApplicationRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeAppError(w, http.StatusBadRequest, err.Error())
		return
	}

	patch := &Application{MinScale: -1}
	if req.TimeoutSeconds != nil {
		patch.TimeoutSeconds = *req.TimeoutSeconds
	}
	if req.Port != nil {
		patch.Port = *req.Port
	}
	if req.MinScale != nil {
		patch.MinScale = *req.MinScale
	}
	if req.MaxScale != nil {
		patch.MaxScale = *req.MaxScale
	}
	if req.ScaleTargetConcurrency != nil {
		patch.ScaleTargetConcurrency = *req.ScaleTargetConcurrency
	}
	if len(req.Components) > 0 {
		patch.Components = componentsFromJSON(req.Components)
	}

	if err := s.store.UpdateApplication(id, patch); err != nil {
		writeAppError(w, http.StatusNotFound, "アプリケーションが見つかりませんでした。")
		return
	}

	app, _ := s.store.ReadApplication(id)

	if req.AllTrafficAvailable != nil && *req.AllTrafficAvailable {
		versions, _ := s.store.ListVersions(id, ListParams{PageSize: 1, SortOrder: "desc"})
		if len(versions) > 0 {
			s.store.PutTraffic(id, []TrafficItem{{
				VersionName:     versions[0].Name,
				IsLatestVersion: true,
				Percent:         100,
			}})
		}
	}

	if s.docker != nil && len(app.Components) > 0 {
		c := app.Components[0]
		if c.DeploySource.ContainerRegistry != nil {
			s.docker.StopContainer(app.ID)
			s.docker.StartContainer(app.ID, c.DeploySource.ContainerRegistry.Image, strconv.Itoa(app.Port), c.Env)
		}
	}

	core.WriteJSON(w, http.StatusOK, appToPatchJSON(app))
}

func (s *Server) handleDeleteApplication(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.logger.Info("DELETE /applications/"+id, "id", id)

	if s.docker != nil {
		s.docker.StopContainer(id)
	}

	if err := s.store.DeleteApplication(id); err != nil {
		writeAppError(w, http.StatusNotFound, "アプリケーションが見つかりませんでした。")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetApplicationStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	app, ok := s.store.ReadApplication(id)
	if !ok {
		writeAppError(w, http.StatusNotFound, "アプリケーションが見つかりませんでした。")
		return
	}
	core.WriteJSON(w, http.StatusOK, applicationStatusJSON{
		Status:  app.Status,
		Message: "",
	})
}

func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	params := listParamsFromQuery(r)
	versions, total := s.store.ListVersions(appID, params)

	if total == 0 {
		if _, ok := s.store.ReadApplication(appID); !ok {
			writeAppError(w, http.StatusNotFound, "アプリケーションが見つかりませんでした。")
			return
		}
	}

	data := make([]versionListJSON, 0, len(versions))
	for _, v := range versions {
		data = append(data, versionListJSON{
			ID:        v.ID,
			Name:      v.Name,
			Status:    v.Status,
			CreatedAt: v.CreatedAt,
		})
	}

	core.WriteJSON(w, http.StatusOK, listVersionsResponse{
		Meta: listMeta{
			PageNum:     params.PageNum,
			PageSize:    params.PageSize,
			ObjectTotal: total,
			SortField:   params.SortField,
			SortOrder:   params.SortOrder,
		},
		Data: data,
	})
}

func (s *Server) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	versionID := r.PathValue("version_id")
	v, ok := s.store.ReadVersion(appID, versionID)
	if !ok {
		writeAppError(w, http.StatusNotFound, "アプリケーションバージョンが見つかりませんでした。")
		return
	}
	core.WriteJSON(w, http.StatusOK, versionToJSON(v))
}

func (s *Server) handleDeleteVersion(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	versionID := r.PathValue("version_id")
	if err := s.store.DeleteVersion(appID, versionID); err != nil {
		writeAppError(w, http.StatusNotFound, "アプリケーションバージョンが見つかりませんでした。")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetVersionStatus(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	versionID := r.PathValue("version_id")
	v, ok := s.store.ReadVersion(appID, versionID)
	if !ok {
		writeAppError(w, http.StatusNotFound, "アプリケーションバージョンが見つかりませんでした。")
		return
	}
	core.WriteJSON(w, http.StatusOK, applicationStatusJSON{
		Status:  v.Status,
		Message: "",
	})
}

func (s *Server) handleListTraffics(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	items, ok := s.store.GetTraffic(appID)
	if !ok {
		writeAppError(w, http.StatusNotFound, "アプリケーションが見つかりませんでした。")
		return
	}
	data := make([]trafficItemJSON, 0, len(items))
	for _, item := range items {
		data = append(data, trafficItemJSON{
			VersionName:     item.VersionName,
			IsLatestVersion: item.IsLatestVersion,
			Percent:         item.Percent,
		})
	}
	core.WriteJSON(w, http.StatusOK, trafficResponse{Meta: nil, Data: data})
}

func (s *Server) handlePutTraffics(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	var items []trafficItemJSON
	if err := core.ReadJSON(r, &items); err != nil {
		writeAppError(w, http.StatusBadRequest, err.Error())
		return
	}

	trafficItems := make([]TrafficItem, len(items))
	for i, item := range items {
		trafficItems[i] = TrafficItem{
			VersionName:     item.VersionName,
			IsLatestVersion: item.IsLatestVersion,
			Percent:         item.Percent,
		}
	}

	if err := s.store.PutTraffic(appID, trafficItems); err != nil {
		writeAppError(w, http.StatusNotFound, "アプリケーションが見つかりませんでした。")
		return
	}

	core.WriteJSON(w, http.StatusOK, trafficResponse{Meta: nil, Data: items})
}

func (s *Server) handleGetPacketFilter(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	pf, ok := s.store.GetPacketFilter(appID)
	if !ok {
		writeAppError(w, http.StatusNotFound, "アプリケーションが見つかりませんでした。")
		return
	}
	core.WriteJSON(w, http.StatusOK, packetFilterToJSON(pf))
}

func (s *Server) handlePatchPacketFilter(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	var req packetFilterJSON
	if err := core.ReadJSON(r, &req); err != nil {
		writeAppError(w, http.StatusBadRequest, err.Error())
		return
	}

	pf := &PacketFilter{
		IsEnabled: req.IsEnabled,
		Settings:  make([]PacketFilterSetting, len(req.Settings)),
	}
	for i, s := range req.Settings {
		pf.Settings[i] = PacketFilterSetting{
			FromIP:             s.FromIP,
			FromIPPrefixLength: s.FromIPPrefixLength,
		}
	}

	if err := s.store.PatchPacketFilter(appID, pf); err != nil {
		writeAppError(w, http.StatusNotFound, "アプリケーションが見つかりませんでした。")
		return
	}
	core.WriteJSON(w, http.StatusOK, packetFilterToJSON(pf))
}

// conversion helpers

func listParamsFromQuery(r *http.Request) ListParams {
	p := ListParams{
		PageNum:   1,
		PageSize:  50,
		SortField: "created_at",
		SortOrder: "desc",
	}
	if v := r.URL.Query().Get("page_num"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			p.PageNum = n
		}
	}
	if v := r.URL.Query().Get("page_size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			p.PageSize = n
		}
	}
	if v := r.URL.Query().Get("sort_field"); v != "" {
		p.SortField = v
	}
	if v := r.URL.Query().Get("sort_order"); v != "" {
		p.SortOrder = v
	}
	return p
}

func componentsFromJSON(comps []componentJSON) []Component {
	out := make([]Component, len(comps))
	for i, c := range comps {
		out[i] = Component{
			Name:      c.Name,
			MaxCPU:    c.MaxCPU,
			MaxMemory: c.MaxMemory,
			DeploySource: DeploySource{
				ContainerRegistry: containerRegistryFromJSON(c.DeploySource.ContainerRegistry),
			},
			Env:   envVarsFromJSON(c.Env),
			Probe: probeFromJSON(c.Probe),
		}
	}
	return out
}

func containerRegistryFromJSON(cr *containerRegistryJSON) *ContainerRegistry {
	if cr == nil {
		return nil
	}
	return &ContainerRegistry{
		Image:    cr.Image,
		Server:   cr.Server,
		Username: cr.Username,
	}
}

func envVarsFromJSON(vars []envVarJSON) []EnvVar {
	out := make([]EnvVar, len(vars))
	for i, v := range vars {
		out[i] = EnvVar{Key: v.Key, Value: v.Value}
	}
	return out
}

func probeFromJSON(p *probeJSON) *Probe {
	if p == nil {
		return nil
	}
	probe := &Probe{}
	if p.HTTPGet != nil {
		probe.HTTPGet = &HTTPGetProbe{
			Path: p.HTTPGet.Path,
			Port: p.HTTPGet.Port,
		}
		for _, h := range p.HTTPGet.Headers {
			probe.HTTPGet.Headers = append(probe.HTTPGet.Headers, Header{Name: h.Name, Value: h.Value})
		}
	}
	return probe
}

func componentsToJSON(comps []Component) []componentJSON {
	out := make([]componentJSON, len(comps))
	for i, c := range comps {
		out[i] = componentJSON{
			Name:      c.Name,
			MaxCPU:    c.MaxCPU,
			MaxMemory: c.MaxMemory,
			DeploySource: deploySourceJSON{
				ContainerRegistry: containerRegistryToJSON(c.DeploySource.ContainerRegistry),
			},
			Env:   envVarsToJSON(c.Env),
			Probe: probeToJSON(c.Probe),
		}
	}
	return out
}

func containerRegistryToJSON(cr *ContainerRegistry) *containerRegistryJSON {
	if cr == nil {
		return nil
	}
	return &containerRegistryJSON{
		Image:    cr.Image,
		Server:   cr.Server,
		Username: cr.Username,
	}
}

func envVarsToJSON(vars []EnvVar) []envVarJSON {
	if len(vars) == 0 {
		return nil
	}
	out := make([]envVarJSON, len(vars))
	for i, v := range vars {
		out[i] = envVarJSON{Key: v.Key, Value: v.Value}
	}
	return out
}

func probeToJSON(p *Probe) *probeJSON {
	if p == nil {
		return nil
	}
	pj := &probeJSON{}
	if p.HTTPGet != nil {
		pj.HTTPGet = &httpGetProbeJSON{
			Path: p.HTTPGet.Path,
			Port: p.HTTPGet.Port,
		}
		for _, h := range p.HTTPGet.Headers {
			pj.HTTPGet.Headers = append(pj.HTTPGet.Headers, headerJSON{Name: h.Name, Value: h.Value})
		}
	}
	return pj
}

func appToJSON(app *Application) applicationJSON {
	return applicationJSON{
		ID:                     app.ID,
		Name:                   app.Name,
		TimeoutSeconds:         app.TimeoutSeconds,
		Port:                   app.Port,
		MinScale:               app.MinScale,
		MaxScale:               app.MaxScale,
		ScaleTargetConcurrency: app.ScaleTargetConcurrency,
		Components:             componentsToJSON(app.Components),
		Status:                 app.Status,
		PublicURL:              app.PublicURL,
		ResourceID:             app.ResourceID,
		CreatedAt:              app.CreatedAt,
	}
}

func appToPatchJSON(app *Application) applicationPatchResponseJSON {
	return applicationPatchResponseJSON{
		ID:                     app.ID,
		Name:                   app.Name,
		TimeoutSeconds:         app.TimeoutSeconds,
		Port:                   app.Port,
		MinScale:               app.MinScale,
		MaxScale:               app.MaxScale,
		ScaleTargetConcurrency: app.ScaleTargetConcurrency,
		Components:             componentsToJSON(app.Components),
		Status:                 app.Status,
		PublicURL:              app.PublicURL,
		ResourceID:             app.ResourceID,
		UpdatedAt:              app.UpdatedAt,
	}
}

func versionToJSON(v *Version) versionJSON {
	return versionJSON{
		ID:                     v.ID,
		Name:                   v.Name,
		Status:                 v.Status,
		TimeoutSeconds:         v.TimeoutSeconds,
		Port:                   v.Port,
		MinScale:               v.MinScale,
		MaxScale:               v.MaxScale,
		ScaleTargetConcurrency: v.ScaleTargetConcurrency,
		Components:             componentsToJSON(v.Components),
		CreatedAt:              v.CreatedAt,
	}
}

func packetFilterToJSON(pf *PacketFilter) packetFilterJSON {
	settings := make([]packetFilterSettingJSON, len(pf.Settings))
	for i, s := range pf.Settings {
		settings[i] = packetFilterSettingJSON{
			FromIP:             s.FromIP,
			FromIPPrefixLength: s.FromIPPrefixLength,
		}
	}
	return packetFilterJSON{
		IsEnabled: pf.IsEnabled,
		Settings:  settings,
	}
}

// ServeHTTP implements the http.Handler interface.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.latency > 0 {
		time.Sleep(s.latency)
	}
	s.logger.Debug("request", "method", r.Method, "path", r.URL.Path)
	s.mux.ServeHTTP(w, r)
}

// unused but referenced by slog; suppress linter
var _ = slog.Info
