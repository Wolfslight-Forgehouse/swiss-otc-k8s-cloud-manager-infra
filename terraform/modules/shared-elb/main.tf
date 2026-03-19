terraform {
  required_providers {
    opentelekomcloud = {
      source  = "opentelekomcloud/opentelekomcloud"
      version = "~> 1.36"
    }
  }
}

# =====================================================
# Pre-deployed Shared ELB (Option B)
#
# Kubernetes Services nutzen diesen ELB via Annotation:
#   kubernetes.io/elb.id: <elb_id>
#
# Mehrere Services → verschiedene Listener-Ports
# CCM fügt automatisch Pools + Members hinzu
# =====================================================

resource "opentelekomcloud_lb_loadbalancer_v3" "shared" {
  name        = "${var.cluster_name}-shared-elb"
  description = "Shared ELB — CCM-managed via kubernetes.io/elb.id annotation"

  # Placement
  availability_zone = [var.availability_zone]
  router_id         = var.vpc_id
  subnet_id         = var.subnet_network_id  # Neutron subnet ID (vip_subnet_cidr_id)
  network_ids       = [var.subnet_id]         # VPC subnet ID

  # Flavors — dedicated ELB
  l4_flavor = "L4_flavor.elb.s1.small"
  l7_flavor = "L7_flavor.elb.s1.small"

  lifecycle {
    ignore_changes = [tags]
  }
}

# EIP für den shared ELB
resource "opentelekomcloud_vpc_eip_v1" "shared_elb" {
  publicip {
    type = "5_bgp"
  }
  bandwidth {
    name        = "${var.cluster_name}-shared-elb-bw"
    size        = 10
    share_type  = "PER"
    charge_mode = "traffic"
  }

  lifecycle {
    ignore_changes = [tags]
  }
}

# EIP an ELB binden
resource "opentelekomcloud_lb_eip_associate_v3" "shared" {
  loadbalancer_id = opentelekomcloud_lb_loadbalancer_v3.shared.id
  publicip_id     = opentelekomcloud_vpc_eip_v1.shared_elb.id
}
