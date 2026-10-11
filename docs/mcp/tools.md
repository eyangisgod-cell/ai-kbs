# MCP Tools Reference

All tools are defined in `.mcp.json` after `ai-kbs mcp install`. The schema is frozen at v1.0; new fields bump the schema version.

## Common input shape

```json
{
  "q": "...",
  "spaces": ["dev"],
  "limit": 10,
  "engine": "auto"
}
```

`spaces` is optional; if omitted, the tool operates over all Spaces the calling key can see.

## Common output shape

```json
{
  "engine": "es",
  "took_ms": 42,
  "hits": [...]
}
```

## ai_kbs_search

```json
{
  "q": "What is the rollback procedure?",
  "spaces": ["dev", "ops"],
  "engine": "auto",
  "limit": 10
}
```

## ai_kbs_read_doc

```json
{
  "doc_code": "aB3xY9zQ",
  "lines": 200,
  "offset": 0
}
```

If the document exceeds 5MB, paginate with `lines` + `offset`.

## ai_kbs_ask (NDJSON stream)

`ai_kbs_ask` returns a stream of NDJSON events. See [the API reference](/api/search#ask-ndjson-stream) for the event types.

```json
{
  "q": "Compare Q3 and Q4 revenue trajectories",
  "session_id": "s_...",
  "kb_ids": ["dev"]
}
```

## ai_kbs_create_doc

```json
{
  "space": "dev",
  "title": "Onboarding",
  "path": "/handbook/onboarding.md",
  "content": "...",
  "format": "markdown",
  "base_version": 0
}
```

## ai_kbs_update_doc (CAS)

```json
{
  "doc_code": "aB3xY9zQ",
  "content": "...",
  "base_version": 7
}
```

On conflict, returns `22 VersionConflict` with `next_action.strategies` populated.

## ai_kbs_edit_doc

```json
{
  "doc_code": "aB3xY9zQ",
  "find": "old text",
  "replacement": "new text",
  "replace_all": false,
  "base_version": 7
}
```

## ai_kbs_move_doc

```json
{
  "doc_code": "aB3xY9zQ",
  "new_path": "/handbook/v2/onboarding.md",
  "base_version": 7
}
```

## ai_kbs_delete_doc

```json
{
  "doc_code": "aB3xY9zQ",
  "hard": false,
  "dry_run": true
}
```

Set `hard: true` for permanent deletion (Owner scope only). Use `dry_run: true` to preview what would be deleted.

## ai_kbs_list_versions

```json
{ "doc_code": "aB3xY9zQ", "limit": 50 }
```

## ai_kbs_diff_versions

```json
{ "doc_code": "aB3xY9zQ", "from": 5, "to": 7, "format": "unified" }
```

## ai_kbs_rollback_version

```json
{ "doc_code": "aB3xY9zQ", "to_version": 5, "base_version": 7 }
```

## ai_kbs_sag_query

```json
{ "q": "A 收购 B 后 C 产品线如何?", "spaces": ["dev"], "limit": 5 }
```

## ai_kbs_sag_explain

```json
{ "doc_code": "aB3xY9zQ", "q": "..." }
```

Returns the event → entity → event path that SAG used to reach the answer.
