// This file is documentation-only: it shows how business code should
// wrap DB / ES / SAG / LLM calls with child spans. The actual unit
// tests for the helpers live in traceparent_test.go and config_test.go.
//
// The OpenTelemetry example convention requires `Example*` functions;
// we leave this file as plain comments so godoc renders the patterns
// without forcing an Example function to compile (godoc's Example
// extractor mistakes `ExampleStartChildSpan_DB` for a method `DB` on
// `StartChildSpan`). The patterns below are copy-pasteable.
package observe_test

// Wrapping a database call:
//
//	mgr := observe.GetManager()
//	ctx, span := observe.StartChildSpan(ctx, mgr,
//	    observe.DB().WithName("doc_repository.get"))
//	defer span.End()
//	doc, err := repo.GetByCode(ctx, code)
//	if err != nil {
//	    span.RecordError(err)
//	    return err
//	}
//	span.SetAttributes(map[string]string{"doc.code": code})
//
// Wrapping an Elasticsearch / OpenSearch search:
//
//	ctx, span := observe.StartChildSpan(ctx, mgr,
//	    observe.ES().WithName("knowledge_search.execute"))
//	defer span.End()
//	hits, err := idx.Search(ctx, q)
//	if err != nil {
//	    span.RecordError(err)
//	    return nil, err
//	}
//	span.SetAttributes(map[string]string{
//	    "search.hits": strconv.Itoa(len(hits)),
//	})
//
// Wrapping a SAG service call:
//
//	ctx, span := observe.StartChildSpan(ctx, mgr,
//	    observe.SAG().WithName("graph.augment"))
//	defer span.End()
//	out, err := sag.Augment(ctx, query)
//	if err != nil { span.RecordError(err); return nil, err }
//
// Wrapping an LLM call (use tracing/langfuse's StartGeneration for
// full token-usage capture; StartChildSpan is for non-generation
// work adjacent to the model call):
//
//	ctx, span := observe.StartChildSpan(ctx, mgr,
//	    observe.LLM().WithName("openai.chat"))
//	defer span.End()
//	resp, err := client.Chat(ctx, msgs)
//	if err != nil { span.RecordError(err); return nil, err }
//
// Stamping the envelope trace_id at the end of the handler:
//
//	env := observe.NewEnvelopeWithTraceID(requestID, c.GetHeader("traceparent"))
//	envelope.WriteJSON(c, http.StatusOK, env)
//
// (the second call to ParseTraceparent inside NewEnvelopeWithTraceID is
//  cheap — it's a single string scan — so production handlers can do
//  this without worrying about double-parse cost.)
//
// ---------------------------------------------------------------------------
// Prometheus aggregate metrics (T1.21) — example call sites.
// The HTTP middleware at internal/middleware/metrics.go covers QPS /
// latency / error rate; the helpers below cover queue depth, LLM token
// usage, and cache hit / miss.
//
// Recording queue depth after an asynq inspect call:
//
//	queues, _ := inspector.Queues()
//	for _, q := range queues {
//	    info, _ := inspector.GetQueueInfo(q)
//	    observe.SetQueueDepth(q, info.Pending)
//	}
//
// Recording LLM token usage at the call site of a chat completion:
//
//	resp, err := client.Chat(ctx, msgs)
//	if err == nil {
//	    observe.AddLLMTokens(modelName, observe.LLMDirectionPrompt,     int64(resp.Usage.PromptTokens))
//	    observe.AddLLMTokens(modelName, observe.LLMDirectionCompletion, int64(resp.Usage.CompletionTokens))
//	}
//
// Recording cache hit / miss inside a cache wrapper:
//
//	if v, ok := cache.Get(key); ok {
//	    observe.IncCacheHit("doc_lookup")
//	    return v
//	}
//	observe.IncCacheMiss("doc_lookup")
//	v := loadFromDB(key)
//	cache.Set(key, v)
//	return v
//
// ---------------------------------------------------------------------------
// Exposing the /metrics scrape endpoint (T1.21 / AC-B10):
//
//	router.GET("/metrics", handler.NewMetricsHandler().Metrics)
//
// The handler is registered in internal/router/router.go WITHOUT auth —
// Prometheus scrapers cannot carry our HMAC / JWT / APIKey credentials.
// Network-level controls (firewall / service-mesh policy) keep the
// endpoint reachable only from the monitoring subnet.
