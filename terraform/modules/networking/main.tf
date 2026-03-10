terraform {
  required_providers {
    opentelekomcloud = {
      source  = "opentelekomcloud/opentelekomcloud"
      version = "~> 1.36"
    }
  }
}

resource "opentelekomcloud_vpc_v1" "rke2" {
  name = "${var.cluster_name}-vpc"
  cidr = var.vpc_cidr

  lifecycle {
    ignore_changes = [tags]
  }
}

resource "opentelekomcloud_vpc_subnet_v1" "rke2" {
  name       = "${var.cluster_name}-subnet"
  cidr       = var.subnet_cidr
  gateway_ip = cidrhost(var.subnet_cidr, 1)
  vpc_id     = opentelekomcloud_vpc_v1.rke2.id
  dns_list   = ["100.125.4.25", "8.8.8.8"]

  lifecycle {
    ignore_changes = [tags]
  }
}

resource "opentelekomcloud_networking_secgroup_v2" "rke2" {
  name        = "${var.cluster_name}-sg"
  description = "RKE2 cluster security group"
}

# =====================================================
# SSH
# =====================================================
resource "opentelekomcloud_networking_secgroup_rule_v2" "ssh" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 22
  port_range_max    = 22
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = opentelekomcloud_networking_secgroup_v2.rke2.id
}

# =====================================================
# Kubernetes Control Plane
# =====================================================
resource "opentelekomcloud_networking_secgroup_rule_v2" "k8s_api" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 6443
  port_range_max    = 6443
  remote_ip_prefix  = var.vpc_cidr
  security_group_id = opentelekomcloud_networking_secgroup_v2.rke2.id
}

resource "opentelekomcloud_networking_secgroup_rule_v2" "rke2_server" {
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

# =====================================================
# NodePort Range — TCP + UDP, from VPC + ELB SNAT
# =====================================================
resource "opentelekomcloud_networking_secgroup_rule_v2" "nodeport_tcp" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 30000
  port_range_max    = 32767
  remote_ip_prefix  = var.vpc_cidr
  security_group_id = opentelekomcloud_networking_secgroup_v2.rke2.id
}

resource "opentelekomcloud_networking_secgroup_rule_v2" "nodeport_udp" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "udp"
  port_range_min    = 30000
  port_range_max    = 32767
  remote_ip_prefix  = var.vpc_cidr
  security_group_id = opentelekomcloud_networking_secgroup_v2.rke2.id
}

# ELB Dedicated uses SNAT from 100.125.0.0/16 for health checks
# Without this, ELB health probes to NodePorts are DROPPED by the SG!
resource "opentelekomcloud_networking_secgroup_rule_v2" "elb_snat_tcp" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 30000
  port_range_max    = 32767
  remote_ip_prefix  = "100.125.0.0/16"
  security_group_id = opentelekomcloud_networking_secgroup_v2.rke2.id
}

resource "opentelekomcloud_networking_secgroup_rule_v2" "elb_snat_udp" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "udp"
  port_range_min    = 30000
  port_range_max    = 32767
  remote_ip_prefix  = "100.125.0.0/16"
  security_group_id = opentelekomcloud_networking_secgroup_v2.rke2.id
}

# =====================================================
# CNI Overlay — VXLAN (Cilium / Flannel)
# ROOT CAUSE of initial networking failures!
# OTC Hypervisor drops UDP 8472 without explicit SG rule.
# =====================================================
resource "opentelekomcloud_networking_secgroup_rule_v2" "vxlan" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "udp"
  port_range_min    = 8472
  port_range_max    = 8472
  remote_ip_prefix  = var.vpc_cidr
  security_group_id = opentelekomcloud_networking_secgroup_v2.rke2.id
}

# GENEVE (alternative overlay, e.g. Cilium native routing)
resource "opentelekomcloud_networking_secgroup_rule_v2" "geneve" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "udp"
  port_range_min    = 6081
  port_range_max    = 6081
  remote_ip_prefix  = var.vpc_cidr
  security_group_id = opentelekomcloud_networking_secgroup_v2.rke2.id
}

# =====================================================
# Cilium specific
# =====================================================
resource "opentelekomcloud_networking_secgroup_rule_v2" "cilium_health" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 4240
  port_range_max    = 4240
  remote_ip_prefix  = var.vpc_cidr
  security_group_id = opentelekomcloud_networking_secgroup_v2.rke2.id
}

resource "opentelekomcloud_networking_secgroup_rule_v2" "cilium_hubble" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 4244
  port_range_max    = 4245
  remote_ip_prefix  = var.vpc_cidr
  security_group_id = opentelekomcloud_networking_secgroup_v2.rke2.id
}

# =====================================================
# ICMP (debugging + health checks)
# =====================================================
resource "opentelekomcloud_networking_secgroup_rule_v2" "icmp" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "icmp"
  remote_ip_prefix  = var.vpc_cidr
  security_group_id = opentelekomcloud_networking_secgroup_v2.rke2.id
}

# =====================================================
# SSH Keypair (shared by all compute resources)
# Created here so it's available before any VMs spawn.
# =====================================================
resource "opentelekomcloud_compute_keypair_v2" "rke2" {
  name       = "${var.cluster_name}-key"
  public_key = var.ssh_public_key
}

# TinyProxy on bastion — allows cluster nodes to reach internet
resource "opentelekomcloud_networking_secgroup_rule_v2" "proxy" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 3128
  port_range_max    = 3128
  remote_ip_prefix  = var.vpc_cidr
  security_group_id = opentelekomcloud_networking_secgroup_v2.rke2.id
}
