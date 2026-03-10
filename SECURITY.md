# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in this project, please report it responsibly:

1. **Do NOT** open a public issue
2. Email: security@your-org.example.com
3. Include: description, steps to reproduce, potential impact

We will respond within 48 hours and provide a fix timeline.

## Security Best Practices

### Credential Management

- **Never** commit credentials to the repository
- Use CI/CD secrets (GitHub Secrets / GitLab CI Variables) for all sensitive values
- Use AK/SK authentication (stateless, no token caching)
- Rotate credentials regularly

### Required Secrets

| Secret | Description |
|---|---|
| `OTC_ACCESS_KEY` | OTC Access Key ID |
| `OTC_SECRET_KEY` | OTC Secret Access Key |
| `OTC_PROJECT_ID` | OTC Project ID |
| `RKE2_TOKEN` | Pre-shared cluster join token |
| `SSH_PUBLIC_KEY` | SSH public key for VM access |
| `SSH_PRIVATE_KEY` | SSH private key (bastion → master) |

### Network Security

- Bastion host is the only public-facing VM (SSH + TinyProxy)
- Kubernetes API is internal-only (accessible via bastion)
- Security groups follow least-privilege principle
- ELB health checks use OTC SNAT range `100.125.0.0/16`
