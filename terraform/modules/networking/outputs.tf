output "vpc_id" {
  value = opentelekomcloud_vpc_v1.rke2.id
}

output "subnet_id" {
  value = opentelekomcloud_vpc_subnet_v1.rke2.id
}

output "subnet_network_id" {
  value = opentelekomcloud_vpc_subnet_v1.rke2.subnet_id
}

output "security_group_id" {
  value = opentelekomcloud_networking_secgroup_v2.rke2.id
}

output "network_id" {
  description = "Network ID (opentelekomcloud_vpc_subnet_v1.id) for NAT gateway"
  value       = opentelekomcloud_vpc_subnet_v1.rke2.id
}

output "keypair_name" {
  value = opentelekomcloud_compute_keypair_v2.rke2.name
}

output "ssh_public_key_hash" {
  description = "Hash of SSH public key (triggers VM recreation on key change)"
  value       = sha256(var.ssh_public_key)
}
