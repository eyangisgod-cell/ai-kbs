# Getting Started

Welcome to AI-KBS — an open-source, AI-Agent-first knowledge base.

## What is AI-KBS?

AI-KBS is a knowledge base system built **for AI coding agents**, not just humans. It exposes a Model Context Protocol (MCP) server so Claude Code, Codex, Cursor, and Gemini CLI can search, read, write, and reason over your documents.

## What you get

- **Three-layer retrieval**: Elasticsearch (BM25 + Dense) + WeKnora RAG + SAG (event-entity hyperedge) — all routed by a per-query classifier
- **MCP server**: stdio / SSE / HTTP transports, OAuth2 with scope-based keys
- **Workspace / Sync**: local directory ↔ remote Space, CAS optimistic locking, docCode stable IDs
- **CLI**: single static binary, cross-platform (Linux / macOS / Windows × amd64 / arm64)
- **Web UI**: Vue 3 SPA with markdown rendering, outline navigation, version history, NDJSON streaming answers
- **Observability**: Langfuse v3 with W3C trace context across all internal calls

## Where to go next

| Page | Purpose |
|------|---------|
| [Installation](/getting-started/installation) | Docker Compose, Helm, or source build |
| [Quick Tour](/getting-started/quick-tour) | 5-minute walkthrough of the core flows |
| [Architecture](/architecture/) | How the three retrieval layers cooperate |
| [CLI](/cli/) | Command reference and SKILL.md |
| [MCP](/mcp/) | Connecting AI agents |
