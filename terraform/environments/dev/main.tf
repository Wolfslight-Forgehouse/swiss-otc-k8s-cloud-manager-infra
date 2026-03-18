terraform {
  required_version = ">= 1.5"

  required_providers {
    opentelekomcloud = {
      source  = "opentelekomcloud/opentelekomcloud"
      version = "~> 1.36"
    }
  }
}

provider "opentelekomcloud" {
  access_key  = var.access_key
  secret_key  = var.secret_key
  domain_name = var.domain_name
  tenant_id   = var.project_id
  tenant_name = var.project_name
  auth_url    = "https://iam-pub.eu-ch2.sc.otc.t-systems.com/v3"
  region      = "eu-ch2"
}

# =====================================================
# 1. Networking — VPC, Subnet, Security Groups, Keypair
# =====================================================
module "networking" {
  source         = "../../modules/networking"
  cluster_name   = var.cluster_name
  environment    = var.environment
  vpc_cidr       = "10.0.0.0/16"
  subnet_cidr    = "10.0.1.0/24"
  ssh_public_key = var.ssh_public_key
}

# =====================================================
# 2. Jumpserver / Bastion — SSH access + TinyProxy
#    Provides outbound internet for cluster nodes
#    (replaces NAT Gateway — no NAT admin role needed)
# =====================================================
module "jumpserver" {
  source              = "../../modules/jumpserver"
  cluster_name        = var.cluster_name
  environment         = var.environment
  subnet_id           = module.networking.subnet_id
  security_group_id   = module.networking.security_group_id
  keypair_name        = module.networking.keypair_name
  ssh_public_key      = var.ssh_public_key
  ssh_key_hash        = module.networking.ssh_public_key_hash
  ssh_private_key     = var.ssh_private_key
  github_runner_token = var.github_runner_token
  ghcr_pull_token     = var.ghcr_pull_token
  gitlab_url          = var.gitlab_url
  gitlab_runner_token = var.gitlab_runner_token

  depends_on = [module.networking]
}

# =====================================================
# 3. Compute — RKE2 Master + Workers
#    Uses jumpserver TinyProxy for outbound internet
#
#    ORDER: networking → jumpserver → compute
# =====================================================
module "compute" {
  source            = "../../modules/compute"
  cluster_name      = var.cluster_name
  environment       = var.environment
  master_flavor     = "s3.large.2"
  worker_flavor     = "s3.large.2"
  worker_count      = 2
  subnet_id         = module.networking.subnet_id
  security_group_id = module.networking.security_group_id
  keypair_name      = module.networking.keypair_name
  cluster_token     = var.rke2_token
  ssh_key_hash      = module.networking.ssh_public_key_hash
  proxy_host        = module.jumpserver.private_ip

  depends_on = [module.networking, module.jumpserver]
}

# =====================================================
# Outputs
# =====================================================
output "bastion_ip" {
  description = "Bastion/Jumpserver public IP (SSH + TinyProxy)"
  value       = module.jumpserver.floating_ip
}

output "bastion_private_ip" {
  description = "Bastion internal IP (proxy endpoint for nodes)"
  value       = module.jumpserver.private_ip
}

output "master_ip" {
  description = "RKE2 master internal IP"
  value       = module.compute.master_ip
}

output "worker_ips" {
  description = "RKE2 worker internal IPs"
  value       = module.compute.worker_ips
}

output "vpc_id" {
  value = module.networking.vpc_id
}

output "subnet_id" {
  value = module.networking.subnet_id
}

output "security_group_id" {
  value = module.networking.security_group_id
}

output "subnet_network_id" {
  description = "Neutron subnet ID (for ELB vip_subnet_cidr_id)"
  value       = module.networking.subnet_network_id
}
