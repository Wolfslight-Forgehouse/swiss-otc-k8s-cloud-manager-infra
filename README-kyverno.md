# Kyverno for RKE2 on SwissOTC

Policy enforcement for the SwissOTC RKE2 cluster using [Kyverno](https://kyverno.io/).

## Architecture

- **Helm Chart** (`charts/kyverno/`): Wrapper chart with `kyverno/kyverno` as a dependency
- **Policies** (`policies/baseline/`): PSS Baseline ClusterPolicies in Audit mode
- **ArgoCD** (`argocd/apps/kyverno.yaml`): GitOps-managed deployment with auto-sync
- **CI** (`.github/workflows/kyverno-ci.yaml`): Lint, test, and publish pipeline

## Policy Catalog

All policies use `validationFailureAction: Audit` (Phase 1). Violations are reported but not blocked.

| Policy | File | Description |
|--------|------|-------------|
| Disallow Privileged Containers | `disallow-privileged-containers.yaml` | Prevents containers from running in privileged mode |
| Disallow Host PID/IPC | `disallow-host-pid-ipc.yaml` | Blocks sharing the host PID or IPC namespaces |
| Disallow Host Path | `disallow-host-path.yaml` | Prevents mounting host filesystem paths as volumes |
| Disallow Unsafe Sysctls | `disallow-unsafe-sysctls.yaml` | Restricts sysctls to the safe subset only |
| Disallow Host Ports | `disallow-host-ports.yaml` | Blocks containers from binding to host ports |
| Require Run As Non-Root | `require-run-as-non-root.yaml` | Requires containers to run as non-root users |

These policies implement the [Kubernetes Pod Security Standards (Baseline)](https://kubernetes.io/docs/concepts/security/pod-security-standards/#baseline) profile.

## ArgoCD Bootstrap

1. Ensure the Kyverno Helm chart is published to GHCR (CI handles this on push to `main`)
2. Apply the ArgoCD Application manifest:
   ```bash
   kubectl apply -f argocd/apps/kyverno.yaml
   ```
3. ArgoCD will automatically sync and create the `kyverno-system` namespace
4. Verify deployment:
   ```bash
   kubectl get pods -n kyverno-system
   kubectl get clusterpolicies
   ```

## Promoting from Audit to Enforce

> Phase 2 handles enforcement. Do not change `validationFailureAction` without reviewing audit reports first.

1. Review policy reports for violations:
   ```bash
   kubectl get policyreport -A
   kubectl get clusterpolicyreport
   ```
2. Verify no critical workloads are violating policies
3. Update `validationFailureAction` from `Audit` to `Enforce` per policy:
   ```yaml
   spec:
     validationFailureAction: Enforce
   ```
4. Commit, push, and let ArgoCD sync the changes
5. Monitor for rejected admissions:
   ```bash
   kubectl get events --field-selector reason=PolicyViolation -A
   ```

## CI Pipeline

The CI pipeline runs on PRs and pushes to `main`:

- **Lint**: `helm lint charts/kyverno/`
- **Template + Validate**: Renders the chart and validates against baseline policies
- **Policy Tests**: Runs Kyverno CLI test suites
- **Publish** (main only): Packages and pushes to `ghcr.io/Wolfslight-Forgehouse/charts/kyverno`

### Required Secrets

| Secret | Description |
|--------|-------------|
| `GHCR_TOKEN` | GitHub PAT with `write:packages` scope for OCI chart publishing |

## Local Development

```bash
# Add upstream Kyverno repo
helm repo add kyverno https://kyverno.github.io/kyverno/
helm repo update

# Build dependencies
helm dependency build charts/kyverno/

# Lint
helm lint charts/kyverno/

# Template and validate
helm template kyverno charts/kyverno/ --namespace kyverno-system \
  | kyverno apply policies/baseline/ --resource -
```
