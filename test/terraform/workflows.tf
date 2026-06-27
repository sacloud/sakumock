data "sakura_workflows_plan" "test" {
  name = "200K"
}

resource "sakura_workflows_subscription" "test" {
  plan_id = data.sakura_workflows_plan.test.id
}

resource "sakura_workflows" "test" {
  subscription_id = sakura_workflows_subscription.test.id
  name            = "sakumock-tf-workflow"
  description     = "test workflow"
  publish         = false
  logging         = false

  latest_revision = {
    runbook = <<-EOF
meta:
  description: test
steps:
  done:
    return: "hello"
EOF
  }
}
