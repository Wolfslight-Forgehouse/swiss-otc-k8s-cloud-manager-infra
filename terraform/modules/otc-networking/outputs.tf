output "vpc_id"     { value = opentelekomcloud_vpc_v1.main.id }
output "subnet_id"  { value = opentelekomcloud_vpc_subnet_v1.main.id }
output "secgroup_id"{ value = opentelekomcloud_networking_secgroup_v2.rke2.id }
