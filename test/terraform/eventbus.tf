resource "sakura_eventbus_process_configuration" "simplenotification" {
  name        = "sakumock-tf-pc-simplenotification"
  description = "description"
  tags        = ["tag1"]

  destination = "simplenotification"
  parameters  = jsonencode({ group_id = sakura_simple_notification_group.test.id, message = "test message" })

  sakura_access_token_wo        = "dummy-token"
  sakura_access_token_secret_wo = "dummy-token-secret"
  credentials_wo_version        = 1
}

resource "sakura_eventbus_process_configuration" "simplemq" {
  name        = "sakumock-tf-pc-simplemq"
  description = "description"

  destination = "simplemq"
  parameters  = jsonencode({ queue_name = sakura_simple_mq.test.name, content = "dGVzdA==" })

  sakura_access_token_wo        = "dummy-token"
  sakura_access_token_secret_wo = "dummy-token-secret"
  credentials_wo_version        = 1
}

resource "sakura_eventbus_schedule" "test" {
  name        = "sakumock-tf-schedule"
  description = "description"

  process_configuration_id = sakura_eventbus_process_configuration.simplenotification.id
  starts_at                = 1700000000000
  recurring_step           = 1
  recurring_unit           = "day"
}

resource "sakura_eventbus_trigger" "test" {
  name        = "sakumock-tf-trigger"
  description = "description"

  process_configuration_id = sakura_eventbus_process_configuration.simplemq.id
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
