# Changelog

All notable changes to this project will be documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/);
versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- Moved Go source and `go.mod` into `app/` subdirectory
- Updated Dockerfile to `COPY app/ .` to avoid duplicate module root
- Updated CI workflow `go-version-file`, `golangci-lint`, `go test`, and `govulncheck` steps to use `app/` working directory

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
