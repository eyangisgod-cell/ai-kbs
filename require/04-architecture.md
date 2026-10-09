# 04 · 总体架构

## 1. 架构总览

```
┌──────────────────────────────────────────────────────────────────────────┐
│                          AI Agent 客户端层                                │
│   Claude Code   Codex   Cursor   Gemini CLI   Other MCP Clients         │
└────────────────────────────┬─────────────────────────────────────────────┘
                             │ MCP (stdio / SSE / HTTP / OAuth2)
                             ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                       ai-kbs MCP Server (Python)                         │
│   Tools: ai_kbs_search | ai_kbs_read_doc | ai_kbs_ask | ai_kbs_write_doc │
│          ai_kbs_edit_doc | ai_kbs_version_* | ai_kbs_sag_query | ...     │
│   Multi-transport: stdio | SSE | HTTP | OAuth2                           │
└────────────────────────────┬─────────────────────────────────────────────┘
                             │ HTTP/REST + WebSocket (NDJSON streaming)
                             ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                    ai-kbs Backend (Go) - forked from WeKnora             │
│ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────────────┐ │
│ │  Handler    │ │  ReAct      │ │  RAG        │ │  Doc/Version/Binding │ │
│ │  (HTTP/WS)  │ │  Agent      │ │  Pipeline   │ │  Service             │ │
│ └──────┬──────┘ └──────┬──────┘ └──────┬──────┘ └──────────┬──────────┘ │
│        └─────────────────┴──────────────┴─────────────────┘              │
│ ┌─────────────────────────────────────────────────────────────────────┐ │
│ │              ai-kbs Container (DI) + Middleware                      │ │
│ │  Auth (HMAC/JWT) | RBAC | Audit | Trace (Langfuse) | Rate Limit    │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
└──────────┬──────────────────┬──────────────────┬───────────────────────┘
           │                  │                  │
           ▼                  ▼                  ▼
┌──────────────────┐ ┌──────────────────┐ ┌──────────────────────┐
│   PostgreSQL     │ │  Elasticsearch   │ │   SAG API (Python)   │
│  (主存+pgvector) │ │  (检索副本)      │ │   FastAPI + LanceDB  │
│                  │ │  BM25+Dense+KG   │ │   Event/Entity store │
└──────────────────┘ └──────────────────┘ └──────────────────────┘
           │                  │                  │
           └──────────────────┴──────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                   Storage & Async Infrastructure                         │
│  Object Storage (S3/MinIO/OSS)  │  Redis (cache/session)  │  RabbitMQ  │
└──────────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────────────┐
│                        AI-KBS CLI (Go + Cobra)                           │
│   init | bind | pull | push | read | write | edit | search | version    │
│   ask | login | logout | mcp | migrate | skills                         │
│   Wire Contract: --json envelope v1.0 | exit codes | next_action        │
└──────────────────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                          Human / CI / Scripts                             │
└──────────────────────────────────────────────────────────────────────────┘
```

## 2. 模块划分

### 2.1 后端模块（Go，基于 WeKnora internal/ 改造）

