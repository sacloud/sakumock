terraform {
  required_providers {
    sakura = {
      source  = "sacloud/sakura"
      version = "~> 3"
    }
  }
}

# Endpoints and dummy credentials come from the environment (the dotenv file
# written by `sakumock all --write-env-file`). Only the zone is set here.
provider "sakura" {
  zone = "is1b"
}

resource "sakura_kms" "test" {
  name = "sakumock-tf-kms"
}

resource "sakura_simple_mq" "test" {
  name = "sakumock-tf-queue"
}

resource "sakura_secret_manager" "test" {
  name       = "sakumock-tf-vault"
  kms_key_id = sakura_kms.test.id
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
