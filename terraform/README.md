# Terraform — RKE2 Swiss OTC

## Module

- `modules/otc-networking` — VPC, Subnet, Router, Security Groups
- `modules/rke2-cluster` — ECS Instances (Master + Worker), Floating IPs

## Environments

- `environments/demo` — 1 Master (s3.xlarge.4) + 2 Worker (s3.large.4)

## Backend

OTC Object Storage (OBS) — Bucket `rke2-sotc-tfstate` in eu-ch2.

## Provider

[opentelekomcloud/opentelekomcloud](https://registry.terraform.io/providers/opentelekomcloud/opentelekomcloud)
