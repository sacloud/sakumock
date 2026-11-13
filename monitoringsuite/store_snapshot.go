package monitoringsuite

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// storeSnapshot is the store's form in the state file (--state-dir). Every
// table is an array in insertion order, which is the order list endpoints
// return. The publishers are built in, not state, so they are not part of it.
type storeSnapshot struct {
	InternalID int64 `json:"internal_id"`

	AlertProjects     []*Project        `json:"alert_projects"`
	DashboardProjects []*Project        `json:"dashboard_projects"`
	LogStorages       []*LogStorage     `json:"log_storages"`
	MetricsStorages   []*MetricsStorage `json:"metrics_storages"`
	TraceStorages     []*TraceStorage   `json:"trace_storages"`

	AlertRules           []*AlertRule           `json:"alert_rules"`
	LogMeasureRules      []*LogMeasureRule      `json:"log_measure_rules"`
	NotificationTargets  []*NotificationTarget  `json:"notification_targets"`
	NotificationRoutings []*NotificationRouting `json:"notification_routings"`

	LogRoutings     []*Routing `json:"log_routings"`
	MetricsRoutings []*Routing `json:"metrics_routings"`

	LogKeys     []*AccessKey `json:"log_keys"`
	MetricsKeys []*AccessKey `json:"metrics_keys"`
	TraceKeys   []*AccessKey `json:"trace_keys"`

	Provisioning provisioningSnapshot `json:"provisioning"`
}

// provisioningSnapshot is the provisioning state of /management/provisioning/.
type provisioningSnapshot struct {
	Logs    ProvisioningExist `json:"logs"`
	Metrics ProvisioningExist `json:"metrics"`
}

// stage validates rows as the new content of t, keyed by key, and returns the
// function that puts them in place. Nothing changes until that function runs,
// so a snapshot with one bad table leaves the whole store untouched.
func stage[T any](t *table[T], name string, rows []*T, key func(*T) string) (func(), error) {
	items := make(map[string]*T, len(rows))
	order := make([]string, 0, len(rows))
	for i, row := range rows {
		if row == nil {
			return nil, fmt.Errorf("%s[%d]: record is missing", name, i)
		}
		k := key(row)
		if k == "" || k == "0" {
			return nil, fmt.Errorf("%s[%d]: record has no id", name, i)
		}
		if _, dup := items[k]; dup {
			return nil, fmt.Errorf("%s[%d]: duplicate id %s", name, i, k)
		}
		items[k] = row
		order = append(order, k)
	}
	return func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.items = items
		t.order = order
	}, nil
}

// Snapshot returns every resource of the control plane and the provisioning
// state as JSON, for core.StateFile.
func (s *MemoryStore) Snapshot() ([]byte, error) {
	snap := storeSnapshot{
		AlertProjects:        s.alertProjects.all(),
		DashboardProjects:    s.dashboardProjects.all(),
		LogStorages:          s.logStorages.all(),
		MetricsStorages:      s.metricsStorages.all(),
		TraceStorages:        s.traceStorages.all(),
		AlertRules:           s.alertRules.all(),
		LogMeasureRules:      s.logMeasureRules.all(),
		NotificationTargets:  s.notificationTargets.all(),
		NotificationRoutings: s.notificationRoutings.all(),
		LogRoutings:          s.logRoutings.all(),
		MetricsRoutings:      s.metricsRoutings.all(),
		LogKeys:              s.logKeys.all(),
		MetricsKeys:          s.metricsKeys.all(),
		TraceKeys:            s.traceKeys.all(),
	}
	s.mu.Lock()
	snap.InternalID = s.internalID
	s.mu.Unlock()
	s.provMu.Lock()
	snap.Provisioning = provisioningSnapshot{Logs: s.provLogs, Metrics: s.provMetrics}
	s.provMu.Unlock()
	return json.MarshalIndent(snap, "", "  ")
}

