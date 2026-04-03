# Additional outputs beyond those defined inline in main.tf

output "worker_ips_json" {
  description = "Worker IPs as JSON array for pipeline use"
  value       = jsonencode(module.compute.worker_ips)
}

output "dns_zone" {
  description = "OTC Private DNS Zone"
  value       = module.dns.dns_zone_name
}

output "dns_zone_id" {
  description = "OTC Private DNS Zone ID"
  value       = module.dns.dns_zone_id
}
