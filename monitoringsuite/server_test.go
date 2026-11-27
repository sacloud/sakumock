package monitoringsuite_test

import (
	"fmt"
	"net/url"
	"strconv"
	"testing"

	mssdk "github.com/sacloud/sacloud-sdk-go/api/monitoring-suite"
	v1 "github.com/sacloud/sacloud-sdk-go/api/monitoring-suite/apis/v1"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	"github.com/sacloud/sakumock/monitoringsuite"
)

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
	closeFn := func() {
		// The handlers exercised by the test must not have drifted from the
		// OpenAPI spec.
		if v := srv.SpecViolations(); len(v) != 0 {
			t.Errorf("spec violations recorded: %+v", v)
		}
		srv.Close()
	}
	return newClient(t, srv.TestURL()), closeFn
}

func ridOf(t *testing.T, opt v1.NilInt64) string {
	t.Helper()
	id, ok := opt.Get()
	if !ok {
		t.Fatal("resource_id not set")
	}
	return fmt.Sprintf("%d", id)
}

func mustInt64(t *testing.T, s string) int64 {
	t.Helper()
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// The SDK's Update ops send PATCH for resources and PUT for access keys, so
// the tests reach the other verb of each pair through the generated client.

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
		Description: new("desc"),
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

	updated, err := op.Update(ctx, id, mssdk.AlertProjectUpdateParams{Name: new("renamed")})
	if err != nil {
		t.Fatal(err)
	}
	if optStr(updated.GetName()) != "renamed" {
		t.Fatalf("update failed: %s", optStr(updated.GetName()))
	}

	put, err := client.UpdateAlertProject(ctx, v1.NewOptAlertProjectRequest(v1.AlertProjectRequest{
		Name:        v1.NewOptString("replaced"),
		Description: v1.NewOptString("replaced desc"),
	}), v1.UpdateAlertProjectParams{ResourceId: mustInt64(t, id)})
	if err != nil {
		t.Fatal(err)
	}
	if optStr(put.GetName()) != "replaced" || optStr(put.GetDescription()) != "replaced desc" {
		t.Fatalf("put failed: %+v", put)
	}

	projects, err = op.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}

	// The mock records no alert history, so the list is always empty.
	histories, err := op.ListHistories(ctx, mssdk.AlertsProjectsHistoriesListParams{ProjectID: id})
	if err != nil {
		t.Fatal(err)
	}
	if len(histories) != 0 {
		t.Fatalf("expected no histories, got %d", len(histories))
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

	projects, err := op.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || optStr(projects[0].GetName()) != "dash" {
		t.Fatalf("unexpected list: %+v", projects)
	}

	updated, err := op.Update(ctx, id, mssdk.DashboardProjectUpdateParams{Name: new("renamed")})
	if err != nil {
		t.Fatal(err)
	}
	if optStr(updated.GetName()) != "renamed" {
		t.Fatalf("update failed: %s", optStr(updated.GetName()))
	}
	put, err := client.UpdateDashboardProject(ctx, v1.NewOptDashboardProjectRequest(v1.DashboardProjectRequest{
		Name:        v1.NewOptString("replaced"),
		Description: v1.NewOptString("replaced desc"),
	}), v1.UpdateDashboardProjectParams{ResourceId: mustInt64(t, id)})
	if err != nil {
		t.Fatal(err)
	}
	if optStr(put.GetName()) != "replaced" || optStr(put.GetDescription()) != "replaced desc" {
		t.Fatalf("put failed: %+v", put)
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

	storages, err := op.List(ctx, mssdk.MetricsStorageListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(storages) != 1 || optStr(storages[0].GetName()) != "metrics" {
		t.Fatalf("unexpected list: %+v", storages)
	}

	updated, err := op.Update(ctx, id, mssdk.MetricsStorageUpdateParams{Name: new("renamed")})
	if err != nil {
		t.Fatal(err)
	}
	if optStr(updated.GetName()) != "renamed" {
		t.Fatalf("update failed: %s", optStr(updated.GetName()))
	}
	put, err := client.UpdateMetricStorage(ctx, v1.NewOptMetricsStorageRequest(v1.MetricsStorageRequest{
		Name:        v1.NewOptString("replaced"),
		Description: v1.NewOptString("replaced desc"),
	}), v1.UpdateMetricStorageParams{ResourceId: mustInt64(t, id)})
	if err != nil {
		t.Fatal(err)
	}
	if optStr(put.GetName()) != "replaced" || optStr(put.GetDescription()) != "replaced desc" {
		t.Fatalf("put failed: %+v", put)
	}

	// The mock holds no telemetry, so usage stats are always empty.
	daily, err := op.ReadDailyStats(ctx, id, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(daily) != 0 {
		t.Fatalf("expected no daily usage, got %d", len(daily))
	}
	monthly, err := op.ReadMonthlyStats(ctx, id, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if len(monthly) != 0 {
		t.Fatalf("expected no monthly usage, got %d", len(monthly))
	}

	key, err := op.CreateKey(ctx, id, new("my key"))
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
	putKey, err := op.UpdateKey(ctx, id, key.GetUID(), new("put key"))
	if err != nil {
		t.Fatal(err)
	}
	if optStr(putKey.GetDescription()) != "put key" {
		t.Fatalf("key put failed: %s", optStr(putKey.GetDescription()))
	}
	patchedKey, err := client.PatchMetricStorageKey(ctx, v1.NewOptPatchedMetricsStorageAccessKeyRequest(v1.PatchedMetricsStorageAccessKeyRequest{
		Description: v1.NewOptString("patched key"),
	}), v1.PatchMetricStorageKeyParams{MetricsResourceId: mustInt64(t, id), UID: key.GetUID()})
	if err != nil {
		t.Fatal(err)
	}
	if optStr(patchedKey.GetDescription()) != "patched key" {
		t.Fatalf("key patch failed: %s", optStr(patchedKey.GetDescription()))
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
		Classification: new(v1.LogStorageCreateRequestClassificationShared),
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

	storages, err := op.List(ctx, mssdk.LogsStoragesListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(storages) != 1 || optStr(storages[0].GetName()) != "logs" {
		t.Fatalf("unexpected list: %+v", storages)
	}

	updated, err := op.Update(ctx, id, mssdk.LogStorageUpdateParams{Name: new("renamed")})
	if err != nil {
		t.Fatal(err)
	}
	if optStr(updated.GetName()) != "renamed" {
		t.Fatalf("update failed: %s", optStr(updated.GetName()))
	}
	put, err := client.UpdateLogStorage(ctx, v1.NewOptLogStorageRequest(v1.LogStorageRequest{
		Name:        v1.NewOptString("replaced"),
		Description: v1.NewOptString("replaced desc"),
	}), v1.UpdateLogStorageParams{ResourceId: mustInt64(t, id)})
	if err != nil {
		t.Fatal(err)
	}
	if optStr(put.GetName()) != "replaced" || optStr(put.GetDescription()) != "replaced desc" {
		t.Fatalf("put failed: %+v", put)
	}

	// The mock holds no telemetry, so usage stats are always empty.
	daily, err := op.ReadDailyStats(ctx, id, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(daily) != 0 {
		t.Fatalf("expected no daily usage, got %d", len(daily))
	}
	monthly, err := op.ReadMonthlyStats(ctx, id, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if len(monthly) != 0 {
		t.Fatalf("expected no monthly usage, got %d", len(monthly))
	}

	key, err := op.CreateKey(ctx, id, nil)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := op.ListKeys(ctx, id, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0].GetUID() != key.GetUID() {
		t.Fatalf("unexpected key list: %+v", keys)
	}
	putKey, err := op.UpdateKey(ctx, id, key.GetUID(), new("put key"))
	if err != nil {
		t.Fatal(err)
	}
	if optStr(putKey.GetDescription()) != "put key" {
		t.Fatalf("key put failed: %s", optStr(putKey.GetDescription()))
	}
	patchedKey, err := client.PatchLogStorageKey(ctx, v1.NewOptPatchedLogStorageAccessKeyRequest(v1.PatchedLogStorageAccessKeyRequest{
		Description: v1.NewOptString("patched key"),
	}), v1.PatchLogStorageKeyParams{LogResourceId: mustInt64(t, id), UID: key.GetUID()})
	if err != nil {
		t.Fatal(err)
	}
	if optStr(patchedKey.GetDescription()) != "patched key" {
		t.Fatalf("key patch failed: %s", optStr(patchedKey.GetDescription()))
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
		Classification: new(v1.TraceStorageCreateRequestClassificationShared),
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

	storages, err := op.List(ctx, mssdk.TracesStorageListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(storages) != 1 || optStr(storages[0].GetName()) != "traces" {
		t.Fatalf("unexpected list: %+v", storages)
	}

	updated, err := op.Update(ctx, id, mssdk.TracesStorageUpdateParams{Name: new("renamed")})
	if err != nil {
		t.Fatal(err)
	}
	if optStr(updated.GetName()) != "renamed" {
		t.Fatalf("update failed: %s", optStr(updated.GetName()))
	}
	put, err := client.UpdateTraceStorage(ctx, v1.NewOptTraceStorageRequest(v1.TraceStorageRequest{
		Name:        v1.NewOptString("replaced"),
		Description: v1.NewOptString("replaced desc"),
	}), v1.UpdateTraceStorageParams{ResourceId: created.GetResourceID()})
	if err != nil {
		t.Fatal(err)
	}
	if optStr(put.GetName()) != "replaced" || optStr(put.GetDescription()) != "replaced desc" {
		t.Fatalf("put failed: %+v", put)
	}

	// The mock holds no telemetry, so usage stats are always empty.
	daily, err := op.ReadDailyStats(ctx, id, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(daily) != 0 {
		t.Fatalf("expected no daily usage, got %d", len(daily))
	}
	monthly, err := op.ReadMonthlyStats(ctx, id, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if len(monthly) != 0 {
		t.Fatalf("expected no monthly usage, got %d", len(monthly))
	}

	key, err := op.CreateKey(ctx, id, nil)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := op.ListKeys(ctx, id, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0].GetUID() != key.GetUID() {
		t.Fatalf("unexpected key list: %+v", keys)
	}
	putKey, err := op.UpdateKey(ctx, id, key.GetUID(), new("put key"))
	if err != nil {
		t.Fatal(err)
	}
	if optStr(putKey.GetDescription()) != "put key" {
		t.Fatalf("key put failed: %s", optStr(putKey.GetDescription()))
	}
	patchedKey, err := client.PatchTraceStorageKey(ctx, v1.NewOptPatchedTraceStorageAccessKeyRequest(v1.PatchedTraceStorageAccessKeyRequest{
		Description: v1.NewOptString("patched key"),
	}), v1.PatchTraceStorageKeyParams{TraceResourceId: created.GetResourceID(), UID: key.GetUID()})
	if err != nil {
		t.Fatal(err)
	}
	if optStr(patchedKey.GetDescription()) != "patched key" {
		t.Fatalf("key patch failed: %s", optStr(patchedKey.GetDescription()))
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

	other, err := storageOp.Create(ctx, mssdk.LogStorageCreateParams{Name: "other-logs", IsSystem: false})
	if err != nil {
		t.Fatal(err)
	}
	patched, err := op.Update(ctx, uid, mssdk.LogsRoutingUpdateParams{LogStorageID: new(fmt.Sprintf("%d", other.GetID()))})
	if err != nil {
		t.Fatal(err)
	}
	if got := patched.GetLogStorage(); got.GetID() != other.GetID() {
		t.Fatalf("patch did not move the routing: storage %d", got.GetID())
	}
	put, err := client.UpdateLogRouting(ctx, &v1.LogRoutingRequest{
		PublisherCode: code,
		Variant:       variant,
		LogStorageID:  v1.NewNilInt64(storage.GetID()),
	}, v1.UpdateLogRoutingParams{UID: uid})
	if err != nil {
		t.Fatal(err)
	}
	if got := put.GetLogStorage(); got.GetID() != storage.GetID() {
		t.Fatalf("put did not move the routing back: storage %d", got.GetID())
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
	uid := created.GetUID()

	routings, err := op.List(ctx, mssdk.MetricsRoutingsListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(routings) != 1 || routings[0].GetUID() != uid {
		t.Fatalf("unexpected list: %+v", routings)
	}

	other, err := storageOp.Create(ctx, mssdk.MetricsStorageCreateParams{Name: "other-metrics"})
	if err != nil {
		t.Fatal(err)
	}
	patched, err := op.Update(ctx, uid, mssdk.MetricsRoutingUpdateParams{MetricsStorageID: new(fmt.Sprintf("%d", other.GetID()))})
	if err != nil {
		t.Fatal(err)
	}
	if got := patched.GetMetricsStorage(); got.GetID() != other.GetID() {
		t.Fatalf("patch did not move the routing: storage %d", got.GetID())
	}
	put, err := client.UpdateMetricRouting(ctx, &v1.MetricsRoutingRequest{
		PublisherCode:    code,
		Variant:          variant,
		MetricsStorageID: v1.NewNilInt64(storage.GetID()),
	}, v1.UpdateMetricRoutingParams{UID: uid})
	if err != nil {
		t.Fatal(err)
	}
	if got := put.GetMetricsStorage(); got.GetID() != storage.GetID() {
		t.Fatalf("put did not move the routing back: storage %d", got.GetID())
	}

	if err := op.Delete(ctx, uid); err != nil {
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

	targets, err := targetOp.List(ctx, pid, mssdk.NotificationTargetsListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].GetUID() != target.GetUID() {
		t.Fatalf("unexpected target list: %+v", targets)
	}
	patchedTarget, err := targetOp.Update(ctx, pid, target.GetUID(), mssdk.NotificationTargetUpdateParams{Description: new("patched")})
	if err != nil {
		t.Fatal(err)
	}
	if optStr(patchedTarget.GetDescription()) != "patched" {
		t.Fatalf("target patch failed: %s", optStr(patchedTarget.GetDescription()))
	}
	putTarget, err := client.UpdateAlertProjectNotificationTarget(ctx, &v1.NotificationTargetRequest{
		ServiceType: v1.NotificationTargetRequestServiceTypeSAKURAEVENTBUS,
		URL:         v1.NewOptString("https://example.com/eventbus"),
		Description: v1.NewOptString("replaced"),
	}, v1.UpdateAlertProjectNotificationTargetParams{ProjectResourceId: mustInt64(t, pid), UID: target.GetUID()})
	if err != nil {
		t.Fatal(err)
	}
	if putTarget.GetServiceType() != v1.NotificationTargetServiceTypeSAKURAEVENTBUS ||
		optStr(putTarget.GetURL()) != "https://example.com/eventbus" || optStr(putTarget.GetDescription()) != "replaced" {
		t.Fatalf("target put failed: %+v", putTarget)
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

	routings, err := routingOp.List(ctx, pid, mssdk.NotificationRoutingsListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(routings) != 1 {
		t.Fatalf("expected 1 routing, got %d", len(routings))
	}

	patchedRouting, err := routingOp.Update(ctx, pid, routing.GetUID(), mssdk.NotificationRoutingUpdateParams{
		MatchLabels: []v1.MatchLabelsItem{{Name: "severity", Value: "warning"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if labels := patchedRouting.GetMatchLabels(); len(labels) != 1 || labels[0].Value != "warning" {
		t.Fatalf("routing patch failed: %+v", labels)
	}
	putRouting, err := client.UpdateAlertProjectNotificationRouting(ctx, &v1.NotificationRoutingRequest{
		NotificationTargetUID: target.GetUID(),
		MatchLabels:           []v1.MatchLabelsItemRequest{{Name: "team", Value: "infra"}},
		ResendIntervalMinutes: v1.NewOptInt(30),
	}, v1.UpdateAlertProjectNotificationRoutingParams{ProjectResourceId: mustInt64(t, pid), UID: routing.GetUID()})
	if err != nil {
		t.Fatal(err)
	}
	if labels := putRouting.GetMatchLabels(); len(labels) != 1 || labels[0].Name != "team" || labels[0].Value != "infra" {
		t.Fatalf("routing put failed: %+v", labels)
	}
	if got, _ := putRouting.GetResendIntervalMinutes().Get(); got != 30 {
		t.Fatalf("routing put did not set resend_interval_minutes: %d", got)
	}

	second, err := routingOp.Create(ctx, pid, mssdk.NotificationRoutingCreateParams{
		NotificationTargetUID: target.GetUID(),
		MatchLabels:           []v1.MatchLabelsItem{{Name: "severity", Value: "info"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := routingOp.Reorder(ctx, pid, []v1.NotificationRoutingOrderRequest{
		{NotificationRoutingUID: routing.GetUID(), Order: second.GetOrder()},
		{NotificationRoutingUID: second.GetUID(), Order: routing.GetOrder()},
	}); err != nil {
		t.Fatal(err)
	}
	routings, err = routingOp.List(ctx, pid, mssdk.NotificationRoutingsListParams{})
	if err != nil {
		t.Fatal(err)
	}
	orders := map[string]int{}
	for _, rt := range routings {
		orders[rt.GetUID().String()] = rt.GetOrder()
	}
	if orders[routing.GetUID().String()] != second.GetOrder() || orders[second.GetUID().String()] != routing.GetOrder() {
		t.Fatalf("reorder not applied: %+v", orders)
	}

	if err := routingOp.Delete(ctx, pid, second.GetUID()); err != nil {
		t.Fatal(err)
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
		Name:             new("rule"),
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
	// The mock records no alert history, so the list is always empty.
	histories, err := op.ListHistories(ctx, pid, uid, mssdk.AlertRuleListHistoriesParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(histories) != 0 {
		t.Fatalf("expected no histories, got %d", len(histories))
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
		Name:             new("measure"),
		Rule: v1.LogMeasureRuleModel{
			Version: v1.LogMeasureRuleVersionEnumV1,
			Query:   v1.LogMeasureRuleV1{Matchers: []v1.FieldMatcher{}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	uid := created.GetUID()

	rules, err := op.List(ctx, pid, mssdk.LogMeasureRulesListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].GetUID() != uid {
		t.Fatalf("unexpected list: %+v", rules)
	}

	patched, err := op.Update(ctx, pid, uid, mssdk.LogMeasureRuleUpdateParams{Name: new("renamed")})
	if err != nil {
		t.Fatal(err)
	}
	if optStr(patched.GetName()) != "renamed" {
		t.Fatalf("patch failed: %s", optStr(patched.GetName()))
	}

	otherMetrics, err := metricsOp.Create(ctx, mssdk.MetricsStorageCreateParams{Name: "other-metrics"})
	if err != nil {
		t.Fatal(err)
	}
	put, err := client.UpdateAlertProjectLogMeasureRule(ctx, &v1.LogMeasureRuleRequest{
		Name:             v1.NewOptString("replaced"),
		Description:      v1.NewOptString("replaced desc"),
		LogStorageID:     logStorage.GetResourceID(),
		MetricsStorageID: otherMetrics.GetResourceID(),
		Rule: v1.LogMeasureRuleModelRequest{
			Version: v1.LogMeasureRuleVersionEnumV1,
			Query:   v1.LogMeasureRuleV1{Matchers: []v1.FieldMatcher{}},
		},
	}, v1.UpdateAlertProjectLogMeasureRuleParams{ProjectResourceId: mustInt64(t, pid), UID: uid})
	if err != nil {
		t.Fatal(err)
	}
	if optStr(put.GetName()) != "replaced" || optStr(put.GetDescription()) != "replaced desc" {
		t.Fatalf("put failed: %+v", put)
	}
	if got := put.GetMetricsStorage(); got.GetID() != otherMetrics.GetID() {
		t.Fatalf("put did not move the rule: metrics storage %d", got.GetID())
	}

	if err := op.Delete(ctx, pid, uid); err != nil {
		t.Fatal(err)
	}
}

func TestDedicatedStorageKMS(t *testing.T) {
	client, closeFn := newServer(t)
	defer closeFn()
	ctx := t.Context()
	const kmsKeyID, spID, newSPID = int64(110000000001), int64(110000000002), int64(110000000003)

	// check asserts the KMS key stays and the service principal is want (nil = null).
	type optInt64 interface{ Get() (int64, bool) }
	check := func(t *testing.T, step string, kms, sp optInt64, want *int64) {
		t.Helper()
		if v, ok := kms.Get(); !ok || v != kmsKeyID {
			t.Fatalf("%s: kms_key_id = %+v, want %d", step, kms, kmsKeyID)
		}
		v, ok := sp.Get()
		if want == nil && ok || want != nil && (!ok || v != *want) {
			t.Fatalf("%s: service_principal_id = %+v, want %v", step, sp, want)
		}
	}

	t.Run("log", func(t *testing.T) {
		op := mssdk.NewLogsStorageOp(client)
		created, err := op.Create(ctx, mssdk.LogStorageCreateParams{
			Name:               "dedicated-logs",
			Classification:     new(v1.LogStorageCreateRequestClassificationDedicated),
			KmsKeyID:           v1.NewOptNilInt64(kmsKeyID),
			ServicePrincipalID: v1.NewOptNilInt64(spID),
		})
		if err != nil {
			t.Fatal(err)
		}
		if created.GetClassification() != v1.LogStorageClassificationDedicated {
			t.Fatalf("classification = %s", created.GetClassification())
		}
		check(t, "create", created.GetKmsKeyID(), created.GetServicePrincipalID(), new(spID))
		id := ridOf(t, created.GetResourceID())

		// An update without service_principal_id leaves it unchanged.
		updated, err := op.Update(ctx, id, mssdk.LogStorageUpdateParams{Name: new("renamed")})
		if err != nil {
			t.Fatal(err)
		}
		check(t, "rename", updated.GetKmsKeyID(), updated.GetServicePrincipalID(), new(spID))

		updated, err = op.Update(ctx, id, mssdk.LogStorageUpdateParams{ServicePrincipalID: v1.NewOptNilInt64(newSPID)})
		if err != nil {
			t.Fatal(err)
		}
		check(t, "replace", updated.GetKmsKeyID(), updated.GetServicePrincipalID(), new(newSPID))

		// An explicit null clears it.
		var null v1.OptNilInt64
		null.SetToNull()
		if _, err := op.Update(ctx, id, mssdk.LogStorageUpdateParams{ServicePrincipalID: null}); err != nil {
			t.Fatal(err)
		}
		read, err := op.Read(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		check(t, "clear", read.GetKmsKeyID(), read.GetServicePrincipalID(), nil)
	})

	t.Run("trace", func(t *testing.T) {
		op := mssdk.NewTracesStorageOp(client)
		created, err := op.Create(ctx, mssdk.TracesStorageCreateParams{
			Name:               "dedicated-traces",
			Classification:     new(v1.TraceStorageCreateRequestClassificationDedicated),
			KmsKeyID:           v1.NewOptNilInt64(kmsKeyID),
			ServicePrincipalID: v1.NewOptNilInt64(spID),
		})
		if err != nil {
			t.Fatal(err)
		}
		if created.GetClassification() != v1.TraceStorageClassificationDedicated {
			t.Fatalf("classification = %s", created.GetClassification())
		}
		check(t, "create", created.GetKmsKeyID(), created.GetServicePrincipalID(), new(spID))
		id := fmt.Sprintf("%d", created.GetResourceID())

		updated, err := op.Update(ctx, id, mssdk.TracesStorageUpdateParams{Name: new("renamed")})
		if err != nil {
			t.Fatal(err)
		}
		check(t, "rename", updated.GetKmsKeyID(), updated.GetServicePrincipalID(), new(spID))

		updated, err = op.Update(ctx, id, mssdk.TracesStorageUpdateParams{ServicePrincipalID: v1.NewOptNilInt64(newSPID)})
		if err != nil {
			t.Fatal(err)
		}
		check(t, "replace", updated.GetKmsKeyID(), updated.GetServicePrincipalID(), new(newSPID))

		var null v1.OptNilInt64
		null.SetToNull()
		if _, err := op.Update(ctx, id, mssdk.TracesStorageUpdateParams{ServicePrincipalID: null}); err != nil {
			t.Fatal(err)
		}
		read, err := op.Read(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		check(t, "clear", read.GetKmsKeyID(), read.GetServicePrincipalID(), nil)
	})
}
