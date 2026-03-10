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