// Restore replaces the store's resources and provisioning state with a
// snapshot, and advances the ID generator past the restored resource IDs so
// new resources never reuse one.
func (s *MemoryStore) Restore(data []byte) error {
	var snap storeSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}

	// The small sequential ids never go back below one already handed out.
	internalID := snap.InternalID
	for _, r := range snap.LogMeasureRules {
		if r != nil {
			internalID = max(internalID, r.ID)
		}
	}
	for _, routings := range [][]*Routing{snap.LogRoutings, snap.MetricsRoutings} {
		for _, r := range routings {
			if r != nil {
				internalID = max(internalID, r.ID)
			}
		}
	}
	for _, keys := range [][]*AccessKey{snap.LogKeys, snap.MetricsKeys, snap.TraceKeys} {
		for _, k := range keys {
			if k != nil {
				internalID = max(internalID, k.ID)
			}
		}
	}
	// The state file is indented as a whole; the rule tree goes back to the
	// compact form it is echoed in.
	for _, r := range snap.LogMeasureRules {
		if r == nil || len(r.Rule) == 0 {
			continue
		}
		var buf bytes.Buffer
		if err := json.Compact(&buf, r.Rule); err != nil {
			return fmt.Errorf("log_measure_rules %s: rule: %w", r.UID, err)
		}
		r.Rule = buf.Bytes()
	}

	var (
		commits []func()
		errs    []error
	)
	add := func(commit func(), err error) {
		commits = append(commits, commit)
		errs = append(errs, err)
	}
	projectKey := func(p *Project) string { return idKey(p.ResourceID) }
	routingKey := func(r *Routing) string { return r.UID }
	accessKeyKey := func(k *AccessKey) string { return k.UID }
	add(stage(s.alertProjects, "alert_projects", snap.AlertProjects, projectKey))
	add(stage(s.dashboardProjects, "dashboard_projects", snap.DashboardProjects, projectKey))
	add(stage(s.logStorages, "log_storages", snap.LogStorages, func(st *LogStorage) string { return idKey(st.ResourceID) }))
	add(stage(s.metricsStorages, "metrics_storages", snap.MetricsStorages, func(st *MetricsStorage) string { return idKey(st.ResourceID) }))
	add(stage(s.traceStorages, "trace_storages", snap.TraceStorages, func(st *TraceStorage) string { return idKey(st.ResourceID) }))
	add(stage(s.alertRules, "alert_rules", snap.AlertRules, func(r *AlertRule) string { return r.UID }))
	add(stage(s.logMeasureRules, "log_measure_rules", snap.LogMeasureRules, func(r *LogMeasureRule) string { return r.UID }))
	add(stage(s.notificationTargets, "notification_targets", snap.NotificationTargets, func(t *NotificationTarget) string { return t.UID }))
	add(stage(s.notificationRoutings, "notification_routings", snap.NotificationRoutings, func(r *NotificationRouting) string { return r.UID }))
	add(stage(s.logRoutings, "log_routings", snap.LogRoutings, routingKey))
	add(stage(s.metricsRoutings, "metrics_routings", snap.MetricsRoutings, routingKey))
	add(stage(s.logKeys, "log_keys", snap.LogKeys, accessKeyKey))
	add(stage(s.metricsKeys, "metrics_keys", snap.MetricsKeys, accessKeyKey))
	add(stage(s.traceKeys, "trace_keys", snap.TraceKeys, accessKeyKey))
	if err := errors.Join(errs...); err != nil {
		return err
	}
	for _, commit := range commits {
		commit()
	}

	s.mu.Lock()
	s.internalID = internalID
	s.mu.Unlock()
	s.provMu.Lock()
	s.provLogs = snap.Provisioning.Logs
	s.provMetrics = snap.Provisioning.Metrics
	s.provMu.Unlock()

	for _, projects := range [][]*Project{snap.AlertProjects, snap.DashboardProjects} {
		for _, p := range projects {
			s.ids.Observe(idKey(p.ResourceID))
		}
	}
	for _, st := range snap.LogStorages {
		s.ids.Observe(idKey(st.ResourceID))
	}
	for _, st := range snap.MetricsStorages {
		s.ids.Observe(idKey(st.ResourceID))
	}
	for _, st := range snap.TraceStorages {
		s.ids.Observe(idKey(st.ResourceID))
	}
	return nil
}
