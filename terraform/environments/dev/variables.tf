variable "access_key" {
  type        = string
  description = "OTC Access Key ID (AK/SK authentication)"
  sensitive   = true
}

variable "secret_key" {
  type        = string
  description = "OTC Secret Access Key"
  sensitive   = true
}

variable "domain_name" {
  type        = string
  description = "OTC domain name (e.g. OTC00000000001000000xxx)"
  default     = ""
}

variable "project_id" {
  type        = string
  description = "OTC project ID"
}

variable "project_name" {
  type        = string
  description = "OTC project name (e.g. eu-ch2_myproject)"
  default     = ""
}

variable "cluster_name" {
  type        = string
  description = "Name prefix for all resources"
  default     = "rke2-dev"
}

variable "ssh_public_key" {
  type        = string
  description = "SSH public key for VM access"
}

variable "ssh_private_key" {
  type        = string
  description = "SSH private key for bastion to master access"
  sensitive   = true
  default     = ""
}

variable "rke2_token" {
  type        = string
  description = "Pre-shared token for RKE2 node join"
  sensitive   = true
}

variable "github_runner_token" {
  type        = string
  description = "GitHub Actions runner registration token"
  sensitive   = true
  default     = ""
}

variable "ghcr_pull_token" {
  type        = string
  description = "GitHub Container Registry pull token"
  sensitive   = true
  default     = ""
}

variable "gitlab_url" {
  type        = string
  description = "GitLab instance URL for runner registration"
  default     = ""
}

variable "gitlab_runner_token" {
  type        = string
  description = "GitLab Runner registration token"
  sensitive   = true
  default     = ""
}

variable "environment" {
  type        = string
  description = "Environment name (dev, staging, prod)"
  default     = "dev"
}

# =====================================================
# Shared ELB (pre-deployed, Terraform-managed)
# =====================================================
variable "enable_shared_elb" {
  type        = bool
  description = "Pre-deployed shared ELB erstellen (Terraform-managed, CCM nutzt ihn via elb.id)"
  default     = false
}

variable "shared_elb_eip" {
  type        = bool
  description = "Shared ELB mit öffentlicher EIP ausstatten (nur wenn enable_shared_elb = true)"
  default     = false
}

# =====================================================
# CCM Ingress / ELB Konfiguration
# =====================================================
variable "ccm_elb_eip" {
  type        = bool
  description = "CCM-managed ELBs bekommen EIP (public). Wenn true: nginx-public IngressClass wird deployed"
  default     = true
}

variable "deploy_ingress_nginx" {
  type        = bool
  description = "ingress-nginx deployen (nginx-internal immer, nginx-public wenn ccm_elb_eip = true)"
  default     = false
}

variable "cni_plugin" {
  type        = string
  description = "CNI plugin: 'cilium' (default) or 'kube-ovn'"
  default     = "cilium"
}

variable "dns_zone" {
  description = "OTC Private DNS Zone"
  type        = string
  default     = "sotc.internal"
}

variable "traefik_elb_ip" {
  description = "Traefik ELB IP (nach erstem Apply befüllen)"
  type        = string
  default     = ""
}

variable "enable_private_dns" {
  description = "OTC Private DNS aktivieren (benötigt IAM-Rolle dns_adm oder te_admin)"
  type        = bool
  default     = false
}
