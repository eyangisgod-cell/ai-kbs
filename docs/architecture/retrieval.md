# Three-Layer Retrieval

AI-KBS runs three retrieval layers in parallel and routes per query.

## The layers

| Layer | Engine | Strengths | Cost |
|-------|--------|-----------|------|
| **ES** | Elasticsearch 8.x with IK Analysis | BM25 keyword, dense vector, hybrid. Strong on exact-term / Chinese. | ~50ms P95 |
| **RAG** | WeKnora RAG (vector + rerank) | Single-engine fallback path; well-tested. | ~200ms P95 |
| **SAG** | Event-entity dynamic hyperedge | Multi-hop reasoning. Catches questions ES+RAG miss. | ~500ms P95 |

The query classifier (LLM-call + rules) decides per-question which layer to use. Single-hop keyword questions go to ES. Multi-hop / entity-relation questions go to SAG. Uncertain → all three, fused with cross-encoder rerank.

## Graceful degradation

- If ES is unavailable, the system falls back to PG full-text search (which is on by default)
- If SAG is unavailable, the system falls back to RAG-only (ES + pgvector)
- If both ES and SAG are unavailable, the system still serves via RAG with degraded recall

## Response envelope

```json
{
  "success": true,
  "data": {
    "engine": "sag",
    "hits": [
      {
        "doc_code": "aB3xY9zQ",
        "title": "...",
        "quote": "...",
        "score": 0.87,
        "path": "events/e_12 -> entities/aB3xY9zQ -> events/e_19"
      }
    ]
  }
}
```

The `engine` field tells the caller which layer produced the answer. The CLI / Web UI highlights the multi-hop path when `engine == "sag"`.
