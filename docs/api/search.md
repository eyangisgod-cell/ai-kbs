# Search

## Basic

```http
POST /api/ai-kbs/search
Content-Type: application/json
Authorization: Bearer <token>

{
  "q": "What was the Q3 revenue?",
  "spaces": ["dev", "staging"],
  "limit": 10,
  "engine": "auto"
}
```

`engine`:

- `auto` — let the query classifier decide (default)
- `es` — Elasticsearch only
- `rag` — WeKnora RAG only
- `sag` — SAG only
- `hybrid` — ES + SAG in parallel, fused with cross-encoder rerank

## Response

```json
{
  "success": true,
  "data": {
    "engine": "hybrid",
    "took_ms": 247,
    "hits": [
      {
        "doc_code": "aB3xY9zQ",
        "title": "Q3 Financials",
        "path": "/finance/q3.md",
        "snippet": "...Q3 revenue was $4.2M, up 12% from Q2...",
        "score": 0.91,
        "engine_breakdown": {"es": 0.85, "sag": 0.93}
      }
    ]
  }
}
```

## Filters

```json
{
  "q": "...",
  "spaces": ["dev"],
  "tags": ["handbook", "policy"],
  "updated_after": "2026-01-01",
  "updated_by": "u_...",
  "limit": 10,
  "offset": 0
}
```

## Ask (NDJSON stream)

```http
POST /api/ai-kbs/ask
Content-Type: application/json
Accept: application/x-ndjson
Authorization: Bearer <token>

{
  "q": "Compare the Q3 and Q4 revenue trajectories",
  "session_id": null,
  "kb_ids": ["dev", "staging"]
}
```

The server streams NDJSON events:

```ndjson
{"type":"init","session_id":"s_..."}
{"type":"thinking","thought":"Multi-hop question; route to SAG."}
{"type":"tool_call","tool":"sag_query","args":{...}}
{"type":"tool_result","engine":"sag","hits":[...]}
{"type":"answer","text":"Q3 was $4.2M, Q4 was $4.8M (+14%)...","citations":[{"doc_code":"...","title":"...","quote":"..."}]}
{"type":"complete","usage":{"input_tokens":1234,"output_tokens":567}}
```

Clients should handle reconnection via `Last-Event-ID` for long streams.
