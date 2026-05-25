# Security — insider-one-service

> This document covers secret management, rotation procedures, and security posture for the service.

---

## 1. Secrets Overview

| Secret | Where stored | Consumed by |
|---|---|---|
| App secrets (e.g. API keys) | Kubernetes `Secret` in `dev` / `prod` namespace | Pod via `envFrom.secretRef` |
| GHCR push token | GitHub Actions secret `GITHUB_TOKEN` | CI workflow (auto-rotated by GitHub) |
| SSH keypair | `~/.ssh/insider-service` (local) + AWS Key Pair | EC2 SSH access |
| ArgoCD admin password | ArgoCD `argocd-initial-admin-secret` | ArgoCD UI / CLI |

Secrets are injected into the Helm chart via `values-*.yaml` under the `secret:` key. **Never commit actual secret values to Git** — use `values-prod.yaml.example` or a secrets manager.

---

## 2. Secret Rotation

### 2a. Application secrets (Kubernetes Secret)

1. Generate or obtain the new secret value.

2. Update the Kubernetes Secret directly (for an immediate rotation without a Git push):

   ```bash
   kubectl create secret generic insider-service \
     --from-literal=MY_SECRET_KEY=<new-value> \
     --dry-run=client -o yaml \
     | kubectl apply -f - -n prod
   ```

3. Restart the deployment so pods mount the new value:

   ```bash
   kubectl rollout restart deployment/insider-service -n prod
   kubectl rollout status deployment/insider-service -n prod
   ```

4. Update `chart/values-prod.yaml` (via a secrets manager or sealed-secrets tooling) so ArgoCD does not overwrite the rotated value on the next sync.

5. Verify the new secret is live:

   ```bash
   kubectl exec -it deploy/insider-service -n prod -- env | grep MY_SECRET_KEY
   ```

### 2b. SSH keypair (EC2 access)

1. Generate a new keypair:

   ```bash
   ssh-keygen -t ed25519 -f ~/.ssh/insider-service-new -C "insider-service rotate $(date +%Y-%m-%d)"
   ```

2. Apply the new public key via Terraform:

   ```bash
   # Edit infra/terraform.tfvars
   ssh_public_key_path = "~/.ssh/insider-service-new.pub"

   cd infra
   terraform apply -target=aws_key_pair.main
   ```

3. Test the new key **before** removing the old one:

   ```bash
   ssh -i ~/.ssh/insider-service-new ubuntu@<elastic-ip> "echo ok"
   ```

4. Remove the old key from `~/.ssh/known_hosts` and delete the old file:

   ```bash
   ssh-keygen -R <elastic-ip>
   rm ~/.ssh/insider-service ~/.ssh/insider-service.pub
   mv ~/.ssh/insider-service-new ~/.ssh/insider-service
   mv ~/.ssh/insider-service-new.pub ~/.ssh/insider-service.pub
   ```

### 2c. ArgoCD admin password

```bash
# Generate a bcrypt hash of the new password
HASHED=$(htpasswd -nbBC 10 "" <new-password> | tr -d ':\n' | sed 's/$2y/$2a/')

# Patch the ArgoCD secret
kubectl -n argocd patch secret argocd-secret \
  -p "{\"stringData\":{\"admin.password\":\"$HASHED\",\"admin.passwordMtime\":\"$(date +%FT%TZ)\"}}"

# Force ArgoCD server to pick it up
kubectl rollout restart deployment/argocd-server -n argocd
```

### 2d. GHCR / GitHub token

The `GITHUB_TOKEN` used in CI is auto-rotated per workflow run by GitHub — no action needed.

For Personal Access Tokens (PATs) used for ArgoCD repo access:

1. Create a new PAT in GitHub → Settings → Developer settings → Tokens.
2. Update the ArgoCD repository credential:

   ```bash
   argocd repo update https://github.com/gizemcell/insider-one-service.git \
     --username <github-username> \
     --password <new-pat>
   ```

---

## 3. Security Controls in Place

| Control | Detail |
|---|---|
| Non-root container | Dockerfile runs as UID 1001 (`app` user) |
| Read-only image layer | Alpine base, minimal attack surface |
| Trivy image scan | Runs in CI on every build; results uploaded to GitHub Security tab |
| Gitleaks | Scans every push for accidental secret commits |
| `govulncheck` | Go vulnerability scan on every PR |
| NetworkPolicy | Enabled in prod — restricts ingress to nginx controller namespace only |
| PodDisruptionBudget | `minAvailable: 2` in prod prevents full-cluster drain |
| SSH restricted | Security Group allows port 22 only from `my_ip_cidr` (set in `terraform.tfvars`) |
| `ReadHeaderTimeout` | 5 s (mitigates Slowloris) |
| HPA | CPU-based autoscaling prevents resource exhaustion |

---

## 4. Incident Response

### Suspected secret compromise

1. **Rotate immediately** — follow section 2 for the relevant secret.
2. Audit pod environment for exposed values:

   ```bash
   kubectl get secret insider-service -n prod -o jsonpath='{.data}' | jq 'to_entries[] | {key: .key, value: (.value | @base64d)}'
   ```

3. Check ArgoCD sync history for any unauthorized changes:

   ```bash
   argocd app history app-prod
   ```

4. Review GitHub Actions run logs for the affected workflow.
5. Rotate the ArgoCD admin password and any repo access tokens.
6. Revoke the compromised secret at the source (GitHub, AWS IAM, etc.).

### Unauthorized image pushed to GHCR

1. Identify the tag and digest:

   ```bash
   # List recent tags via GitHub API or GHCR UI
   ```

2. Pin the deployment to a known-good digest immediately:

   ```bash
   # Edit chart/values-prod.yaml
   image:
     tag: sha-<known-good-sha>
   git commit -am "security: pin image to known-good sha"
   git push origin main
   ```

3. Delete the suspicious tag from GHCR (Packages tab in GitHub).
4. Audit CI workflow logs for unexpected runs.

---

## 5. Threat Model Notes

- **Secrets in Helm values**: `values-prod.yaml` must never be committed with real secrets. Use `git-crypt`, Sealed Secrets, or an external secrets operator for production hardening.
- **Minikube single-node**: The EC2 host is the single point of failure for the cluster. The Elastic IP ensures reachability after reboots but does not provide HA at the host level.
- **SSH exposure**: Port 22 is restricted by Security Group to a single IP CIDR. If your IP changes, update `my_ip_cidr` in `terraform.tfvars` and run `terraform apply`.
