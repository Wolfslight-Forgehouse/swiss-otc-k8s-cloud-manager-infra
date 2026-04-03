# Additional outputs beyond those defined inline in main.tf

output "worker_ips_json" {
  description = "Worker IPs as JSON array for pipeline use"
  value       = jsonencode(module.compute.worker_ips)
}

output "dns_zone" {
  description = "OTC Private DNS Zone"
  value       = length(module.dns) > 0 ? module.dns[0].dns_zone_name : "disabled"
}

output "dns_zone_id" {
  description = "OTC Private DNS Zone ID"
  value       = length(module.dns) > 0 ? module.dns[0].dns_zone_id : ""
}
