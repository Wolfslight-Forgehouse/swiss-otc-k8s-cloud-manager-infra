resource "opentelekomcloud_vpc_v1" "main" {
  name = "${var.cluster_name}-vpc"
  cidr = var.vpc_cidr
}

resource "opentelekomcloud_vpc_subnet_v1" "main" {
  name       = "${var.cluster_name}-subnet"
  cidr       = var.subnet_cidr
  vpc_id     = opentelekomcloud_vpc_v1.main.id
  gateway_ip = cidrhost(var.subnet_cidr, 1)
  dns_list   = ["100.125.4.25", "8.8.8.8"]
}

resource "opentelekomcloud_networking_router_v2" "main" {
  name                = "${var.cluster_name}-router"
  admin_state_up      = true
  external_network_id = data.opentelekomcloud_networking_network_v2.ext.id
}

data "opentelekomcloud_networking_network_v2" "ext" {
  name = "admin_external_net"
}

resource "opentelekomcloud_networking_router_interface_v2" "main" {
  router_id = opentelekomcloud_networking_router_v2.main.id
  subnet_id = opentelekomcloud_vpc_subnet_v1.main.id
}

resource "opentelekomcloud_networking_secgroup_v2" "rke2" {
  name        = "${var.cluster_name}-sg"
  description = "Security group for RKE2 cluster"
}

resource "opentelekomcloud_networking_secgroup_rule_v2" "kube_api" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 6443
  port_range_max    = 6443
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = opentelekomcloud_networking_secgroup_v2.rke2.id
}

resource "opentelekomcloud_networking_secgroup_rule_v2" "rke2_supervisor" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 9345
  port_range_max    = 9345
  remote_ip_prefix  = var.vpc_cidr
  security_group_id = opentelekomcloud_networking_secgroup_v2.rke2.id
}

resource "opentelekomcloud_networking_secgroup_rule_v2" "etcd" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 2379
  port_range_max    = 2380
  remote_ip_prefix  = var.vpc_cidr
  security_group_id = opentelekomcloud_networking_secgroup_v2.rke2.id
}

resource "opentelekomcloud_networking_secgroup_rule_v2" "kubelet" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 10250
  port_range_max    = 10250
  remote_ip_prefix  = var.vpc_cidr
  security_group_id = opentelekomcloud_networking_secgroup_v2.rke2.id
}

resource "opentelekomcloud_networking_secgroup_rule_v2" "nodeport" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 30000
  port_range_max    = 32767
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = opentelekomcloud_networking_secgroup_v2.rke2.id
}

resource "opentelekomcloud_networking_secgroup_rule_v2" "ssh" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 22
  port_range_max    = 22
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = opentelekomcloud_networking_secgroup_v2.rke2.id
}

resource "opentelekomcloud_networking_secgroup_rule_v2" "internal_all" {
  direction         = "ingress"
  ethertype         = "IPv4"
  remote_ip_prefix  = var.vpc_cidr
  security_group_id = opentelekomcloud_networking_secgroup_v2.rke2.id
}
