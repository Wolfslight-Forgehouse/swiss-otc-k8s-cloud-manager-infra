#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
ENV="${1:-demo}"

echo "🚀 Bootstrapping RKE2 Cluster on Swiss OTC (env: $ENV)"
echo "======================================================="

# Step 1: Terraform
echo ""
echo "📦 Step 1: Provisioning infrastructure with Terraform..."
cd "$ROOT_DIR/terraform/environments/$ENV"
terraform init
terraform apply -auto-approve

MASTER_IP=$(terraform output -raw master_ip)
echo "✅ Master IP: $MASTER_IP"

# Step 2: Wait for RKE2
echo ""
echo "⏳ Step 2: Waiting for RKE2 to be ready (up to 10 minutes)..."
for i in $(seq 1 60); do
  if ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 ubuntu@$MASTER_IP \
      'sudo systemctl is-active rke2-server' 2>/dev/null | grep -q "^active$"; then
    echo "✅ RKE2 server is active"
    break
  fi
  echo "  Attempt $i/60 — waiting 10s..."
  sleep 10
done

# Step 3: Fetch kubeconfig
echo ""
echo "🔑 Step 3: Fetching kubeconfig..."
ssh -o StrictHostKeyChecking=no ubuntu@$MASTER_IP \
  'sudo cat /etc/rancher/rke2/rke2.yaml' | \
  sed "s/127.0.0.1/$MASTER_IP/g" > "$ROOT_DIR/kubeconfig-$ENV"
echo "✅ Saved to kubeconfig-$ENV"

export KUBECONFIG="$ROOT_DIR/kubeconfig-$ENV"

# Step 4: Wait for nodes
echo ""
echo "🔍 Step 4: Waiting for all nodes to be Ready..."
kubectl wait --for=condition=Ready nodes --all --timeout=300s
echo "✅ All nodes ready:"
kubectl get nodes -o wide

# Step 5: Install Cloud Manager
echo ""
echo "☁️  Step 5: Installing OTC Cloud Manager..."
"$SCRIPT_DIR/install-cloud-manager.sh" $ENV

echo ""
echo "🎉 Cluster bootstrap complete!"
echo "   kubeconfig: $ROOT_DIR/kubeconfig-$ENV"
echo "   Nodes: $(kubectl get nodes --no-headers | wc -l)"
