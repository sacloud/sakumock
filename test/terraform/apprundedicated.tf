resource "sakura_apprun_dedicated_cluster" "test" {
  name                 = "sakumock-tf-cluster"
  service_principal_id = sakura_iam_service_principal.test.id
  ports = [{
    port     = 443
    protocol = "https"
  }]
}

resource "sakura_apprun_dedicated_application" "test" {
  name       = "sakumock-tf-app"
  cluster_id = sakura_apprun_dedicated_cluster.test.id
}

resource "sakura_apprun_dedicated_version" "test" {
  application_id = sakura_apprun_dedicated_application.test.id
  cpu            = 1000
  memory         = 2048
  scaling_mode   = "manual"
  fixed_scale    = 1
  image          = "nginx:latest"
  exposed_ports = [{
    target_port      = 80
    use_lets_encrypt = false
    host             = []
    health_check = {
      path             = "/"
      interval_seconds = 10
      timeout_seconds  = 5
    }
  }]
  env_vars = [{
    key    = "FOO"
    value  = "bar"
    secret = false
  }]
}

resource "sakura_apprun_dedicated_auto_scaling_group" "test" {
  name                      = "sakumock-tf-asg"
  cluster_id                = sakura_apprun_dedicated_cluster.test.id
  zone                      = "is1a"
  worker_service_class_path = "cloud/apprun/dedicated/worker/1vcpu_2gb"
  min_nodes                 = 1
  max_nodes                 = 3
  name_servers              = ["210.188.224.10"]
  interfaces = [{
    interface_index = 0
    upstream        = "shared"
    connects_to_lb  = false
  }]
}

resource "sakura_apprun_dedicated_lb" "test" {
  name                  = "sakumock-tf-lb"
  cluster_id            = sakura_apprun_dedicated_cluster.test.id
  auto_scaling_group_id = sakura_apprun_dedicated_auto_scaling_group.test.id
  service_class_path    = "cloud/apprun/dedicated/lb/1vcpu_2gb"
  name_servers          = ["210.188.224.10"]
  interfaces = [{
    interface_index = 0
    upstream        = "shared"
  }]
}

resource "sakura_apprun_dedicated_certificate" "test" {
  name            = "sakumock-tf-cert"
  cluster_id      = sakura_apprun_dedicated_cluster.test.id
  certificate_pem = "-----BEGIN CERTIFICATE-----\nMIItest\n-----END CERTIFICATE-----"
  private_key_pem = "-----BEGIN PRIVATE KEY-----\nMIItest\n-----END PRIVATE KEY-----"
}
