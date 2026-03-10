variable "cluster_name" {
  type = string
}

variable "environment" {
  type = string
}

variable "flavor" {
  type    = string
  default = "s3.medium.1"
}

variable "az" {
  type    = string
  default = "eu-ch2a"
}

variable "subnet_id" {
  type = string
}

variable "security_group_id" {
  type = string
}

variable "keypair_name" {
  type = string
}

variable "ssh_public_key" {
  type = string
}

variable "ssh_key_hash" {
  type        = string
  description = "Hash of SSH public key — triggers VM recreation on key change"
  default     = ""
}

variable "github_runner_token" {
  type        = string
  description = "GitHub Actions runner registration token"
  sensitive   = true
  default     = ""
}

variable "github_repo_url" {
  type        = string
  description = "GitHub repository URL for runner registration"
  default     = "https://github.com/Wolfslight-Forgehouse/swiss-otc-cloud-manager"
}

variable "github_runner_labels" {
  type        = string
  description = "Comma-separated labels for the runner"
  default     = "self-hosted,linux,x64,otc"
}

variable "ghcr_pull_token" {
  type        = string
  description = "GitHub PAT with read:packages for GHCR image pulls"
  sensitive   = true
  default     = ""
}

variable "ssh_private_key" {
  type        = string
  description = "SSH private key for accessing cluster nodes"
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

variable "gitlab_runner_tags" {
  type        = string
  description = "Comma-separated tags for the GitLab runner"
  default     = "self-hosted,linux,x64,otc"
}
