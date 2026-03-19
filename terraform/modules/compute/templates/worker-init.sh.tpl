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
# CSI-S3 (GeeseFS) Voraussetzungen
# geesefs wird aus OTC OBS geladen (Nodes haben keinen GitHub-Zugang)
# ────────────────────────────────────────────────────────────────

# geesefs v0.42.4 von OBS holen (OBS-Endpoint intern erreichbar)
# HINWEIS: OBS benötigt x-amz-content-sha256 Header beim sigv4 curl
GEESEFS_URL="https://obs.eu-ch2.sc.otc.t-systems.com/rke2-sotc-tfstate/binaries/geesefs-linux-amd64-v0.42.4"

echo "Downloading geesefs v0.42.4 from OBS..."
for attempt in 1 2 3 4 5; do
  HTTP_CODE=$(curl -sf \
    --aws-sigv4 "aws:amz:eu-ch2:s3" \
    --user "${obs_access_key}:${obs_secret_key}" \
    -H "x-amz-content-sha256: UNSIGNED-PAYLOAD" \
    -w "%%{http_code}" \
    "$${GEESEFS_URL}" \
    -o /usr/local/bin/geesefs 2>/dev/null)
  if [ "$${HTTP_CODE}" = "200" ] && [ -s /usr/local/bin/geesefs ]; then
    echo "geesefs download OK (attempt $${attempt})"
    break
  fi
  echo "Download attempt $${attempt} failed (HTTP $${HTTP_CODE}), retrying in 10s..."
  sleep 10
done

if [ -s /usr/local/bin/geesefs ]; then
  chmod +x /usr/local/bin/geesefs
  ln -sf /usr/local/bin/geesefs /usr/bin/geesefs
  echo "geesefs $(geesefs --version 2>&1 || echo 'installed') ✅"
else
  echo "WARNING: geesefs download failed — CSI-S3 mounts will not work until installed manually"
fi

# FUSE: allow_other für non-root Prozesse (CSI Driver läuft als root, aber sicherheitshalber)
echo "user_allow_other" >> /etc/fuse.conf

# Start RKE2 agent
systemctl enable rke2-agent.service
systemctl start rke2-agent.service

echo "RKE2 worker setup complete"
