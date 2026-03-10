output "master_ip" {
  value = opentelekomcloud_compute_instance_v2.master.access_ip_v4
}

output "master_id" {
  value = opentelekomcloud_compute_instance_v2.master.id
}

output "worker_ips" {
  value = opentelekomcloud_compute_instance_v2.worker[*].access_ip_v4
}

output "worker_ids" {
  value = opentelekomcloud_compute_instance_v2.worker[*].id
}