```
ai-kbs-backend/
├── cmd/
│   └── server/
│       └── main.go              # 入口
├── internal/
│   ├── handler/                 # HTTP/WS handlers
│   │   ├── document.go          # /api/ai-kbs/documents/*
│   │   ├── version.go           # /api/ai-kbs/documents/*/versions
│   │   ├── binding.go           # /api/ai-kbs/bindings/*
│   │   ├── sync.go              # /api/ai-kbs/sync/*
│   │   ├── search.go            # /api/ai-kbs/search
│   │   ├── ask.go               # /api/ai-kbs/ask (NDJSON)
│   │   ├── space.go             # /api/ai-kbs/spaces/*
│   │   └── admin.go             # /api/ai-kbs/admin/*
│   ├── service/                 # 业务逻辑
│   │   ├── document_service.go
│   │   ├── version_service.go
│   │   ├── binding_service.go
│   │   ├── sync_service.go
│   │   ├── search_service.go
│   │   ├── ask_service.go
│   │   └── audit_service.go
│   ├── repository/              # 数据访问（接口 + PG 实现 + ES 实现 + SAG 实现）
│   │   ├── document_repo.go
│   │   ├── version_repo.go
│   │   ├── binding_repo.go
│   │   ├── search_backend.go    # 接口
│   │   ├── search_pg.go         # PostgreSQL 全文 + pgvector
│   │   ├── search_es.go         # Elasticsearch
│   │   ├── sag_client.go        # SAG HTTP 客户端
│   │   └── rag_router.go        # 检索路由（双引擎）
│   ├── agent/                   # ReAct Agent（沿用 WeKnora）
│   │   ├── engine.go
│   │   ├── tools/
│   │   │   ├── search.go
│   │   │   ├── read_doc.go
│   │   │   ├── sag_query.go     # 新增
│   │   │   ├── web_search.go
│   │   │   └── final_answer.go
│   │   ├── prompts/
│   │   └── memory/
│   ├── auth/                    # 鉴权
│   │   ├── hmac.go              # HMAC-SHA256
│   │   ├── jwt.go               # JWT 验签
│   │   ├── api_key.go           # scoped API Key
│   │   └── middleware.go        # Gin middleware
│   ├── rbac/                    # 角色权限
│   │   ├── matrix.go            # 4 级角色
│   │   └── guard.go
│   ├── doc/                     # 文档模型
│   │   ├── doc_code.go          # 8 位 base62 生成
│   │   ├── cas.go               # CAS 乐观锁
│   │   └── version.go           # 版本链
│   ├── sync/                    # 双向同步引擎
│   │   ├── workspace.go         # 工作空间配置
│   │   ├── binding.go           # 绑定管理
│   │   ├── sync_state.go        # sync-state.json
│   │   ├── pull.go              # 8 路并发读
│   │   ├── push.go              # 8 路并发写
│   │   ├── rename_detect.go     # docCode rename 检测
│   │   └── conflict.go          # 冲突解决
│   ├── config/                  # 配置加载
│   ├── container/               # DI 容器
│   ├── middleware/              # trace / cors / ratelimit / audit
│   ├── errors/                  # 退出码 + 业务码
│   ├── logger/
│   ├── observe/                 # Langfuse OTLP
│   └── mcp/                     # 嵌入式 MCP server（轻量）
│       └── server.go
├── migrations/                  # SQL 迁移（沿用 WeKnora + 扩展）
├── config/
│   ├── config.example.yaml
│   └── prompts/
└── go.mod
```

### 2.2 MCP Server 模块（Python，独立进程）

```
ai-kbs-mcp-server/
├── pyproject.toml
├── ai_kbs_mcp/
│   ├── __init__.py
│   ├── main.py                  # FastMCP 入口
│   ├── tools/
│   │   ├── __init__.py
│   │   ├── search.py
│   │   ├── read_doc.py
│   │   ├── write_doc.py
│   │   ├── edit_doc.py
│   │   ├── version.py
│   │   ├── ask.py
│   │   ├── sag_query.py
│   │   └── list_spaces.py
│   ├── transports/
│   │   ├── stdio.py
│   │   ├── sse.py
│   │   └── http.py
│   ├── oauth.py                 # OAuth2 客户端
│   └── client/
│       └── ai_kbs_client.py     # 调用 ai-kbs Backend
└── tests/
```

### 2.3 Docreader 模块（沿用 WeKnora）

```
ai-kbs-docreader/                 # 沿用 WeKnora docreader 代码
├── parser/
│   ├── pdf.py
│   ├── word.py
│   ├── markdown.py
│   ├── html.py
│   ├── image_ocr.py
│   ├── excel.py
│   ├── ppt.py
│   └── ...
├── splitter/
│   ├── semantic.py
│   ├── parent_child.py
│   └── ...
├── proto/
│   └── docreader.proto
├── server.py                    # gRPC server
└── client.py
```

### 2.4 SAG 包装层（Python，桥接 ai-kbs）

```
ai-kbs-sag-bridge/
├── pyproject.toml
├── sag_bridge/
│   ├── main.py                  # FastAPI 入口
│   ├── indexer.py               # chunk → event + entity
│   ├── retriever.py             # query → events
│   ├── client.py                # ai-kbs 后端调用
│   └── store/
│       ├── events.py
│       ├── entities.py
│       └── links.py
└── tests/
```

### 2.5 CLI 模块（Go）

