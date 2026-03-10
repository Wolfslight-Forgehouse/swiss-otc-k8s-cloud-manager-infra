#!/bin/bash
set -euo pipefail

# ============================================================
# 1. TinyProxy — Outbound internet for cluster nodes
# ============================================================
apt-get update -qq
apt-get install -y -qq tinyproxy jq

cat > /etc/tinyproxy/tinyproxy.conf << 'TPCONF'
User tinyproxy
Group tinyproxy
Port 3128
Listen 0.0.0.0
Timeout 600
MaxClients 100
Allow 10.0.0.0/8
Allow 127.0.0.1
LogFile "/var/log/tinyproxy/tinyproxy.log"
LogLevel Info
ConnectPort 443
ConnectPort 80
TPCONF

mkdir -p /var/log/tinyproxy
chown tinyproxy:tinyproxy /var/log/tinyproxy

mkdir -p /etc/systemd/system/tinyproxy.service.d
cat > /etc/systemd/system/tinyproxy.service.d/override.conf << 'SYSD'
[Service]
Type=simple
ExecStart=
ExecStart=/usr/bin/tinyproxy -d
PIDFile=
SYSD

systemctl daemon-reload
systemctl enable tinyproxy
systemctl restart tinyproxy

# Enable SSH forwarding for jump host
sed -i 's/AllowTcpForwarding no/AllowTcpForwarding yes/' /etc/ssh/sshd_config
systemctl restart sshd

# ============================================================
# 2. kubectl + helm — For cluster management
# ============================================================
curl -sLO "https://dl.k8s.io/release/v1.34.0/bin/linux/amd64/kubectl"
chmod +x kubectl && mv kubectl /usr/local/bin/

curl -sL https://get.helm.sh/helm-v3.14.0-linux-amd64.tar.gz | tar xz
mv linux-amd64/helm /usr/local/bin/ && rm -rf linux-amd64

# ============================================================
# 3. SSH key for cluster node access
# ============================================================
%{ if ssh_private_key != "" }
mkdir -p /home/ubuntu/.ssh
cat > /home/ubuntu/.ssh/deploy-key << 'SSHKEY'
${ssh_private_key}
SSHKEY
chmod 600 /home/ubuntu/.ssh/deploy-key
chown ubuntu:ubuntu /home/ubuntu/.ssh/deploy-key

cat > /home/ubuntu/.ssh/config << 'SSHCFG'
Host 10.0.1.*
  User ubuntu
  StrictHostKeyChecking no
  IdentityFile ~/.ssh/deploy-key
SSHCFG
chown ubuntu:ubuntu /home/ubuntu/.ssh/config
%{ endif }

# ============================================================
# 4. GitHub Actions Runner
# ============================================================
%{ if github_runner_token != "" }
RUNNER_DIR=/home/ubuntu/actions-runner
mkdir -p $RUNNER_DIR
cd $RUNNER_DIR

curl -sL https://github.com/actions/runner/releases/download/v2.324.0/actions-runner-linux-x64-2.324.0.tar.gz | tar xz
chown -R ubuntu:ubuntu $RUNNER_DIR

# Configure as ubuntu user
su - ubuntu -c "cd $RUNNER_DIR && ./config.sh \
  --url ${github_repo_url} \
  --token ${github_runner_token} \
  --name otc-bastion \
  --labels ${github_runner_labels} \
  --work _work \
  --unattended \
  --replace"

# Install and start as systemd service
cd $RUNNER_DIR
./svc.sh install ubuntu
./svc.sh start
%{ endif }


# ============================================================
# 5. GitLab Runner (optional — when gitlab_runner_token is set)
# ============================================================
%{ if gitlab_runner_token != "" }
curl -sL --output /usr/local/bin/gitlab-runner \
  https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-amd64
chmod +x /usr/local/bin/gitlab-runner
useradd --comment 'GitLab Runner' --create-home gitlab-runner --shell /bin/bash || true

# Copy kubeconfig and SSH key for gitlab-runner user
mkdir -p /home/gitlab-runner/.ssh /home/gitlab-runner/.kube
%{ if ssh_private_key != "" }
cp /home/ubuntu/.ssh/deploy-key /home/gitlab-runner/.ssh/deploy-key
cp /home/ubuntu/.ssh/config /home/gitlab-runner/.ssh/config
chmod 600 /home/gitlab-runner/.ssh/deploy-key
%{ endif }
chown -R gitlab-runner:gitlab-runner /home/gitlab-runner/.ssh

# Register and start
gitlab-runner register \
  --non-interactive \
  --url "${gitlab_url}" \
  --registration-token "${gitlab_runner_token}" \
  --executor shell \
  --name "otc-bastion" \
  --tag-list "${gitlab_runner_tags}" \
  --run-untagged=false \
  --locked=false

gitlab-runner install --user=gitlab-runner --working-directory=/home/gitlab-runner
gitlab-runner start
%{ endif }

echo "Jumpserver bootstrap complete" > /tmp/jumpserver-ready
