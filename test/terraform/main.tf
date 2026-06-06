terraform {
  required_providers {
    sakura = {
      source  = "sacloud/sakura"
      version = "~> 3"
    }
  }
}

# Endpoints and dummy credentials come from the environment (the dotenv file
# written by `sakumock all --write-env-file`). tk1v is SAKURA Cloud's sandbox
# zone (used for testing); the mock ignores the zone regardless.
provider "sakura" {
  zone = "tk1v"
}

resource "sakura_kms" "test" {
  name = "sakumock-tf-kms"
}

resource "sakura_simple_mq" "test" {
  name                       = "sakumock-tf-queue"
  description                = "description"
  tags                       = ["tag1"]
  visibility_timeout_seconds = 60
  expire_seconds             = 3600
}

resource "sakura_secret_manager" "test" {
  name       = "sakumock-tf-vault"
  kms_key_id = sakura_kms.test.id
}

resource "sakura_secret_manager_secret" "test" {
  name             = "foobar"
  vault_id         = sakura_secret_manager.test.id
  value_wo         = "secret value!"
  value_wo_version = 1
}

resource "sakura_simple_notification_destination" "test" {
  name  = "sakumock-tf-dest"
  type  = "email"
  value = "ops@example.com"
}

resource "sakura_simple_notification_group" "test" {
  name         = "sakumock-tf-group"
  destinations = [sakura_simple_notification_destination.test.id]
}

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
