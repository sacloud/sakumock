# The activation and evaluation rules are per-project singletons with no
# DELETE in the API: destroying them leaves them as they are, since
# no_action_on_delete defaults to true (false disables them instead).

resource "sakura_security_control_activation" "test" {
  service_principal_id = "111111111111"
}

resource "sakura_security_control_evaluation_rule" "test" {
  id      = "server-no-public-ip"
  enabled = true
  parameters = {
    service_principal_id = "111111111111"
    targets              = ["is1a", "tk1a"]
  }
}

resource "sakura_security_control_automated_action" "test" {
  name                = "sakumock-tf-security-control"
  description         = "Notify rejected evaluations"
  execution_condition = "event.evaluationResult.status == \"REJECTED\""
  action = {
    type = "simple_notification"
    parameters = {
      service_principal_id = "111111111111"
      target_id            = "111111111112"
    }
  }
}
