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
