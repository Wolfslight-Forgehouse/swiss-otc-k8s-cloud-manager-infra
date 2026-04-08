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

# --- ExternalDNS Integration Outputs ---

output "externaldns_zone_id" {
  description = "DNS Zone ID for ExternalDNS Designate provider configuration"
  value       = opentelekomcloud_dns_zone_v2.private.id
}

output "externaldns_zone_name" {
  description = "DNS Zone name for ExternalDNS domainFilter configuration"
  value       = var.dns_zone
}

output "externaldns_managed_domains" {
  description = "List of domains ExternalDNS is configured to manage"
  value       = var.externaldns_managed_domains
}

output "externaldns_txt_owner_id" {
  description = "TXT owner ID used by ExternalDNS for record ownership"
  value       = var.externaldns_txt_owner_id
}
