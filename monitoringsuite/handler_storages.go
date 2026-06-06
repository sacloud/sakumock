package monitoringsuite

import (
	"net/http"
	"time"
)

// ===== shared object types =====

type ingesterEndpoint struct {
	Address  string `json:"address"`
	Insecure bool   `json:"insecure"`
}

type ingesterEndpoints struct {
	Ingester ingesterEndpoint `json:"ingester"`
}

type addressEndpoints struct {
	Address string `json:"address"`
}

// ===== Log storage =====

type logStorageUsage struct {
	LogRoutings     int `json:"log_routings"`
	LogMeasureRules int `json:"log_measure_rules"`
}

type logStorageJSON struct {
	ID                 int64             `json:"id"`
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	Tags               []string          `json:"tags"`
	Icon               any               `json:"icon"`
	ExpireDay          int               `json:"expire_day"`
	CreatedAt          string            `json:"created_at"`
	Endpoints          ingesterEndpoints `json:"endpoints"`
	AccountID          string            `json:"account_id"`
	ResourceID         int64             `json:"resource_id"`
	IsSystem           bool              `json:"is_system"`
	Classification     string            `json:"classification"`
	KMSKeyID           *int64            `json:"kms_key_id"`
	ServicePrincipalID *int64            `json:"service_principal_id"`
	Usage              logStorageUsage   `json:"usage"`
	IsOk               *bool             `json:"is_ok,omitempty"`
}

func (s *Server) logStorageToJSON(st *LogStorage, wrapped bool) logStorageJSON {
	usage := logStorageUsage{}
	for _, r := range s.store.logRoutings.all() {
		if r.StorageID == st.ID {
			usage.LogRoutings++
		}
	}
	for _, r := range s.store.logMeasureRules.all() {
		if r.LogStorageID != nil && *r.LogStorageID == st.ID {
			usage.LogMeasureRules++
		}
	}
	j := logStorageJSON{
		ID:                 st.ID,
		Name:               st.Name,
		Description:        st.Description,
		Tags:               st.Tags,
		Icon:               nil,
		ExpireDay:          st.ExpireDay,
		CreatedAt:          formatTime(st.CreatedAt),
		Endpoints:          ingesterEndpoints{Ingester: ingesterEndpoint{Address: "logs-ingester.monitoring.local:443"}},
		AccountID:          st.AccountID,
		ResourceID:         st.ResourceID,
		IsSystem:           st.IsSystem,
		Classification:     st.Classification,
		KMSKeyID:           st.KMSKeyID,
		ServicePrincipalID: st.ServicePrincipalID,
		Usage:              usage,
	}
	if wrapped {
		j.IsOk = boolPtr(true)
	}
	return j
}

type logStorageCreateRequest struct {
	Classification     *string `json:"classification"`
	IsSystem           bool    `json:"is_system"`
	Name               string  `json:"name"`
	Description        *string `json:"description"`
	KMSKeyID           *int64  `json:"kms_key_id"`
	ServicePrincipalID *int64  `json:"service_principal_id"`
}

type logStorageRequest struct {
	Name               *string `json:"name"`
	Description        *string `json:"description"`
	ServicePrincipalID *int64  `json:"service_principal_id"`
}

func (s *Server) handleListLogStorages(w http.ResponseWriter, r *http.Request) {
	items := s.store.logStorages.all()
	out := make([]logStorageJSON, len(items))
	for i, st := range items {
		out[i] = s.logStorageToJSON(st, false)
	}
	writePage(w, out)
}

