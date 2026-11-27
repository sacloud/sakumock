resource "sakura_monitoring_suite_alert_project" "test" {
  name        = "sakumock-tf-alert-project"
  description = "description"
}

resource "sakura_monitoring_suite_metric_storage" "test" {
  name = "sakumock-tf-metric-storage"
}

resource "sakura_monitoring_suite_metric_storage_access_key" "test" {
  storage_id  = sakura_monitoring_suite_metric_storage.test.id
  description = "metric storage key"
}

resource "sakura_monitoring_suite_log_storage" "test" {
  name = "sakumock-tf-log-storage"
}

resource "sakura_monitoring_suite_log_storage_access_key" "test" {
  storage_id  = sakura_monitoring_suite_log_storage.test.id
  description = "log storage key"
}

resource "sakura_monitoring_suite_trace_storage" "test" {
  name = "sakumock-tf-trace-storage"
}

resource "sakura_monitoring_suite_trace_storage_access_key" "test" {
  storage_id  = sakura_monitoring_suite_trace_storage.test.id
  description = "trace storage key"
}

resource "sakura_monitoring_suite_alert_log_measure_rule" "test" {
  name              = "sakumock-tf-log-measure-rule"
  description       = "description"
  alert_project_id  = sakura_monitoring_suite_alert_project.test.id
  log_storage_id    = sakura_monitoring_suite_log_storage.test.id
  metric_storage_id = sakura_monitoring_suite_metric_storage.test.id
  rule = {
    version = "v1"
    query = {
      matchers = jsonencode([
        {
          type       = "string"
          field      = "text_payload"
          value      = "value"
          operator   = "eq"
          value_list = []
        }
      ])
    }
  }
}

# Dedicated storages are encrypted with a KMS key, reached through a service
# principal (provider >= 3.16.0).
resource "sakura_monitoring_suite_log_storage" "dedicated" {
  name                 = "sakumock-tf-dedicated-log-storage"
  classification       = "dedicated"
  kms_key_id           = sakura_kms.test.id
  service_principal_id = sakura_iam_service_principal.test.id
}

resource "sakura_monitoring_suite_trace_storage" "dedicated" {
  name                 = "sakumock-tf-dedicated-trace-storage"
  classification       = "dedicated"
  kms_key_id           = sakura_kms.test.id
  service_principal_id = sakura_iam_service_principal.test.id
}
