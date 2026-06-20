package monitoringsuite

import (
	"strconv"
	"sync"

	"github.com/google/uuid"
	"github.com/sacloud/sakumock/core"
)

// table is a generic insertion-ordered, concurrency-safe collection keyed by a
// string (a numeric resource ID or a UUID, rendered as a string).
type table[T any] struct {
	mu    sync.RWMutex
	items map[string]*T
	order []string
}

func newTable[T any]() *table[T] {
	return &table[T]{items: make(map[string]*T)}
}

func (t *table[T]) get(key string) (*T, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	v, ok := t.items[key]
	return v, ok
}

// all returns every record in insertion order.
func (t *table[T]) all() []*T {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]*T, 0, len(t.order))
	for _, k := range t.order {
		out = append(out, t.items[k])
	}
	return out
}

func (t *table[T]) set(key string, v *T) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.items[key]; !ok {
		t.order = append(t.order, key)
	}
	t.items[key] = v
}

func (t *table[T]) delete(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.items[key]; !ok {
		return false
	}
	delete(t.items, key)
	for i, k := range t.order {
		if k == key {
			t.order = append(t.order[:i], t.order[i+1:]...)
			break
		}
	}
	return true
}

// MemoryStore is an in-memory backend for the monitoring-suite mock.
type MemoryStore struct {
	ids *core.IDGenerator // resource IDs (12-digit), shared in the unified binary

	mu         sync.Mutex
	internalID int64 // small sequential "id" field, distinct from resource_id

	alertProjects     *table[Project]
	dashboardProjects *table[Project]
	logStorages       *table[LogStorage]
	metricsStorages   *table[MetricsStorage]
	traceStorages     *table[TraceStorage]

	alertRules           *table[AlertRule]
	logMeasureRules      *table[LogMeasureRule]
	notificationTargets  *table[NotificationTarget]
	notificationRoutings *table[NotificationRouting]

	logRoutings     *table[Routing]
	metricsRoutings *table[Routing]

	logKeys     *table[AccessKey]
	metricsKeys *table[AccessKey]
	traceKeys   *table[AccessKey]

	publishers []Publisher

	// provisioning state returned by /management/provisioning/.
	provMu      sync.Mutex
	provLogs    ProvisioningExist
	provMetrics ProvisioningExist
}

// ProvisioningExist tracks whether the system/user storages have been
// provisioned for a telemetry kind.
type ProvisioningExist struct {
	SystemExist bool
	UserExist   bool
}

// NewMemoryStore creates an empty store seeded with the read-only publishers.
func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{
		ids:                  core.NewIDGenerator(core.DefaultIDBase()),
		alertProjects:        newTable[Project](),
		dashboardProjects:    newTable[Project](),
		logStorages:          newTable[LogStorage](),
		metricsStorages:      newTable[MetricsStorage](),
		traceStorages:        newTable[TraceStorage](),
		alertRules:           newTable[AlertRule](),
		logMeasureRules:      newTable[LogMeasureRule](),
		notificationTargets:  newTable[NotificationTarget](),
		notificationRoutings: newTable[NotificationRouting](),
		logRoutings:          newTable[Routing](),
		metricsRoutings:      newTable[Routing](),
		logKeys:              newTable[AccessKey](),
		metricsKeys:          newTable[AccessKey](),
		traceKeys:            newTable[AccessKey](),
		publishers:           seedPublishers(),
	}
	return s
}

// nextResourceID returns the next 12-digit resource ID as an int64.
func (s *MemoryStore) nextResourceID() int64 {
	n, _ := strconv.ParseInt(s.ids.Next(), 10, 64)
	return n
}

// nextInternalID returns the next small sequential "id" value.
func (s *MemoryStore) nextInternalID() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.internalID++
	return s.internalID
}

func newUUID() string { return uuid.NewString() }

// dummyAccountID is the placeholder member ID stamped on every resource.
const dummyAccountID = "123456789012"

// seedPublishers returns the fixed set of publishers the mock exposes. The
// codes and variants mirror the real API closely enough for routing creation.
func seedPublishers() []Publisher {
	return []Publisher{
		{
			Code:        "appliance",
			Description: "SAKURA Cloud appliance telemetry",
			Variants: []PublisherVariant{
				{Name: "metrics", Label: "Metrics", Storage: "metrics", System: "disallow"},
				{Name: "logs", Label: "Logs", Storage: "logs", System: "disallow"},
			},
		},
		{
			Code:        "otel",
			Description: "OpenTelemetry collector",
			Variants: []PublisherVariant{
				{Name: "metrics", Label: "Metrics", Storage: "metrics", System: "disallow"},
				{Name: "logs", Label: "Logs", Storage: "logs", System: "disallow"},
			},
		},
	}
}

// publisher returns the seeded publisher with the given code.
func (s *MemoryStore) publisher(code string) (Publisher, bool) {
	for _, p := range s.publishers {
		if p.Code == code {
			return p, true
		}
	}
	return Publisher{}, false
}

// publisherHasVariant reports whether the publisher exposes the named variant.
func (p Publisher) hasVariant(name string) bool {
	for _, v := range p.Variants {
		if v.Name == name {
			return true
		}
	}
	return false
}

// findLogStorage resolves a storage by its resource_id or its id, since routing
// references use the id field while URL paths use the resource_id.
func (s *MemoryStore) findLogStorage(idOrResourceID int64) (*LogStorage, bool) {
	for _, st := range s.logStorages.all() {
		if st.ID == idOrResourceID || st.ResourceID == idOrResourceID {
			return st, true
		}
	}
	return nil, false
}

func (s *MemoryStore) findMetricsStorage(idOrResourceID int64) (*MetricsStorage, bool) {
	for _, st := range s.metricsStorages.all() {
		if st.ID == idOrResourceID || st.ResourceID == idOrResourceID {
			return st, true
		}
	}
	return nil, false
}

// nextOrder returns the next notification-routing order within a project.
func (s *MemoryStore) nextOrder(projectID int64) int {
	max := 0
	for _, r := range s.notificationRoutings.all() {
		if r.ProjectID == projectID && r.Order > max {
			max = r.Order
		}
	}
	return max + 1
}

// Close releases resources. The in-memory store has nothing to release.
func (s *MemoryStore) Close() error { return nil }
