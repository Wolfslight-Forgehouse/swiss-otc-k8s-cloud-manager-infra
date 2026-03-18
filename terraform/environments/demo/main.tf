terraform {
  required_providers {
    opentelekomcloud = {
      source  = "opentelekomcloud/opentelekomcloud"
      version = "~> 1.36"
    }
  }
  required_version = ">= 1.5.0"
}

provider "opentelekomcloud" {
  access_key  = var.otc_access_key
  secret_key  = var.otc_secret_key
  domain_name = var.otc_domain_name
  tenant_name = var.otc_project_id
  region      = var.region
  auth_url    = "https://iam.eu-ch2.otc.t-systems.com/v3"
}

module "networking" {
  source = "../../modules/otc-networking"

  cluster_name = var.cluster_name
  vpc_cidr     = var.vpc_cidr
  subnet_cidr  = var.subnet_cidr
  region       = var.region
}

module "rke2_cluster" {
  source = "../../modules/rke2-cluster"

  cluster_name      = var.cluster_name
  region            = var.region
  vpc_id            = module.networking.vpc_id
  subnet_id         = module.networking.subnet_id
  secgroup_id       = module.networking.secgroup_id
  master_flavor     = var.master_flavor
  worker_flavor     = var.worker_flavor
  worker_count      = var.worker_count
  image_name        = var.image_name
  key_pair          = var.key_pair
}

output "master_ip" {
  description = "Master node floating IP"
  value       = module.rke2_cluster.master_floating_ip
}

output "worker_ips" {
  description = "Worker node IPs"
  value       = module.rke2_cluster.worker_ips
}

output "kubeconfig_command" {
  description = "Command to fetch kubeconfig"
  value       = "ssh ubuntu@${module.rke2_cluster.master_floating_ip} 'sudo cat /etc/rancher/rke2/rke2.yaml'"
}
