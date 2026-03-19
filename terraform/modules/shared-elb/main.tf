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

# EIP für shared ELB
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

resource "opentelekomcloud_lb_loadbalancer_v3" "shared" {
  name        = "${var.cluster_name}-shared-elb"
  description = "Shared ELB — CCM-managed via kubernetes.io/elb.id annotation"

  availability_zones = [var.availability_zone]
  subnet_id          = var.subnet_network_id  # Neutron subnet ID
  network_ids        = [var.subnet_id]         # VPC subnet ID

  # EIP direkt binden
  public_ip {
    id = opentelekomcloud_vpc_eip_v1.shared_elb.id
  }

  lifecycle {
    ignore_changes = [tags]
  }
}
