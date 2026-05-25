# Runbook — insider-one-service

> **Stack**: Go HTTP service · Helm chart · ArgoCD (GitOps) · Minikube on EC2 (eu-central-1)
> **Namespaces**: `dev` | `prod`
> **Image registry**: `ghcr.io/gizemcell/insider-one-service`

---

## 1. Restart

ArgoCD's `selfHeal: true` means the desired state in Git always wins. A manual restart is done by rolling the deployment — ArgoCD will not fight it because the replica count and image tag remain unchanged.

```bash
# Development environment
kubectl rollout restart deployment/app-dev -n dev
kubectl rollout status deployment/app-dev -n dev

# Production environment
kubectl rollout restart deployment/app-prod -n prod
kubectl rollout status deployment/app-prod -n prod
```

To force a full pod replacement (e.g. to pick up a refreshed Secret):

```bash
kubectl delete pod -l app.kubernetes.io/name=insider-service -n <ns>
```

To restart the ArgoCD application sync controller itself:

```bash
kubectl rollout restart deployment/argocd-application-controller -n argocd
```

---

## 2. Rollback

### 2a. GitOps rollback (preferred)

ArgoCD deploys whatever is in `main`. The safest rollback is to revert the offending commit and push.

```bash
# Find the last good commit
git log --oneline

# Revert the bad commit (creates a new commit — keeps history clean)
git revert <bad-commit-sha>
git push origin main
```

ArgoCD detects the push and syncs automatically (automated + prune enabled). Watch in the ArgoCD UI or:


### 2b. Image-only rollback (fast)

If only the image tag is wrong, patch `values-prod.yaml` (or `values-dev.yaml`) and push:

```bash
# Edit chart/values-prod.yaml
image:
  tag: <previous-good-sha>

git add chart/values-prod.yaml
git commit -m "chore(prod): rollback image to <previous-good-sha>"
git push origin main
```


## 3. Viewing Logs

### Live pod logs

```bash
# Stream logs for all pods in namespace
kubectl logs -l app.kubernetes.io/name=insider-service -n <ns> --follow

# Last 200 lines from a specific pod
kubectl logs <pod-name> -n <ns> --tail=200

# Include previous (crashed) container
kubectl logs <pod-name> -n <ns> --previous
```

### Structured log filtering

The service emits JSON logs. Use `jq` to filter:

```bash
# All 5xx responses
kubectl logs -l app.kubernetes.io/name=insider-service -n prod --follow \
  | jq 'select(.status >= 500)'

# Errors only
kubectl logs -l app.kubernetes.io/name=insider-service -n prod \
  | jq 'select(.level == "ERROR")'

# By request ID
kubectl logs -l app.kubernetes.io/name=insider-service -n prod \
  | jq 'select(.request_id == "<id>")'
```

### Events (crash loops, OOM, scheduling failures)

```bash
kubectl get events -n <ns> --sort-by='.lastTimestamp' | tail -30
kubectl describe pod <pod-name> -n <ns>
```

## 4. Health Check

```bash
# Via kubectl port-forward
# Port-forward ingress controller
kubectl port-forward -n ingress-nginx svc/ingress-nginx-controller 8080:80 --address 0.0.0.0

# Development ingress endpoint
curl http://dev.api.insider.local:8080/ping

# Production ingress endpoint
curl http://api.insider-service.com:8080/ping

# Metrics endpoint (through ingress)
curl http://dev.api.insider.local:8080/metrics
```

---

## 5. Scaling

```bash
# Permanent: edit chart/values-prod.yaml
autoscaling:
  minReplicas: 2
  maxReplicas: 4
```

---

## 6. SSH to EC2 Host

```bash
ssh -i ~/.ssh/insider-service ubuntu@<elastic-ip>

# Get current Elastic IP from Terraform
cd infra && terraform output public_ip
```

### Minikube status on the host

```bash
minikube status
minikube kubectl -- get nodes
```
