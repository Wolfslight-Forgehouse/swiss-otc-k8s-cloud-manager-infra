variable "dns_zone" {
  description = "Private DNS Zone (ohne trailing dot), z.B. 'sotc.internal'"
  type        = string
  default     = "sotc.internal"
}

variable "vpc_id" {
  description = "OTC VPC ID — DNS Zone wird an diesen VPC gebunden"
  type        = string
}

variable "traefik_elb_ip" {
  description = "IP der Traefik ELB (nach Cluster-Apply befüllen)"
  type        = string
  default     = ""  # Leer = DNS-Records werden übersprungen
}

variable "dns_contact_email" {
  description = "E-Mail für DNS Zone"
  type        = string
  default     = "ops@sotc.internal"
}
