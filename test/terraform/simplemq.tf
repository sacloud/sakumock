resource "sakura_simple_mq" "test" {
  name                       = "sakumock-tf-queue"
  description                = "description"
  tags                       = ["tag1"]
  visibility_timeout_seconds = 60
  expire_seconds             = 3600
}
