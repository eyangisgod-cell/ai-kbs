# ADR-0002: Add SAG as a Multi-Hop Retrieval Layer

- **Status:** Accepted
- **Date:** 2026-10-02

## Context

ES (BM25 + Dense + Hybrid) handles single-hop keyword and semantic recall well, but multi-hop questions ("A 收购 B 后 C 产品线如何?") require explicit reasoning over entities and their relations. WeKnora RAG alone is a single-engine vector store and is not multi-hop-aware.

## Decision

Add SAG (Structure-Aware Graph for multi-hop reasoning) as a third retrieval layer, alongside ES and RAG. The query classifier routes multi-hop / entity-relation questions to SAG, and fuses results with cross-encoder rerank.

SAG ships as a separate Python service (FastAPI + LanceDB), with the backend orchestrating via a thin gRPC / HTTP bridge.

## Consequences

Easier:
- Better multi-hop recall (target: ≥ WeKnora baseline +5pp on 2WikiMultiHopQA / MuSiQue)
- A clear route to GraphRAG-style reasoning without building a full GraphRAG stack

Harder:
- One more service to operate
- SAG indexer pipeline must keep pace with document writes

## Alternatives considered

1. **GraphRAG (Microsoft).** Rejected: high build cost, poor incremental update story.
2. **HyDE.** Deferred to v1.1 as a query-expansion enhancement.
3. **ColBERT.** Deferred to v1.1; better for keyword-rare queries.
