# Additional outputs beyond those defined inline in main.tf

output "worker_ips_json" {
  description = "Worker IPs as JSON array for pipeline use"
  value       = jsonencode(module.compute.worker_ips)
}
