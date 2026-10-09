# CLI Command Reference

The full surface, grouped by area.

## Workspace / Sync

| Command | Flags | Notes |
|---------|-------|-------|
| `ai-kbs init` | `--space <code>` | Creates `.ai-kbs/workspace.yaml` |
| `ai-kbs status` | `--json` | Workspace, current Space, sync state summary |
| `ai-kbs bind <local-path> <space-code>:<remote-prefix>` | `--read-only`, `--no-push` | Write binding into `workspace.yaml` |
| `ai-kbs unbind <local-path>` | | |
| `ai-kbs pull` | `--concurrency 8`, `--force` | 8-way concurrent; respects `sync-state.json` |
| `ai-kbs push` | `--concurrency 8`, `--yes` | 8-way concurrent; requires `--yes` for >10 files or >5MB single file |
| `ai-kbs resolve <file>...` | `--strategy keep-mine\|keep-theirs\|both\|auto` | Apply a CAS conflict resolution |

## Documents

| Command | Flags | Notes |
|---------|-------|-------|
| `ai-kbs read` | `--url <u>`, `--space <s> --doc <d>`, `--space <s> --path <p>` (mutually exclusive) | Address by URL, docCode, or path |
| `ai-kbs write` | `--title`, `--content @file`, `--base-version <n>`, `--dir` (batch) | Create or update with CAS |
| `ai-kbs edit` | `--replace-all`, `--find`, `--replacement`, `--dry-run`, `--base-version <n>` | Local edit, push with CAS |
| `ai-kbs search <query>` | `--space`, `--limit`, `--engine auto\|es\|rag\|sag\|hybrid` | |
| `ai-kbs grep <regex>` | `--space`, `--include`, `--exclude` | |
| `ai-kbs tree` | `--space`, `--depth` | |

## Versions

| Command | Flags | Notes |
|---------|-------|-------|
| `ai-kbs version list` | `--space`, `--doc` | |
| `ai-kbs version diff <doc> --from <n> --to <n>` | `--format unified\|json` | |
| `ai-kbs version rollback <doc> --to <n>` | `--base-version <n>` | Produces a new version |

## Maintenance

| Command | Flags | Notes |
|---------|-------|-------|
| `ai-kbs move <doc> <new-path>` | `--base-version <n>` | docCode is stable; just changes the path |
| `ai-kbs undelete <doc>` | | Restores a soft-deleted document |
| `ai-kbs spaces` | `--json` | List visible Spaces and current user role |

## AI / RAG

| Command | Flags | Notes |
|---------|-------|-------|
| `ai-kbs ask <question>` | `--space`, `--session-id`, `--engine auto\|es\|rag\|sag\|hybrid` | NDJSON stream |

## MCP

| Command | Flags | Notes |
|---------|-------|-------|
| `ai-kbs mcp install --agent <claude-code\|codex\|cursor\|gemini-cli>` | `--api-key`, `--scopes` | Writes `.mcp.json` (or platform equivalent) and prints restart instructions |
| `ai-kbs mcp serve` | `--transport stdio\|sse\|http`, `--port` | Run the MCP server manually |
| `ai-kbs mcp tools list` | | Print all tool schemas |

## Global

| Flag | Notes |
|------|-------|
| `--url <url>` | Backend URL (overrides env) |
| `--profile <name>` | Config profile |
| `--format text\|json\|ndjson` | Output format |
| `--quiet` | Suppress non-essential output |
| `--yes` | Skip confirmation prompts |
| `--timeout <dur>` | Per-request timeout |
| `--debug` | Debug logging |

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | success |
| 1 | generic failure |
| 2 | misuse (bad input) |
| 10 | auth missing / invalid |
| 11 | permission denied (RBAC) |
| 20 | not found |
| 21 | already exists |
| 22 | version conflict (CAS) |
| 23 | partial sync failure |
| 24 | rate-limited |
| 25 | upstream timeout |
| 30 | search error |
| 31 | SAG error |
| 32 | LLM error |
| 40 | internal error |
| 41 | config error |
