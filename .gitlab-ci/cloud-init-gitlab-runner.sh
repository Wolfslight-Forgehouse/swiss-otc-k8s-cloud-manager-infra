#!/bin/bash
# Cloud-init snippet for GitLab Runner on bastion
# Add this to your jumpserver cloud-init template
#
# Required Terraform variables:
#   gitlab_url           = "https://gitlab.com"
#   gitlab_runner_token  = "<registration-token>"

# --- GitLab Runner (add after kubectl/helm install) ---
%{ if gitlab_runner_token != "" }
curl -sL --output /usr/local/bin/gitlab-runner \
  https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-amd64
chmod +x /usr/local/bin/gitlab-runner
useradd --comment 'GitLab Runner' --create-home gitlab-runner --shell /bin/bash || true

gitlab-runner register \
  --non-interactive \
  --url "${gitlab_url}" \
  --registration-token "${gitlab_runner_token}" \
  --executor shell \
  --name "otc-bastion" \
  --tag-list "self-hosted,linux,x64,otc" \
  --run-untagged=false

gitlab-runner install --user=gitlab-runner --working-directory=/home/gitlab-runner
gitlab-runner start
%{ endif }
