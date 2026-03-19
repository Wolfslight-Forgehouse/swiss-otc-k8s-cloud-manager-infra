output "elb_id" {
  description = "ELB ID — für kubernetes.io/elb.id Annotation"
  value       = opentelekomcloud_lb_loadbalancer_v3.shared.id
}

output "elb_vip" {
  description = "ELB VIP (private, interne IP)"
  value       = opentelekomcloud_lb_loadbalancer_v3.shared.vip_address
}

output "public_ip" {
  description = "Public EIP des shared ELB"
  value       = opentelekomcloud_vpc_eip_v1.shared_elb.publicip[0].ip_address
}
