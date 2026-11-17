# sacloud/sakura 3.15.x fails to create a destination or a group without tags
# ("Tags (nil is invalid value)", rejected by the provider before any request
# is sent), so both set at least one.

resource "sakura_simple_notification_destination" "test" {
  name  = "sakumock-tf-dest"
  type  = "email"
  value = "ops@example.com"
  tags  = ["sakumock"]
}

resource "sakura_simple_notification_group" "test" {
  name         = "sakumock-tf-group"
  destinations = [sakura_simple_notification_destination.test.id]
  tags         = ["sakumock"]
}
