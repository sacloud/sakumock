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
	ID          int64
	ResourceID  int64
	Name        string
	Description string
	Tags        []string
	AccountID   string
	CreatedAt   time.Time
}

// LogStorage is a logs storage container (/logs/storages/).
type LogStorage struct {
	ID                 int64
	ResourceID         int64
	Name               string
	Description        string
	Tags               []string
	AccountID          string
	CreatedAt          time.Time
	ExpireDay          int
	IsSystem           bool
	Classification     string // "shared" or "dedicated"
	KMSKeyID           *int64
	ServicePrincipalID *int64
}

// MetricsStorage is a metrics storage container (/metrics/storages/).
type MetricsStorage struct {
	ID          int64
	ResourceID  int64
	Name        string
	Description string
	Tags        []string
	AccountID   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	IsSystem    bool
}

// TraceStorage is a traces storage container (/traces/storages/).
type TraceStorage struct {
	ID                  int64
	ResourceID          int64
	Name                string
	Description         string
	Tags                []string
	AccountID           string
	CreatedAt           time.Time
	RetentionPeriodDays int
	Classification      string
	KMSKeyID            *int64
	ServicePrincipalID  *int64
}

// AccessKey is an access key issued for a log/metrics/trace storage. ParentKey
// is the resource-ID string of the owning storage, used to scope list results.
type AccessKey struct {
	ID          int64
	UID         string
	Secret      string
	Token       string
	Description string
	ParentKey   string
}

// AlertRule is an alerting rule under an alert project.
type AlertRule struct {
	UID                       string
	ProjectID                 int64
	MetricsStorageID          *int64
	Name                      string
	Query                     string
	Format                    string
	Template                  string
	Open                      bool
	EnabledWarning            bool
	EnabledCritical           bool
	ThresholdWarning          *string
	ThresholdCritical         *string
	ThresholdDurationWarning  int64
	ThresholdDurationCritical int64
}

// LogMeasureRule is a log-measure rule under an alert project. The polymorphic
// matcher tree in Rule is stored verbatim and echoed back.
type LogMeasureRule struct {
	UID              string
	ID               int64
	ProjectID        int64
	Name             string
	Description      string
	LogStorageID     *int64
	MetricsStorageID *int64
	Rule             json.RawMessage
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// NotificationTarget is a notification target under an alert project.
type NotificationTarget struct {
	UID         string
	ProjectID   int64
	ServiceType string // "SAKURA_SIMPLE_NOTICE" or "SAKURA_EVENT_BUS"
	URL         string
	Description string
}

// MatchLabel is a single name/value matcher of a notification routing.
type MatchLabel struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// NotificationRouting routes alerts to a notification target under a project.
type NotificationRouting struct {
	UID                   string
	ProjectID             int64
	NotificationTargetUID string
	MatchLabels           []MatchLabel
	ResendIntervalMinutes int
	Order                 int
}

// Routing is a log or metrics routing (/logs/routings/, /metrics/routings/).
type Routing struct {
	ID            int64
	UID           string
	ResourceID    *int64
	PublisherCode string
	Variant       string
	StorageID     int64 // references a LogStorage or MetricsStorage by ID
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// PublisherVariant is one variant a publisher exposes.
type PublisherVariant struct {
	Name          string  `json:"name"`
	Label         string  `json:"label"`
	Storage       string  `json:"storage"` // "metrics" or "logs"
	System        string  `json:"system"`  // "disallow" or "required"
	MetricsPrefix *string `json:"metrics_prefix"`
}

// Publisher is a read-only telemetry publisher (/publishers/).
type Publisher struct {
	Code        string
	Description string
	Variants    []PublisherVariant
}