```
ai-kbs-cli/
├── cmd/
│   └── ai-kbs/
│       └── main.go
├── internal/
│   ├── cli/
│   │   ├── root.go              # cobra root + global flags
│   │   ├── init.go
│   │   ├── bind.go
│   │   ├── pull.go
│   │   ├── push.go
│   │   ├── read.go
│   │   ├── write.go
│   │   ├── edit.go
│   │   ├── search.go
│   │   ├── version.go
│   │   ├── move.go
│   │   ├── undelete.go
│   │   ├── resolve.go
│   │   ├── ask.go               # 调用 ai-kbs ask
│   │   ├── login.go
│   │   ├── logout.go
│   │   ├── mcp.go               # mcp install / start
│   │   └── migrate.go           # from-losu-wiki
│   ├── api/                     # HTTP 客户端
│   │   ├── client.go            # 含重试 + HMAC 签名
│   │   └── envelope.go          # --json 解析
│   ├── auth/
│   │   ├── credential.go        # 凭证优先级链
│   │   ├── hmac.go
│   │   └── store.go             # ~/.ai-kbs/credentials
│   ├── resolve/
│   │   ├── target.go            # 4 flag 寻址
│   │   └── api_pathclient.go    # path → docCode
│   ├── workspace/
│   │   ├── conf.go              # .ai-kbs/workspace.yaml
│   │   ├── binding.go
│   │   └── sync_state.go
│   ├── errors/
│   ├── output/
│   │   ├── machine.go           # --json envelope
│   │   └── text.go
│   └── config/
├── skills/                      # SKILL 操作手册
│   ├── SKILL.md
│   └── references/
│       ├── auth.md
│       ├── workspace.md
│       ├── sync.md
│       ├── conflict.md
│       ├── version.md
│       └── mcp.md
└── go.mod
```

### 2.6 前端模块（沿用 WeKnora 风格）

```
ai-kbs-frontend/
├── src/
│   ├── views/
│   │   ├── workspace/
│   │   ├── document/
│   │   ├── ask/                 # 问答界面
│   │   ├── graph/               # KG 可视化
│   │   ├── admin/
│   │   └── settings/
│   ├── components/
│   ├── stores/                  # Pinia
│   ├── api/                     # REST + WS 客户端
│   ├── i18n/
│   └── router/
├── tests/
│   └── e2e/                     # Playwright
├── package.json
└── vite.config.ts
```

## 3. 数据流

### 3.1 文档写入流

```
User/Agent
   │  write_doc(content)
   ▼
MCP Server ──HTTP──▶ ai-kbs Backend ──┬──▶ PostgreSQL (主存: documents, versions)
                                     ├──▶ Object Storage (二进制)
                                     └──▶ MQ (异步索引任务)
                                              │
                                              ▼
                                     ┌────────────────────┐
                                     │  Index Worker      │
                                     └────────┬───────────┘
                                              │
                          ┌───────────────────┼───────────────────┐
                          ▼                   ▼                   ▼
                   ES Index            Docreader gRPC       SAG Indexer
                   (BM25 + Dense       (PDF/Word/MD→       (chunk→event+
                    + KG 节点)         chunks)             entity)
                          │                   │                   │
                          └───────────────────┴───────────────────┘
                                              │
                                              ▼
                                    Langfuse (trace 汇聚)
```

### 3.2 问答查询流

```
User/Agent
   │  ask(question)
   ▼
MCP Server ──HTTP──▶ ai-kbs Backend
                          │
                          ▼
                  Query Classifier
                  (LLM-call + rules)
                          │
            ┌─────────────┼─────────────┐
            ▼             ▼             ▼
       ES Path      Hybrid Path    SAG Path
       (单跳)       (混合重排)     (多跳推理)
            │             │             │
            └─────────────┼─────────────┘
                          ▼
                  Result Fusion
                  (RRF or Cross-encoder)
                          ▼
                  LLM Generation
                  (with citations)
                          ▼
                  NDJSON Stream
                  (thinking | tool_call | answer | complete)
```

### 3.3 双向同步流（CLI pull/push）

