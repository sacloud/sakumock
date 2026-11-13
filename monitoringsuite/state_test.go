package monitoringsuite_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/google/uuid"
	mssdk "github.com/sacloud/sacloud-sdk-go/api/monitoring-suite"
	v1 "github.com/sacloud/sacloud-sdk-go/api/monitoring-suite/apis/v1"

	"github.com/sacloud/sakumock/core/coretest"
	"github.com/sacloud/sakumock/monitoringsuite"
)

// publisherVariant returns a built-in publisher code and variant that routes
// to the given kind of storage.
func publisherVariant(t *testing.T, client *v1.Client, storage v1.PublisherVariantStorage) (code, variant string) {
	t.Helper()
	publishers, err := mssdk.NewPublisherOp(client).List(t.Context(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range publishers {
		for _, v := range p.GetVariants() {
			if v.GetStorage() == storage {
				return p.GetCode(), v.GetName()
			}
		}
	}
	t.Fatalf("no publisher variant for %s storage", storage)
	return "", ""
}

func TestStateSurvivesRestart(t *testing.T) {
	var (
		alertProjectID, deletedProjectID, dashboardID                 string
		logStorageID, metricsStorageID, traceStorageID                string
		logKey, metricsKey, traceKey                                  uuid.UUID
		metricsKeyToken                                               string
		alertRuleUID, measureRuleUID, targetUID, notificationRouteUID uuid.UUID
		logRoutingUID, metricsRoutingUID                              uuid.UUID
		publisherCount                                                int
	)

	coretest.StateRestart(t, monitoringsuite.Config{},
		func(t *testing.T, serverURL string) {
			ctx := t.Context()
			client := newClient(t, serverURL)

			projOp := mssdk.NewAlertProjectOp(client)
			project, err := projOp.Create(ctx, mssdk.AlertProjectCreateParams{Name: "persisted", Description: new("desc")})
			if err != nil {
				t.Fatal(err)
			}
			alertProjectID = ridOf(t, project.GetResourceID())
			// A deleted row must stay deleted, and must not disturb the order
			// of the rows around it.
			deleted, err := projOp.Create(ctx, mssdk.AlertProjectCreateParams{Name: "deleted"})
			if err != nil {
				t.Fatal(err)
			}
			deletedProjectID = ridOf(t, deleted.GetResourceID())
			if _, err := projOp.Create(ctx, mssdk.AlertProjectCreateParams{Name: "second"}); err != nil {
				t.Fatal(err)
			}
			if err := projOp.Delete(ctx, deletedProjectID); err != nil {
				t.Fatal(err)
			}

			dashboard, err := mssdk.NewDashboardOp(client).Create(ctx, mssdk.DashboardProjectCreateParams{Name: "persisted-dash"})
			if err != nil {
				t.Fatal(err)
			}
			dashboardID = ridOf(t, dashboard.GetResourceID())

			logOp := mssdk.NewLogsStorageOp(client)
			logStorage, err := logOp.Create(ctx, mssdk.LogStorageCreateParams{
				Name:           "persisted-logs",
				Classification: new(v1.LogStorageCreateRequestClassificationShared),
			})
			if err != nil {
				t.Fatal(err)
			}
			logStorageID = ridOf(t, logStorage.GetResourceID())
			if _, err := logOp.SetExpire(ctx, logStorageID, 7); err != nil {
				t.Fatal(err)
			}
			lk, err := logOp.CreateKey(ctx, logStorageID, new("log key"))
			if err != nil {
				t.Fatal(err)
			}
			logKey = lk.GetUID()

			metricsOp := mssdk.NewMetricsStorageOp(client)
			metricsStorage, err := metricsOp.Create(ctx, mssdk.MetricsStorageCreateParams{Name: "persisted-metrics"})
			if err != nil {
				t.Fatal(err)
			}
			metricsStorageID = ridOf(t, metricsStorage.GetResourceID())
			mk, err := metricsOp.CreateKey(ctx, metricsStorageID, new("metrics key"))
			if err != nil {
				t.Fatal(err)
			}
			metricsKey, metricsKeyToken = mk.GetUID(), mk.GetToken()

			traceOp := mssdk.NewTracesStorageOp(client)
			traceStorage, err := traceOp.Create(ctx, mssdk.TracesStorageCreateParams{
				Name:           "persisted-traces",
				Classification: new(v1.TraceStorageCreateRequestClassificationShared),
			})
			if err != nil {
				t.Fatal(err)
			}
			traceStorageID = fmt.Sprintf("%d", traceStorage.GetResourceID())
			if _, err := traceOp.SetExpire(ctx, traceStorageID, 14); err != nil {
				t.Fatal(err)
			}
			tk, err := traceOp.CreateKey(ctx, traceStorageID, new("trace key"))
			if err != nil {
				t.Fatal(err)
			}
			traceKey = tk.GetUID()

			rule, err := mssdk.NewAlertRuleOp(client).Create(ctx, alertProjectID, mssdk.AlertRuleCreateParams{
				MetricsStorageID: metricsStorageID,
				Query:            "up == 0",
				Name:             new("persisted-rule"),
			})
			if err != nil {
				t.Fatal(err)
			}
			alertRuleUID = rule.GetUID()

			measure, err := mssdk.NewLogMeasureRuleOp(client).Create(ctx, alertProjectID, mssdk.LogMeasureRuleCreateParams{
				LogStorageID:     logStorageID,
				MetricsStorageID: metricsStorageID,
				Name:             new("persisted-measure"),
				Rule: v1.LogMeasureRuleModel{
					Version: v1.LogMeasureRuleVersionEnumV1,
					Query:   v1.LogMeasureRuleV1{Matchers: []v1.FieldMatcher{}},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			measureRuleUID = measure.GetUID()

			webhook, _ := url.Parse("https://example.com/webhook")
			target, err := mssdk.NewNotificationTargetOp(client).Create(ctx, alertProjectID, mssdk.NotificationTargetCreateParams{
				ServiceType: v1.NotificationTargetServiceTypeSAKURASIMPLENOTICE,
				URL:         webhook,
			})
			if err != nil {
				t.Fatal(err)
			}
			targetUID = target.GetUID()
			notificationRoute, err := mssdk.NewNotificationRoutingOp(client).Create(ctx, alertProjectID, mssdk.NotificationRoutingCreateParams{
				NotificationTargetUID: targetUID,
				MatchLabels:           []v1.MatchLabelsItem{{Name: "severity", Value: "critical"}},
			})
			if err != nil {
				t.Fatal(err)
			}
			notificationRouteUID = notificationRoute.GetUID()

			code, variant := publisherVariant(t, client, v1.PublisherVariantStorageLogs)
			logRouting, err := mssdk.NewLogRoutingOp(client).Create(ctx, mssdk.LogsRoutingCreateParams{
				PublisherCode: code,
				Variant:       variant,
				LogStorageID:  fmt.Sprintf("%d", logStorage.GetID()),
			})
			if err != nil {
				t.Fatal(err)
			}
			logRoutingUID = logRouting.GetUID()
			code, variant = publisherVariant(t, client, v1.PublisherVariantStorageMetrics)
			metricsRouting, err := mssdk.NewMetricsRoutingOp(client).Create(ctx, mssdk.MetricsRoutingCreateParams{
				PublisherCode:    code,
				Variant:          variant,
				MetricsStorageID: fmt.Sprintf("%d", metricsStorage.GetID()),
			})
			if err != nil {
				t.Fatal(err)
			}
			metricsRoutingUID = metricsRouting.GetUID()

			if _, err := mssdk.NewManagementOp(client).CreateProvisioning(ctx, mssdk.ProvisioningCreateParam{
				Metrics: &v1.ProvisioningExist{SystemExist: true, UserExist: false},
			}); err != nil {
				t.Fatal(err)
			}

			publishers, err := mssdk.NewPublisherOp(client).List(ctx, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			publisherCount = len(publishers)
		},
		func(t *testing.T, serverURL string) {
			ctx := t.Context()
			client := newClient(t, serverURL)

			projects, err := mssdk.NewAlertProjectOp(client).List(ctx, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(projects) != 2 || ridOf(t, projects[0].GetResourceID()) != alertProjectID ||
				optStr(projects[0].GetName()) != "persisted" || optStr(projects[1].GetName()) != "second" {
				t.Errorf("restored alert projects = %+v, want persisted then second", projects)
			}
			dashboard, err := mssdk.NewDashboardOp(client).Read(ctx, dashboardID)
			if err != nil {
				t.Fatal(err)
			}
			if optStr(dashboard.GetName()) != "persisted-dash" {
				t.Errorf("restored dashboard project = %+v", dashboard)
			}

			logOp := mssdk.NewLogsStorageOp(client)
			logStorage, err := logOp.Read(ctx, logStorageID)
			if err != nil {
				t.Fatal(err)
			}
			if optStr(logStorage.GetName()) != "persisted-logs" || logStorage.GetExpireDay() != 7 {
				t.Errorf("restored log storage = %+v", logStorage)
			}
			if _, err := logOp.ReadKey(ctx, logStorageID, logKey); err != nil {
				t.Errorf("restored log storage key: %v", err)
			}

			metricsOp := mssdk.NewMetricsStorageOp(client)
			metricsStorage, err := metricsOp.Read(ctx, metricsStorageID)
			if err != nil {
				t.Fatal(err)
			}
			if optStr(metricsStorage.GetName()) != "persisted-metrics" {
				t.Errorf("restored metrics storage = %+v", metricsStorage)
			}
			mk, err := metricsOp.ReadKey(ctx, metricsStorageID, metricsKey)
			if err != nil {
				t.Fatal(err)
			}
			if mk.GetToken() != metricsKeyToken {
				t.Errorf("restored metrics key token = %q, want %q", mk.GetToken(), metricsKeyToken)
			}

			traceOp := mssdk.NewTracesStorageOp(client)
			traceStorage, err := traceOp.Read(ctx, traceStorageID)
			if err != nil {
				t.Fatal(err)
			}
			if optStr(traceStorage.GetName()) != "persisted-traces" || traceStorage.GetRetentionPeriodDays() != 14 {
				t.Errorf("restored trace storage = %+v", traceStorage)
			}
			if _, err := traceOp.ReadKey(ctx, traceStorageID, traceKey); err != nil {
				t.Errorf("restored trace storage key: %v", err)
			}

			rule, err := mssdk.NewAlertRuleOp(client).Read(ctx, alertProjectID, alertRuleUID)
			if err != nil {
				t.Fatal(err)
			}
			if rule.GetQuery() != "up == 0" {
				t.Errorf("restored alert rule = %+v", rule)
			}
			measure, err := mssdk.NewLogMeasureRuleOp(client).Read(ctx, alertProjectID, measureRuleUID)
			if err != nil {
				t.Fatal(err)
			}
			if measure.GetRule().Version != v1.LogMeasureRuleVersionEnumV1 {
				t.Errorf("restored log measure rule = %+v", measure)
			}

			target, err := mssdk.NewNotificationTargetOp(client).Read(ctx, alertProjectID, targetUID)
			if err != nil {
				t.Fatal(err)
			}
			if target.GetServiceType() != v1.NotificationTargetServiceTypeSAKURASIMPLENOTICE {
				t.Errorf("restored notification target = %+v", target)
			}
			notificationRoute, err := mssdk.NewNotificationRoutingOp(client).Read(ctx, alertProjectID, notificationRouteUID)
			if err != nil {
				t.Fatal(err)
			}
			nt := notificationRoute.GetNotificationTarget()
			labels := notificationRoute.GetMatchLabels()
			if nt.GetUID() != targetUID || len(labels) != 1 || labels[0].Value != "critical" {
				t.Errorf("restored notification routing = %+v", notificationRoute)
			}

			if _, err := mssdk.NewLogRoutingOp(client).Read(ctx, logRoutingUID); err != nil {
				t.Errorf("restored log routing: %v", err)
			}
			if _, err := mssdk.NewMetricsRoutingOp(client).Read(ctx, metricsRoutingUID); err != nil {
				t.Errorf("restored metrics routing: %v", err)
			}

			prov, err := mssdk.NewManagementOp(client).ReadProvisioning(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if !prov.GetLogs().SystemExist || !prov.GetLogs().UserExist || !prov.GetMetrics().SystemExist || prov.GetMetrics().UserExist {
				t.Errorf("restored provisioning = %+v", prov)
			}

			// The publishers are built in: a restart neither loses nor
			// duplicates them.
			publishers, err := mssdk.NewPublisherOp(client).List(ctx, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(publishers) != publisherCount {
				t.Errorf("publishers after restart = %d, want %d", len(publishers), publisherCount)
			}
		})
}

// A resource created after a restart reuses neither a restored resource ID nor
// a restored access key's sequential id.
func TestStateRestoredIDsAreNotReused(t *testing.T) {
	restored := map[string]string{}
	var keyID int64
	coretest.StateRestart(t, monitoringsuite.Config{},
		func(t *testing.T, serverURL string) {
			ctx := t.Context()
			client := newClient(t, serverURL)
			project, err := mssdk.NewAlertProjectOp(client).Create(ctx, mssdk.AlertProjectCreateParams{Name: "p"})
			if err != nil {
				t.Fatal(err)
			}
			restored[ridOf(t, project.GetResourceID())] = "alert project"
			dashboard, err := mssdk.NewDashboardOp(client).Create(ctx, mssdk.DashboardProjectCreateParams{Name: "d"})
			if err != nil {
				t.Fatal(err)
			}
			restored[ridOf(t, dashboard.GetResourceID())] = "dashboard project"
			metricsOp := mssdk.NewMetricsStorageOp(client)
			storage, err := metricsOp.Create(ctx, mssdk.MetricsStorageCreateParams{Name: "m"})
			if err != nil {
				t.Fatal(err)
			}
			id := ridOf(t, storage.GetResourceID())
			restored[id] = "metrics storage"
			key, err := metricsOp.CreateKey(ctx, id, nil)
			if err != nil {
				t.Fatal(err)
			}
			keyID = key.GetID()
		},
		func(t *testing.T, serverURL string) {
			ctx := t.Context()
			client := newClient(t, serverURL)
			logOp := mssdk.NewLogsStorageOp(client)
			storage, err := logOp.Create(ctx, mssdk.LogStorageCreateParams{Name: "new"})
			if err != nil {
				t.Fatal(err)
			}
			id := ridOf(t, storage.GetResourceID())
			if kind, ok := restored[id]; ok {
				t.Errorf("new log storage reused the ID %s of a restored %s", id, kind)
			}
			key, err := logOp.CreateKey(ctx, id, nil)
			if err != nil {
				t.Fatal(err)
			}
			if key.GetID() == keyID {
				t.Errorf("new access key reused the restored id %d", keyID)
			}
		}, true)
}
