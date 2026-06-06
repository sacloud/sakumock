package monitoringsuite_test

import (
	"fmt"
	"net/url"
	"testing"

	mssdk "github.com/sacloud/sacloud-sdk-go/api/monitoring-suite"
	v1 "github.com/sacloud/sacloud-sdk-go/api/monitoring-suite/apis/v1"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	"github.com/sacloud/sakumock/monitoringsuite"
)

func ref[T any](v T) *T { return &v }

func optStr(o v1.OptString) string { v, _ := o.Get(); return v }

func newClient(t *testing.T, serverURL string) *v1.Client {
	t.Helper()
	var sa saclient.Client
	if err := sa.SetEnviron([]string{
		"SAKURA_ENDPOINTS_MONITORING_SUITE=" + serverURL,
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
	}); err != nil {
		t.Fatal(err)
	}
	client, err := mssdk.NewClient(&sa)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func newServer(t *testing.T) (*v1.Client, func()) {
	t.Helper()
	srv := monitoringsuite.NewTestServer(monitoringsuite.Config{})
	return newClient(t, srv.TestURL()), srv.Close
}

func ridOf(t *testing.T, opt v1.NilInt64) string {
	t.Helper()
	id, ok := opt.Get()
	if !ok {
		t.Fatal("resource_id not set")
	}
	return fmt.Sprintf("%d", id)
}

func TestAlertProjectLifecycle(t *testing.T) {
	client, closeFn := newServer(t)
	defer closeFn()
	ctx := t.Context()
	op := mssdk.NewAlertProjectOp(client)

	projects, err := op.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 0 {
		t.Fatalf("expected 0 projects, got %d", len(projects))
	}

	created, err := op.Create(ctx, mssdk.AlertProjectCreateParams{
		Name:        "test-project",
		Description: ref("desc"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if optStr(created.GetName()) != "test-project" {
		t.Fatalf("unexpected name: %s", optStr(created.GetName()))
	}
	id := ridOf(t, created.GetResourceID())

	read, err := op.Read(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if optStr(read.GetName()) != "test-project" {
		t.Fatalf("unexpected name on read: %s", optStr(read.GetName()))
	}

	updated, err := op.Update(ctx, id, mssdk.AlertProjectUpdateParams{Name: ref("renamed")})
	if err != nil {
		t.Fatal(err)
	}
	if optStr(updated.GetName()) != "renamed" {
		t.Fatalf("update failed: %s", optStr(updated.GetName()))
	}

	projects, err = op.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}

	if err := op.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := op.Read(ctx, id); err == nil {
		t.Fatal("expected error reading deleted project")
	}
}

func TestDashboardProjectLifecycle(t *testing.T) {
	client, closeFn := newServer(t)
	defer closeFn()
	ctx := t.Context()
	op := mssdk.NewDashboardOp(client)

	created, err := op.Create(ctx, mssdk.DashboardProjectCreateParams{Name: "dash"})
	if err != nil {
		t.Fatal(err)
	}
	id := ridOf(t, created.GetResourceID())

	if _, err := op.Read(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := op.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
}

func TestMetricsStorageLifecycleAndKeys(t *testing.T) {
	client, closeFn := newServer(t)
	defer closeFn()
	ctx := t.Context()
	op := mssdk.NewMetricsStorageOp(client)

	created, err := op.Create(ctx, mssdk.MetricsStorageCreateParams{Name: "metrics", IsSystem: false})
	if err != nil {
		t.Fatal(err)
	}
	id := ridOf(t, created.GetResourceID())

	if _, err := op.Read(ctx, id); err != nil {
		t.Fatal(err)
	}

	key, err := op.CreateKey(ctx, id, ref("my key"))
	if err != nil {
		t.Fatal(err)
	}
	keys, err := op.ListKeys(ctx, id, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
	read, err := op.ReadKey(ctx, id, key.GetUID())
	if err != nil {
		t.Fatal(err)
	}
	if read.GetUID() != key.GetUID() {
		t.Fatal("key uid mismatch")
	}
	if err := op.DeleteKey(ctx, id, key.GetUID()); err != nil {
		t.Fatal(err)
	}

	if err := op.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
}

func TestLogStorageLifecycle(t *testing.T) {
	client, closeFn := newServer(t)
	defer closeFn()
	ctx := t.Context()
	op := mssdk.NewLogsStorageOp(client)

	created, err := op.Create(ctx, mssdk.LogStorageCreateParams{
		Name:           "logs",
		IsSystem:       false,
		Classification: ref(v1.LogStorageCreateRequestClassificationShared),
	})
	if err != nil {
		t.Fatal(err)
	}
	id := ridOf(t, created.GetResourceID())

	expired, err := op.SetExpire(ctx, id, 7)
	if err != nil {
		t.Fatal(err)
	}
	if expired.GetExpireDay() != 7 {
		t.Fatalf("expire day not applied: %d", expired.GetExpireDay())
	}

	if _, err := op.ReadDailyStats(ctx, id, nil, nil); err != nil {
		t.Fatal(err)
	}

	key, err := op.CreateKey(ctx, id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := op.DeleteKey(ctx, id, key.GetUID()); err != nil {
		t.Fatal(err)
	}

	if err := op.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
}

func TestTraceStorageLifecycle(t *testing.T) {
	client, closeFn := newServer(t)
	defer closeFn()
	ctx := t.Context()
	op := mssdk.NewTracesStorageOp(client)

	created, err := op.Create(ctx, mssdk.TracesStorageCreateParams{
		Name:           "traces",
		Classification: ref(v1.TraceStorageCreateRequestClassificationShared),
	})
	if err != nil {
		t.Fatal(err)
	}
	id := fmt.Sprintf("%d", created.GetResourceID())

	if _, err := op.Read(ctx, id); err != nil {
		t.Fatal(err)
	}
	expired, err := op.SetExpire(ctx, id, 14)
	if err != nil {
		t.Fatal(err)
	}
	if expired.GetRetentionPeriodDays() != 14 {
		t.Fatalf("retention not applied: %d", expired.GetRetentionPeriodDays())
	}
	key, err := op.CreateKey(ctx, id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := op.DeleteKey(ctx, id, key.GetUID()); err != nil {
		t.Fatal(err)
	}
	if err := op.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
}

func TestPublishers(t *testing.T) {
	client, closeFn := newServer(t)
	defer closeFn()
	ctx := t.Context()
	op := mssdk.NewPublisherOp(client)

	publishers, err := op.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(publishers) == 0 {
		t.Fatal("expected seeded publishers")
	}
	code := publishers[0].GetCode()
	read, err := op.Read(ctx, code)
	if err != nil {
		t.Fatal(err)
	}
	if read.GetCode() != code {
		t.Fatalf("publisher code mismatch: %s != %s", read.GetCode(), code)
	}
	if _, err := op.Read(ctx, "nonexistent"); err == nil {
		t.Fatal("expected 404 for unknown publisher")
	}
}

func TestManagement(t *testing.T) {
	client, closeFn := newServer(t)
	defer closeFn()
	ctx := t.Context()
	op := mssdk.NewManagementOp(client)

	if _, err := op.ResourceLimits(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := op.CreateProvisioning(ctx, mssdk.ProvisioningCreateParam{}); err != nil {
		t.Fatal(err)
	}
	prov, err := op.ReadProvisioning(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !prov.GetLogs().SystemExist {
		t.Fatal("expected logs provisioned after initialize")
	}
}

func TestLogRoutingLifecycle(t *testing.T) {
	client, closeFn := newServer(t)
	defer closeFn()
	ctx := t.Context()

	storageOp := mssdk.NewLogsStorageOp(client)
	storage, err := storageOp.Create(ctx, mssdk.LogStorageCreateParams{Name: "logs", IsSystem: false})
	if err != nil {
		t.Fatal(err)
	}
	sid := fmt.Sprintf("%d", storage.GetID())

	pubOp := mssdk.NewPublisherOp(client)
	publishers, err := pubOp.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var code, variant string
	for _, p := range publishers {
		for _, v := range p.GetVariants() {
			if v.GetStorage() == v1.PublisherVariantStorageLogs {
				code, variant = p.GetCode(), v.GetName()
			}
		}
	}
	if code == "" {
		t.Fatal("no logs publisher variant found")
	}

	op := mssdk.NewLogRoutingOp(client)
	created, err := op.Create(ctx, mssdk.LogsRoutingCreateParams{
		PublisherCode: code,
		Variant:       variant,
		LogStorageID:  sid,
	})
	if err != nil {
		t.Fatal(err)
	}
	uid := created.GetUID()

	read, err := op.Read(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if read.GetVariant() != variant {
		t.Fatalf("variant mismatch: %s", read.GetVariant())
	}

	routings, err := op.List(ctx, mssdk.LogsRoutingsListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(routings) != 1 {
		t.Fatalf("expected 1 routing, got %d", len(routings))
	}

	if err := op.Delete(ctx, uid); err != nil {
		t.Fatal(err)
	}
}

func TestMetricsRoutingLifecycle(t *testing.T) {
	client, closeFn := newServer(t)
	defer closeFn()
	ctx := t.Context()

	storageOp := mssdk.NewMetricsStorageOp(client)
	storage, err := storageOp.Create(ctx, mssdk.MetricsStorageCreateParams{Name: "metrics"})
	if err != nil {
		t.Fatal(err)
	}
	sid := fmt.Sprintf("%d", storage.GetID())

	pubOp := mssdk.NewPublisherOp(client)
	publishers, _ := pubOp.List(ctx, nil, nil)
	var code, variant string
	for _, p := range publishers {
		for _, v := range p.GetVariants() {
			if v.GetStorage() == v1.PublisherVariantStorageMetrics {
				code, variant = p.GetCode(), v.GetName()
			}
		}
	}
	if code == "" {
		t.Fatal("no metrics publisher variant found")
	}

	op := mssdk.NewMetricsRoutingOp(client)
	created, err := op.Create(ctx, mssdk.MetricsRoutingCreateParams{
		PublisherCode:    code,
		Variant:          variant,
		MetricsStorageID: sid,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := op.Delete(ctx, created.GetUID()); err != nil {
		t.Fatal(err)
	}
}

func TestNotificationTargetAndRouting(t *testing.T) {
	client, closeFn := newServer(t)
	defer closeFn()
	ctx := t.Context()

	projOp := mssdk.NewAlertProjectOp(client)
	project, err := projOp.Create(ctx, mssdk.AlertProjectCreateParams{Name: "proj"})
	if err != nil {
		t.Fatal(err)
	}
	pid := ridOf(t, project.GetResourceID())

	targetOp := mssdk.NewNotificationTargetOp(client)
	u, _ := url.Parse("https://example.com/webhook")
	target, err := targetOp.Create(ctx, pid, mssdk.NotificationTargetCreateParams{
		ServiceType: v1.NotificationTargetServiceTypeSAKURASIMPLENOTICE,
		URL:         u,
	})
	if err != nil {
		t.Fatal(err)
	}

	routingOp := mssdk.NewNotificationRoutingOp(client)
	routing, err := routingOp.Create(ctx, pid, mssdk.NotificationRoutingCreateParams{
		NotificationTargetUID: target.GetUID(),
		MatchLabels:           []v1.MatchLabelsItem{{Name: "severity", Value: "critical"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	nt := routing.GetNotificationTarget()
	if nt.GetUID() != target.GetUID() {
		t.Fatal("routing target mismatch")
	}

	routings, err := routingOp.List(ctx, pid, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(routings) != 1 {
		t.Fatalf("expected 1 routing, got %d", len(routings))
	}

	if err := routingOp.Delete(ctx, pid, routing.GetUID()); err != nil {
		t.Fatal(err)
	}
	if err := targetOp.Delete(ctx, pid, target.GetUID()); err != nil {
		t.Fatal(err)
	}
}

func TestAlertRuleLifecycle(t *testing.T) {
	client, closeFn := newServer(t)
	defer closeFn()
	ctx := t.Context()

	storageOp := mssdk.NewMetricsStorageOp(client)
	storage, err := storageOp.Create(ctx, mssdk.MetricsStorageCreateParams{Name: "metrics"})
	if err != nil {
		t.Fatal(err)
	}
	sid := ridOf(t, storage.GetResourceID())

	projOp := mssdk.NewAlertProjectOp(client)
	project, err := projOp.Create(ctx, mssdk.AlertProjectCreateParams{Name: "proj"})
	if err != nil {
		t.Fatal(err)
	}
	pid := ridOf(t, project.GetResourceID())

	op := mssdk.NewAlertRuleOp(client)
	created, err := op.Create(ctx, pid, mssdk.AlertRuleCreateParams{
		MetricsStorageID: sid,
		Query:            "up == 0",
		Name:             ref("rule"),
	})
	if err != nil {
		t.Fatal(err)
	}
	uid := created.GetUID()

	if _, err := op.Read(ctx, pid, uid); err != nil {
		t.Fatal(err)
	}
	rules, err := op.List(ctx, pid, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if err := op.Delete(ctx, pid, uid); err != nil {
		t.Fatal(err)
	}
}

func TestLogMeasureRuleLifecycle(t *testing.T) {
	client, closeFn := newServer(t)
	defer closeFn()
	ctx := t.Context()

	logOp := mssdk.NewLogsStorageOp(client)
	logStorage, err := logOp.Create(ctx, mssdk.LogStorageCreateParams{Name: "logs", IsSystem: false})
	if err != nil {
		t.Fatal(err)
	}
	metricsOp := mssdk.NewMetricsStorageOp(client)
	metricsStorage, err := metricsOp.Create(ctx, mssdk.MetricsStorageCreateParams{Name: "metrics"})
	if err != nil {
		t.Fatal(err)
	}

	projOp := mssdk.NewAlertProjectOp(client)
	project, err := projOp.Create(ctx, mssdk.AlertProjectCreateParams{Name: "proj"})
	if err != nil {
		t.Fatal(err)
	}
	pid := ridOf(t, project.GetResourceID())

	op := mssdk.NewLogMeasureRuleOp(client)
	created, err := op.Create(ctx, pid, mssdk.LogMeasureRuleCreateParams{
		LogStorageID:     ridOf(t, logStorage.GetResourceID()),
		MetricsStorageID: ridOf(t, metricsStorage.GetResourceID()),
		Name:             ref("measure"),
		Rule: v1.LogMeasureRuleModel{
			Version: v1.LogMeasureRuleVersionEnumV1,
			Query:   v1.LogMeasureRuleV1{Matchers: []v1.FieldMatcher{}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := op.Delete(ctx, pid, created.GetUID()); err != nil {
		t.Fatal(err)
	}
}