```
CLI init → bind → pull
   │
   ▼
Workspace Manager 读取 .ai-kbs/workspace.yaml
   │
   ▼
API Client GET /api/ai-kbs/spaces/{code}/tree (8 路并发)
   │
   ▼
Diff Engine 对比 sync-state.json vs 远端 tree
   │
   ▼
Sync Executor (8 路并发)
   ├── Pull: GET /docs/read → 写本地 → 更新 sync-state
   └── Push: 读本地 → md5 diff → POST /docs/write (CAS) → 更新 sync-state
   │
   ▼
Error Aggregator → sync_results[] + exit code
```

## 4. 关键技术决策

### 4.1 后端语言选择：Go（沿用 WeKnora）

**理由**：
- WeKnora 已是 Go 生态，迁移成本最低
- 并发模型适合 IO 密集（HTTP、DB、SAG）
- 单二进制分发适合 CLI
- 与 SAG（Python）通过 HTTP/gRPC 集成无摩擦

### 4.2 检索后端：ES + SAG 双引擎

**理由**：
- ES 适合大规模文档的 BM25 + 向量混合检索（成熟、可观测）
- SAG 适合多跳问答与结构化推理（SOTA 准确率）
- 双引擎并存避免相互妥协，按 query 类型路由最优

**路由策略**：
```python
# pseudocode
def route_query(query, query_class):
    if query_class == "single_hop":
        return es.search(query)
    elif query_class == "multi_hop" or has_entity_mentions(query):
        return sag.search(query)
    else:
        # 并行召回 + 重排融合
        es_results = es.search(query, top_k=20)
        sag_results = sag.search(query, top_k=20)
        return cross_encoder.rerank(merge(es_results, sag_results), top_k=5)
```

### 4.3 文档同步：CAS + docCode（沿用 losu-wiki）

**理由**：
- CAS（baseVersion）防止并发覆盖
- docCode（8 位 base62 稳定标识）让 rename/移动不破坏引用
- git revert 语义的 rollback 保留所有历史

### 4.4 AI Agent 入口：MCP 优先

**理由**：
- MCP 已是事实标准（Claude Code / Codex / Cursor / Gemini CLI）
- 工具描述自动生成，无需手写 SDK
- 多传输（stdio/SSE/HTTP/OAuth2）覆盖所有部署形态

### 4.5 部署：Docker Compose（开发）+ Helm（生产）

**理由**：
- 开发友好（一键起全套依赖）
- 生产可观测（HPA + 健康检查 + 灰度）
- 与 WeKnora 体系一致

## 5. 接口契约

### 5.1 REST API（简化版）

> 仅 GET/POST，与 losu-wiki 风格一致。

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/ai-kbs/spaces` | 列出可用空间 |
| GET | `/api/ai-kbs/spaces/{code}/tree` | 目录树 |
| GET | `/api/ai-kbs/documents/{docCode}` | 读文档 |
| GET | `/api/ai-kbs/documents/{docCode}/versions` | 版本列表 |
| GET | `/api/ai-kbs/documents/{docCode}/diff?from=&to=` | 版本 diff |
| POST | `/api/ai-kbs/documents/search` | 搜索 |
| POST | `/api/ai-kbs/documents` | 创建/更新文档 |
| POST | `/api/ai-kbs/documents/{docCode}/edit` | 局部编辑 |
| POST | `/api/ai-kbs/documents/{docCode}/versions` | 创建版本（CAS） |
| POST | `/api/ai-kbs/documents/{docCode}/rollback` | 回滚 |
| POST | `/api/ai-kbs/documents/{docCode}/move` | 移动 |
| POST | `/api/ai-kbs/documents/{docCode}/undelete` | 恢复 |
| POST | `/api/ai-kbs/bindings` | 创建绑定 |
| DELETE | `/api/ai-kbs/bindings/{id}` | 删除绑定 |
| POST | `/api/ai-kbs/sync/pull` | 触发 pull（管理） |
| POST | `/api/ai-kbs/sync/push` | 触发 push（管理） |
| POST | `/api/ai-kbs/ask` | 问答（NDJSON 流） |
| POST | `/api/ai-kbs/auth/login` | 登录 |
| POST | `/api/ai-kbs/auth/logout` | 登出 |
| GET | `/api/ai-kbs/auth/status` | 当前登录态 |

### 5.2 MCP Tools

详见 [07-mcp-tools.md](./07-mcp-tools.md)。

### 5.3 CLI Wire Contract

详见 [06-cli-spec.md](./06-cli-spec.md)。
