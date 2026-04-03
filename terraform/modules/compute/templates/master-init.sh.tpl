#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

%{ if proxy_host != "" }
# Configure proxy for outbound internet (via jumpserver TinyProxy)
export HTTP_PROXY=http://${proxy_host}:3128
export HTTPS_PROXY=http://${proxy_host}:3128
export NO_PROXY=10.0.0.0/8,127.0.0.1,localhost,.svc,.cluster.local

# Persist proxy for RKE2 service AND containerd (RKE2 embedded)
mkdir -p /etc/systemd/system/rke2-server.service.d
cat > /etc/systemd/system/rke2-server.service.d/proxy.conf <<PROXY
[Service]
Environment="HTTP_PROXY=http://${proxy_host}:3128"
Environment="HTTPS_PROXY=http://${proxy_host}:3128"
Environment="NO_PROXY=10.0.0.0/8,127.0.0.1,localhost,.svc,.cluster.local"
PROXY

# RKE2 embedded containerd braucht eigene proxy.conf (erbt nicht vom Parent-Service)
mkdir -p /etc/systemd/system/rke2-server.service.d
# containerd in RKE2 wird durch rke2-server gemanagt — wir setzen env auch in containerd config
mkdir -p /var/lib/rancher/rke2/agent/etc/containerd/
cat > /var/lib/rancher/rke2/agent/etc/containerd/config.toml.tmpl <<CTRD
version = 2

[plugins."io.containerd.grpc.v1.cri".registry]
  [plugins."io.containerd.grpc.v1.cri".registry.configs]
CTRD

# Proxy via environment in containerd host-process
mkdir -p /etc/systemd/system/containerd.service.d/ 2>/dev/null || true
cat > /etc/profile.d/proxy.sh <<PROFXY
export HTTP_PROXY=http://${proxy_host}:3128
export HTTPS_PROXY=http://${proxy_host}:3128
export NO_PROXY=10.0.0.0/8,127.0.0.1,localhost,.svc,.cluster.local
PROFXY
%{ endif }

# Wait for internet access (via proxy or NAT)
for i in $(seq 1 30); do
  curl -sf --max-time 5 https://get.rke2.io > /dev/null 2>&1 && break
  echo "Waiting for internet... attempt $i/30"
  sleep 10
done

# Install RKE2
curl -sfL https://get.rke2.io | sh -

mkdir -p /etc/rancher/rke2
MASTER_IP=$(hostname -I | awk '{print $1}')

%{ if cni_plugin == "kube-ovn" }
# ── Kube-OVN: RKE2 ohne built-in CNI, Kube-OVN via Pipeline deployen ──────
cat > /etc/rancher/rke2/config.yaml <<CONFIG
token: ${cluster_token}
cloud-provider-name: external
cni: none
disable-kube-proxy: true
tls-san:
  - $MASTER_IP
CONFIG
echo "CNI: kube-ovn (pipeline will deploy after cluster ready)"
%{ else }
# ── Cilium: built-in RKE2 CNI mit kube-proxy replacement ──────────────────
cat > /etc/rancher/rke2/config.yaml <<CONFIG
token: ${cluster_token}
cloud-provider-name: external
cni: cilium
disable-kube-proxy: true
tls-san:
  - $MASTER_IP
CONFIG

# Cilium HelmChartConfig
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
echo "CNI: cilium (built-in)"
%{ endif }

# user_allow_other für FUSE mounts (CSI-S3)
grep -q "^user_allow_other" /etc/fuse.conf || echo "user_allow_other" >> /etc/fuse.conf

# Start RKE2
# RKE2 Registry Mirror — bessere Erreichbarkeit von registry.k8s.io
cat > /etc/rancher/rke2/registries.yaml << 'REGEOF'
mirrors:
  registry.k8s.io:
    endpoint:
      - "https://registry.k8s.io"
  docker.io:
    endpoint:
      - "https://registry-1.docker.io"
REGEOF
chmod 600 /etc/rancher/rke2/registries.yaml

%{ if proxy_host != "" }
# RKE2 embedded containerd proxy config (über systemd env-file)
# containerd erbt HTTP_PROXY nicht von rke2-server — eigenes override nötig
mkdir -p /etc/rancher/rke2
cat > /etc/rancher/rke2/proxy.env <<PROXYENV
HTTP_PROXY=http://${proxy_host}:3128
HTTPS_PROXY=http://${proxy_host}:3128
NO_PROXY=10.0.0.0/8,127.0.0.1,localhost,.svc,.cluster.local,169.254.0.0/16
PROXYENV

# RKE2 server service: EnvironmentFile setzen
mkdir -p /etc/systemd/system/rke2-server.service.d
cat >> /etc/systemd/system/rke2-server.service.d/proxy.conf <<PROXYOVERRIDE

EnvironmentFile=-/etc/rancher/rke2/proxy.env
PROXYOVERRIDE
systemctl daemon-reload
%{ endif }

systemctl enable rke2-server.service
systemctl start rke2-server.service


# CIS Benchmark Fix (SDE-284): Datei-Permissions für 1.1.1, 1.1.3, 1.1.5
# RKE2 legt diese Dateien mit 644 an — CIS erwartet 600
# Fix via systemd ExecStartPost oder Post-Start-Skript
cat > /etc/systemd/system/rke2-cis-fix.service << 'CISFIX'
[Unit]
Description=CIS Benchmark Permission Fix for RKE2
After=rke2-server.service
Requires=rke2-server.service

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/bin/bash -c 'sleep 30 &&   chmod 600 /etc/rancher/rke2/rke2.yaml 2>/dev/null || true &&   find /var/lib/rancher/rke2/server/manifests/ -name "*.yaml" -exec chmod 600 {} \; 2>/dev/null || true &&   find /etc/rancher/rke2/ -name "*.yaml" -exec chmod 600 {} \; 2>/dev/null || true &&   find /etc/rancher/rke2/ -name "*.conf" -exec chmod 600 {} \; 2>/dev/null || true'

[Install]
WantedBy=multi-user.target
CISFIX

systemctl enable rke2-cis-fix.service

echo "RKE2 master setup complete (CNI: ${cni_plugin})"
