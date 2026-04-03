# OTC Private DNS Zone + Records für Cluster-Services
# Terraform Resource: openstack_dns_zone_v2 (Designate)

resource "openstack_dns_zone_v2" "private" {
  name        = "${var.dns_zone}."
  email       = var.dns_contact_email
  description = "Private DNS Zone für RKE2 Cluster-Services"
  ttl         = 300
  type        = "PRIVATE"

  # OTC: Zone an VPC binden
  router {
    router_id = var.vpc_id
  }
}

# Traefik ELB — dynamisch nach Apply befüllt
resource "openstack_dns_recordset_v2" "traefik" {
  zone_id     = openstack_dns_zone_v2.private.id
  name        = "traefik.${var.dns_zone}."
  description = "Traefik Ingress LoadBalancer"
  ttl         = 60
  type        = "A"
  records     = [var.traefik_elb_ip]
}

# Wildcard → Traefik (alle *.sotc.internal → Traefik ELB)
resource "openstack_dns_recordset_v2" "wildcard" {
  zone_id     = openstack_dns_zone_v2.private.id
  name        = "*.${var.dns_zone}."
  description = "Wildcard → Traefik ELB"
  ttl         = 60
  type        = "A"
  records     = [var.traefik_elb_ip]
}

# Service-spezifische A-Records
resource "openstack_dns_recordset_v2" "argocd" {
  zone_id     = openstack_dns_zone_v2.private.id
  name        = "argocd.${var.dns_zone}."
  description = "ArgoCD GitOps UI"
  ttl         = 300
  type        = "CNAME"
  records     = ["traefik.${var.dns_zone}."]
}

resource "openstack_dns_recordset_v2" "grafana" {
  zone_id     = openstack_dns_zone_v2.private.id
  name        = "grafana.${var.dns_zone}."
  description = "Grafana Monitoring UI"
  ttl         = 300
  type        = "CNAME"
  records     = ["traefik.${var.dns_zone}."]
}

resource "openstack_dns_recordset_v2" "prometheus" {
  zone_id     = openstack_dns_zone_v2.private.id
  name        = "prometheus.${var.dns_zone}."
  description = "Prometheus"
  ttl         = 300
  type        = "CNAME"
  records     = ["traefik.${var.dns_zone}."]
}
