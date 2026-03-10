#!/bin/bash
# GitLab Runner Setup for Swiss OTC Bastion
#
# Run this on the bastion host to register a GitLab Runner
# that can deploy to the RKE2 cluster.
#
# Prerequisites:
#   - kubectl and helm installed (done by cloud-init)
#   - Kubeconfig at ~/.kube/config
#
# Usage:
#   chmod +x runner-setup.sh
#   ./runner-setup.sh <GITLAB_URL> <REGISTRATION_TOKEN>
#
# Example:
#   ./runner-setup.sh https://gitlab.com xYz123AbC

set -euo pipefail

GITLAB_URL="${1:?Usage: $0 <GITLAB_URL> <REGISTRATION_TOKEN>}"
REG_TOKEN="${2:?Usage: $0 <GITLAB_URL> <REGISTRATION_TOKEN>}"
RUNNER_NAME="${3:-otc-bastion}"
RUNNER_TAGS="${4:-self-hosted,linux,x64,otc}"

echo "=== Installing GitLab Runner ==="
curl -L --output /tmp/gitlab-runner https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-amd64
chmod +x /tmp/gitlab-runner
sudo mv /tmp/gitlab-runner /usr/local/bin/gitlab-runner
sudo useradd --comment 'GitLab Runner' --create-home gitlab-runner --shell /bin/bash || true

echo "=== Registering Runner ==="
sudo gitlab-runner register \
  --non-interactive \
  --url "${GITLAB_URL}" \
  --registration-token "${REG_TOKEN}" \
  --executor shell \
  --name "${RUNNER_NAME}" \
  --tag-list "${RUNNER_TAGS}" \
  --run-untagged=false \
  --locked=false

echo "=== Installing as Service ==="
sudo gitlab-runner install --user=gitlab-runner --working-directory=/home/gitlab-runner
sudo gitlab-runner start

echo "=== Copying Kubeconfig ==="
sudo mkdir -p /home/gitlab-runner/.kube
sudo cp ~/.kube/config /home/gitlab-runner/.kube/config
sudo chown -R gitlab-runner:gitlab-runner /home/gitlab-runner/.kube

echo "=== Runner Ready ==="
echo "Name:   ${RUNNER_NAME}"
echo "Tags:   ${RUNNER_TAGS}"
echo "Status: $(sudo gitlab-runner status)"
