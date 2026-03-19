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
  availability_zones = [var.availability_zone]
  subnet_id          = var.subnet_network_id  # Neutron subnet ID (opentelekomcloud_vpc_subnet_v1.subnet_id)
  network_ids        = [var.subnet_id]         # VPC subnet ID (opentelekomcloud_vpc_subnet_v1.id)

  # Public IP — direkt im ELB Resource
  public_ip {
    bandwidth_name       = "${var.cluster_name}-shared-elb-bw"
    ip_type              = "5_bgp"
    bandwidth_size       = 10
    bandwidth_share_type = "PER"
    bandwidth_charge_mode = "traffic"
  }

  lifecycle {
    ignore_changes = [tags]
  }
}
