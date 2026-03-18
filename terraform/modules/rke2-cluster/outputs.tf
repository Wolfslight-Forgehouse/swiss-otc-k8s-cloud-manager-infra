output "master_floating_ip" {
  value = opentelekomcloud_networking_floatingip_v2.master.address
}
output "worker_ips" {
  value = opentelekomcloud_compute_instance_v2.worker[*].network[0].fixed_ip_v4
}
output "master_token_command" {
  value = "ssh ubuntu@${opentelekomcloud_networking_floatingip_v2.master.address} 'sudo cat /var/lib/rancher/rke2/server/node-token'"
}
