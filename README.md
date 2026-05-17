# pingsvc

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
docker build -t pingsvc .
docker run --rm -p 8080:8080 pingsvc
```

Pass a version at build time and override the listen address at runtime:

```sh
docker build --build-arg VERSION=v1.2.3 -t pingsvc .
docker run --rm -e ADDR=:9090 -p 9090:9090 pingsvc
```

Test the endpoints:

```sh
curl http://localhost:8080/ping
curl http://localhost:8080/healthz
curl http://localhost:8080/version
```

## Notes

- `/healthz` currently has no dependencies to check, so it always returns OK.
  Add real dependency checks (DB, cache, downstream services) inside
  `handleHealthz` as the service grows — that's the intended extension point.
- The server does a graceful shutdown on `SIGINT`/`SIGTERM` with a 10s timeout,
  so it plays nicely with rolling deploys.
- Sensible HTTP timeouts are set to avoid slowloris-style resource exhaustion.
