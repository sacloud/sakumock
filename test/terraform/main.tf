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

resource "sakura_eventbus_process_configuration" "test" {
  name        = "sakumock-tf-process-configuration"
  description = "description"
  tags        = ["tag1"]

  destination = "simplenotification"
  parameters  = "{\"group_id\": \"123456789012\", \"message\": \"test message\"}"

  sakura_access_token_wo        = "dummy-token"
  sakura_access_token_secret_wo = "dummy-token-secret"
  credentials_wo_version        = 1
}

resource "sakura_eventbus_schedule" "test" {
  name        = "sakumock-tf-schedule"
  description = "description"

  process_configuration_id = sakura_eventbus_process_configuration.test.id
  starts_at                = 1700000000000
  recurring_step           = 1
  recurring_unit           = "day"
}

resource "sakura_eventbus_trigger" "test" {
  name        = "sakumock-tf-trigger"
  description = "description"

  process_configuration_id = sakura_eventbus_process_configuration.test.id
  source                   = "test-source"
  types                    = ["type1"]
  conditions = [
    {
      key    = "key1"
      op     = "eq"
      values = ["foo"]
    },
  ]
}

resource "sakura_object_storage_bucket" "test" {
  name    = "sakumock-tf-bucket"
  site_id = "isk01"
}

resource "sakura_object_storage_bucket_encryption_config" "test" {
  bucket     = sakura_object_storage_bucket.test.name
  site_id    = sakura_object_storage_bucket.test.site_id
  kms_key_id = sakura_kms.test.id
}

resource "sakura_iam_folder" "test" {
  name        = "sakumock-tf-folder"
  description = "description"
}

resource "sakura_iam_project" "test" {
  name             = "sakumock-tf-project"
  code             = "sakumock-tf-project"
  description      = "description"
  parent_folder_id = sakura_iam_folder.test.id
}

resource "sakura_iam_user" "test" {
  name                = "sakumock-tf-user"
  code                = "sakumock-tf-user"
  password_wo         = "Password12345!"
  password_wo_version = 1
  description         = "description"
}

resource "sakura_iam_group" "test" {
  name        = "sakumock-tf-group"
  description = "description"
}

resource "sakura_iam_service_principal" "test" {
  name        = "sakumock-tf-sp"
  project_id  = sakura_iam_project.test.id
  description = "description"
}

resource "sakura_iam_project_apikey" "test" {
  name        = "sakumock-tf-apikey"
  project_id  = sakura_iam_project.test.id
  iam_roles   = ["resource-creator"]
  description = "description"
}

resource "sakura_apprun_shared" "test" {
  name            = "sakumock-tf-apprun"
  timeout_seconds = 30
  port            = 80
  min_scale       = 0
  max_scale       = 1
  components = [{
    name       = "app"
    max_cpu    = "0.5"
    max_memory = "1Gi"
    deploy_source = {
      container_registry = {
        image = "nginx:latest"
      }
    }
    env = [{
      key   = "FOO"
      value = "bar"
    }]
  }]
}

resource "sakura_apprun_dedicated_cluster" "test" {
  name                 = "sakumock-tf-cluster"
  service_principal_id = sakura_iam_service_principal.test.id
  ports = [{
    port     = 443
    protocol = "https"
  }]
}

resource "sakura_apprun_dedicated_application" "test" {
  name       = "sakumock-tf-app"
  cluster_id = sakura_apprun_dedicated_cluster.test.id
}

resource "sakura_apprun_dedicated_version" "test" {
  application_id = sakura_apprun_dedicated_application.test.id
  cpu            = 1000
  memory         = 2048
  scaling_mode   = "manual"
  fixed_scale    = 1
  image          = "nginx:latest"
  exposed_ports = [{
    target_port      = 80
    use_lets_encrypt = false
    host             = []
    health_check = {
      path             = "/"
      interval_seconds = 10
      timeout_seconds  = 5
    }
  }]
  env_vars = [{
    key    = "FOO"
    value  = "bar"
    secret = false
  }]
}

resource "sakura_apprun_dedicated_auto_scaling_group" "test" {
  name                      = "sakumock-tf-asg"
  cluster_id                = sakura_apprun_dedicated_cluster.test.id
  zone                      = "is1a"
  worker_service_class_path = "cloud/apprun/dedicated/worker/1vcpu_2gb"
  min_nodes                 = 1
  max_nodes                 = 3
  name_servers              = ["210.188.224.10"]
  interfaces = [{
    interface_index = 0
    upstream        = "shared"
    connects_to_lb  = false
  }]
}

resource "sakura_apprun_dedicated_lb" "test" {
  name                  = "sakumock-tf-lb"
  cluster_id            = sakura_apprun_dedicated_cluster.test.id
  auto_scaling_group_id = sakura_apprun_dedicated_auto_scaling_group.test.id
  service_class_path    = "cloud/apprun/dedicated/lb/1vcpu_2gb"
  name_servers          = ["210.188.224.10"]
  interfaces = [{
    interface_index = 0
    upstream        = "shared"
  }]
}

resource "sakura_apprun_dedicated_certificate" "test" {
  name            = "sakumock-tf-cert"
  cluster_id      = sakura_apprun_dedicated_cluster.test.id
  certificate_pem = "-----BEGIN CERTIFICATE-----\nMIItest\n-----END CERTIFICATE-----"
  private_key_pem = "-----BEGIN PRIVATE KEY-----\nMIItest\n-----END PRIVATE KEY-----"
}

resource "sakura_object_storage_permission" "test" {
  name    = "sakumock-tf-permission"
  site_id = "isk01"

  bucket_controls = [
    {
      bucket    = sakura_object_storage_bucket.test.name
      can_read  = true
      can_write = true
    },
  ]
}
