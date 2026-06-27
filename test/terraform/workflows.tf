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
  publish         = true
  logging         = false

  latest_revision = {
    runbook = file("workflows-runbook.yaml")
  }
}

resource "sakura_workflows_revision_alias" "test" {
  workflow_id = sakura_workflows.test.id
  revision_id = sakura_workflows.test.latest_revision.id
  alias       = "current"
}
