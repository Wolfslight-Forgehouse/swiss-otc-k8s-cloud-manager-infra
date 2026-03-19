#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

%{ if proxy_host != "" }
export HTTP_PROXY=http://${proxy_host}:3128
export HTTPS_PROXY=http://${proxy_host}:3128
export NO_PROXY=10.0.0.0/8,127.0.0.1,localhost,.svc,.cluster.local

mkdir -p /etc/systemd/system/rke2-agent.service.d
cat > /etc/systemd/system/rke2-agent.service.d/proxy.conf <<PROXY
[Service]
Environment="HTTP_PROXY=http://${proxy_host}:3128"
Environment="HTTPS_PROXY=http://${proxy_host}:3128"
Environment="NO_PROXY=10.0.0.0/8,127.0.0.1,localhost,.svc,.cluster.local"
PROXY
%{ endif }

# Wait for internet / proxy
for i in $(seq 1 30); do
  curl -sf --max-time 5 https://get.rke2.io > /dev/null 2>&1 && break
  echo "Waiting for internet... attempt $i/30"
  sleep 10
done

# Install RKE2 agent
curl -sfL https://get.rke2.io | INSTALL_RKE2_TYPE="agent" sh -

# Configure agent
mkdir -p /etc/rancher/rke2
cat > /etc/rancher/rke2/config.yaml <<CONFIG
server: https://${master_ip}:9345
token: ${cluster_token}
cloud-provider-name: external
CONFIG

# ────────────────────────────────────────────────────────────────
echo "user_allow_other" >> /etc/fuse.conf

# Start RKE2 agent
systemctl enable rke2-agent.service
systemctl start rke2-agent.service

echo "RKE2 worker setup complete"
