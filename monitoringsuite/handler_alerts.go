package monitoringsuite

import (
	"encoding/json"
	"net/http"
	"time"
)

// alertProject resolves the project addressed by {project_resource_id}, writing
// a 404 when it does not exist.
func (s *Server) alertProject(w http.ResponseWriter, r *http.Request) (*Project, bool) {
	p, ok := s.store.alertProjects.get(r.PathValue("project_resource_id"))
	if !ok {
		writeError(w, http.StatusNotFound, "No AlertProject matches the given query.")
		return nil, false
	}
	return p, true
}

// ===== Alert rules =====

type alertRuleJSON struct {
	UID                       string  `json:"uid"`
	ProjectID                 *int64  `json:"project_id"`
	MetricsStorageID          *int64  `json:"metrics_storage_id"`
	Name                      string  `json:"name"`
	Query                     string  `json:"query"`
	Format                    string  `json:"format"`
	Template                  string  `json:"template"`
	Open                      bool    `json:"open"`
	EnabledWarning            bool    `json:"enabled_warning"`
	EnabledCritical           bool    `json:"enabled_critical"`
	ThresholdWarning          *string `json:"threshold_warning"`
	ThresholdCritical         *string `json:"threshold_critical"`
	ThresholdDurationWarning  int64   `json:"threshold_duration_warning"`
	ThresholdDurationCritical int64   `json:"threshold_duration_critical"`
}

func alertRuleToJSON(rule *AlertRule) alertRuleJSON {
	pid := rule.ProjectID
	return alertRuleJSON{
		UID:                       rule.UID,
		ProjectID:                 &pid,
		MetricsStorageID:          rule.MetricsStorageID,
		Name:                      rule.Name,
		Query:                     rule.Query,
		Format:                    rule.Format,
		Template:                  rule.Template,
		Open:                      rule.Open,
		EnabledWarning:            rule.EnabledWarning,
		EnabledCritical:           rule.EnabledCritical,
		ThresholdWarning:          rule.ThresholdWarning,
		ThresholdCritical:         rule.ThresholdCritical,
		ThresholdDurationWarning:  rule.ThresholdDurationWarning,
		ThresholdDurationCritical: rule.ThresholdDurationCritical,
	}
}

type alertRuleRequest struct {
	MetricsStorageID          *int64  `json:"metrics_storage_id"`
	Name                      *string `json:"name"`
	Query                     *string `json:"query"`
	Format                    *string `json:"format"`
	Template                  *string `json:"template"`
	EnabledWarning            *bool   `json:"enabled_warning"`
	EnabledCritical           *bool   `json:"enabled_critical"`
	ThresholdWarning          *string `json:"threshold_warning"`
	ThresholdCritical         *string `json:"threshold_critical"`
	ThresholdDurationWarning  *int64  `json:"threshold_duration_warning"`
	ThresholdDurationCritical *int64  `json:"threshold_duration_critical"`
}

func (req alertRuleRequest) applyTo(rule *AlertRule) {
	if req.MetricsStorageID != nil {
		rule.MetricsStorageID = req.MetricsStorageID
	}
	if req.Name != nil {
		rule.Name = *req.Name
	}
	if req.Query != nil {
		rule.Query = *req.Query
	}
	if req.Format != nil {
		rule.Format = *req.Format
	}
	if req.Template != nil {
		rule.Template = *req.Template
	}
	if req.EnabledWarning != nil {
		rule.EnabledWarning = *req.EnabledWarning
	}
	if req.EnabledCritical != nil {
		rule.EnabledCritical = *req.EnabledCritical
	}
	if req.ThresholdWarning != nil {
		rule.ThresholdWarning = req.ThresholdWarning
	}
	if req.ThresholdCritical != nil {
		rule.ThresholdCritical = req.ThresholdCritical
	}
	if req.ThresholdDurationWarning != nil {
		rule.ThresholdDurationWarning = *req.ThresholdDurationWarning
	}
	if req.ThresholdDurationCritical != nil {
		rule.ThresholdDurationCritical = *req.ThresholdDurationCritical
	}
}

