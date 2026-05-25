# insider-one-service

A tiny Go HTTP service built end-to-end as the Insider One DevOps internship case study.
It is containerised with Docker, deployed to Kubernetes via Helm, shipped through a GitHub Actions
CI/CD pipeline, observed with Prometheus + Grafana, and exposed to the public internet.

---

## Table of contents

1. [Architecture](#architecture)
2. [Architecture diagram](#architecture-diagram)
3. [Chosen track](#chosen-track)
4. [Endpoints](#endpoints)
5. [Environment variables](#environment-variables)
6. [Repository structure](#repository-structure)
7. [Local development](#local-development)
8. [Docker](#docker)
9. [Kubernetes & Helm](#kubernetes--helm)
10. [CI/CD pipeline](#cicd-pipeline)
11. [Observability](#observability)
12. [Infrastructure as Code](#infrastructure-as-code)
13. [Upgrade & rollback](#upgrade--rollback)
14. [Public demo](#public-demo)
15. [Runbook & operational docs](#runbook--operational-docs)
16. [Evidence & screenshots](#evidence--screenshots)
17. [Tool decisions (ADRs)](#tool-decisions-adrs)
18. [Security notes](#security-notes)
19. [AI usage note](#ai-usage-note)

---

## Architecture

```text
Internet
   │
   ▼
[Elastic IP / Public URL]
   │
   ▼
Ingress (ingress-nginx)
   │
   ▼
Kubernetes Service
   │
   ▼
Pod: insider-service
   │
   ├──► /ping
   ├──► /healthz
   ├──► /version
   └──► /metrics
               │
               ▼
         Prometheus
               │
               ▼
            Grafana
```

**Track A (AWS):**
An EC2 instance (Ubuntu 22.04) is provisioned with Terraform and attached to an Elastic IP.
The Kubernetes cluster runs on minikube using the Docker driver. The application is exposed
through ingress-nginx and monitored by Prometheus + Grafana.

ArgoCD watches the `main` branch and reconciles the cluster whenever
`chart/values-dev.yaml` or `chart/values-prod.yaml` changes.

---

## Architecture diagram

Architecture diagram source:

```text
docs/architecture.drawio
```

Exported image:

```text
docs/architecture.png
```

The diagram includes:

- EC2 + Elastic IP
- minikube cluster
- ingress-nginx
- insider-service Deployment + Service
- Prometheus + Grafana
- ArgoCD GitOps flow
- GitHub Actions pipeline
- GHCR image registry

---

## Chosen track

**Track A — Minikube on EC2**

The EC2 instance and Elastic IP are provisioned with Terraform under `infra/`.

The infrastructure includes:

- EC2 instance
- Elastic IP
- Security Group
- SSH key pair

SSH access is restricted to a single source IP.
HTTP/NodePort traffic is intentionally public for demo purposes.

The bootstrap process is automated through:

```text
infra/bootstrap-minikube.sh
```

The script installs:

- Docker
- kubectl
- minikube
- Helm

on a fresh Ubuntu instance.

---

## Endpoints

| Method & path | Response | Purpose |
|---|---|---|
| `GET /ping` | `pong` | Quick liveness ping |
| `GET /healthz` | `{"status":"ok"}` | Readiness/liveness probe |
| `GET /version` | `{"version":"<sha>"}` | Build SHA |
| `GET /metrics` | Prometheus metrics | Observability |

Non-GET requests return:

```text
405 Method Not Allowed
```

---

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `ADDR` | `:8080` | Listen address |
| `VERSION` | `v1.0.0` | Build SHA / semver |

No secrets are baked into the image.

Example local environment configuration:

```text
.env.example
```

---

## Repository structure

```text
.
├── app/
├── chart/
├── infra/
├── argocd/
├── docs/
│   ├── screenshots/
│   ├── architecture.drawio
│   └── architecture.png
├── .github/workflows/
├── Dockerfile
├── README.md
├── RUNBOOK.md
├── SECURITY.md
└── adr/
```

---

## Local development

### Run tests

```sh
cd app
go test -race ./...
```

### Run locally

```sh
cd app
go run .
```

### Test endpoints

```sh
curl http://localhost:8080/ping
```

Expected response:

```text
pong
```

---

## Docker

The image uses a two-stage Docker build:

1. `golang:1.24-alpine` builder stage
2. `alpine:3.22` runtime stage

The container runs as a non-root user:

```text
UID 1001
```

A Docker `HEALTHCHECK` probes `/healthz`.

### Build image

```sh
docker build \
  --build-arg VERSION=v1.2.3 \
  -t insider-service:v1.2.3 .
```

### Run container

```sh
docker run --rm -p 8080:8080 insider-service:v1.2.3
```

### Test

```sh
curl http://localhost:8080/ping
curl http://localhost:8080/healthz
curl http://localhost:8080/version
```

### Why Alpine instead of distroless?

Distroless images are smaller and reduce shell attack surface,
but the Docker `HEALTHCHECK` requires a probe utility such as `wget`.

Alpine provides:

- small image size
- shell utilities
- easier debugging
- compatibility with the current health-check setup

---

## Kubernetes & Helm

The Helm chart lives under:

```text
chart/
```

The chart extends the default `helm create` scaffold with:

- Deployment
- Service
- Ingress
- ConfigMap
- Secret
- HPA
- NetworkPolicy
- PodDisruptionBudget
- ServiceMonitor
- PrometheusRule

---

## Install / upgrade

### Development

```sh
helm upgrade --install insider-service ./chart \
  -f chart/values-dev.yaml
```

### Production

```sh
helm upgrade --install insider-service ./chart \
  -f chart/values-prod.yaml
```

---

## Environment differences

| Setting | dev | prod |
|---|---|---|
| `replicaCount` | 1 | 1 (HPA floor) |
| `image.pullPolicy` | `Always` | `IfNotPresent` |
| `ingress.host` | `dev.api.insider.local` | `api.insider-service.com` |
| CPU request / limit | 50m / 200m | 200m / 500m |
| Memory request / limit | 32Mi / 64Mi | 128Mi / 256Mi |
| HPA | disabled | enabled |
| NetworkPolicy | disabled | enabled |
| PodDisruptionBudget | disabled | enabled |

---

## Probes & resources

All probes target:

```text
/healthz
```

Configured probes:

- startupProbe
- livenessProbe
- readinessProbe

The startup probe prevents premature restarts during image pulls or slow node startup.

Resource requests and limits were selected for a lightweight HTTP service
without external dependencies.

---

## HPA

Production enables a CPU-based Horizontal Pod Autoscaler.

Configuration:

- target CPU: 70%
- max replicas: 2

This provides headroom for short traffic spikes.

---

## NetworkPolicy

Production restricts ingress traffic to the ingress controller namespace.

Without a NetworkPolicy, any pod in the cluster could directly access the service.

---

## PodDisruptionBudget

The PodDisruptionBudget helps maintain service availability during:

- node drain
- voluntary disruption
- cluster maintenance

---

## CI/CD pipeline

Pipeline file:

```text
.github/workflows/ci.yml
```

Pipeline stages:

```text
Lint
  ↓
Test
  ↓
Build
  ↓
Trivy scan
  ↓
Push to GHCR
  ↓
Update Helm values
  ↓
ArgoCD sync
```

The pipeline performs:

- golangci-lint
- go test -race
- govulncheck
- Trivy image scan
- Gitleaks secret scan
- Docker buildx
- GHCR push

---

## Secrets & authentication

- GHCR push uses `GITHUB_TOKEN`
- No long-lived AWS credentials are stored
- Gitleaks scans commits for secrets
- OIDC federation is preferred for AWS access

---

## Image tags

| Event | Tag |
|---|---|
| Push to `main` | `sha-<7-char>` |
| Pull request | `pr-<number>` |
| Semver tag | `v1.2.3` |

---

## Deployment strategy

Deployments follow a GitOps workflow:

1. CI builds and scans the image
2. Image is pushed to GHCR
3. CI updates Helm values
4. ArgoCD detects git changes
5. ArgoCD synchronizes the cluster

This avoids storing Kubernetes credentials in GitHub Actions.

---

## Observability

### Metrics

The service exposes Prometheus metrics through:

```text
GET /metrics
```

Metrics include:

- `http_requests_total`
- `http_request_duration_seconds`

A `ServiceMonitor` allows Prometheus Operator to scrape the application.

---

## Logs

The application emits structured JSON logs:

```json
{
  "timestamp":"2026-05-20T10:00:00Z",
  "level":"INFO",
  "msg":"request",
  "request_id":"a3f2c1d4e5b6a7f8",
  "method":"GET",
  "path":"/ping",
  "status":200,
  "duration_ms":1
}
```

---

## Prometheus + Grafana

Install kube-prometheus-stack:

```sh
helm repo add prometheus-community \
  https://prometheus-community.github.io/helm-charts

helm upgrade --install kube-prometheus-stack \
  prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace
```

Access Grafana:

```sh
kubectl port-forward \
  -n monitoring \
  svc/kube-prometheus-stack-grafana \
  3000:80
```

Open:

```text
http://localhost:3000
```

Default credentials:

```text
admin / prom-operator
```

---

## Dashboard panels

The Grafana dashboard includes:

- Requests per second (RPS)
- HTTP error rate
- p95 latency
- Pod restart count

### RPS

```promql
sum(rate(http_requests_total[1m]))
```

### Error rate

```promql
sum(rate(http_requests_total{status=~"5..|4.."}[5m]))
/
sum(rate(http_requests_total[5m]))
* 100
```

### p95 latency

```promql
histogram_quantile(
  0.95,
  sum(rate(http_request_duration_seconds_bucket[5m])) by (le)
)
```

### Pod restarts

```promql
sum(kube_pod_container_status_restarts_total)
```

---

## Alerts

A `PrometheusRule` resource is deployed through Helm.

Current alert:

```text
HighErrorRate
```

The alert fires when HTTP error rate exceeds 5%.

Example expression:

```promql
sum(rate(http_requests_total{status=~"5.."}[5m]))
/
sum(rate(http_requests_total[5m]))
> 0.05
```

---

## Infrastructure as Code

### Terraform (Track A)

Infrastructure provisioning:

```sh
cd infra/

cp terraform.tfvars.example terraform.tfvars

terraform init
terraform apply
```

Resources provisioned:

- EC2 instance
- Elastic IP
- Security Group
- SSH key pair

Outputs:

- public IP
- SSH command

---

## Bootstrap cluster

```sh
ssh -i ~/.ssh/insider-service ubuntu@<EIP>

bash bootstrap-minikube.sh
```

The bootstrap script installs and configures:

- Docker
- kubectl
- Helm
- minikube

---

## Upgrade & rollback

### Build and push image

```sh
docker build \
  --build-arg VERSION=v1.3.0 \
  -t ghcr.io/<owner>/insider-one-service:v1.3.0 .

docker push ghcr.io/<owner>/insider-one-service:v1.3.0
```

### Upgrade deployment

```sh
helm upgrade insider-service ./chart \
  -f chart/values-prod.yaml \
  --set image.tag=v1.3.0
```

### Watch rollout

```sh
kubectl rollout status deployment/insider-service
```

### View release history

```sh
helm history insider-service
```

### Rollback

```sh
helm rollback insider-service
```

---

## Public demo

Public endpoint:

```text
http://<elastic-ip>/ping
```

Example:

```sh
curl http://<elastic-ip>/ping
```

Expected response:

```text
pong
```

---

## Runbook & operational docs

Operational documentation lives under:

```text
docs/
```

Included files:

- `RUNBOOK.md`
- `SECURITY.md`
- `adr/ADR-01-helm.md`
- `adr/ADR-02-base-image.md`
- `adr/ADR-03-gitops.md`

The runbook covers:

- restart procedures
- rollback procedures
- viewing logs
- troubleshooting probes
- rotating secrets

---

## Evidence & screenshots

Operational evidence is stored under:

```text
docs/screenshots/
```

Included screenshots:

- `kubectl-get-pods.png`
- `helm-list.png`
- `helm-history.png`
- `rollout-status.png`
- `grafana-dashboard.png`
- `grafana-alert.png`
- `argocd-app.png`
- `gh-actions-green-pipeline.png`

Example:

```md
![Grafana Dashboard](docs/screenshots/grafana-dashboard.png)
```

```md
![Helm History](docs/screenshots/helm-history.png)
```

These screenshots demonstrate:

- healthy Kubernetes workloads
- successful Helm deployments
- rollout + rollback capability
- active Grafana dashboards
- Prometheus alerts
- successful CI/CD runs
- ArgoCD synchronization

---

## Tool decisions (ADRs)

### ADR-01 — Go for the service

Go compiles to a single static binary with no runtime dependency,
which keeps the container image small and portable.

The standard library already provides:

- HTTP server
- JSON handling
- structured logging

Alternative options such as Node.js or Python would introduce
larger runtime layers and dependency management overhead.

---

### ADR-02 — Helm over raw manifests

Helm enables environment-specific configuration through:

- `values-dev.yaml`
- `values-prod.yaml`

without duplicating Kubernetes manifests.

Helm also provides:

- release history
- rollback support
- templating
- reusable charts

---

### ADR-03 — ArgoCD for GitOps deployment

Instead of deploying directly from CI using `kubectl`,
the pipeline updates the git repository and ArgoCD reconciles the cluster.

Advantages:

- no cluster credentials in CI
- declarative desired state
- automatic drift correction
- deployment history in git

---

### ADR-04 — Alpine runtime image

Distroless was considered for security and smaller image size.

Alpine was selected because it:

- supports Docker HEALTHCHECK tooling
- simplifies debugging
- remains lightweight
- works well for demo and development workflows

---

### ADR-05 — Trivy + govulncheck split

Trivy scans OS-level vulnerabilities in the container image.

`govulncheck` scans Go dependencies against the Go vulnerability database.

Using both tools reduces false positives and provides better coverage.

---

## Security notes

- No secrets are committed to the repository
- Gitleaks scans all commits
- Containers run as non-root
- SSH access is IP restricted
- Trivy scans images during CI
- govulncheck scans Go dependencies
- OIDC federation is preferred over static AWS credentials
- `/metrics` is intentionally public for demo purposes

Production systems should additionally protect metrics endpoints through:

- authentication
- NetworkPolicy
- ingress restrictions

---

## AI usage note

Claude Code (`claude-sonnet-4-6`) and ChatGPT were used to assist with:

- README structure
- Helm boilerplate
- CI/CD review
- documentation refinement

All architectural decisions, infrastructure choices,
and implementation details were authored, reviewed,
and validated manually by the submitter.