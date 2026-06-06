package monitoringsuite

import "net/http"

// ===== Publishers (read-only) =====

type publisherVariantJSON struct {
	Name          string  `json:"name"`
	Label         string  `json:"label"`
	Storage       string  `json:"storage"`
	System        string  `json:"system"`
	MetricsPrefix *string `json:"metrics_prefix"`
}

type publisherJSON struct {
	Code        string                 `json:"code"`
	Description string                 `json:"description"`
	Variants    []publisherVariantJSON `json:"variants"`
	IsOk        *bool                  `json:"is_ok,omitempty"`
}

func publisherToJSON(p Publisher, wrapped bool) publisherJSON {
	variants := make([]publisherVariantJSON, len(p.Variants))
	for i, v := range p.Variants {
		variants[i] = publisherVariantJSON{
			Name:          v.Name,
			Label:         v.Label,
			Storage:       v.Storage,
			System:        v.System,
			MetricsPrefix: v.MetricsPrefix,
		}
	}
	j := publisherJSON{Code: p.Code, Description: p.Description, Variants: variants}
	if wrapped {
		j.IsOk = boolPtr(true)
	}
	return j
}

func (s *Server) handleListPublishers(w http.ResponseWriter, r *http.Request) {
	out := make([]publisherJSON, len(s.store.publishers))
	for i, p := range s.store.publishers {
		out[i] = publisherToJSON(p, false)
	}
	writePage(w, out)
}

func (s *Server) handleReadPublisher(w http.ResponseWriter, r *http.Request) {
	p, ok := s.store.publisher(r.PathValue("code"))
	if !ok {
		writeError(w, http.StatusNotFound, "No Publisher matches the given query.")
		return
	}
	writeJSON(w, http.StatusOK, publisherToJSON(p, true))
}

// ===== Management =====

type resourceItemLimits struct {
	MaxUserCount          int `json:"max_user_count"`
	MaxUserDedicatedCount int `json:"max_user_dedicated_count"`
}

type resourcesLimits struct {
	Logs       resourceItemLimits `json:"logs"`
	Metrics    resourceItemLimits `json:"metrics"`
	Traces     resourceItemLimits `json:"traces"`
	Alerts     resourceItemLimits `json:"alerts"`
	Dashboards resourceItemLimits `json:"dashboards"`
}

func (s *Server) handleGetResourceLimits(w http.ResponseWriter, r *http.Request) {
	limit := resourceItemLimits{MaxUserCount: 10, MaxUserDedicatedCount: 2}
	writeJSON(w, http.StatusOK, resourcesLimits{
		Logs:       limit,
		Metrics:    limit,
		Traces:     limit,
		Alerts:     limit,
		Dashboards: limit,
	})
}

type provisioningExistJSON struct {
	SystemExist bool `json:"system_exist"`
	UserExist   bool `json:"user_exist"`
}

type provisioningJSON struct {
	Logs    provisioningExistJSON `json:"logs"`
	Metrics provisioningExistJSON `json:"metrics"`
}

type provisioningCreateRequest struct {
	Logs    *provisioningExistJSON `json:"logs"`
	Metrics *provisioningExistJSON `json:"metrics"`
}

func (s *Server) provisioningResponse() provisioningJSON {
	s.store.provMu.Lock()
	defer s.store.provMu.Unlock()
	return provisioningJSON{
		Logs:    provisioningExistJSON{SystemExist: s.store.provLogs.SystemExist, UserExist: s.store.provLogs.UserExist},
		Metrics: provisioningExistJSON{SystemExist: s.store.provMetrics.SystemExist, UserExist: s.store.provMetrics.UserExist},
	}
}

func (s *Server) handleGetProvisioning(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.provisioningResponse())
}

func (s *Server) handleInitializeProvisioning(w http.ResponseWriter, r *http.Request) {
	var req provisioningCreateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.store.provMu.Lock()
	// Default to provisioning everything; honor explicit request values.
	s.store.provLogs = ProvisioningExist{SystemExist: true, UserExist: true}
	s.store.provMetrics = ProvisioningExist{SystemExist: true, UserExist: true}
	if req.Logs != nil {
		s.store.provLogs = ProvisioningExist{SystemExist: req.Logs.SystemExist, UserExist: req.Logs.UserExist}
	}
	if req.Metrics != nil {
		s.store.provMetrics = ProvisioningExist{SystemExist: req.Metrics.SystemExist, UserExist: req.Metrics.UserExist}
	}
	s.store.provMu.Unlock()
	writeJSON(w, http.StatusCreated, s.provisioningResponse())
}
