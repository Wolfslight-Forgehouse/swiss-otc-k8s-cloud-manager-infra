# OTC Private DNS Zone + Records für Cluster-Services
# Provider: opentelekomcloud/opentelekomcloud
# Resources: opentelekomcloud_dns_zone_v2 + opentelekomcloud_dns_recordset_v2

resource "opentelekomcloud_dns_zone_v2" "private" {
  name        = "${var.dns_zone}."
  email       = var.dns_contact_email
  description = "Private DNS Zone für RKE2 Cluster-Services"
  ttl         = 300
  type        = "private"

  router {
    router_id     = var.vpc_id
    router_region = "eu-ch2"
  }
}

# Traefik ELB — Haupteinstiegspunkt
resource "opentelekomcloud_dns_recordset_v2" "traefik" {
  count   = var.traefik_elb_ip != "" ? 1 : 0
  zone_id = opentelekomcloud_dns_zone_v2.private.id
  name    = "traefik.${var.dns_zone}."
  type    = "A"
  ttl     = 60
  records = [var.traefik_elb_ip]
}

# Wildcard → Traefik (alle *.sotc.internal → Traefik ELB)
resource "opentelekomcloud_dns_recordset_v2" "wildcard" {
  count   = var.traefik_elb_ip != "" ? 1 : 0
  zone_id = opentelekomcloud_dns_zone_v2.private.id
  name    = "*.${var.dns_zone}."
  type    = "A"
  ttl     = 60
  records = [var.traefik_elb_ip]
}

# Service-CNAMEs (immer erstellen, zeigen auf traefik.sotc.internal)
resource "opentelekomcloud_dns_recordset_v2" "argocd" {
  zone_id = opentelekomcloud_dns_zone_v2.private.id
  name    = "argocd.${var.dns_zone}."
  type    = "CNAME"
  ttl     = 300
  records = ["traefik.${var.dns_zone}."]

  depends_on = [opentelekomcloud_dns_recordset_v2.traefik]
}

resource "opentelekomcloud_dns_recordset_v2" "grafana" {
  zone_id = opentelekomcloud_dns_zone_v2.private.id
  name    = "grafana.${var.dns_zone}."
  type    = "CNAME"
  ttl     = 300
  records = ["traefik.${var.dns_zone}."]

  depends_on = [opentelekomcloud_dns_recordset_v2.traefik]
}

resource "opentelekomcloud_dns_recordset_v2" "prometheus" {
  zone_id = opentelekomcloud_dns_zone_v2.private.id
  name    = "prometheus.${var.dns_zone}."
  type    = "CNAME"
  ttl     = 300
  records = ["traefik.${var.dns_zone}."]

  depends_on = [opentelekomcloud_dns_recordset_v2.traefik]
}
