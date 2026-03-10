terraform {
  required_providers {
    opentelekomcloud = {
      source  = "opentelekomcloud/opentelekomcloud"
      version = "~> 1.36"
    }
  }
}

data "opentelekomcloud_images_image_v2" "ubuntu" {
  name        = "Standard_Ubuntu_22.04_latest"
  most_recent = true
}

# Trigger for VM recreation when SSH key changes
resource "terraform_data" "ssh_key_trigger" {
  input = var.ssh_key_hash
}

resource "opentelekomcloud_compute_instance_v2" "master" {
  name              = "${var.cluster_name}-master"
  flavor_id         = var.master_flavor
  key_pair          = var.keypair_name
  security_groups   = [var.security_group_id]
  availability_zone = var.az

  block_device {
    uuid                  = data.opentelekomcloud_images_image_v2.ubuntu.id
    source_type           = "image"
    destination_type      = "volume"
    volume_size           = 40
    boot_index            = 0
    delete_on_termination = true
  }

  network {
    uuid = var.subnet_id
  }

  user_data = base64encode(templatefile("${path.module}/templates/master-init.sh.tpl", {
    cluster_token = var.cluster_token
    proxy_host    = var.proxy_host
  }))

  lifecycle {
    ignore_changes       = [tags, image_id, image_name]
    replace_triggered_by = [terraform_data.ssh_key_trigger]
  }
}

resource "opentelekomcloud_compute_instance_v2" "worker" {
  count             = var.worker_count
  name              = "${var.cluster_name}-worker-${count.index + 1}"
  flavor_id         = var.worker_flavor
  key_pair          = var.keypair_name
  security_groups   = [var.security_group_id]
  availability_zone = var.az

  block_device {
    uuid                  = data.opentelekomcloud_images_image_v2.ubuntu.id
    source_type           = "image"
    destination_type      = "volume"
    volume_size           = 40
    boot_index            = 0
    delete_on_termination = true
  }

  network {
    uuid = var.subnet_id
  }

  user_data = base64encode(templatefile("${path.module}/templates/worker-init.sh.tpl", {
    cluster_token = var.cluster_token
    master_ip     = opentelekomcloud_compute_instance_v2.master.access_ip_v4
    proxy_host    = var.proxy_host
  }))

  lifecycle {
    ignore_changes       = [tags, image_id, image_name]
    replace_triggered_by = [terraform_data.ssh_key_trigger]
  }

  depends_on = [opentelekomcloud_compute_instance_v2.master]
}
