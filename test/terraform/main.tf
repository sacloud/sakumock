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
