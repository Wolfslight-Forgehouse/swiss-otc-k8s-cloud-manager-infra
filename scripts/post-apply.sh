#!/bin/bash
# post-apply.sh — Automatic cluster configuration after Terraform Apply
#
# Runs on the bastion host (self-hosted runner) after infrastructure is created.
# Fetches kubeconfig from master, creates K8s secrets, removes taints.
#
# Required environment variables:
#   MASTER_IP         — Master node internal IP (from Terraform output)
#   SSH_DEPLOY_KEY    — Path to SSH key for master access (default: ~/.ssh/deploy-key)
#   CLOUD_CONFIG      — cloud.conf content (for CCM K8s secret)
#   GHCR_PULL_USER    — GitHub Container Registry username (optional)
#   GHCR_PULL_TOKEN   — GitHub Container Registry token (optional)
#
# Optional (for GitHub Secret auto-update):
#   GH_TOKEN          — GitHub token with repo scope
#   GH_REPO           — owner/repo (e.g. Wolfslight-Forgehouse/swiss-otc-cloud-manager)

set -euo pipefail

MASTER_IP="${MASTER_IP:?MASTER_IP is required}"
SSH_KEY="${SSH_DEPLOY_KEY:-$HOME/.ssh/deploy-key}"
MAX_RETRIES=30
RETRY_INTERVAL=10

echo "============================================"
echo "  Post-Apply Configuration"
echo "  Master: ${MASTER_IP}"
echo "============================================"

# ─────────────────────────────────────────────
# 1. Wait for cluster to be ready
# ─────────────────────────────────────────────
echo ""
echo ">>> Step 1: Waiting for RKE2 cluster..."

for i in $(seq 1 $MAX_RETRIES); do
  if ssh -i "$SSH_KEY" -o StrictHostKeyChecking=no -o ConnectTimeout=5 \
    ubuntu@"$MASTER_IP" "sudo /var/lib/rancher/rke2/bin/kubectl \
    --kubeconfig /etc/rancher/rke2/rke2.yaml get nodes" >/dev/null 2>&1; then
    echo "Cluster is ready! (attempt $i)"
    break
  fi
  echo "  Waiting... ($i/$MAX_RETRIES)"
  sleep $RETRY_INTERVAL
done

# ─────────────────────────────────────────────
# 2. Fetch kubeconfig from master
# ─────────────────────────────────────────────
echo ""
echo ">>> Step 2: Fetching kubeconfig from master..."

ssh -i "$SSH_KEY" -o StrictHostKeyChecking=no ubuntu@"$MASTER_IP" \
  "sudo cat /etc/rancher/rke2/rke2.yaml" > /tmp/rke2-kubeconfig.yaml

# Replace localhost with master IP
sed -i "s|server: https://127.0.0.1:6443|server: https://${MASTER_IP}:6443|" /tmp/rke2-kubeconfig.yaml

# Install locally — explicit path (GitHub Actions may override HOME)
KUBE_DIR="/home/$(whoami)/.kube"
mkdir -p "$KUBE_DIR"
cp /tmp/rke2-kubeconfig.yaml "$KUBE_DIR/config"
chmod 600 "$KUBE_DIR/config"
export KUBECONFIG="$KUBE_DIR/config"

echo "Kubeconfig installed. Testing..."
kubectl get nodes
echo ""

# ─────────────────────────────────────────────
# 3. Update GitHub Secret (if GH_TOKEN provided)
# ─────────────────────────────────────────────
if [ -n "${GH_TOKEN:-}" ] && [ -n "${GH_REPO:-}" ]; then
  echo ">>> Step 3: Updating GitHub KUBECONFIG secret..."

  KUBECONFIG_B64=$(cat ~/.kube/config | base64 -w0)

  # Get repo public key for secret encryption
  PUB_KEY_RESPONSE=$(curl -s -H "Authorization: token ${GH_TOKEN}" \
    "https://api.github.com/repos/${GH_REPO}/actions/secrets/public-key")
  PUB_KEY=$(echo "$PUB_KEY_RESPONSE" | jq -r '.key')
  KEY_ID=$(echo "$PUB_KEY_RESPONSE" | jq -r '.key_id')

  if [ "$PUB_KEY" != "null" ] && command -v python3 >/dev/null; then
    # Ensure PyNaCl is available
    python3 -c "import nacl" 2>/dev/null || pip3 install pynacl -q 2>/dev/null || true

    ENCRYPTED=$(python3 -c "
import base64, sys
try:
    from nacl.public import PublicKey, SealedBox
    from nacl.encoding import Base64Encoder
    pk = PublicKey('${PUB_KEY}', Base64Encoder)
    box = SealedBox(pk)
    encrypted = box.encrypt(sys.stdin.read().encode())
    print(base64.b64encode(encrypted).decode())
except ImportError:
    print('SKIP')
" <<< "$KUBECONFIG_B64")

    if [ "$ENCRYPTED" != "SKIP" ]; then
      curl -s -X PUT \
        "https://api.github.com/repos/${GH_REPO}/actions/secrets/KUBECONFIG" \
        -H "Authorization: token ${GH_TOKEN}" \
        -H "Accept: application/vnd.github+json" \
        -d "{\"encrypted_value\":\"${ENCRYPTED}\",\"key_id\":\"${KEY_ID}\"}" >/dev/null
      echo "  ✅ KUBECONFIG secret updated"
    else
      echo "  ⚠️ PyNaCl not installed — skipping secret update"
    fi
  else
    echo "  ⚠️ Cannot update secret (missing python3 or public key)"
  fi
else
  echo ">>> Step 3: Skipping GitHub secret update (GH_TOKEN not set)"
fi

# ─────────────────────────────────────────────
# 4. Create K8s secrets for CCM
# ─────────────────────────────────────────────
echo ""
echo ">>> Step 4: Creating K8s secrets..."

# Cloud config secret
if [ -n "${CLOUD_CONFIG:-}" ]; then
  echo "$CLOUD_CONFIG" > /tmp/cloud.conf
  kubectl create secret generic swiss-otc-cloud-config \
    --namespace kube-system \
    --from-file=cloud.conf=/tmp/cloud.conf \
    --dry-run=client -o yaml | kubectl apply -f -
  rm -f /tmp/cloud.conf
  echo "  ✅ swiss-otc-cloud-config secret created/updated"
fi

# GHCR pull secret
if [ -n "${GHCR_PULL_USER:-}" ] && [ -n "${GHCR_PULL_TOKEN:-}" ]; then
  kubectl create secret docker-registry ghcr-pull-secret \
    --namespace kube-system \
    --docker-server=ghcr.io \
    --docker-username="${GHCR_PULL_USER}" \
    --docker-password="${GHCR_PULL_TOKEN}" \
    --dry-run=client -o yaml | kubectl apply -f -
  echo "  ✅ ghcr-pull-secret created/updated"
fi

# ─────────────────────────────────────────────
# 5. Remove cloud-provider taints
# ─────────────────────────────────────────────
echo ""
echo ">>> Step 5: Removing cloud-provider taints..."
kubectl taint nodes --all node.cloudprovider.kubernetes.io/uninitialized- 2>/dev/null || echo "  (no taints to remove)"

# ─────────────────────────────────────────────
# Summary
# ─────────────────────────────────────────────
echo ""
echo "============================================"
echo "  Post-Apply Complete! ✅"
echo "============================================"
echo ""
kubectl get nodes -o wide
echo ""
kubectl get secrets -n kube-system | grep -E "swiss-otc|ghcr"
