output "dns_zone_id" {
  description = "OTC DNS Zone ID"
  value       = opentelekomcloud_dns_zone_v2.private.id
}

output "dns_zone_name" {
  description = "DNS Zone Name"
  value       = var.dns_zone
}

output "wildcard_record" {
  description = "Wildcard DNS Record (*.sotc.internal → Traefik ELB)"
  value       = var.traefik_elb_ip != "" ? "*.${var.dns_zone} → ${var.traefik_elb_ip}" : "*.${var.dns_zone} → (ELB IP noch nicht gesetzt)"
}
