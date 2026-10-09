---
layout: home

hero:
  name: "AI-KBS"
  text: "AI-Agent-first Knowledge Base"
  tagline: "Open-source. MCP-native. Multi-hop retrieval via ES + RAG + SAG."
  actions:
    - theme: brand
      text: Get Started
      link: /getting-started/
    - theme: alt
      text: View on GitHub
      link: https://github.com/eyangisgod-cell/ai-kbs
    - theme: alt
      text: MCP Tools
      link: /mcp/

features:
  - title: MCP-Native
    details: First-class Model Context Protocol server (stdio / SSE / HTTP / OAuth2). One `ai-kbs mcp install` command connects Claude Code, Codex, Cursor, or Gemini CLI.
  - title: Three-Layer Retrieval
    details: ES (BM25 + Dense + Hybrid) + WeKnora RAG + SAG (event-entity dynamic hyperedge) coexist. Query classifier routes per question; auto-fallback if any layer is unavailable.
  - title: Workspace / Sync
    details: Local directory ↔ remote Space bidirectional sync. docCode stable identifiers survive rename/move. CAS optimistic locking with auto conflict resolution.
  - title: Audit & Rollback
    details: Every write operation is auditable. Document versions are an append-only chain; rollback produces a new version and never deletes intermediate ones.
  - title: Open Standards
    details: HMAC + JWT + Scoped API Keys. OAuth2 (Google, GitHub, email+password). OpenTelemetry / Langfuse observability. Apache 2.0 licensed.
  - title: Self-Hostable
    details: Docker Compose one-shot dev stack ≤ 10 min. Helm Chart for production Kubernetes. No managed-service lock-in.
---
