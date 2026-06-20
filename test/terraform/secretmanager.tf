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
