package monitoringsuite

import (
	"encoding/json"
	"time"
)

// The monitoring-suite control-plane API exposes many resource types. Top-level
// resources (alert/dashboard projects, log/metrics/trace storages) are
// addressed by a numeric resource ID minted from the shared core.IDGenerator;
// sub-resources (rules, routings, notification targets/routings, access keys)
// are addressed by a UUID. Data-plane ingestion/query (sending and reading
// logs, metrics and traces) is out of scope for this mock.

// Project is an alert or dashboard project. Both share an identical shape, so a
// single record type backs the /alerts/projects/ and /dashboards/projects/
// collections.
type Project struct {
	ID          int64     `json:"id"`
	ResourceID  int64     `json:"resource_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	AccountID   string    `json:"account_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// LogStorage is a logs storage container (/logs/storages/).
type LogStorage struct {
	ID                 int64     `json:"id"`
	ResourceID         int64     `json:"resource_id"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	Tags               []string  `json:"tags"`
	AccountID          string    `json:"account_id"`
	CreatedAt          time.Time `json:"created_at"`
	ExpireDay          int       `json:"expire_day"`
	IsSystem           bool      `json:"is_system"`
	Classification     string    `json:"classification"` // "shared" or "dedicated"
	KMSKeyID           *int64    `json:"kms_key_id"`
	ServicePrincipalID *int64    `json:"service_principal_id"`
}

// MetricsStorage is a metrics storage container (/metrics/storages/).
type MetricsStorage struct {
	ID          int64     `json:"id"`
	ResourceID  int64     `json:"resource_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	AccountID   string    `json:"account_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	IsSystem    bool      `json:"is_system"`
}

// TraceStorage is a traces storage container (/traces/storages/).
type TraceStorage struct {
	ID                  int64     `json:"id"`
	ResourceID          int64     `json:"resource_id"`
	Name                string    `json:"name"`
	Description         string    `json:"description"`
	Tags                []string  `json:"tags"`
	AccountID           string    `json:"account_id"`
	CreatedAt           time.Time `json:"created_at"`
	RetentionPeriodDays int       `json:"retention_period_days"`
	Classification      string    `json:"classification"`
	KMSKeyID            *int64    `json:"kms_key_id"`
	ServicePrincipalID  *int64    `json:"service_principal_id"`
}

// AccessKey is an access key issued for a log/metrics/trace storage. ParentKey
// is the resource-ID string of the owning storage, used to scope list results.
type AccessKey struct {
	ID          int64  `json:"id"`
	UID         string `json:"uid"`
	Secret      string `json:"secret"`
	Token       string `json:"token"`
	Description string `json:"description"`
	ParentKey   string `json:"parent_key"`
}

// AlertRule is an alerting rule under an alert project.
type AlertRule struct {
	UID                       string  `json:"uid"`
	ProjectID                 int64   `json:"project_id"`
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

// LogMeasureRule is a log-measure rule under an alert project. The polymorphic
// matcher tree in Rule is stored verbatim and echoed back.
type LogMeasureRule struct {
	UID              string          `json:"uid"`
	ID               int64           `json:"id"`
	ProjectID        int64           `json:"project_id"`
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	LogStorageID     *int64          `json:"log_storage_id"`
	MetricsStorageID *int64          `json:"metrics_storage_id"`
	Rule             json.RawMessage `json:"rule"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

// NotificationTarget is where an alert project delivers its notifications.
type NotificationTarget struct {
	UID         string `json:"uid"`
	ProjectID   int64  `json:"project_id"`
	ServiceType string `json:"service_type"` // "SAKURA_SIMPLE_NOTICE" or "SAKURA_EVENT_BUS"
	URL         string `json:"url"`
	Description string `json:"description"`
}

// MatchLabel is a single name/value matcher of a notification routing.
type MatchLabel struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// NotificationRouting routes an alert project's alerts to a target.
type NotificationRouting struct {
	UID                   string       `json:"uid"`
	ProjectID             int64        `json:"project_id"`
	NotificationTargetUID string       `json:"notification_target_uid"`
	MatchLabels           []MatchLabel `json:"match_labels"`
	ResendIntervalMinutes int          `json:"resend_interval_minutes"`
	Order                 int          `json:"order"`
}

// Routing is a log or metrics routing (/logs/routings/, /metrics/routings/).
type Routing struct {
	ID            int64     `json:"id"`
	UID           string    `json:"uid"`
	ResourceID    *int64    `json:"resource_id"`
	PublisherCode string    `json:"publisher_code"`
	Variant       string    `json:"variant"`
	StorageID     int64     `json:"storage_id"` // references a LogStorage or MetricsStorage by ID
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// PublisherVariant is one telemetry variant a publisher exposes.
type PublisherVariant struct {
	Name          string  `json:"name"`
	Label         string  `json:"label"`
	Storage       string  `json:"storage"` // "metrics" or "logs"
	System        string  `json:"system"`  // "disallow" or "required"
	MetricsPrefix *string `json:"metrics_prefix"`
}

// Publisher is a read-only telemetry publisher (/publishers/).
type Publisher struct {
	Code        string             `json:"code"`
	Description string             `json:"description"`
	Variants    []PublisherVariant `json:"variants"`
}
