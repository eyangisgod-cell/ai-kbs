# Installation

AI-KBS has three deployment paths.

## 1. Docker Compose (development)

```bash
git clone https://github.com/eyangisgod-cell/ai-kbs
cd ai-kbs
cp .env.example .env       # edit secrets
docker compose -f docker-compose.dev.yml up -d
```

The dev stack brings up: postgres, redis, rabbitmq, elasticsearch (with IK Analysis), docreader, sag-bridge, backend, mcp-server, langfuse. First start takes ~3 minutes for image pulls.

Verify: `curl http://localhost:8080/healthz` → `200 OK`.

## 2. Helm Chart (production)

```bash
helm repo add ai-kbs https://eyangisgod-cell.github.io/ai-kbs
helm install ai-kbs ai-kbs/ai-kbs \
  --namespace ai-kbs --create-namespace \
  --set backend.image.tag=v1.0.0 \
  --set ingress.host=kbs.example.com
```

See [helm/values.yaml](https://github.com/eyangisgod-cell/ai-kbs/blob/main/helm/values.yaml) for the full parameter catalog.

## 3. Source build

```bash
git clone https://github.com/eyangisgod-cell/ai-kbs
cd ai-kbs
make cli-build           # outputs ./bin/ai-kbs-<os>-<arch>
make backend-build      # outputs ./bin/ai-kbs-backend
```

The CLI is a single static binary; copy it anywhere on `$PATH`.

## Next steps

- [Quick Tour](/getting-started/quick-tour)
- [CLI reference](/cli/)
- [MCP server setup](/mcp/install)