func (s *Server) handleCreateLogStorage(w http.ResponseWriter, r *http.Request) {
	var req logStorageCreateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	classification := "shared"
	if req.Classification != nil && *req.Classification != "" {
		classification = *req.Classification
	}
	rid := s.store.nextResourceID()
	st := &LogStorage{
		ID:                 rid,
		ResourceID:         rid,
		Name:               req.Name,
		Description:        derefString(req.Description),
		Tags:               []string{},
		AccountID:          dummyAccountID,
		CreatedAt:          time.Now(),
		ExpireDay:          31,
		IsSystem:           req.IsSystem,
		Classification:     classification,
		KMSKeyID:           req.KMSKeyID,
		ServicePrincipalID: req.ServicePrincipalID,
	}
	s.store.logStorages.set(idKey(rid), st)
	writeJSON(w, http.StatusCreated, s.logStorageToJSON(st, false))
}

func (s *Server) handleReadLogStorage(w http.ResponseWriter, r *http.Request) {
	st, ok := s.store.logStorages.get(r.PathValue("resource_id"))
	if !ok {
		writeError(w, http.StatusNotFound, "No LogStorage matches the given query.")
		return
	}
	writeJSON(w, http.StatusOK, s.logStorageToJSON(st, true))
}

func (s *Server) handleUpdateLogStorage(w http.ResponseWriter, r *http.Request) {
	st, ok := s.store.logStorages.get(r.PathValue("resource_id"))
	if !ok {
		writeError(w, http.StatusNotFound, "No LogStorage matches the given query.")
		return
	}
	var req logStorageRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name != nil {
		st.Name = *req.Name
	}
	if req.Description != nil {
		st.Description = *req.Description
	}
	if req.ServicePrincipalID != nil {
		st.ServicePrincipalID = req.ServicePrincipalID
	}
	writeJSON(w, http.StatusOK, s.logStorageToJSON(st, true))
}

