# insider-service

A tiny Go HTTP service with three endpoints, intended as a probe/health target.

## Endpoints

| Method & Path | Response | Purpose |
|---|---|---|
| `GET /ping` | `pong` (text/plain) | Quick liveness ping |
| `GET /healthz` | `{"status":"ok"}` (json) | Orchestrator readiness/liveness probe |
| `GET /version` | `{"version":"<sha>"}` (json) | Reports the deployed build SHA |

Non-GET requests to these paths return `405 Method Not Allowed` (handled by
the Go 1.22 method-aware `ServeMux`).

## Build & run

```sh
docker build -t insider-service .
docker run --rm -p 8080:8080 insider-service
```

Pass a version at build time and override the listen address at runtime:

```sh
docker build --build-arg VERSION=v1.2.3 -t insider-service .
docker run --rm -e ADDR=:9090 -p 9090:9090 insider-service
```

Test the endpoints:

```sh
curl http://localhost:8080/ping
curl http://localhost:8080/healthz
curl http://localhost:8080/version
```

## Helm

The chart lives in `chart/`. A base `values.yaml` provides defaults; environment
overrides are layered on top with `-f`:

```sh
# dev — 1 replica, relaxed resources, ingress on insider-service.dev.local
helm install insider-service ./chart -f chart/values-dev.yaml --set image.tag=v1.2.3

# prod — 3 replicas, tighter resources, ingress on insider-service.example.com with TLS
helm install insider-service ./chart -f chart/values-prod.yaml --set image.tag=v1.2.3
```

| Setting | dev | prod |
|---|---|---|
| `replicaCount` | 1 | 3 |
| `image.pullPolicy` | `Always` | `IfNotPresent` |
| `ingress.host` | `insider-service.dev.local` | `insider-service.example.com` |
| `ingress.tls` | — | cert via `insider-service-tls` secret |
| CPU request / limit | 50 m / 200 m | 100 m / 500 m |
| Memory request / limit | 32 Mi / 64 Mi | 64 Mi / 128 Mi |

**HPA** — a CPU-based autoscaler (target 70 %) is enabled in prod with a floor
of 3 and a ceiling of 10 replicas. 70 % leaves enough headroom for a traffic
spike to be absorbed before a new pod is scheduled and ready.

**NetworkPolicy** — allows inbound traffic only from the `ingress-nginx`
namespace. Without this, any pod in the cluster can reach the service directly,
bypassing the ingress layer entirely.

**PodDisruptionBudget** — `minAvailable: 2` ensures at least two replicas stay
up during voluntary disruptions (node drain, cluster upgrade). With three
replicas in prod this allows one to be evicted at a time, keeping the service
live throughout a rolling maintenance window.

**Probes** — all three probes (startup, liveness, readiness) hit `/healthz`.
The startup probe allows up to 60 s (12 × 5 s) for the process to become
healthy before the liveness probe takes over, which avoids premature restarts
during slow node pulls. Liveness and readiness both use a 10 s period with 3
failures before acting, giving transient hiccups room to self-recover.

**Resources** — values are sized for a pure HTTP process with no external
dependencies. Requests are set low enough for bin-packing in dev and to give
the scheduler an accurate picture in prod; limits cap runaway memory growth
without being so tight that normal GC spikes trigger an OOMKill.

## Upgrade & rollback

Build and push the new image, then hand the new tag to `helm upgrade`:

```sh
docker build --build-arg VERSION=v1.3.0 -t insider-service:v1.3.0 .
helm upgrade insider-service ./chart -f chart/values-prod.yaml --set image.tag=v1.3.0
```

Watch the rollout complete before declaring success:

```sh
kubectl rollout status deployment/insider-service
# Waiting for deployment "insider-service" rollout to finish: 1 out of 3 new replicas have been updated...
# Waiting for deployment "insider-service" rollout to finish: 2 out of 3 new replicas have been updated...
# Waiting for deployment "insider-service" rollout to finish: 1 old replicas are pending termination...
# deployment "insider-service" successfully rolled out
```

Inspect the release history to see what changed and when:

```sh
helm history insider-service
# REVISION  UPDATED                   STATUS      CHART          APP VERSION  DESCRIPTION
# 1         Sun May 17 10:00:00 2026  superseded  insider-service-0.1.0  v1.0.0       Install complete
# 2         Sun May 17 10:05:00 2026  deployed    insider-service-0.1.0  v1.0.0       Upgrade complete
```

If something looks wrong, roll back to the previous revision:

```sh
helm rollback insider-service
kubectl rollout status deployment/insider-service
# deployment "insider-service" successfully rolled out

helm history insider-service
# REVISION  UPDATED                   STATUS      CHART          APP VERSION  DESCRIPTION
# 1         Sun May 17 10:00:00 2026  superseded  insider-service-0.1.0  v1.0.0       Install complete
# 2         Sun May 17 10:05:00 2026  superseded  insider-service-0.1.0  v1.0.0       Upgrade complete
# 3         Sun May 17 10:08:00 2026  deployed    insider-service-0.1.0  v1.0.0       Rollback to 1
```

To roll back to a specific revision rather than the previous one, pass its number: `helm rollback insider-service 1`.

## Notes

- `/healthz` currently has no dependencies to check, so it always returns OK.
  Add real dependency checks (DB, cache, downstream services) inside
  `handleHealthz` as the service grows — that's the intended extension point.
- The server does a graceful shutdown on `SIGINT`/`SIGTERM` with a 10s timeout,
  so it plays nicely with rolling deploys.
- Sensible HTTP timeouts are set to avoid slowloris-style resource exhaustion.