func (s *Server) handleListAlertRules(w http.ResponseWriter, r *http.Request) {
	p, ok := s.alertProject(w, r)
	if !ok {
		return
	}
	out := []alertRuleJSON{}
	for _, rule := range s.store.alertRules.all() {
		if rule.ProjectID == p.ResourceID {
			out = append(out, alertRuleToJSON(rule))
		}
	}
	writePage(w, out)
}

func (s *Server) handleCreateAlertRule(w http.ResponseWriter, r *http.Request) {
	p, ok := s.alertProject(w, r)
	if !ok {
		return
	}
	var req alertRuleRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Query == nil || *req.Query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}
	rule := &AlertRule{
		UID:       newUUID(),
		ProjectID: p.ResourceID,
		Open:      true,
	}
	req.applyTo(rule)
	s.store.alertRules.set(rule.UID, rule)
	writeJSON(w, http.StatusCreated, alertRuleToJSON(rule))
}

func (s *Server) handleReadAlertRule(w http.ResponseWriter, r *http.Request) {
	rule, ok := s.store.alertRules.get(r.PathValue("uid"))
	if !ok {
		writeError(w, http.StatusNotFound, "No AlertRule matches the given query.")
		return
	}
	writeJSON(w, http.StatusOK, alertRuleToJSON(rule))
}

func (s *Server) handleUpdateAlertRule(w http.ResponseWriter, r *http.Request) {
	rule, ok := s.store.alertRules.get(r.PathValue("uid"))
	if !ok {
		writeError(w, http.StatusNotFound, "No AlertRule matches the given query.")
		return
	}
	var req alertRuleRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.applyTo(rule)
	writeJSON(w, http.StatusOK, alertRuleToJSON(rule))
}

