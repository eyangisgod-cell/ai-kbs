# ADR-0005: MCP as the Canonical AI Agent Entry Point

- **Status:** Accepted
- **Date:** 2026-10-02

## Context

The primary users of AI-KBS are AI coding agents (Claude Code, Codex, Cursor, Gemini CLI). Each has its own integration story; building a custom one per agent is unsustainable.

## Decision

Adopt the **Model Context Protocol (MCP)** as the canonical agent entry point. The MCP server supports `stdio` (for local agents), `sse` and `http` (for remote / SaaS), and `oauth2` (for first-party agent registration).

`ai-kbs mcp install --agent <name>` writes the platform-specific config file (`.mcp.json` for Claude Code, `~/.codex/config.toml` for Codex, etc.) and prints the agent restart command.

## Consequences

Easier:
- One integration story across all major agent platforms
- The tool surface is documented and stable (frozen schema v1.0)

Harder:
- We depend on the MCP standard evolving predictably
- OAuth2 + capability-scope setup is non-trivial

## Alternatives considered

1. **Build per-agent integrations (Claude Code extension, Codex plugin, etc.).** Rejected: 4× the work, 4× the maintenance.
2. **Adopt LangChain tool format.** Rejected: LangChain is a Python-only library and the agents we target are not LangChain-based.
