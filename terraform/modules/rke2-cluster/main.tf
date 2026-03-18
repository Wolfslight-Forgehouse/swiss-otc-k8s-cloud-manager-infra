locals {
  master_userdata = <<-USERDATA
    #!/bin/bash
    set -e
    # Install RKE2 Server
    curl -sfL https://get.rke2.io | sh -
    mkdir -p /etc/rancher/rke2

    cat > /etc/rancher/rke2/config.yaml << CONFIG
    node-name: ${var.cluster_name}-master
    cloud-provider-name: openstack
    cloud-provider-config: /etc/rancher/rke2/cloud.conf
    tls-san:
      - $(curl -s http://169.254.169.254/latest/meta-data/public-ipv4)
      - $(hostname -I | awk '{print $1}')
    CONFIG

    systemctl enable rke2-server.service
    systemctl start rke2-server.service

    # Fix kubeconfig permissions
    sleep 30
    chmod 644 /etc/rancher/rke2/rke2.yaml
  USERDATA
}

resource "opentelekomcloud_compute_instance_v2" "master" {
  name            = "${var.cluster_name}-master"
  image_name      = var.image_name
  flavor_name     = var.master_flavor
  key_pair        = var.key_pair
  security_groups = [var.secgroup_id]
  user_data       = local.master_userdata

  network {
    uuid = var.subnet_id
  }

  metadata = {
    cluster = var.cluster_name
    role    = "master"
  }
}

resource "opentelekomcloud_networking_floatingip_v2" "master" {
  pool = "admin_external_net"
}

resource "opentelekomcloud_compute_floatingip_associate_v2" "master" {
  floating_ip = opentelekomcloud_networking_floatingip_v2.master.address
  instance_id = opentelekomcloud_compute_instance_v2.master.id
}

resource "opentelekomcloud_compute_instance_v2" "worker" {
  count           = var.worker_count
  name            = "${var.cluster_name}-worker-${count.index + 1}"
  image_name      = var.image_name
  flavor_name     = var.worker_flavor
  key_pair        = var.key_pair
  security_groups = [var.secgroup_id]

  user_data = <<-USERDATA
    #!/bin/bash
    set -e
    curl -sfL https://get.rke2.io | INSTALL_RKE2_TYPE="agent" sh -
    mkdir -p /etc/rancher/rke2

    # Wait for master token
    sleep 60

    cat > /etc/rancher/rke2/config.yaml << CONFIG
    server: https://${opentelekomcloud_compute_instance_v2.master.network[0].fixed_ip_v4}:9345
    token: $(ssh -o StrictHostKeyChecking=no ubuntu@${opentelekomcloud_networking_floatingip_v2.master.address} 'sudo cat /var/lib/rancher/rke2/server/node-token' 2>/dev/null || echo 'PENDING')
    node-name: ${var.cluster_name}-worker-${count.index + 1}
    cloud-provider-name: openstack
    CONFIG

    systemctl enable rke2-agent.service
    systemctl start rke2-agent.service
  USERDATA

  network {
    uuid = var.subnet_id
  }

  metadata = {
    cluster = var.cluster_name
    role    = "worker"
  }
}
