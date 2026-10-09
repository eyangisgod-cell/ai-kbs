# MCP Server

The Model Context Protocol server is the canonical entry point for AI agents.

## Transports

| Transport | Use case | Auth |
|-----------|----------|------|
| `stdio` | Local Claude Code / Codex (spawned subprocess) | n/a (parent process trust) |
| `sse`   | Long-lived browser / SaaS (port 8081) | OAuth2 or scoped API key |
| `http`  | One-shot web requests (port 8081) | OAuth2 or scoped API key |
| `oauth2`| First-party MCP client registration | OAuth2 code + PKCE |

## Tools (v1.0)

Tools are organized in layers. Each layer is a separate scope on a scoped API key.

### Core (read)

| Tool | Purpose |
|------|---------|
| `ai_kbs_search` | Multi-layer search |
| `ai_kbs_read_doc` | Read a single document |
| `ai_kbs_ask` | NDJSON streaming Agent answer |
| `ai_kbs_list_spaces` | List visible Spaces for the current key |
| `ai_kbs_get_doc_outline` | Outline / TOC of a document |

### Write

| Tool | Purpose |
|------|---------|
| `ai_kbs_create_doc` | Create a new document |
| `ai_kbs_update_doc` | Update with CAS |
| `ai_kbs_edit_doc` | Local edit, push with CAS |
| `ai_kbs_move_doc` | Move / rename (docCode stable) |
| `ai_kbs_delete_doc` | Soft-delete (or `?hard=true` for permanent) |

### Version

| Tool | Purpose |
|------|---------|
| `ai_kbs_list_versions` | Version history |
| `ai_kbs_diff_versions` | Unified diff between two versions |
| `ai_kbs_rollback_version` | Rollback (produces a new version) |

### SAG (multi-hop)

| Tool | Purpose |
|------|---------|
| `ai_kbs_sag_query` | Force-route a query through SAG |
| `ai_kbs_sag_explain` | Return the multi-hop path that produced a result |

### Admin (P1 — v1.1)

| Tool | Purpose |
|------|---------|
| `ai_kbs_create_space` | Create a new Space |
| `ai_kbs_manage_binding` | Bind / unbind a workspace |
| `ai_kbs_get_audit_log` | Audit log entries |
| `ai_kbs_manage_api_keys` | CRUD on scoped API keys |

## Input / output frozen schema v1.0

Every tool declares an `inputSchema` and `outputSchema` (JSON Schema). The schema is byte-stable across the v1.x line; new fields require a schema-version bump and a deprecation cycle.

Example: `ai_kbs_search`

```json
{
  "name": "ai_kbs_search",
  "inputSchema": {
    "type": "object",
    "required": ["q"],
    "properties": {
      "q": {"type": "string", "minLength": 1, "maxLength": 2000},
      "spaces": {"type": "array", "items": {"type": "string"}},
      "engine": {"enum": ["auto", "es", "rag", "sag", "hybrid"], "default": "auto"},
      "limit": {"type": "integer", "minimum": 1, "maximum": 100, "default": 10}
    }
  },
  "outputSchema": {
    "type": "object",
    "required": ["engine", "hits"],
    "properties": {
      "engine": {"type": "string"},
      "took_ms": {"type": "integer"},
      "hits": {"type": "array", "items": {"$ref": "#/Hit"}}
    }
  }
}
```

## Limits

- 60 requests per minute per API key (configurable per-key)
- `ai_kbs_read_doc` returns at most 5MB; larger documents must be read with `lines + offset` pagination
- All destructive tools accept `dry_run: true` to preview without committing
