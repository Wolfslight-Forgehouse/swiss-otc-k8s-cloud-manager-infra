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

resource "terraform_data" "ssh_key_trigger" {
  input = var.ssh_key_hash
}

resource "opentelekomcloud_compute_instance_v2" "jumpserver" {
  name              = "${var.cluster_name}-jumpserver"
  flavor_id         = var.flavor
  key_pair          = var.keypair_name
  security_groups   = [var.security_group_id]
  availability_zone = var.az


  block_device {
    uuid                  = data.opentelekomcloud_images_image_v2.ubuntu.id
    source_type           = "image"
    destination_type      = "volume"
    volume_size           = 20
    boot_index            = 0
    delete_on_termination = true
  }
  network {
    uuid = var.subnet_id
  }

  user_data = base64encode(templatefile("${path.module}/templates/jumpserver-init.sh.tpl", {
    ssh_public_key       = var.ssh_public_key
    ssh_private_key      = var.ssh_private_key
    github_runner_token  = var.github_runner_token
    github_repo_url      = var.github_repo_url
    github_runner_labels = var.github_runner_labels
    ghcr_pull_token      = var.ghcr_pull_token
    gitlab_url           = var.gitlab_url
    gitlab_runner_token  = var.gitlab_runner_token
    gitlab_runner_tags   = var.gitlab_runner_tags
  }))

  lifecycle {
    ignore_changes       = [tags, image_id, image_name]
    replace_triggered_by = [terraform_data.ssh_key_trigger]
  }
}

resource "opentelekomcloud_networking_floatingip_v2" "jumpserver" {}

resource "opentelekomcloud_networking_floatingip_associate_v2" "jumpserver" {
  floating_ip = opentelekomcloud_networking_floatingip_v2.jumpserver.address
  port_id     = opentelekomcloud_compute_instance_v2.jumpserver.network[0].port
}