func (s *Server) handleDeleteAlertRule(w http.ResponseWriter, r *http.Request) {
	if !s.store.alertRules.delete(r.PathValue("uid")) {
		writeError(w, http.StatusNotFound, "No AlertRule matches the given query.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ===== Log-measure rules =====

type logMeasureRuleJSON struct {
	ID             int64              `json:"id"`
	UID            string             `json:"uid"`
	ProjectID      *int64             `json:"project_id"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	LogStorage     logStorageJSON     `json:"log_storage"`
	MetricsStorage metricsStorageJSON `json:"metrics_storage"`
	Rule           json.RawMessage    `json:"rule"`
	CreatedAt      string             `json:"created_at"`
	UpdatedAt      string             `json:"updated_at"`
}

func (s *Server) logMeasureRuleToJSON(rule *LogMeasureRule) (logMeasureRuleJSON, bool) {
	var ls *LogStorage
	var ms *MetricsStorage
	if rule.LogStorageID != nil {
		ls, _ = s.store.findLogStorage(*rule.LogStorageID)
	}
	if rule.MetricsStorageID != nil {
		ms, _ = s.store.findMetricsStorage(*rule.MetricsStorageID)
	}
	if ls == nil || ms == nil {
		return logMeasureRuleJSON{}, false
	}
	pid := rule.ProjectID
	return logMeasureRuleJSON{
		ID:             rule.ID,
		UID:            rule.UID,
		ProjectID:      &pid,
		Name:           rule.Name,
		Description:    rule.Description,
		LogStorage:     s.logStorageToJSON(ls, false),
		MetricsStorage: s.metricsStorageToJSON(ms, false),
		Rule:           rule.Rule,
		CreatedAt:      formatTime(rule.CreatedAt),
		UpdatedAt:      formatTime(rule.UpdatedAt),
	}, true
}

type logMeasureRuleRequest struct {
	Name             *string         `json:"name"`
	Description      *string         `json:"description"`
	LogStorageID     *int64          `json:"log_storage_id"`
	MetricsStorageID *int64          `json:"metrics_storage_id"`
	Rule             json.RawMessage `json:"rule"`
}

func (s *Server) handleListLogMeasureRules(w http.ResponseWriter, r *http.Request) {
	p, ok := s.alertProject(w, r)
	if !ok {
		return
	}
	out := []logMeasureRuleJSON{}
	for _, rule := range s.store.logMeasureRules.all() {
		if rule.ProjectID != p.ResourceID {
			continue
		}
		if j, ok := s.logMeasureRuleToJSON(rule); ok {
			out = append(out, j)
		}
	}
	writePage(w, out)
}

func (s *Server) handleCreateLogMeasureRule(w http.ResponseWriter, r *http.Request) {
	p, ok := s.alertProject(w, r)
	if !ok {
		return
	}
	var req logMeasureRuleRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.LogStorageID == nil || req.MetricsStorageID == nil {
		writeError(w, http.StatusBadRequest, "log_storage_id and metrics_storage_id are required")
		return
	}
	if _, ok := s.store.findLogStorage(*req.LogStorageID); !ok {
		writeError(w, http.StatusBadRequest, "log_storage not found")
		return
	}
	if _, ok := s.store.findMetricsStorage(*req.MetricsStorageID); !ok {
		writeError(w, http.StatusBadRequest, "metrics_storage not found")
		return
	}
	now := time.Now()
	rule := &LogMeasureRule{
		UID:              newUUID(),
		ID:               s.store.nextInternalID(),
		ProjectID:        p.ResourceID,
		Name:             derefString(req.Name),
		Description:      derefString(req.Description),
		LogStorageID:     req.LogStorageID,
		MetricsStorageID: req.MetricsStorageID,
		Rule:             req.Rule,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	s.store.logMeasureRules.set(rule.UID, rule)
	j, _ := s.logMeasureRuleToJSON(rule)
	writeJSON(w, http.StatusCreated, j)
}

func (s *Server) handleReadLogMeasureRule(w http.ResponseWriter, r *http.Request) {
	rule, ok := s.store.logMeasureRules.get(r.PathValue("uid"))
	if !ok {
		writeError(w, http.StatusNotFound, "No LogMeasureRule matches the given query.")
		return
	}
	j, _ := s.logMeasureRuleToJSON(rule)
	writeJSON(w, http.StatusOK, j)
}

func (s *Server) handleUpdateLogMeasureRule(w http.ResponseWriter, r *http.Request) {
	rule, ok := s.store.logMeasureRules.get(r.PathValue("uid"))
	if !ok {
		writeError(w, http.StatusNotFound, "No LogMeasureRule matches the given query.")
		return
	}
	var req logMeasureRuleRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name != nil {
		rule.Name = *req.Name
	}
	if req.Description != nil {
		rule.Description = *req.Description
	}
	if req.LogStorageID != nil {
		rule.LogStorageID = req.LogStorageID
	}
	if req.MetricsStorageID != nil {
		rule.MetricsStorageID = req.MetricsStorageID
	}
	if len(req.Rule) > 0 {
		rule.Rule = req.Rule
	}
	rule.UpdatedAt = time.Now()
	j, _ := s.logMeasureRuleToJSON(rule)
	writeJSON(w, http.StatusOK, j)
}

func (s *Server) handleDeleteLogMeasureRule(w http.ResponseWriter, r *http.Request) {
	if !s.store.logMeasureRules.delete(r.PathValue("uid")) {
		writeError(w, http.StatusNotFound, "No LogMeasureRule matches the given query.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ===== Notification targets =====

type notificationTargetJSON struct {
	UID         string `json:"uid"`
	ProjectID   *int64 `json:"project_id"`
	ServiceType string `json:"service_type"`
	URL         string `json:"url"`
	Config      any    `json:"config"`
	Description string `json:"description"`
}

func notificationTargetToJSON(t *NotificationTarget) notificationTargetJSON {
	pid := t.ProjectID
	return notificationTargetJSON{
		UID:         t.UID,
		ProjectID:   &pid,
		ServiceType: t.ServiceType,
		URL:         t.URL,
		Config:      map[string]any{},
		Description: t.Description,
	}
}

type notificationTargetRequest struct {
	ServiceType *string `json:"service_type"`
	URL         *string `json:"url"`
	Description *string `json:"description"`
}

func (s *Server) handleListNotificationTargets(w http.ResponseWriter, r *http.Request) {
	p, ok := s.alertProject(w, r)
	if !ok {
		return
	}
	out := []notificationTargetJSON{}
	for _, t := range s.store.notificationTargets.all() {
		if t.ProjectID == p.ResourceID {
			out = append(out, notificationTargetToJSON(t))
		}
	}
	writePage(w, out)
}

func (s *Server) handleCreateNotificationTarget(w http.ResponseWriter, r *http.Request) {
	p, ok := s.alertProject(w, r)
	if !ok {
		return
	}
	var req notificationTargetRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ServiceType == nil || *req.ServiceType == "" {
		writeError(w, http.StatusBadRequest, "service_type is required")
		return
	}
	t := &NotificationTarget{
		UID:         newUUID(),
		ProjectID:   p.ResourceID,
		ServiceType: *req.ServiceType,
		URL:         derefString(req.URL),
		Description: derefString(req.Description),
	}
	s.store.notificationTargets.set(t.UID, t)
	writeJSON(w, http.StatusCreated, notificationTargetToJSON(t))
}

func (s *Server) handleReadNotificationTarget(w http.ResponseWriter, r *http.Request) {
	t, ok := s.store.notificationTargets.get(r.PathValue("uid"))
	if !ok {
		writeError(w, http.StatusNotFound, "No NotificationTarget matches the given query.")
		return
	}
	writeJSON(w, http.StatusOK, notificationTargetToJSON(t))
}

func (s *Server) handleUpdateNotificationTarget(w http.ResponseWriter, r *http.Request) {
	t, ok := s.store.notificationTargets.get(r.PathValue("uid"))
	if !ok {
		writeError(w, http.StatusNotFound, "No NotificationTarget matches the given query.")
		return
	}
	var req notificationTargetRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ServiceType != nil {
		t.ServiceType = *req.ServiceType
	}
	if req.URL != nil {
		t.URL = *req.URL
	}
	if req.Description != nil {
		t.Description = *req.Description
	}
	writeJSON(w, http.StatusOK, notificationTargetToJSON(t))
}

func (s *Server) handleDeleteNotificationTarget(w http.ResponseWriter, r *http.Request) {
	if !s.store.notificationTargets.delete(r.PathValue("uid")) {
		writeError(w, http.StatusNotFound, "No NotificationTarget matches the given query.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ===== Notification routings =====

type notificationRoutingJSON struct {
	UID                   string                 `json:"uid"`
	ProjectID             *int64                 `json:"project_id"`
	NotificationTarget    notificationTargetJSON `json:"notification_target"`
	MatchLabels           []MatchLabel           `json:"match_labels"`
	ResendIntervalMinutes int                    `json:"resend_interval_minutes,omitempty"`
	Order                 int                    `json:"order"`
}

func (s *Server) notificationRoutingToJSON(rt *NotificationRouting) (notificationRoutingJSON, bool) {
	t, ok := s.store.notificationTargets.get(rt.NotificationTargetUID)
	if !ok {
		return notificationRoutingJSON{}, false
	}
	pid := rt.ProjectID
	labels := rt.MatchLabels
	if labels == nil {
		labels = []MatchLabel{}
	}
	return notificationRoutingJSON{
		UID:                   rt.UID,
		ProjectID:             &pid,
		NotificationTarget:    notificationTargetToJSON(t),
		MatchLabels:           labels,
		ResendIntervalMinutes: rt.ResendIntervalMinutes,
		Order:                 rt.Order,
	}, true
}

type notificationRoutingRequest struct {
	NotificationTargetUID *string      `json:"notification_target_uid"`
	MatchLabels           []MatchLabel `json:"match_labels"`
	ResendIntervalMinutes *int         `json:"resend_interval_minutes"`
}

func (s *Server) handleListNotificationRoutings(w http.ResponseWriter, r *http.Request) {
	p, ok := s.alertProject(w, r)
	if !ok {
		return
	}
	out := []notificationRoutingJSON{}
	for _, rt := range s.store.notificationRoutings.all() {
		if rt.ProjectID != p.ResourceID {
			continue
		}
		if j, ok := s.notificationRoutingToJSON(rt); ok {
			out = append(out, j)
		}
	}
	writePage(w, out)
}

func (s *Server) handleCreateNotificationRouting(w http.ResponseWriter, r *http.Request) {
	p, ok := s.alertProject(w, r)
	if !ok {
		return
	}
	var req notificationRoutingRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.NotificationTargetUID == nil || *req.NotificationTargetUID == "" {
		writeError(w, http.StatusBadRequest, "notification_target_uid is required")
		return
	}
	if _, ok := s.store.notificationTargets.get(*req.NotificationTargetUID); !ok {
		writeError(w, http.StatusBadRequest, "notification_target not found")
		return
	}
	labels := req.MatchLabels
	if labels == nil {
		labels = []MatchLabel{}
	}
	rt := &NotificationRouting{
		UID:                   newUUID(),
		ProjectID:             p.ResourceID,
		NotificationTargetUID: *req.NotificationTargetUID,
		MatchLabels:           labels,
		Order:                 s.store.nextOrder(p.ResourceID),
	}
	if req.ResendIntervalMinutes != nil {
		rt.ResendIntervalMinutes = *req.ResendIntervalMinutes
	}
	s.store.notificationRoutings.set(rt.UID, rt)
	j, _ := s.notificationRoutingToJSON(rt)
	writeJSON(w, http.StatusCreated, j)
}

func (s *Server) handleReadNotificationRouting(w http.ResponseWriter, r *http.Request) {
	rt, ok := s.store.notificationRoutings.get(r.PathValue("uid"))
	if !ok {
		writeError(w, http.StatusNotFound, "No NotificationRouting matches the given query.")
		return
	}
	j, _ := s.notificationRoutingToJSON(rt)
	writeJSON(w, http.StatusOK, j)
}

func (s *Server) handleUpdateNotificationRouting(w http.ResponseWriter, r *http.Request) {
	rt, ok := s.store.notificationRoutings.get(r.PathValue("uid"))
	if !ok {
		writeError(w, http.StatusNotFound, "No NotificationRouting matches the given query.")
		return
	}
	var req notificationRoutingRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.NotificationTargetUID != nil {
		if _, ok := s.store.notificationTargets.get(*req.NotificationTargetUID); !ok {
			writeError(w, http.StatusBadRequest, "notification_target not found")
			return
		}
		rt.NotificationTargetUID = *req.NotificationTargetUID
	}
	if req.MatchLabels != nil {
		rt.MatchLabels = req.MatchLabels
	}
	if req.ResendIntervalMinutes != nil {
		rt.ResendIntervalMinutes = *req.ResendIntervalMinutes
	}
	j, _ := s.notificationRoutingToJSON(rt)
	writeJSON(w, http.StatusOK, j)
}

func (s *Server) handleDeleteNotificationRouting(w http.ResponseWriter, r *http.Request) {
	if !s.store.notificationRoutings.delete(r.PathValue("uid")) {
		writeError(w, http.StatusNotFound, "No NotificationRouting matches the given query.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type reorderItem struct {
	NotificationRoutingUID string `json:"notification_routing_uid"`
	Order                  int    `json:"order"`
}

func (s *Server) handleReorderNotificationRoutings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.alertProject(w, r); !ok {
		return
	}
	var items []reorderItem
	if err := readJSON(r, &items); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	for _, it := range items {
		if rt, ok := s.store.notificationRoutings.get(it.NotificationRoutingUID); ok {
			rt.Order = it.Order
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// ===== Histories (read-only; the mock records no alert history) =====

type historyJSON struct {
	UID       string  `json:"uid"`
	ProjectID int64   `json:"project_id"`
	RuleUID   string  `json:"rule_uid"`
	StartsAt  string  `json:"startsAt"`
	EndsAt    *string `json:"endsAt"`
	Open      bool    `json:"open"`
	Labels    string  `json:"labels"`
	Severity  string  `json:"severity"`
}

func (s *Server) handleListProjectHistories(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.alertProject(w, r); !ok {
		return
	}
	writePage(w, []historyJSON{})
}

func (s *Server) handleReadProjectHistory(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.alertProject(w, r); !ok {
		return
	}
	writeError(w, http.StatusNotFound, "No History matches the given query.")
}

func (s *Server) handleListRuleHistories(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.alertProject(w, r); !ok {
		return
	}
	writePage(w, []historyJSON{})
}

func (s *Server) handleReadRuleHistory(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.alertProject(w, r); !ok {
		return
	}
	writeError(w, http.StatusNotFound, "No History matches the given query.")
}
