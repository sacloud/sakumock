resource "sakura_cloudhsm" "test" {
  name                 = "sakumock-tf-cloudhsm"
  ipv4_network_address = "192.168.100.0"
  ipv4_netmask         = 24
}