func (s *Server) handleDeleteLogStorage(w http.ResponseWriter, r *http.Request) {
	if !s.store.logStorages.delete(r.PathValue("resource_id")) {
		writeError(w, http.StatusNotFound, "No LogStorage matches the given query.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type setExpireRequest struct {
	Days int `json:"days"`
}

func (s *Server) handleSetLogStorageExpire(w http.ResponseWriter, r *http.Request) {
	st, ok := s.store.logStorages.get(r.PathValue("resource_id"))
	if !ok {
		writeError(w, http.StatusNotFound, "No LogStorage matches the given query.")
		return
	}
	var req setExpireRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	st.ExpireDay = req.Days
	writeJSON(w, http.StatusOK, s.logStorageToJSON(st, false))
}

// ===== Metrics storage =====

type metricsStorageUsage struct {
	MetricsRoutings int `json:"metrics_routings"`
	AlertRules      int `json:"alert_rules"`
	LogMeasureRules int `json:"log_measure_rules"`
}

type metricsStorageJSON struct {
	ID          int64               `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Tags        []string            `json:"tags"`
	Icon        any                 `json:"icon"`
	IsSystem    bool                `json:"is_system"`
	AccountID   string              `json:"account_id"`
	ResourceID  int64               `json:"resource_id"`
	Endpoints   addressEndpoints    `json:"endpoints"`
	CreatedAt   string              `json:"created_at"`
	UpdatedAt   string              `json:"updated_at"`
	Usage       metricsStorageUsage `json:"usage"`
	IsOk        *bool               `json:"is_ok,omitempty"`
}

func (s *Server) metricsStorageToJSON(st *MetricsStorage, wrapped bool) metricsStorageJSON {
	usage := metricsStorageUsage{}
	for _, r := range s.store.metricsRoutings.all() {
		if r.StorageID == st.ID {
			usage.MetricsRoutings++
		}
	}
	for _, r := range s.store.alertRules.all() {
		if r.MetricsStorageID != nil && *r.MetricsStorageID == st.ID {
			usage.AlertRules++
		}
	}
	for _, r := range s.store.logMeasureRules.all() {
		if r.MetricsStorageID != nil && *r.MetricsStorageID == st.ID {
			usage.LogMeasureRules++
		}
	}
	j := metricsStorageJSON{
		ID:          st.ID,
		Name:        st.Name,
		Description: st.Description,
		Tags:        st.Tags,
		Icon:        nil,
		IsSystem:    st.IsSystem,
		AccountID:   st.AccountID,
		ResourceID:  st.ResourceID,
		Endpoints:   addressEndpoints{Address: "metrics-ingester.monitoring.local:443"},
		CreatedAt:   formatTime(st.CreatedAt),
		UpdatedAt:   formatTime(st.UpdatedAt),
		Usage:       usage,
	}
	if wrapped {
		j.IsOk = boolPtr(true)
	}
	return j
}

type metricsStorageCreateRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	IsSystem    bool    `json:"is_system"`
}

type metricsStorageRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

func (s *Server) handleListMetricsStorages(w http.ResponseWriter, r *http.Request) {
	items := s.store.metricsStorages.all()
	out := make([]metricsStorageJSON, len(items))
	for i, st := range items {
		out[i] = s.metricsStorageToJSON(st, false)
	}
	writePage(w, out)
}

func (s *Server) handleCreateMetricsStorage(w http.ResponseWriter, r *http.Request) {
	var req metricsStorageCreateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	now := time.Now()
	rid := s.store.nextResourceID()
	st := &MetricsStorage{
		ID:          rid,
		ResourceID:  rid,
		Name:        req.Name,
		Description: derefString(req.Description),
		Tags:        []string{},
		AccountID:   dummyAccountID,
		CreatedAt:   now,
		UpdatedAt:   now,
		IsSystem:    req.IsSystem,
	}
	s.store.metricsStorages.set(idKey(rid), st)
	writeJSON(w, http.StatusCreated, s.metricsStorageToJSON(st, false))
}

func (s *Server) handleReadMetricsStorage(w http.ResponseWriter, r *http.Request) {
	st, ok := s.store.metricsStorages.get(r.PathValue("resource_id"))
	if !ok {
		writeError(w, http.StatusNotFound, "No MetricsStorage matches the given query.")
		return
	}
	writeJSON(w, http.StatusOK, s.metricsStorageToJSON(st, true))
}

func (s *Server) handleUpdateMetricsStorage(w http.ResponseWriter, r *http.Request) {
	st, ok := s.store.metricsStorages.get(r.PathValue("resource_id"))
	if !ok {
		writeError(w, http.StatusNotFound, "No MetricsStorage matches the given query.")
		return
	}
	var req metricsStorageRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name != nil {
		st.Name = *req.Name
	}
	if req.Description != nil {
		st.Description = *req.Description
	}
	st.UpdatedAt = time.Now()
	writeJSON(w, http.StatusOK, s.metricsStorageToJSON(st, true))
}

func (s *Server) handleDeleteMetricsStorage(w http.ResponseWriter, r *http.Request) {
	if !s.store.metricsStorages.delete(r.PathValue("resource_id")) {
		writeError(w, http.StatusNotFound, "No MetricsStorage matches the given query.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ===== Trace storage =====

type traceStorageJSON struct {
	ID                  int64             `json:"id"`
	Name                string            `json:"name"`
	Description         string            `json:"description"`
	Tags                []string          `json:"tags"`
	Icon                any               `json:"icon"`
	RetentionPeriodDays int               `json:"retention_period_days"`
	CreatedAt           string            `json:"created_at"`
	Endpoints           ingesterEndpoints `json:"endpoints"`
	AccountID           string            `json:"account_id"`
	ResourceID          int64             `json:"resource_id"`
	Classification      string            `json:"classification"`
	KMSKeyID            *int64            `json:"kms_key_id"`
	ServicePrincipalID  *int64            `json:"service_principal_id"`
	IsOk                *bool             `json:"is_ok,omitempty"`
}

func (s *Server) traceStorageToJSON(st *TraceStorage, wrapped bool) traceStorageJSON {
	j := traceStorageJSON{
		ID:                  st.ID,
		Name:                st.Name,
		Description:         st.Description,
		Tags:                st.Tags,
		Icon:                nil,
		RetentionPeriodDays: st.RetentionPeriodDays,
		CreatedAt:           formatTime(st.CreatedAt),
		Endpoints:           ingesterEndpoints{Ingester: ingesterEndpoint{Address: "traces-ingester.monitoring.local:443"}},
		AccountID:           st.AccountID,
		ResourceID:          st.ResourceID,
		Classification:      st.Classification,
		KMSKeyID:            st.KMSKeyID,
		ServicePrincipalID:  st.ServicePrincipalID,
	}
	if wrapped {
		j.IsOk = boolPtr(true)
	}
	return j
}

type traceStorageCreateRequest struct {
	Classification     *string `json:"classification"`
	Name               string  `json:"name"`
	Description        *string `json:"description"`
	KMSKeyID           *int64  `json:"kms_key_id"`
	ServicePrincipalID *int64  `json:"service_principal_id"`
}

type traceStorageRequest struct {
	Name               *string `json:"name"`
	Description        *string `json:"description"`
	ServicePrincipalID *int64  `json:"service_principal_id"`
}

func (s *Server) handleListTraceStorages(w http.ResponseWriter, r *http.Request) {
	items := s.store.traceStorages.all()
	out := make([]traceStorageJSON, len(items))
	for i, st := range items {
		out[i] = s.traceStorageToJSON(st, false)
	}
	writePage(w, out)
}

func (s *Server) handleCreateTraceStorage(w http.ResponseWriter, r *http.Request) {
	var req traceStorageCreateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	classification := "shared"
	if req.Classification != nil && *req.Classification != "" {
		classification = *req.Classification
	}
	rid := s.store.nextResourceID()
	st := &TraceStorage{
		ID:                  rid,
		ResourceID:          rid,
		Name:                req.Name,
		Description:         derefString(req.Description),
		Tags:                []string{},
		AccountID:           dummyAccountID,
		CreatedAt:           time.Now(),
		RetentionPeriodDays: 31,
		Classification:      classification,
		KMSKeyID:            req.KMSKeyID,
		ServicePrincipalID:  req.ServicePrincipalID,
	}
	s.store.traceStorages.set(idKey(rid), st)
	writeJSON(w, http.StatusCreated, s.traceStorageToJSON(st, false))
}

func (s *Server) handleReadTraceStorage(w http.ResponseWriter, r *http.Request) {
	st, ok := s.store.traceStorages.get(r.PathValue("resource_id"))
	if !ok {
		writeError(w, http.StatusNotFound, "No TraceStorage matches the given query.")
		return
	}
	writeJSON(w, http.StatusOK, s.traceStorageToJSON(st, true))
}

func (s *Server) handleUpdateTraceStorage(w http.ResponseWriter, r *http.Request) {
	st, ok := s.store.traceStorages.get(r.PathValue("resource_id"))
	if !ok {
		writeError(w, http.StatusNotFound, "No TraceStorage matches the given query.")
		return
	}
	var req traceStorageRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name != nil {
		st.Name = *req.Name
	}
	if req.Description != nil {
		st.Description = *req.Description
	}
	if req.ServicePrincipalID != nil {
		st.ServicePrincipalID = req.ServicePrincipalID
	}
	writeJSON(w, http.StatusOK, s.traceStorageToJSON(st, true))
}

func (s *Server) handleDeleteTraceStorage(w http.ResponseWriter, r *http.Request) {
	if !s.store.traceStorages.delete(r.PathValue("resource_id")) {
		writeError(w, http.StatusNotFound, "No TraceStorage matches the given query.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSetTraceStorageExpire(w http.ResponseWriter, r *http.Request) {
	st, ok := s.store.traceStorages.get(r.PathValue("resource_id"))
	if !ok {
		writeError(w, http.StatusNotFound, "No TraceStorage matches the given query.")
		return
	}
	var req setExpireRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	st.RetentionPeriodDays = req.Days
	writeJSON(w, http.StatusOK, s.traceStorageToJSON(st, false))
}

// ===== Access keys (log / metrics / trace) =====

type logMetricsKeyJSON struct {
	ID          int64  `json:"id"`
	UID         string `json:"uid"`
	Secret      string `json:"secret"`
	Token       string `json:"token"`
	Description string `json:"description"`
	IsOk        *bool  `json:"is_ok,omitempty"`
}

type traceKeyJSON struct {
	UID         string `json:"uid"`
	Secret      string `json:"secret"`
	Token       string `json:"token"`
	Description string `json:"description"`
	IsOk        *bool  `json:"is_ok,omitempty"`
}

func logMetricsKeyToJSON(k *AccessKey, wrapped bool) logMetricsKeyJSON {
	j := logMetricsKeyJSON{ID: k.ID, UID: k.UID, Secret: k.Secret, Token: k.Token, Description: k.Description}
	if wrapped {
		j.IsOk = boolPtr(true)
	}
	return j
}

func traceKeyToJSON(k *AccessKey, wrapped bool) traceKeyJSON {
	j := traceKeyJSON{UID: k.UID, Secret: k.Secret, Token: k.Token, Description: k.Description}
	if wrapped {
		j.IsOk = boolPtr(true)
	}
	return j
}

type accessKeyRequest struct {
	Description *string `json:"description"`
}

func newAccessKey(s *MemoryStore, parent string, desc *string) *AccessKey {
	return &AccessKey{
		ID:          s.nextInternalID(),
		UID:         newUUID(),
		Secret:      newUUID(),
		Token:       "mt-" + newUUID(),
		Description: derefString(desc),
		ParentKey:   parent,
	}
}

// getAccessKey resolves an access key by its UUID or its numeric id. The
// monitoring-suite API path is keyed by the UUID, but the Terraform provider
// addresses a key by the response `id` field, so accept either.
func getAccessKey(tbl *table[AccessKey], key string) (*AccessKey, bool) {
	if k, ok := tbl.get(key); ok {
		return k, true
	}
	for _, k := range tbl.all() {
		if idKey(k.ID) == key {
			return k, true
		}
	}
	return nil, false
}

// getStorageKey resolves a key by UUID or numeric id and verifies it belongs to
// the storage addressed by parent, so a key cannot be read through a different
// storage's path.
func getStorageKey(tbl *table[AccessKey], parent, key string) (*AccessKey, bool) {
	k, ok := getAccessKey(tbl, key)
	if !ok || k.ParentKey != parent {
		return nil, false
	}
	return k, true
}

// deleteStorageKey removes a key found by UUID or numeric id only when it belongs
// to the storage addressed by parent, returning false otherwise.
func deleteStorageKey(tbl *table[AccessKey], parent, key string) bool {
	k, ok := getStorageKey(tbl, parent, key)
	if !ok {
		return false
	}
	return tbl.delete(k.UID)
}

// storageKeyParent reports whether the storage addressed by the given path param
// exists, returning a 404 when not.
func storageKeyParent[T any](w http.ResponseWriter, tbl *table[T], key string, kind string) bool {
	if _, ok := tbl.get(key); !ok {
		writeError(w, http.StatusNotFound, "No "+kind+" matches the given query.")
		return false
	}
	return true
}

// Log storage keys.
func (s *Server) handleListLogStorageKeys(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("log_resource_id")
	if !storageKeyParent(w, s.store.logStorages, pid, "LogStorage") {
		return
	}
	out := []logMetricsKeyJSON{}
	for _, k := range s.store.logKeys.all() {
		if k.ParentKey == pid {
			out = append(out, logMetricsKeyToJSON(k, false))
		}
	}
	writePage(w, out)
}

func (s *Server) handleCreateLogStorageKey(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("log_resource_id")
	if !storageKeyParent(w, s.store.logStorages, pid, "LogStorage") {
		return
	}
	var req accessKeyRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	k := newAccessKey(s.store, pid, req.Description)
	s.store.logKeys.set(k.UID, k)
	writeJSON(w, http.StatusCreated, logMetricsKeyToJSON(k, true))
}

func (s *Server) handleReadLogStorageKey(w http.ResponseWriter, r *http.Request) {
	k, ok := getStorageKey(s.store.logKeys, r.PathValue("log_resource_id"), r.PathValue("uid"))
	if !ok {
		writeError(w, http.StatusNotFound, "No LogStorageAccessKey matches the given query.")
		return
	}
	writeJSON(w, http.StatusOK, logMetricsKeyToJSON(k, true))
}

func (s *Server) handleUpdateLogStorageKey(w http.ResponseWriter, r *http.Request) {
	k, ok := getStorageKey(s.store.logKeys, r.PathValue("log_resource_id"), r.PathValue("uid"))
	if !ok {
		writeError(w, http.StatusNotFound, "No LogStorageAccessKey matches the given query.")
		return
	}
	var req accessKeyRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Description != nil {
		k.Description = *req.Description
	}
	writeJSON(w, http.StatusOK, logMetricsKeyToJSON(k, true))
}

func (s *Server) handleDeleteLogStorageKey(w http.ResponseWriter, r *http.Request) {
	if !deleteStorageKey(s.store.logKeys, r.PathValue("log_resource_id"), r.PathValue("uid")) {
		writeError(w, http.StatusNotFound, "No LogStorageAccessKey matches the given query.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Metrics storage keys.
func (s *Server) handleListMetricsStorageKeys(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("metrics_resource_id")
	if !storageKeyParent(w, s.store.metricsStorages, pid, "MetricsStorage") {
		return
	}
	out := []logMetricsKeyJSON{}
	for _, k := range s.store.metricsKeys.all() {
		if k.ParentKey == pid {
			out = append(out, logMetricsKeyToJSON(k, false))
		}
	}
	writePage(w, out)
}

func (s *Server) handleCreateMetricsStorageKey(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("metrics_resource_id")
	if !storageKeyParent(w, s.store.metricsStorages, pid, "MetricsStorage") {
		return
	}
	var req accessKeyRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	k := newAccessKey(s.store, pid, req.Description)
	s.store.metricsKeys.set(k.UID, k)
	writeJSON(w, http.StatusCreated, logMetricsKeyToJSON(k, true))
}

func (s *Server) handleReadMetricsStorageKey(w http.ResponseWriter, r *http.Request) {
	k, ok := getStorageKey(s.store.metricsKeys, r.PathValue("metrics_resource_id"), r.PathValue("uid"))
	if !ok {
		writeError(w, http.StatusNotFound, "No MetricsStorageAccessKey matches the given query.")
		return
	}
	writeJSON(w, http.StatusOK, logMetricsKeyToJSON(k, true))
}

func (s *Server) handleUpdateMetricsStorageKey(w http.ResponseWriter, r *http.Request) {
	k, ok := getStorageKey(s.store.metricsKeys, r.PathValue("metrics_resource_id"), r.PathValue("uid"))
	if !ok {
		writeError(w, http.StatusNotFound, "No MetricsStorageAccessKey matches the given query.")
		return
	}
	var req accessKeyRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Description != nil {
		k.Description = *req.Description
	}
	writeJSON(w, http.StatusOK, logMetricsKeyToJSON(k, true))
}

func (s *Server) handleDeleteMetricsStorageKey(w http.ResponseWriter, r *http.Request) {
	if !deleteStorageKey(s.store.metricsKeys, r.PathValue("metrics_resource_id"), r.PathValue("uid")) {
		writeError(w, http.StatusNotFound, "No MetricsStorageAccessKey matches the given query.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Trace storage keys.
func (s *Server) handleListTraceStorageKeys(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("trace_resource_id")
	if !storageKeyParent(w, s.store.traceStorages, pid, "TraceStorage") {
		return
	}
	out := []traceKeyJSON{}
	for _, k := range s.store.traceKeys.all() {
		if k.ParentKey == pid {
			out = append(out, traceKeyToJSON(k, false))
		}
	}
	writePage(w, out)
}

func (s *Server) handleCreateTraceStorageKey(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("trace_resource_id")
	if !storageKeyParent(w, s.store.traceStorages, pid, "TraceStorage") {
		return
	}
	var req accessKeyRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	k := newAccessKey(s.store, pid, req.Description)
	s.store.traceKeys.set(k.UID, k)
	writeJSON(w, http.StatusCreated, traceKeyToJSON(k, true))
}

func (s *Server) handleReadTraceStorageKey(w http.ResponseWriter, r *http.Request) {
	k, ok := getStorageKey(s.store.traceKeys, r.PathValue("trace_resource_id"), r.PathValue("uid"))
	if !ok {
		writeError(w, http.StatusNotFound, "No TraceStorageAccessKey matches the given query.")
		return
	}
	writeJSON(w, http.StatusOK, traceKeyToJSON(k, true))
}

func (s *Server) handleUpdateTraceStorageKey(w http.ResponseWriter, r *http.Request) {
	k, ok := getStorageKey(s.store.traceKeys, r.PathValue("trace_resource_id"), r.PathValue("uid"))
	if !ok {
		writeError(w, http.StatusNotFound, "No TraceStorageAccessKey matches the given query.")
		return
	}
	var req accessKeyRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Description != nil {
		k.Description = *req.Description
	}
	writeJSON(w, http.StatusOK, traceKeyToJSON(k, true))
}

func (s *Server) handleDeleteTraceStorageKey(w http.ResponseWriter, r *http.Request) {
	if !deleteStorageKey(s.store.traceKeys, r.PathValue("trace_resource_id"), r.PathValue("uid")) {
		writeError(w, http.StatusNotFound, "No TraceStorageAccessKey matches the given query.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ===== Stats (daily / monthly) =====
// The mock holds no telemetry, so usage stats are always empty.

type usagesBody struct {
	Usages []any `json:"usages"`
}

// writeStorageStats returns an empty usage body once the storage is confirmed to
// exist; the mock holds no telemetry so there is nothing to aggregate.
func writeStorageStats[T any](w http.ResponseWriter, tbl *table[T], key, kind string) {
	if _, ok := tbl.get(key); !ok {
		writeError(w, http.StatusNotFound, "No "+kind+" matches the given query.")
		return
	}
	writeJSON(w, http.StatusOK, usagesBody{Usages: []any{}})
}

func (s *Server) handleLogStorageStatsDaily(w http.ResponseWriter, r *http.Request) {
	writeStorageStats(w, s.store.logStorages, r.PathValue("resource_id"), "LogStorage")
}
func (s *Server) handleLogStorageStatsMonthly(w http.ResponseWriter, r *http.Request) {
	writeStorageStats(w, s.store.logStorages, r.PathValue("resource_id"), "LogStorage")
}
func (s *Server) handleMetricsStorageStatsDaily(w http.ResponseWriter, r *http.Request) {
	writeStorageStats(w, s.store.metricsStorages, r.PathValue("resource_id"), "MetricsStorage")
}
func (s *Server) handleMetricsStorageStatsMonthly(w http.ResponseWriter, r *http.Request) {
	writeStorageStats(w, s.store.metricsStorages, r.PathValue("resource_id"), "MetricsStorage")
}
func (s *Server) handleTraceStorageStatsDaily(w http.ResponseWriter, r *http.Request) {
	writeStorageStats(w, s.store.traceStorages, r.PathValue("resource_id"), "TraceStorage")
}
func (s *Server) handleTraceStorageStatsMonthly(w http.ResponseWriter, r *http.Request) {
	writeStorageStats(w, s.store.traceStorages, r.PathValue("resource_id"), "TraceStorage")
}
