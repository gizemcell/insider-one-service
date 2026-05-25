# Changelog

All notable changes to this project will be documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/);
versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
## [1.1.1] — 2026-05-25

### Added
- Terraform IaC for AWS EC2 provisioning (Ubuntu 22.04, Elastic IP, Security Group)
- Structured JSON logging via `slog` with `request_id`, method, path, status, and duration fields
- Prometheus metrics endpoint (`/metrics`) with request counters and latency histograms
- ServiceMonitor for Prometheus scraping, scoped to `dev` and `prod` namespaces
- Error rate PrometheusRule alert
- `RUNBOOK.md` — restart, rollback, log inspection, and health check procedures
- `SECURITY.md` — secret rotation steps, security controls, and incident response playbook
- `DECISIONS.md` — Architecture Decision Records (ADR-01 – ADR-03)

### Fixed
- Corrected open `if` condition in metrics handler

### Reverted
- Temporary replica scale-up used during load testing

## [0.1.1] — 2026-05-23

### Added
- Introduced ArgoCD-based GitOps deployment workflow
- Added automated Helm manifest updates for dev and prod environments
- Added semantic version-based production release flow
- Added environment-specific deployment separation between dev and prod namespaces

### Changed
- Aligned GHCR image tags with Git semantic version tags for production releases
- Switched development deployments to immutable SHA-based image tags

## [0.1.0] — 2026-05-20

### Added
- Initial Go 1.22 HTTP service with `/ping`, `/healthz`, and `/version` endpoints
- Graceful shutdown on `SIGINT`/`SIGTERM` with configurable `ADDR` and `VERSION` env vars
- Multi-stage Dockerfile (`golang:1.24-alpine` builder → `alpine:3.22` final) with non-root user and `HEALTHCHECK`
- Helm chart with Deployment, Service, Ingress, HPA, PodDisruptionBudget, NetworkPolicy, ConfigMap, and Secret templates
- Environment-specific Helm values (`values-dev.yaml`, `values-prod.yaml`)
- GitHub Actions CI/CD pipeline:
  - **Lint** — Gitleaks secret scan + golangci-lint
  - **Test** — `go test -race` + `govulncheck` (Go vulnerability database)
  - **Docker** — BuildKit image build, Trivy OS-package scan (CRITICAL/HIGH, unfixed ignored), GHCR push
  - **Deploy** — secretless AWS OIDC authentication + Helm upgrade via SSM on push to `main`
- CODEOWNERS file and pull request template
- `.trivyignore`-ready pipeline with SARIF upload to GitHub Security tab

### Changed
- Renamed module and Helm chart from `pingsvc` to `insider-service`

[0.1.0]: https://github.com/gizemcell/insider-one-service/releases/tag/v0.1.0
