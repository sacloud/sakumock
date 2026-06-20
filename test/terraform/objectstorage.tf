resource "sakura_object_storage_bucket" "test" {
  name    = "sakumock-tf-bucket"
  site_id = "isk01"
}

resource "sakura_object_storage_bucket_encryption_config" "test" {
  bucket     = sakura_object_storage_bucket.test.name
  site_id    = sakura_object_storage_bucket.test.site_id
  kms_key_id = sakura_kms.test.id
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
