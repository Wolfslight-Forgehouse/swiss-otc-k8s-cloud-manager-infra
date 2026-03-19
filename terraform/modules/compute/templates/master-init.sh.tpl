#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

%{ if proxy_host != "" }
# Configure proxy for outbound internet (via jumpserver TinyProxy)
export HTTP_PROXY=http://${proxy_host}:3128
export HTTPS_PROXY=http://${proxy_host}:3128
export NO_PROXY=10.0.0.0/8,127.0.0.1,localhost,.svc,.cluster.local

# Persist proxy for RKE2 service
mkdir -p /etc/systemd/system/rke2-server.service.d
cat > /etc/systemd/system/rke2-server.service.d/proxy.conf <<PROXY
[Service]
Environment="HTTP_PROXY=http://${proxy_host}:3128"
Environment="HTTPS_PROXY=http://${proxy_host}:3128"
Environment="NO_PROXY=10.0.0.0/8,127.0.0.1,localhost,.svc,.cluster.local"
PROXY
%{ endif }

# Wait for internet access (via proxy or NAT)
for i in $(seq 1 30); do
  curl -sf --max-time 5 https://get.rke2.io > /dev/null 2>&1 && break
  echo "Waiting for internet... attempt $i/30"
  sleep 10
done

# Install RKE2
curl -sfL https://get.rke2.io | sh -

# Configure RKE2 with Cilium CNI
mkdir -p /etc/rancher/rke2
MASTER_IP=$(hostname -I | awk '{print $1}')

cat > /etc/rancher/rke2/config.yaml <<CONFIG
token: ${cluster_token}
cloud-provider-name: external
cni: cilium
disable-kube-proxy: true
tls-san:
  - $MASTER_IP
CONFIG

# Cilium HelmChartConfig for kube-proxy replacement
mkdir -p /var/lib/rancher/rke2/server/manifests
cat > /var/lib/rancher/rke2/server/manifests/rke2-cilium-config.yaml <<CILIUM
apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-cilium
  namespace: kube-system
spec:
  valuesContent: |-
    kubeProxyReplacement: true
    k8sServiceHost: "$MASTER_IP"
    k8sServicePort: "6443"
    routingMode: tunnel
    tunnelProtocol: vxlan
    MTU: 1450
    hubble:
      enabled: true
      relay:
        enabled: true
CILIUM


# user_allow_other für FUSE mounts (CSI-S3 läuft als root aber braucht allow_other)
grep -q "^user_allow_other" /etc/fuse.conf || echo "user_allow_other" >> /etc/fuse.conf
echo "geesefs $(geesefs --version 2>&1 || echo 'installed') ✅"

# Start RKE2
systemctl enable rke2-server.service
systemctl start rke2-server.service

echo "RKE2 master setup complete"
