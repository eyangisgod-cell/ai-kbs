# AI-KBS (AI Knowledge Base System)

> **AI-Agent-first open-source enterprise knowledge base system.**
> 面向 AI Agent 开发协作者的开源企业级知识库系统。

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](./LICENSE)
[![Status](https://img.shields.io/badge/status-pre--alpha-orange.svg)]()
[![Backend](https://img.shields.io/badge/backend-forked%20from%20WeKnora-blueviolet.svg)](https://github.com/Tencent/WeKnora)

## Introduction / 简介

**AI-KBS** is an open-source enterprise knowledge base system built from the ground up to be the
**primary substrate for AI Agents**. While traditional wikis force agents to scrape HTTP pages and
RAG tools offer only a thin REST facade, AI-KBS treats AI Agents as first-class citizens with:

- **MCP (Model Context Protocol) as the primary entry point** — `ai-kbs mcp install --agent {claude-code|codex|cursor|gemini-cli}` configures Claude Code / Codex / Cursor / Gemini CLI in one command.
- **CLI with frozen JSON envelope** — every command emits a deterministic `next_action` envelope so agents can branch on outcomes without parsing prose.
- **Multi-hop retrieval** — a structure-aware graph (SAG) layer works alongside Elasticsearch + RAG to deliver state-of-the-art Recall@5 on 2WikiMultiHopQA / MuSiQue style questions.
- **CAS + docCode version chain** — every write is optimistic-lock with a stable 8-char base62 identifier; renames never break references; rollback produces new versions and keeps history.
- **Workspace / bind / pull / push** — local directories sync bidirectionally to remote Spaces with 8-way concurrency, the same ergonomic model engineers already use with Git, but designed for documents (fine-grained RBAC, audit, no Git LFS complexity).

## Features / 核心特性

| Capability | What you get |
|---|---|
| **MCP server** | stdio / SSE / HTTP transports, OAuth2 scopes (`kbs:read` / `kbs:write` / `kbs:version` / `kbs:admin`) |
| **AI-friendly CLI** | ~20 subcommands; `--json` frozen envelope v1.0; SKILL.md bundled with the binary |
| **Triple-layer retrieval** | Elasticsearch (BM25 + Dense + Hybrid) + RAG framework (WeKnora) + SAG (multi-hop); query classifier routes per-question |
| **Document CRUD + version chain** | CAS optimistic lock; rollback = new version, history preserved; soft delete + undelete |
| **Workspace sync** | 8-way concurrent pull / push; rename detection via stable docCode; >10-file or >5MB deletes require `--yes` |
| **ReAct Agent + Q&A** | Streamed NDJSON (`init / thinking / tool_call / tool_result / answer / complete`); every answer carries `doc_code / title / quote / url` citations |
| **Auth + RBAC** | HMAC-SHA256 (CLI/MCP), JWT (Web/API), Scoped API Key (embedded); 4-level RBAC (Owner/Admin/Contributor/Viewer); IDOR + SSRF guards |
| **Document parsing** | Markdown / PDF / Word / HTML / Txt (v1); async worker pool + DLQ |
| **Object storage** | Pluggable adapters: Local / MinIO / S3 / OSS / TOS / KS3 / OBS |
| **Observability** | Langfuse OTLP integration; W3C `traceparent` propagation; key metrics (QPS / latency / token usage / queue depth) |
| **Deployment** | Docker Compose (dev, ≤ 10 min) + Helm Chart (prod, HPA + health checks + ingress) |

## Quick Start / 快速开始

> Requirements: Docker 24+ / Docker Compose v2 / Go 1.23+ (for self-build) / Node 20+ (for frontend).

```bash
# 1. Clone
git clone https://github.com/eyangisgod-cell/ai-kbs.git
cd ai-kbs

# 2. Bring up the full stack (PostgreSQL + Elasticsearch + Redis + RabbitMQ + Langfuse + backend + mcp-server + frontend)
docker compose -f docker-compose.dev.yml up -d

# 3. Install the AI-KBS CLI (single static binary)
#    see ./cli/ — built with CGO_ENABLED=0 for linux/darwin/windows × amd64/arm64

# 4. Configure credentials
ai-kbs login --url http://localhost:8080

# 5. Initialize a workspace and bind to a Space
mkdir docs && cd docs
ai-kbs init
ai-kbs bind --space dev
ai-kbs pull

# 6. Install MCP for Claude Code in one command
ai-kbs mcp install --agent claude-code
```

After step 6, restart Claude Code; it will see tools such as
`ai_kbs_search`, `ai_kbs_read_doc`, `ai_kbs_ask`, `ai_kbs_create_doc`, `ai_kbs_update_doc`,
`ai_kbs_edit_doc`, `ai_kbs_list_versions`, `ai_kbs_rollback_version`, etc.

## Architecture / 架构

```
┌──────────────────────────────────────────────────────────────┐
│                  AI Agent 客户端层                            │
│   Claude Code   Codex   Cursor   Gemini CLI                  │
└───────────────────────────┬──────────────────────────────────┘
                            │ MCP (stdio / SSE / HTTP / OAuth2)
                            ▼
┌──────────────────────────────────────────────────────────────┐
│            ai-kbs-mcp-server  (Python, FastMCP)              │
└───────────────────────────┬──────────────────────────────────┘
                            │ HTTPS REST + NDJSON
                            ▼
┌──────────────────────────────────────────────────────────────┐
│           ai-kbs-backend  (Go, forked from WeKnora)          │
│  Auth(HMAC/JWT/APIKey) | RBAC | Audit | Trace | Limiter     │
└────┬───────────┬───────────┬──────────────┬─────────────────┘
     ▼           ▼           ▼              ▼
 PostgreSQL   Elastic       ai-kbs-       ai-kbs-
 +pgvector    Search 8.x    sag-bridge    docreader
                              (SAG)       (gRPC)
```

See [`docs/architecture.md`](./docs/architecture.md) (forthcoming) for the full diagram including
storage, queueing, observability, and frontend.

## Repository Layout / 仓库结构

| Path | Purpose |
|---|---|
| `backend/` | Go backend (forked from WeKnora v0.7.1, module `github.com/eyangisgod-cell/ai-kbs`) |
| `cli/` | Go CLI (`ai-kbs`), bundled SKILL.md |
| `mcp-server/` | Python FastMCP server (stdio / SSE / HTTP, OAuth2) |
| `sag-bridge/` | Python FastAPI bridge exposing the SAG multi-hop retrieval layer |
| `docreader/` | Python gRPC document parser (PDF / Word / Markdown / HTML / Txt) |
| `frontend/` | Vue 3 SPA (forked from WeKnora frontend; re-branded to AI-KBS) |
| `helm/` | Production Helm Chart (K8s deployment) |
| `docs/` | Public documentation |
| `require/` | Requirements discovery documents (zh-CN) |

## Documentation / 文档

- 中文需求文档 (PRD / 设计 / 任务): [`require/`](./require/)
- 公开文档入口 (待补全): [`docs/`](./docs/)
- CLI 操作手册 (SKILL): shipped inside the CLI binary at `cli/skills/SKILL.md`
- MCP 工具 frozen schema v1.0: shipped as JSON Schema alongside the mcp-server release artifact

## Contributing / 参与贡献

We welcome issues and pull requests. Before opening a PR, please read
[`AGENTS.md`](./AGENTS.md) for the working agreements and the Trellis multi-agent pipeline.

**Hard rules**:

1. **No internal platform names** appear in any source file, config, comment, doc, or commit message
   of this repository. The CI scrub audit will block PRs that violate this.
2. Frozen schemas (`api.frozen.json`, `--json` envelope, MCP tool input/output) are version-locked.
   Any change must bump the `version` field and add a migration note.
3. All Go identifiers are `camelCase` / `PascalCase`; CLI flags are `kebab-case`; env vars are
   `AIKBS_*`; JSON wire fields are `camelCase`.

## License / 许可证

This project is licensed under the **Apache License 2.0**. See [`LICENSE`](./LICENSE).

The `backend/` and parts of the `frontend/` directories are forks of
[WeKnora](https://github.com/Tencent/WeKnora) (also Apache 2.0). The original copyright notices are
preserved in those subtrees. ai-kbs is an independent project; we have re-themed the product name,
white-labelled the frontend, and redesigned CLI commands / config schema to remove all upstream
proprietary touchpoints.
