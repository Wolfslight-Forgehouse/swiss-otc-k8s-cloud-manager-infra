variable "cluster_name" {
  type = string
}

variable "vpc_id" {
  type        = string
  description = "VPC ID (router_id für ELB)"
}

variable "subnet_id" {
  type        = string
  description = "VPC Subnet ID (network_ids für ELB)"
}

variable "subnet_network_id" {
  type        = string
  description = "Neutron Subnet ID (vip_subnet_cidr_id) — opentelekomcloud_vpc_subnet_v1.subnet_id"
}

variable "availability_zone" {
  type    = string
  default = "eu-ch2a"
}
