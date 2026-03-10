# Quick Start Guide

Deploy a cloud-native RKE2 Kubernetes cluster on Swiss Open Telekom Cloud with automated LoadBalancer provisioning.

## Prerequisites

- Swiss OTC account with `eu-ch2` region access
- IAM user with AK/SK credentials and permissions:
  - `ELB FullAccess`
  - `VPC FullAccess`
  - `ECS FullAccess`
  - `OBS FullAccess` (for Terraform state)
- GitHub repository (or GitLab) with CI/CD enabled
- SSH key pair for VM access

## Step 1: Configure Terraform State Backend

Create an OBS bucket for Terraform remote state:

```bash
# Via OTC Console: Storage → OBS → Create Bucket
# Name: your-tfstate-bucket
# Region: eu-ch2
# Versioning: Enabled
```

Update `terraform/environments/dev/backend.tf` with your bucket name.

## Step 2: Set CI/CD Secrets

### GitHub Actions

Go to Settings → Secrets and variables → Actions:

| Secret | Value |
|---|---|
| `OTC_ACCESS_KEY` | Your AK |
| `OTC_SECRET_KEY` | Your SK |
| `OTC_PROJECT_ID` | Your project ID |
| `SSH_PUBLIC_KEY` | Your SSH public key |
| `SSH_PRIVATE_KEY` | Your SSH private key |
| `RKE2_TOKEN` | `openssl rand -hex 32` |
| `GH_PAT` | GitHub PAT (for runner registration) |
| `GHCR_PULL_TOKEN` | GitHub PAT with `read:packages` |

### GitLab CI

Go to Settings → CI/CD → Variables and set the same values.

## Step 3: Deploy Infrastructure

```bash
# Trigger via GitHub Actions UI:
# Actions → Terraform Apply → Run workflow → Type "APPLY"
```

This creates:
- VPC + Subnet + Security Groups
- Bastion host (with kubectl, helm, CI runner)
- RKE2 master + 2 workers
- Post-apply: kubeconfig, K8s secrets, taint removal

## Step 4: Build & Deploy CCM

```bash
# Build container image:
# Actions → CCM Build & Push → Run workflow

# Deploy to cluster:
# Actions → CCM Deploy (Helm) → Run workflow → dry_run: false
```

## Step 5: Test

```yaml
# Apply a LoadBalancer service:
apiVersion: v1
kind: Service
metadata:
  name: my-app
  annotations:
    otc.io/eip-bandwidth: "10"  # Public IP with 10 Mbit/s
spec:
  type: LoadBalancer
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: my-app
```

The CCM will automatically:
1. Create an ELB v3 Dedicated load balancer
2. Bind an Elastic IP (if annotated)
3. Configure listeners, pools, and health monitors
4. Update security group rules
5. Patch the service with the external IP

## Step 6: Destroy (Cleanup)

```bash
# Actions → Terraform Destroy → Run workflow → Type "DESTROY"
# Pre-destroy job automatically cleans up ELBs before removing infrastructure
```

## Architecture

See [docs/ARCHITECTURE.md](ARCHITECTURE.md) for detailed diagrams.

## Customization

| File | Purpose |
|---|---|
| `terraform/environments/dev/main.tf` | Cluster size, flavors, CIDRs |
| `deploy/helm/swiss-otc-ccm/values.yaml` | CCM config, replicas, logging |
| `terraform.tfvars` | Your environment-specific values |
