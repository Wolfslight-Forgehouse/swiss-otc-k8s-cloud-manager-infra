output "public_ip" {
  value = opentelekomcloud_networking_floatingip_v2.jumpserver.address
}

output "floating_ip" {
  value = opentelekomcloud_networking_floatingip_v2.jumpserver.address
}

output "private_ip" {
  value = opentelekomcloud_compute_instance_v2.jumpserver.access_ip_v4
}
