#!/bin/bash
set -euo pipefail

ENV="${1:-demo}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

export KUBECONFIG="${KUBECONFIG:-$ROOT_DIR/kubeconfig-$ENV}"

# Required env vars
: "${OTC_AUTH_URL:?OTC_AUTH_URL required}"
: "${OTC_USERNAME:?OTC_USERNAME required}"
: "${OTC_PASSWORD:?OTC_PASSWORD required}"
: "${OTC_PROJECT_ID:?OTC_PROJECT_ID required}"
: "${OTC_DOMAIN_NAME:?OTC_DOMAIN_NAME required}"
: "${OTC_TENANT_NAME:?OTC_TENANT_NAME required}"
: "${OTC_ELB_SUBNET_ID:?OTC_ELB_SUBNET_ID required}"
: "${OTC_FLOATING_NETWORK_ID:?OTC_FLOATING_NETWORK_ID required}"
: "${OTC_NETWORK_ID:?OTC_NETWORK_ID required}"

echo "☁️  Installing OTC Cloud Manager (env: $ENV)..."

helm upgrade --install otc-cloud-manager "$ROOT_DIR/helm/otc-cloud-manager/" \
  -f "$ROOT_DIR/helm/otc-cloud-manager/values-demo.yaml" \
  --namespace kube-system \
  --set otc.authUrl="$OTC_AUTH_URL" \
  --set otc.username="$OTC_USERNAME" \
  --set otc.password="$OTC_PASSWORD" \
  --set otc.projectId="$OTC_PROJECT_ID" \
  --set otc.domainName="$OTC_DOMAIN_NAME" \
  --set otc.tenantName="$OTC_TENANT_NAME" \
  --set otc.elb.subnetId="$OTC_ELB_SUBNET_ID" \
  --set otc.elb.floatingNetworkId="$OTC_FLOATING_NETWORK_ID" \
  --set otc.elb.networkId="$OTC_NETWORK_ID" \
  --wait --timeout=5m

echo "✅ Cloud Manager deployed"
kubectl get pods -n kube-system -l app=otc-cloud-manager
