variable "otc_access_key" {
  description = "OTC Access Key ID"
  type        = string
  sensitive   = true
}

variable "otc_secret_key" {
  description = "OTC Secret Access Key"
  type        = string
  sensitive   = true
}

variable "otc_domain_name" {
  description = "OTC Domain/Tenant Name"
  type        = string
}

variable "otc_project_id" {
  description = "OTC Project ID"
  type        = string
}

variable "region" {
  description = "OTC Region"
  type        = string
  default     = "eu-ch2"
}

variable "cluster_name" {
  description = "RKE2 Cluster Name"
  type        = string
  default     = "rke2-demo"
}

variable "vpc_cidr" {
  description = "VPC CIDR Block"
  type        = string
  default     = "10.0.0.0/16"
}

variable "subnet_cidr" {
  description = "Subnet CIDR"
  type        = string
  default     = "10.0.1.0/24"
}

variable "master_flavor" {
  description = "Flavor for master nodes"
  type        = string
  default     = "s3.xlarge.4"
}

variable "worker_flavor" {
  description = "Flavor for worker nodes"
  type        = string
  default     = "s3.large.4"
}

variable "worker_count" {
  description = "Number of worker nodes"
  type        = number
  default     = 2
}

variable "image_name" {
  description = "OS Image (Ubuntu 22.04)"
  type        = string
  default     = "Standard_Ubuntu_22.04_latest"
}

variable "key_pair" {
  description = "SSH Key Pair Name in OTC"
  type        = string
}
