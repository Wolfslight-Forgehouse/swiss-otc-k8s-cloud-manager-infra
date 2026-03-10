# Swiss OTC Cloud Controller Manager

A Kubernetes Cloud Controller Manager (CCM) for [Open Telekom Cloud](https://open-telekom-cloud.com/) (Swiss region `eu-ch2`), enabling cloud-native LoadBalancer services with automatic ELB v3 provisioning.

## ✨ Features

- **Automatic ELB v3 Provisioning** — `type: LoadBalancer` creates Dedicated ELB on Swiss OTC
- **Automatic EIP Binding** — Annotate with `otc.io/eip-bandwidth` for public internet access
- **AK/SK Authentication** — Stateless HMAC-SHA256 request signing (no token management)
- **Full Lifecycle Management** — Create, update, delete load balancers with pools, listeners, health monitors
- **Security Group Automation** — Automatically configures ingress rules for service ports
- **GitOps Ready** — Terraform infrastructure + GitHub Actions CI/CD pipelines

## 🏗️ Architecture

```
kubectl apply (type: LoadBalancer)
        │
        ▼
   CCM Service Controller
        │
        ▼
   Swiss OTC ELB v3 API ──► ELB + Listener + Pool + Members
        │
        ▼
   Swiss OTC VPC API ──► EIP + Security Group Rules
        │
        ▼
   Service gets EXTERNAL-IP 🌍
```

## 🚀 Quick Start

### Prerequisites

- RKE2 cluster on Swiss OTC (Terraform modules included)
- AK/SK credentials with ELB + VPC permissions
- Helm 3.x

### Deploy

```bash
# Create cloud config secret
kubectl create secret generic swiss-otc-cloud-config \
  --namespace kube-system \
  --from-file=cloud.conf=cloud.conf

# Install via Helm
helm upgrade --install swiss-otc-ccm deploy/helm/swiss-otc-ccm \
  --namespace kube-system \
  --set existingSecret=swiss-otc-cloud-config \
  --set 'imagePullSecrets[0].name=ghcr-pull-secret'
```

### Test

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  annotations:
    otc.io/eip-bandwidth: "10"  # Mbit/s, omit for internal-only LB
spec:
  type: LoadBalancer
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: my-app
```

### Cloud Config

```yaml
auth:
  auth_url: "https://iam-pub.eu-ch2.sc.otc.t-systems.com/v3"
  access_key: "<AK>"
  secret_key: "<SK>"
  project_id: "<project-id>"
  project_name: "YOUR_PROJECT_NAME"
  user_domain_name: "<domain-name>"
region: "eu-ch2"
network:
  vpc_id: "<vpc-id>"
  subnet_id: "<neutron-subnet-id>"     # For vip_subnet_cidr_id
  network_id: "<vpc-subnet-id>"        # For elb_virsubnet_ids
loadbalancer:
  availability_zones:
    - "eu-ch2a"
```

> **Note:** Swiss OTC requires two different subnet IDs. `subnet_id` is the Neutron/IPv4 subnet ID, `network_id` is the VPC subnet ID. Use `GET /v1/{project}/subnets` to find both.

## 🔑 Authentication

Uses **HWS SDK-HMAC-SHA256** request signing — each API call is signed with the Secret Key directly. No IAM tokens, no expiry management, no refresh logic.

Key implementation details (learned the hard way):
- Sign with `HMAC-SHA256(SecretKey, StringToSign)` — NOT a derived key
- `X-Project-Id` header is **required** and must be included in signed headers
- Canonical URI must always end with trailing slash `/`
- StringToSign is 3 lines: `algorithm\ndatetime\nhash(canonical_request)`

## 📋 Annotations

| Annotation | Default | Description |
|---|---|---|
| `otc.io/eip-bandwidth` | - | EIP bandwidth in Mbit/s (omit = internal LB) |
| `otc.io/subnet-cidr-id` | config | Override VIP subnet |
| `otc.io/elb-virsubnet-id` | config | Override backend subnet |

## 🏭 Infrastructure (Terraform)

Included Terraform modules for a complete RKE2 cluster:

| Module | Resources |
|---|---|
| `networking` | VPC, Subnet, Security Groups, SSH Keypair |
| `jumpserver` | Bastion host with TinyProxy, kubectl, helm, GitHub Actions runner |
| `compute` | RKE2 master + workers with Cilium CNI |

### One-Click Deploy

```bash
# Plan → Apply → Runner auto-bootstraps → CCM Deploy
gh workflow run "Terraform Apply" -f confirm=APPLY
gh workflow run "CCM Deploy (Helm)"
```

## 📦 CI/CD Pipelines

| Workflow | Trigger | Description |
|---|---|---|
| `infra-plan.yml` | Push to `main` | Terraform validate + plan |
| `infra-apply.yml` | Manual (`APPLY`) | Provisions infrastructure + auto-generates runner token |
| `infra-destroy.yml` | Manual (`DESTROY`) | Tears down all resources |
| `ccm-build.yml` | Manual / Push | Build + push container image to GHCR |
| `ccm-deploy.yml` | Manual | Helm deploy to cluster (via self-hosted runner) |

## 📐 Why This Exists

Swiss OTC `eu-ch2` only supports **ELB v3 Dedicated** — the legacy v2.0 API used by existing OpenStack CCMs doesn't work. This CCM implements the v3 API from scratch with proper Swiss OTC authentication.

## 🦊 GitLab CI/CD (Alternative)

Pipeline and runner configs are available in `.gitlab-ci/`:

| File | Description |
|---|---|
| `.gitlab-ci.yml` | Full pipeline (validate, plan, apply, build, deploy, destroy) |
| `runner-setup.sh` | Install & register GitLab Runner on bastion |
| `cloud-init-gitlab-runner.sh` | Cloud-init snippet for Terraform automation |

```bash
# Quick setup:
cp .gitlab-ci/.gitlab-ci.yml .
# Set CI/CD variables: OTC_ACCESS_KEY, OTC_SECRET_KEY, OTC_PROJECT_ID, etc.
# Register runner on bastion:
.gitlab-ci/runner-setup.sh https://gitlab.com <REG_TOKEN>
```

### Dual-Ready Runner (Terraform)

The bastion cloud-init supports **both** GitHub and GitLab runners simultaneously. Set the variables for whichever platform(s) you use:

```hcl
# GitHub Actions Runner (optional)
TF_VAR_github_runner_token = "<auto-generated in pipeline>"

# GitLab Runner (optional)
TF_VAR_gitlab_url          = "https://gitlab.com"
TF_VAR_gitlab_runner_token = "<registration-token>"
```

Both are conditional — omit the token and that runner is skipped. You can run both side by side on the same bastion.

## License

MIT
