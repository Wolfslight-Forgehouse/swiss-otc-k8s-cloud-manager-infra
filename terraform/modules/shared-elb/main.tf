terraform {
  required_providers {
    opentelekomcloud = {
      source  = "opentelekomcloud/opentelekomcloud"
      version = "~> 1.36"
    }
  }
}

# EIP — nur wenn enable_eip = true
resource "opentelekomcloud_vpc_eip_v1" "shared_elb" {
  count = var.enable_eip ? 1 : 0

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
  description = "Shared ELB — CCM via kubernetes.io/elb.id | EIP: ${var.enable_eip}"

  availability_zones = [var.availability_zone]
  subnet_id          = var.subnet_network_id
  network_ids        = [var.subnet_id]

  # EIP nur binden wenn vorhanden
  dynamic "public_ip" {
    for_each = var.enable_eip ? [1] : []
    content {
      id = opentelekomcloud_vpc_eip_v1.shared_elb[0].id
    }
  }

  lifecycle {
    ignore_changes = [tags]
  }
}
