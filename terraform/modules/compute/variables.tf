variable "cluster_name" {
  type = string
}

variable "environment" {
  type = string
}

variable "master_flavor" {
  type    = string
  default = "s3.large.2"
}

variable "worker_flavor" {
  type    = string
  default = "s3.large.2"
}

variable "worker_count" {
  type    = number
  default = 2
}

variable "az" {
  type    = string
  default = "eu-ch2a"
}

variable "subnet_id" {
  type = string
}

variable "security_group_id" {
  type        = string
  description = "Security group ID (not name!) for proper Terraform dependency tracking"
}

variable "keypair_name" {
  type        = string
  description = "Name of pre-created SSH keypair from networking module"
}

variable "cluster_token" {
  type      = string
  sensitive = true
}

variable "proxy_host" {
  type        = string
  description = "Internal IP of proxy server (jumpserver) for outbound internet"
  default     = ""
}

variable "ssh_key_hash" {
  type        = string
  description = "Hash of SSH public key — triggers VM recreation on key change"
  default     = ""
}

variable "obs_access_key" {
  description = "OTC AK for geesefs binary download from OBS (cloud-init)"
  type        = string
  sensitive   = true
}

variable "obs_secret_key" {
  description = "OTC SK for geesefs binary download from OBS (cloud-init)"
  type        = string
  sensitive   = true
}
