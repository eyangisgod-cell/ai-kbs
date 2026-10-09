# Langfuse observability stack (T0.7)

This directory ships the assets for the AI-KBS observability layer.

## Layout

```
docker/langfuse/
├── README.md
├── dashboards/
│   └── ai-kbs-default.json       # 7-panel default dashboard (NFR-OBS-02/03)
└── bootstrap/
    ├── Dockerfile                 # sidecar image: python:3.12-slim + requests
    ├── bootstrap.py               # imports the dashboard via NextAuth + REST API
    └── requirements.txt
```

## How the bootstrap is wired

`docker-compose.dev.yml` defines a `langfuse-bootstrap` service that:

1. Waits for `langfuse` to be healthy
2. Logs in as `$LANGFUSE_INIT_USER_EMAIL` (NextAuth credentials provider)
3. Resolves the project id (`$LANGFUSE_INIT_PROJECT_NAME` → "ai-kbs")
4. If a dashboard named "AI-KBS Default" doesn't already exist, POSTs
   `dashboards/ai-kbs-default.json` to `/api/projects/{id}/dashboards`
5. Exits 0

The sidecar uses `restart: on-failure` with a `max_attempts: 3` policy so a
transient Langfuse startup hiccup doesn't poison the whole dev stack.

## The 7 panels

| # | Panel | Metric | NFR target |
|---|-------|--------|------------|
| 1 | QPS | trace.count | — |
| 2 | Latency P50 | trace.latency.p50 | — |
| 3 | Latency P95 | trace.latency.p95 | NFR-PERF-01 (ES) / NFR-PERF-02 (SAG) / NFR-PERF-06 (MCP) |
| 4 | Latency P99 | trace.latency.p99 | — |
| 5 | Error rate | trace.error_rate | — |
| 6 | LLM token usage | llm.tokens (stacked) | — |
| 7 | Queue depth | queue.depth | NFR-OBS-03 |

The dashboard JSON references these metric names; the actual exporter that
populates them ships in M1 (T1.8 / T1.21). Until then the dashboard renders
empty panels in the Langfuse UI — that's expected and harmless.

## Manual re-import (if the bootstrap fails)

```bash
# from inside the langfuse-bootstrap container
docker compose -f docker-compose.dev.yml exec langfuse-bootstrap python /app/bootstrap.py
```

The script is idempotent: it skips if the dashboard already exists by name.

## K8s (Helm chart)

The Helm chart (T0.4) deploys Langfuse as a subchart. The bootstrap sidecar
becomes a Kubernetes `Job` (not a sidecar) that runs once on first install.
The exact manifest lands in T0.4 (this directory does not ship a k8s Job
template — that lives with the rest of the Helm chart).
