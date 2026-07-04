data "sakura_apigw_plan" "test" {
  name = "エンタープライズ"
}

resource "sakura_apigw_subscription" "test" {
  name    = "sakumock-tf-apigw"
  plan_id = data.sakura_apigw_plan.test.id
}

resource "sakura_apigw_service" "test" {
  name            = "sakumock_tf_service"
  protocol        = "https"
  host            = "upstream.example.com"
  subscription_id = sakura_apigw_subscription.test.id
}

resource "sakura_apigw_route" "test" {
  name       = "sakumock_tf_route"
  service_id = sakura_apigw_service.test.id
  protocols  = "http,https"
  path       = "/"
  methods    = ["GET", "POST"]

  # The provider cannot round-trip a route without transformations (the API
  # returns {} for unset ones), so configure both explicitly.
  request_transformation = {
    http_method = "GET"
    remove = {
      header_keys = ["X-Remove-Header"]
    }
  }
  response_transformation = {
    remove = {
      header_keys = ["Server"]
    }
  }
}

resource "sakura_apigw_group" "test" {
  name = "sakumock_tf_group"
}

resource "sakura_apigw_user" "test" {
  name   = "sakumock_tf_user"
  groups = [{ name = sakura_apigw_group.test.name }]

  # The provider cannot round-trip a user without credentials (the API
  # returns {} for unset authentication), so configure one explicitly.
  authentication = {
    basic_auth = {
      username            = "sakumock"
      password_wo         = "sakumock-password"
      password_wo_version = 1
    }
  }
}
