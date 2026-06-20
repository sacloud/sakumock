resource "sakura_simple_notification_destination" "test" {
  name  = "sakumock-tf-dest"
  type  = "email"
  value = "ops@example.com"
}

resource "sakura_simple_notification_group" "test" {
  name         = "sakumock-tf-group"
  destinations = [sakura_simple_notification_destination.test.id]
}
