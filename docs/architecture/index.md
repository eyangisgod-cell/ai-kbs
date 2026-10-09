# Architecture

AI-KBS is built around three cooperating subsystems.

## At a glance

```
            ┌─────────────────────────────────────────────┐
            │            AI Agent (Claude Code)            │
            │   via MCP ────────────┐                      │
            └───────────────────────┼──────────────────────┘
                                    │ stdio / SSE / HTTP / OAuth2
                                    ▼
            ┌─────────────────────────────────────────────┐
            │           ai-kbs mcp-server (FastMCP)        │
            │   tools: search, read, ask, write, version   │
            └───────────────────────┬──────────────────────┘
                                    │
                                    ▼
            ┌─────────────────────────────────────────────┐
            │         ai-kbs backend (Go / WeKnora)        │
            │   ┌─────────────────────────────────────┐   │
            │   │  ReAct Agent ─→ Query Classifier   │   │
            │   └────────┬─────────────┬─────────────┘   │
            │            ▼             ▼             ▼   │
            │        ┌─────┐      ┌──────┐     ┌──────┐  │
            │        │ ES  │      │ RAG  │     │ SAG  │  │
            │        │BM25 │      │WeKno-│     │Lance-│  │
            │        │+DNS │      │ra    │     │DB    │  │
            │        └─────┘      └──────┘     └──────┘  │
            └───────────────────────┬──────────────────────┘
                                    │
                                    ▼
            ┌─────────────────────────────────────────────┐
            │  PostgreSQL (pgvector) + Redis + RabbitMQ   │
            └─────────────────────────────────────────────┘
```

## Subsystems

- **Backend (Go)** — forked from WeKnora v0.7.1, with internal auth / agent / storage replaced and the three-layer retrieval stitched in
- **CLI (Go, single static binary)** — workspace / sync / version / search / ask / mcp commands
- **MCP server (Python, FastMCP)** — exposes the backend as MCP tools with capability-scoped keys
- **SAG bridge (Python, FastAPI)** — multi-hop retrieval over events, entities, and dynamic hyperedges, stored in LanceDB
- **Docreader (Python, gRPC)** — PDF / Word / Markdown / HTML / Txt parsing with chunking, embedding, and async workers

## Key design choices

- **Workspace / Space** model: a local directory is bound to a remote `Space` (a named KB); documents have stable `docCode` IDs that survive rename
- **CAS optimistic locking**: writes carry `base_version`; conflicts return `next_action.strategies`
- **Append-only versions**: rollback produces a new version; intermediate versions are never deleted
- **Capability-scoped API keys**: `kbs:read`, `kbs:write`, `kbs:version`, `kbs:admin`
- **W3C trace context**: every external call carries `traceparent`; Langfuse dashboard shows full span tree

## Where to read more

- [Three-layer retrieval deep-dive](/architecture/retrieval)
- [Workspace / sync model](/architecture/workspace)
- [ADR-001 (fork WeKnora)](/adr/0001-fork-weknora)
