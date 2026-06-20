resource "sakura_apprun_shared" "test" {
  name            = "sakumock-tf-apprun"
  timeout_seconds = 30
  port            = 80
  min_scale       = 0
  max_scale       = 1
  components = [{
    name       = "app"
    max_cpu    = "0.5"
    max_memory = "1Gi"
    deploy_source = {
      container_registry = {
        image = "nginx:latest"
      }
    }
    env = [{
      key   = "FOO"
      value = "bar"
    }]
  }]
}
