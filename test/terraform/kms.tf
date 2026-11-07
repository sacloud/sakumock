resource "sakura_kms" "test" {
  name = "sakumock-tf-kms"
  # The instant-delete API is abolished; destroy schedules the destruction.
  schedule_destruction_days = 7
}
