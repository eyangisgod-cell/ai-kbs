# AI-KBS v1 MVP — 技术设计(design.md)

> 项目代号:**AI-KBS**(AI Knowledge Base System)
> 文档版本:v1.0 — 2026-10-02
> 范围:v1.0.0 GA(Backend + CLI + MCP + **Web UI** + Helm)
> 配套 PRD:`prd.md`(同目录)

---

## 1. 设计目标与边界

### 1.1 设计目标

| 目标 | 关键约束 |
|------|----------|
| **模块边界清晰** | 每个模块可独立部署、独立测试、独立升级 |
| **AI Agent 优先** | MCP server 是 AI Agent 的主要入口,CLI 是次要入口,Web UI 是用户可视化层,REST 是兜底 |
| **检索 SOTA** | ES + SAG 双引擎路由,按 query 类型分发;不接受单引擎妥协 |
| **运维可观测** | 所有外部调用携带 trace_id + span_id;Langfuse dashboard 默认开启 |
| **跨平台 CLI** | 单二进制静态编译,无运行时依赖 |
| **Web UI 友好** | 基于 WeKnora Vue 3 SPA fork,白标化改造,浏览器用户可访问文档 / 提问 / 管理空间 |
| **脱敏红线** | 公开仓库代码 / 文档 / 配置 / 命令名 **零内部平台专有字样** |

### 1.2 设计边界(模块清单)

```
┌──────────────────────────────────────────────────────────────────────────┐
│                         AI Agent 客户端层                                │
│   Claude Code   Codex   Cursor   Gemini CLI   Other MCP Clients         │
└────────────────────────────┬─────────────────────────────────────────────┘
                             │ MCP (stdio / SSE / HTTP / OAuth2)
                             ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                   ai-kbs-mcp-server (Python, FastMCP)                    │
│   Tools: ai_kbs_search | ai_kbs_read_doc | ai_kbs_ask | ai_kbs_*         │
│   Transports: stdio | SSE:8081 | HTTP:8081 | OAuth2                      │
└────────────────────────────┬─────────────────────────────────────────────┘
                             │ HTTPS / REST + NDJSON streaming
                             ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                    ai-kbs-backend (Go, forked from WeKnora)               │
│ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────────────┐ │
│ │  HTTP/WS    │ │  ReAct      │ │  RAG        │ │  Doc / Version /    │ │
│ │  Handler    │ │  Agent      │ │  Pipeline   │ │  Binding Service    │ │
│ └──────┬──────┘ └──────┬──────┘ └──────┬──────┘ └──────────┬──────────┘ │
│        └─────────────────┴──────────────┴─────────────────┘              │
│ ┌─────────────────────────────────────────────────────────────────────┐ │
│ │              ai-kbs Container (DI) + Middleware                      │ │
│ │  Auth (HMAC/JWT/APIKey) | RBAC | Audit | Trace (Langfuse) | Limiter │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
└──────────┬──────────────────┬──────────────────┬───────────────────────┘
           │                  │                  │
           ▼                  ▼                  ▼
┌──────────────────┐ ┌──────────────────┐ ┌──────────────────────┐
│   PostgreSQL     │ │  Elasticsearch   │ │ ai-kbs-sag-bridge    │
│  + pgvector      │ │ 8.x (IK 分词)    │ │ (Python, FastAPI)    │
│                  │ │                  │ │ + LanceDB / SQLite   │
└──────────────────┘ └──────────────────┘ └──────────────────────┘
           │                  │                  │
           └──────────────────┴──────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                Storage & Async Infrastructure                            │
│  Object Storage (S3/MinIO/OSS/TOS/KS3/OBS) │ Redis │ RabbitMQ             │
└──────────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────────────┐
│              ai-kbs-docreader (Python, gRPC, from WeKnora)               │
│  pdf / word / markdown / html / txt / ... (12+ format)                    │
└──────────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────────────┐
│                       ai-kbs CLI (Go + Cobra)                            │
│  ~20 subcommands · single binary · --json envelope · SKILL.md           │
└──────────────────────────────────────────────────────────────────────────┘
                             │
                             ▼
                Human / CI / Scripts

┌──────────────────────────────────────────────────────────────────────────┐
│              ai-kbs-frontend (Vue 3 SPA, WeKnora fork)                  │
│  fork WeKnora v0.7.1 → 白标化(logo / 品牌色 / 产品名 → AI-KBS)         │
│  移除公司专有 SSO / 监控 SDK / 内部 CDN / 配置 / package.json 私有源   │
│  API 客户端 baseURL /api → /api/ai-kbs                                  │
│  views: workspace / document / ask / spaces / login                     │
└──────────────────────────────────────────────────────────────────────────┘
                             │
                             ▼
                Browser Users(查看 CLI push 的文档 / 提问走 SAG)
```

### 1.3 模块边界契约

| 模块 | 进程边界 | 接口 | 复用关系 |
|------|----------|------|----------|
| **ai-kbs-backend** | Go 进程(可水平扩展) | REST/WS(端口 8080)+ 内嵌精简 MCP(可选) | 内部消费 docreader / sag-bridge |
| **ai-kbs-cli** | 单二进制(随用户分发) | HTTPS REST(同 backend 端口) | 调用 backend |
| **ai-kbs-mcp-server** | Python 独立进程(可水平扩展) | MCP stdio/SSE/HTTP | 调用 backend REST |
| **ai-kbs-sag-bridge** | Python 独立进程(可水平扩展) | FastAPI(端口 8090) | backend 调用 |
| **ai-kbs-docreader** | Python gRPC 进程(可水平扩展) | gRPC(:50051) | backend 调用 |
| **ai-kbs-frontend** | Vue 3 SPA(浏览器加载) | HTTPS REST(同源代理) | 浏览器 → nginx 静态 + backend REST |

---

## 2. 模块设计

### 2.1 ai-kbs-backend(Go,基于 WeKnora fork)

#### 2.1.1 目录结构

```
backend/
├── cmd/
│   └── server/
│       └── main.go                  # 入口
├── internal/
│   ├── handler/                     # HTTP/WS handlers
│   │   ├── auth.go                  # /api/ai-kbs/auth/*
│   │   ├── spaces.go                # /api/ai-kbs/spaces/*
│   │   ├── documents.go             # /api/ai-kbs/documents/*
│   │   ├── versions.go              # /api/ai-kbs/documents/*/versions
│   │   ├── bindings.go              # /api/ai-kbs/bindings/*
│   │   ├── sync.go                  # /api/ai-kbs/sync/*
│   │   ├── search.go                # /api/ai-kbs/search
│   │   ├── ask.go                   # /api/ai-kbs/ask (NDJSON)
│   │   ├── sag_query.go             # /api/ai-kbs/sag/query
│   │   └── admin.go                 # /api/ai-kbs/admin/*
│   ├── service/                     # 业务逻辑
│   │   ├── auth_service.go
│   │   ├── space_service.go
│   │   ├── document_service.go
│   │   ├── version_service.go
│   │   ├── binding_service.go
│   │   ├── sync_service.go
│   │   ├── search_service.go
│   │   ├── ask_service.go
│   │   ├── sag_service.go
│   │   └── audit_service.go
│   ├── repository/                  # 数据访问
│   │   ├── user_repo.go
│   │   ├── workspace_repo.go
│   │   ├── space_repo.go
│   │   ├── document_repo.go
│   │   ├── version_repo.go
│   │   ├── binding_repo.go
│   │   ├── chunk_repo.go
│   │   ├── sag_event_repo.go
│   │   ├── sag_entity_repo.go
│   │   ├── api_key_repo.go
│   │   ├── audit_repo.go
│   │   ├── search_backend.go        # 接口
│   │   ├── search_pg.go             # PG 全文 + pgvector
│   │   ├── search_es.go             # ES 实现
│   │   ├── sag_client.go            # SAG HTTP 客户端
│   │   └── rag_router.go            # 双引擎路由
│   ├── agent/                       # ReAct Agent(沿用 WeKnora + 替换工具集)
│   │   ├── engine.go
│   │   ├── tools/
│   │   │   ├── search.go
│   │   │   ├── read_doc.go
│   │   │   ├── sag_query.go
│   │   │   ├── web_search.go
│   │   │   └── final_answer.go
│   │   ├── prompts/
│   │   └── memory/
│   ├── auth/                        # 鉴权
│   │   ├── hmac.go                  # HMAC-SHA256
│   │   ├── jwt.go                   # JWT(HS256/RS256)
│   │   ├── api_key.go               # Scoped API Key
│   │   ├── password.go              # bcrypt
│   │   └── middleware.go            # Gin middleware
│   ├── rbac/                        # 角色权限
│   │   ├── matrix.go                # 4 级
│   │   └── guard.go                 # 中间件
│   ├── doc/                         # 文档模型
│   │   ├── doc_code.go              # 8 位 base62 生成
│   │   ├── cas.go                   # CAS 乐观锁
│   │   └── version.go               # 版本链
│   ├── parse/                       # 文档解析客户端
│   │   └── docreader_client.go      # gRPC client
│   ├── embedding/                   # Embedding 抽象
│   │   ├── embedder.go              # 接口
│   │   ├── bge.go
│   │   ├── openai.go
│   │   └── ollama.go
│   ├── llm/                         # LLM 抽象
│   │   ├── provider.go              # 接口
│   │   ├── openai.go
│   │   ├── deepseek.go
│   │   ├── anthropic.go
│   │   ├── ollama.go
│   │   └── qwen.go
│   ├── config/
│   ├── container/                   # DI 容器
│   ├── middleware/                  # trace / cors / ratelimit
│   ├── errors/                      # 业务错误码
│   ├── logger/
│   └── observe/                     # Langfuse OTLP
├── migrations/                      # SQL 迁移
├── config/
│   ├── config.example.yaml
│   └── prompts/
└── go.mod
```

#### 2.1.2 WeKnora fork 适配边界

| WeKnora 子模块 | 适配动作 | 风险 |
|----------------|----------|------|
| `internal/auth` | **完全替换** 为 ai-kbs 自研(HMAC + JWT + API Key) | 高 — 涉及 middleware 全链路 |
| `internal/agent` | **替换工具集**(ai_kbs_search / read_doc / sag_query / web_search / final_answer);引擎骨架沿用 | 中 — 工具接口定义 |
| `internal/handler` | **路径前缀全部改为 `/api/ai-kbs/...`** | 低 — 命名替换 |
| `internal/storage` | **可插拔对象存储**(Local/MinIO/S3/OSS/TOS/KS3/OBS),不复用 WeKnora 默认 MinIO 配置 | 中 — adapter 抽象 |
| `internal/rag` | **替换为双引擎路由**(RAG Router → ES / SAG) | 高 — 核心能力 |
| `internal/llm` | 沿用 provider 抽象,**新增 DeepSeek / Qwen / Ollama adapter** | 低 |
| `internal/embedding` | 沿用 provider 抽象,**默认 BGE-large-zh-v1.5** | 低 |
| `internal/observability` | 沿用 Langfuse OTLP,**新增 CLI/MCP 调用 span** | 低 |
| `internal/parser` | **完全沿用 WeKnora docreader 集成**(gRPC) | 低 |
| `web/` | **整体删除**(v1 不交付 Web UI) | 极低 |
| `migrations/` | 沿用 + 新增 ai-kbs 专有表(`workspaces / spaces / documents / versions / sag_* / audit_logs`) | 低 |
| `cmd/server/main.go` | 替换入口,使用 ai-kbs 自研 container | 低 |

#### 2.1.3 DI 容器启动序列

```text
main()
  ├─ config.Load()
  ├─ logger.Init()
  ├─ observe.Init(langfuse)
  ├─ db.ConnectPostgres()
  ├─ db.ConnectRedis()
  ├─ es.Connect()
  ├─ rabbitmq.Connect()
  ├─ docreader.Connect()
  ├─ sag_bridge.Connect()
  ├─ container.New()         // 注入所有 repo / service / handler
  ├─ auth.InitMiddleware()
  ├─ rbac.InitMiddleware()
  ├─ router.Build()
  └─ http.ListenAndServe(8080)
```

### 2.2 ai-kbs-cli(Go + Cobra)

#### 2.2.1 目录结构

```
cli/
├── cmd/
│   └── ai-kbs/
│       └── main.go                  # cobra root + global flags
├── internal/
│   ├── cli/
│   │   ├── root.go
│   │   ├── login.go
│   │   ├── logout.go
│   │   ├── init.go
│   │   ├── status.go
│   │   ├── bind.go
│   │   ├── unbind.go
│   │   ├── pull.go
│   │   ├── push.go
│   │   ├── read.go
│   │   ├── write.go
│   │   ├── edit.go
│   │   ├── search.go
│   │   ├── grep.go
│   │   ├── tree.go
│   │   ├── spaces.go
│   │   ├── version.go
│   │   ├── move.go
│   │   ├── undelete.go
│   │   ├── resolve.go
│   │   ├── ask.go
│   │   └── mcp.go                   # mcp install / serve
│   ├── api/
│   │   ├── client.go                # 含重试 + HMAC 签名
│   │   └── envelope.go              # --json 解析
│   ├── auth/
│   │   ├── credential.go            # 凭证优先级链
│   │   ├── hmac.go
│   │   └── store.go                 # ~/.ai-kbs/credentials
│   ├── resolve/
│   │   ├── target.go                # 4 flag 寻址
│   │   └── path_client.go           # path → docCode
│   ├── workspace/
│   │   ├── conf.go                  # .ai-kbs/workspace.yaml
│   │   ├── binding.go
│   │   └── sync_state.go
│   ├── sync/
│   │   ├── diff.go
│   │   ├── pull.go                  # 8 路并发
│   │   ├── push.go                  # 8 路并发
│   │   ├── rename_detect.go
│   │   └── conflict.go
│   ├── errors/
│   ├── output/
│   │   ├── machine.go               # --json envelope
│   │   └── text.go
│   └── config/
├── skills/                          # SKILL 操作手册(随二进制分发)
│   ├── SKILL.md
│   └── references/
│       ├── auth.md
│       ├── workspace.md
│       ├── sync.md
│       ├── conflict.md
│       ├── version.md
│       └── mcp.md
├── go.mod
└── README.md
```

#### 2.2.2 命令列表(20 个,非镜像内部平台)

| 分类 | 命令 | 用途 |
|------|------|------|
| 认证 | `login` / `logout` | 凭证管理 |
| 工作空间 | `init` / `status` / `bind` / `unbind` | 本地工作空间生命周期 |
| 同步 | `pull` / `push` / `resolve` | 双向同步与冲突解决 |
| 单文档 | `read` / `write` / `edit` / `move` / `undelete` | 单文档操作 |
| 版本 | `version list/diff/rollback` | 版本管理 |
| 浏览 | `search` / `grep` / `tree` / `spaces` | 检索与浏览 |
| 问答 | `ask` | 自然语言提问(NDJSON 流) |
| MCP | `mcp install / serve` | AI Agent 接入 |

> 命令动词虽与 Git/通用惯例(init/bind/pull/push/read/write/edit)有重合,但子命令分组、flag 集合、envelope schema 均为本项目独立设计(ADR-007)。

#### 2.2.3 寻址 4 flag 优先级

```text
1. --url <pageURL>          # 自包含页面 URL(最高)
2. --space <s> --doc <c>    # docCode(推荐,rename 不变)
3. --space <s> --path <p>   # 路径解析(回退)

互斥规则:
  --url 与 (--space / --doc / --path) 同时出现 → exit 2
  --doc 与 --path 同时出现 → exit 2
  --space 不带 --doc/--path → exit 2
```

### 2.3 ai-kbs-mcp-server(Python + FastMCP)

#### 2.3.1 进程边界

- **独立 Python 进程**(与 backend 解耦)
- 多传输:`stdio` / `SSE(:8081)` / `HTTP(:8081)`
- 远程部署支持 OAuth2(scope:`kbs:read` / `kbs:write` / `kbs:version` / `kbs:admin`)

#### 2.3.2 工具分层

| 层 | 工具 |
|----|------|
| **Core** | `ai_kbs_search` / `ai_kbs_read_doc` / `ai_kbs_ask` / `ai_kbs_list_spaces` / `ai_kbs_get_doc_outline` |
| **Write**(需 write capability) | `ai_kbs_create_doc` / `ai_kbs_update_doc` / `ai_kbs_edit_doc` / `ai_kbs_move_doc` / `ai_kbs_delete_doc` |
| **Version**(需 version capability) | `ai_kbs_list_versions` / `ai_kbs_diff_versions` / `ai_kbs_rollback_version` |
| **SAG** | `ai_kbs_sag_query` / `ai_kbs_sag_explain` |
| **Admin**(v1.1) | `ai_kbs_create_space` / `ai_kbs_manage_binding` / `ai_kbs_get_audit_log` / `ai_kbs_manage_api_keys` |

> 所有工具输入/输出遵循 MCP 标准 + frozen schema v1.0,字段命名 `snake_case`(MCP 协议惯例)。

#### 2.3.3 目录结构

```
mcp-server/
├── pyproject.toml
├── ai_kbs_mcp/
│   ├── __init__.py
│   ├── main.py                      # FastMCP 入口
│   ├── tools/
│   │   ├── search.py
│   │   ├── read_doc.py
│   │   ├── write_doc.py
│   │   ├── edit_doc.py
│   │   ├── move_doc.py
│   │   ├── delete_doc.py
│   │   ├── version.py
│   │   ├── ask.py
│   │   ├── sag_query.py
│   │   ├── sag_explain.py
│   │   ├── list_spaces.py
│   │   ├── get_doc_outline.py
│   │   └── admin.py
│   ├── transports/
│   │   ├── stdio.py
│   │   ├── sse.py
│   │   └── http.py
│   ├── oauth.py                     # OAuth2 client + PKCE
│   ├── client/
│   │   └── ai_kbs_client.py         # 调用 backend REST
│   ├── auth/
│   │   └── token_store.py           # ~/.ai-kbs/mcp-tokens.json
│   └── errors.py
└── tests/
```

### 2.4 ai-kbs-sag-bridge(Python + FastAPI)

#### 2.4.1 SAG 接入方式

- SAG 论文实现已开源(Python),通过 `ai-kbs-sag-bridge` 包装
- bridge 暴露 HTTP API,backend 内部通过 HTTP 调用
- LanceDB 作为默认本地副本存储;SQLite 作为轻量备选
- **事件溯源**:每个 chunk 写入 PG 后,异步任务触发 bridge 的 `/events/append` 接口,bridge 完成 event/entity 抽取并写入 LanceDB + PG `sag_*` 表

#### 2.4.2 数据流

```
backend chunk insert → PG chunks 表
                       ↓ (异步 MQ)
              bridge.events.append(event)
                       ↓
              bridge.events.embed()       # SAG 内置 embedding
                       ↓
              LanceDB.events.add()
              LanceDB.entities.add()
              PG sag_events 表 sync
              PG sag_entities 表 sync
                       ↓
              PG sag_event_entity_links 表 sync
```

#### 2.4.3 检索时动态超边生成

```text
query → bridge.search(question)
  → LanceDB events similarity search (top-k)
  → LanceDB entities join via event-entity links
  → dynamic hyperedge construction (shared entities)
  → top-K events + linked entities
  → return chunk_ids → backend 回查 PG 拿原文
```

#### 2.4.4 目录结构

```
sag-bridge/
├── pyproject.toml
├── sag_bridge/
│   ├── main.py                      # FastAPI 入口
│   ├── indexer.py                   # chunk → event + entity
│   ├── retriever.py                 # query → events
│   ├── explainer.py                 # 推理路径解释
│   ├── client.py                    # ai-kbs backend 调用
│   ├── store/
│   │   ├── events.py
│   │   ├── entities.py
│   │   └── links.py
│   └── schema.py
└── tests/
```

### 2.5 ai-kbs-docreader(Python gRPC,from WeKnora)

#### 2.5.1 接口

```protobuf
service DocReader {
  rpc Parse( ParseRequest) returns (ParseResponse);
  rpc Chunk(  ChunkRequest) returns (ChunkResponse);
  rpc Health(Empty)                returns (HealthResponse);
}

message ParseRequest {
  string storage_key = 1;
  string doc_type    = 2;             // pdf / word / ...
  string mime_type   = 3;
  ChunkConfig config = 4;
}

message ParseResponse {
  repeated Chunk chunks = 1;
  repeated Image images = 2;
  ParseMetadata metadata = 3;
}
```

#### 2.5.2 v1 支持格式

| 格式 | 优先级 | 解析器 |
|------|--------|--------|
| Markdown | P0 | markdown parser |
| PDF | P0 | pdf parser(pdfplumber / pymupdf) |
| Word(docx) | P0 | python-docx |
| HTML | P0 | beautifulsoup4 + readability |
| Txt | P0 | 直接读取 |

#### 2.5.3 异步解析任务与 DLQ(覆盖 FR-PARSE-03)

```text
RabbitMQ queue: aikbs.doc.parse
  ├─ worker pool(N=4, 可配)
  │    ├─ 调 docreader.Parse(storage_key, doc_type)
  │    ├─ 成功 → 写 PG chunks + 投递 aikbs.doc.index(下一阶段)
  │    └─ 失败 → retry ×3(指数退避)
  └─ 最终失败 → DLQ: aikbs.doc.parse.dlq
       └─ admin UI / admin API 手动重试 + 跳过
```

重试次数、DLQ TTL、worker pool 大小均通过 `config.example.yaml` 暴露。

### 2.6 Auth 模块

#### 2.6.1 HMAC-SHA256(CLI / MCP / 自动化)

```text
签名字符串 = HTTP-Verb + "\n" + Request-Path + "\n" + Timestamp + "\n" + Nonce + "\n" + sha256(Body)

请求头:
  X-AIKBS-KeyId:      ak_xxx
  X-AIKBS-Timestamp:  1704067200           # unix seconds, ±300s 窗口
  X-AIKBS-Nonce:      <uuid>                # 24h 防重放
  X-AIKBS-Signature:  hex(hmac-sha256(secret, sign_string))
```

#### 2.6.2 JWT(Web / API)

```text
header.payload.signature
- alg: HS256(默认) / RS256
- iss: aikbs
- aud: aikbs-api
- exp: 3600s(可配)
- sub: user_id
- roles: [{workspace_id, role}]

后端验签:middleware/auth.go
```

#### 2.6.3 Scoped API Key(嵌入 / 自动化)

```text
key_id: ak_xxx (公钥,出现在请求头)
key_secret: sk_xxx (私钥,仅本地签名使用,不传输)
capabilities: ["search", "read_doc", "ask", ...]
space_ids: [uuid, ...]            # 限定到特定 space
expires_at: 2026-12-31T00:00:00Z
```

存储:
- key_secret_hash = bcrypt(key_secret)
- 客户端持有明文;服务端只存哈希

#### 2.6.4 安全附属能力(覆盖 FR-AUTH-09/10/11)

| 能力 | 实施位置 |
|------|----------|
| FR-AUTH-09 AES-256-GCM 静态加密 | `internal/auth/crypto.go`:API Key / 对象存储凭据入 DB 前 GCM 加密;key 由 K8s Secret 注入 |
| FR-AUTH-10 SSRF 防护 | `internal/net/ssrf_safe_client.go`:URL 导入与数据源同步专用 client;拒绝 RFC1918 / loopback / link-local 地址 |
| FR-AUTH-11 IDOR 防护 | `internal/rbac/guard.go`:每次写操作强制 `(resource.workspace_id, user.role)` 校验;失败返回 `11 PermissionDenied` |

### 2.7 RBAC(4 级)

| 角色 | 权限 |
|------|------|
| **Owner** | 工作空间 + 所有 Space 全权限;可转让所有权 |
| **Admin** | 工作空间内全权限(除转让所有权);管理成员 |
| **Contributor** | 文档读写 + 版本管理;不可管理成员 |
| **Viewer** | 只读 + 检索 + 问答 |

矩阵在 `internal/rbac/matrix.go` 单点定义,middleware 统一拦截,不依赖业务代码自觉。

### 2.8 ai-kbs-frontend(Vue 3 SPA,WeKnora fork)

> **依据 PRD §3.3 ADR-009**(WeKnora Vue 3 SPA fork + 白标化改造,1 名熟练 Vue 工程师,~7 人天)。详见 §11 ADR 表。

#### 2.8.1 目录结构

```
frontend/
├── public/                         # logo / favicon / OG 分享图(AI-KBS 资产)
│   ├── favicon.ico
│   ├── logo.svg
│   └── og-image.png
├── src/
│   ├── views/
│   │   ├── Login.vue               # OAuth2 / 邮箱密码登录
│   │   ├── Workspace.vue           # 空间列表 + 角色 + doc_count
│   │   ├── Document.vue            # 文档内容 + outline + 版本历史
│   │   ├── Ask.vue                 # 问答(NDJSON 流式渲染 + citations)
│   │   └── Spaces.vue              # 冒烟测试:列出可用 space
│   ├── components/                 # AppHeader / AppFooter / MarkdownRender / CitationChip
│   ├── stores/                     # Pinia: auth / workspace / ask
│   ├── api/                        # REST + WS 客户端(baseURL: /api/ai-kbs)
│   │   ├── client.ts               # axios 实例 + JWT 注入
│   │   ├── auth.ts
│   │   ├── document.ts
│   │   ├── search.ts
│   │   └── ask.ts                  # NDJSON 流式处理
│   ├── i18n/                       # 中英双语包(已清洗)
│   ├── router/                     # Vue Router 4
│   ├── assets/
│   │   └── styles/                 # CSS 变量(AI-KBS 品牌色)
│   ├── App.vue
│   └── main.ts
├── tests/
│   ├── unit/                       # Vitest(覆盖率 ≥ 70%)
│   └── e2e/                        # Playwright(登录 / 浏览 / 提问 / 引用跳转)
├── .env.example                    # 公司专有 flag 已清除
├── package.json                    # 公司私有 npm 源已移除
├── vite.config.ts                  # 公司专有插件已替换
└── README.md
```

#### 2.8.2 白标化改造清单(覆盖 FR-WEB-01..07)

| 改造项 | 文件 | 工作量 |
|--------|------|--------|
| **品牌 logo / favicon / OG** | `public/logo.svg` / `favicon.ico` / `og-image.png` / `index.html` | 0.5d |
| **CSS 品牌色** | `src/assets/styles/variables.scss` + 通用组件 prop | 0.5d |
| **i18n 字符串清洗** | `src/i18n/zh-CN.ts` / `en-US.ts` | 0.5d |
| **登录页 SSO 替换** | `src/views/Login.vue` + `src/views/SsoCallback.vue`(移除公司专有 SSO,改标准 OAuth2) | 1d |
| **API 客户端路径对齐** | `src/api/*.ts` 全部 `/api` → `/api/ai-kbs` | 1d |
| **第三方 SDK 替换** | 公司监控 SDK 移除 / Langfuse Web SDK 接入;公司 CDN 替换 | 1d |
| **配置清洗** | `.env.example` / `vite.config.ts` / `package.json` | 1d |
| **测试 + 视觉回归** | `tests/e2e/*.spec.ts` Playwright 截图比对 | 1d |
| **README / CHANGELOG** | `frontend/README.md` | 0.5d |
| **合计** | **~75 文件** | **~7d(1 名熟练 Vue 工程师)** |

#### 2.8.3 关键视图契约(覆盖 FR-WEB-08..13)

| 视图 | API 依赖 | 关键交互 |
|------|----------|----------|
| **Login** | `POST /api/ai-kbs/auth/login` + OAuth2 callback | JWT 存 localStorage;登录后跳转 `/workspace` |
| **Workspace** | `GET /api/ai-kbs/spaces` + `GET /api/ai-kbs/auth/status` | 显示所有可用 space + 当前用户角色 + doc_count |
| **Document** | `GET /api/ai-kbs/documents/{docCode}` + `GET /api/ai-kbs/documents/{docCode}/versions` | markdown 渲染 + outline + 版本下拉;citations 可点击跳转 |
| **Ask** | `POST /api/ai-kbs/ask`(NDJSON 流) | 流式渲染 `init / thinking / tool_call / answer / complete`;engine=sag 时高亮多跳路径;session_id 跨多轮 |
| **Spaces**(冒烟) | `GET /api/ai-kbs/spaces` | 列出所有 space + 角色 |

#### 2.8.4 安全与性能契约(覆盖 NFR-FE-01..06)

| 项 | 实施 |
|----|------|
| NFR-FE-01 首屏 < 3s | Vite 按需 code-splitting + CDN 缓存 + gzip/brotli |
| NFR-FE-02 文档浏览 P95 < 500ms | Document.vue 预取 outline + CDN 缓存静态资源 + 后端 GET 响应 < 100ms |
| NFR-FE-03 提问首 token < 1s | Ask.vue NDJSON 流式 + 骨架屏 + 后端 SSE streaming + LLM provider 选择低延迟模型 |
| NFR-FE-04 XSS 防护 | Vue 模板默认转义 + DOMPurify 清洗 markdown HTML |
| NFR-FE-05 浏览器兼容 | babel-preset-env targets Chrome 100+ / Firefox 100+ / Safari 15+ / Edge 100+ |
| NFR-FE-06 响应式 | CSS `@media (min-width: 768px)` + `@media (max-width: 375px)`;Pinia store 复用 |

---

## 3. 核心数据流

> **trace_id 全链路透传**:所有 §3.x 数据流中,每个跨进程 / 跨服务调用都通过 W3C `traceparent` header 传递 `trace_id` 与 `span_id`(详见 §7.1)。下图中每条箭头隐含该 trace 透传,所有外部调用上报到 Langfuse。

### 3.1 文档写入流

```
User / Agent
   │ write_doc(content)
   ▼
MCP Server ─HTTP─▶ ai-kbs-backend ─┬─▶ PostgreSQL(主存:documents / versions / chunks)
                                   ├─▶ Object Storage(二进制)
                                   └─▶ RabbitMQ(async index task)
                                              │
                                              ▼
                                   ┌──────────────────────┐
                                   │  Index Worker        │
                                   └────────┬─────────────┘
                                            │
                ┌───────────────────────────┼───────────────────────────┐
                ▼                           ▼                           ▼
         ES Index                    Docreader gRPC                SAG Bridge
         (BM25 + Dense +             (PDF/Word/MD →              (chunk → event
          KG 节点)                   chunks)                     + entity)
                │                           │                           │
                └───────────────────────────┴───────────────────────────┘
                                            │
                                            ▼
                                  Langfuse(trace 汇聚)
```

**事务边界**:PG `documents` + `versions` + `chunks` 三表同事务提交;索引(ES / SAG / Langfuse span)异步,不阻塞响应。

### 3.2 问答查询流

```
User / Agent
   │ ask(question)
   ▼
MCP Server ─HTTP─▶ ai-kbs-backend
                       │
                       ▼
              Query Classifier(LLM-call + rules)
                       │
         ┌─────────────┼─────────────┐
         ▼             ▼             ▼
    ES-RAG Path    Hybrid Path    SAG Path
    (单跳 RAG)    (混合重排)     (多跳推理)
         │             │             │
         └─────────────┼─────────────┘
                       ▼
              Result Fusion
              (RRF or cross-encoder)
                       │
                       ▼
              LLM Generation(with citations)
                       │
                       ▼
              NDJSON Stream
              (init / thinking / tool_call / answer / complete)
```

**ES + RAG + SAG 三层并存说明**:
- **ES**(Elasticsearch):底层检索引擎,提供 BM25 + Dense 向量 + Hybrid 混合检索
- **RAG**(WeKnora RAG 框架):编排层,组合 ES + pgvector + LLM,**完整保留作单引擎降级路径**
- **SAG**(论文实现):多跳增强层,事件 - 实体 - 动态超边推理,SOTA 准确率

三层关系:**ES 是引擎**,**RAG 是使用 ES 的框架**,**SAG 是并行的多跳引擎**。三者并存而非替换。

**Query Classifier 规则(简化)**:
- 含 ≥2 个大写实体名 → multi_hop → **SAG Path**
- 含 "为什么 / 怎么 / 比较 / 关联" → multi_hop → **SAG Path**
- 关键词 + 单实体 → single_hop → **ES-RAG Path**(WeKnora 单引擎 RAG)
- 不确定 → parallel recall + rerank → **Hybrid Path**(ES-RAG + SAG 并行 + RRF 融合)
- **SAG 不可用** → 自动降级到 **ES-RAG Path**(不阻塞可用性)
- **ES 不可用** → 自动降级到 **PG 全文 + pgvector**(在 RAG 框架内)

### 3.3 双向同步流(CLI pull / push)

```
CLI init → bind → pull
   │
   ▼
Workspace Manager 读取 .ai-kbs/workspace.yaml
   │
   ▼
API Client GET /api/ai-kbs/spaces/{code}/tree(8 路并发)
   │
   ▼
Diff Engine 对比 sync-state.json vs 远端 tree
   │
   ▼
Sync Executor(8 路并发)
   ├── Pull:GET /documents/{docCode} → 写本地 → 更新 sync-state
   └── Push:读本地 → md5 diff → POST /documents(CAS) → 更新 sync-state
   │
   ▼
Error Aggregator → sync_results[] + exit code
```

**删除护栏**:`pull --prune` 时若超过 10 文件或 5MB 单文件 → 需 `--yes`;Detached 文件(sync-state 中无记录)不被删除。

### 3.4 MCP 工具调用流

```
Claude Code(.mcp.json 启动 stdio MCP server)
   │
   │ tool_call: ai_kbs_read_doc({doc_code, version})
   ▼
ai-kbs-mcp-server(本地进程)
   │
   │ 1. 加载 ~/.ai-kbs/mcp-tokens.json(OAuth2 token)
   │ 2. HMAC 签名(或 Bearer token)
   ▼
HTTPS → ai-kbs-backend
   │
   │ handler/documents.go → service → repo → PG
   ▼
返回 frozen schema v1.0 envelope → MCP server → Claude Code
```

---

## 4. 关键数据契约

### 4.1 CLI `--json` envelope(frozen schema v1.0)

成功:
```json
{
  "json_schema_version": "1.0",
  "success": true,
  "exit_code": 0,
  "data": { /* 业务 payload */ },
  "next_action": null,
  "request_id": "req-xxx"
}
```

错误:
```json
{
  "json_schema_version": "1.0",
  "success": false,
  "exit_code": 22,
  "error": { "code": "doc.version_conflict", "message": "文档版本冲突", "detail": { } },
  "data": null,
  "next_action": { "cmd": "ai-kbs read", "args": ["--space", "dev", "--doc", "aB3kX9q2"], "reason": "版本冲突,先 read 获取最新 version 后重试", "strategies": ["keep-mine", "keep-theirs", "both"] },
  "request_id": "req-xxx"
}
```

### 4.2 MCP 工具 frozen schema v1.0

每个工具的 input/output 由 JSON Schema 定义,随二进制 release artifact 发布。MCP server 启动时校验 schema 一致性,不匹配则启动失败。

**核心工具输入摘要**(完整定义随 release artifact 发布):

| 工具 | 必填输入 | 输出关键字段 |
|------|---------|--------------|
| `ai_kbs_search` | `query` | `engine / total / items[] / took_ms` |
| `ai_kbs_read_doc` | `doc_code` | `content / outline / summary / metadata / version / md5` |
| `ai_kbs_ask` | `question` | 流式 NDJSON: `init / thinking / tool_call / tool_result / answer / complete` |
| `ai_kbs_create_doc` | `space_code / title / path / content` | `doc_code / version / url` |
| `ai_kbs_update_doc` | `doc_code / content / base_version` | 同 create;冲突返回 `next_action.tool = ai_kbs_read_doc` |
| `ai_kbs_edit_doc` | `doc_code / old_text / new_text / base_version` | `replaced_count / new_version` |
| `ai_kbs_sag_query` | `question` | `events[] / entities[] / hyperedges_count / explanation?` |
| `ai_kbs_list_versions` | `doc_code` | `items[]:{version, status, md5, author, message, created_at}` |
| `ai_kbs_rollback_version` | `doc_code / target_version` | `new_version` |
| `ai_kbs_list_spaces` | — | `items[]:{code, name, role, doc_count}` |

**OAuth scopes**:`kbs:read`(核心读) / `kbs:write`(写层) / `kbs:version`(版本层) / `kbs:admin`(管理,v1.1)。

### 4.3 REST API 路径

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/ai-kbs/spaces` | 列出可用空间 |
| GET | `/api/ai-kbs/spaces/{code}/tree` | 目录树 |
| GET | `/api/ai-kbs/documents/{docCode}` | 读文档 |
| GET | `/api/ai-kbs/documents/{docCode}/versions` | 版本列表 |
| GET | `/api/ai-kbs/documents/{docCode}/diff?from=&to=` | 版本 diff |
| POST | `/api/ai-kbs/search` | 搜索 |
| POST | `/api/ai-kbs/documents` | 创建/更新 |
| POST | `/api/ai-kbs/documents/{docCode}/edit` | 局部编辑 |
| POST | `/api/ai-kbs/documents/{docCode}/versions` | 创建版本(CAS) |
| POST | `/api/ai-kbs/documents/{docCode}/rollback` | 回滚 |
| POST | `/api/ai-kbs/documents/{docCode}/move` | 移动 |
| POST | `/api/ai-kbs/documents/{docCode}/undelete` | 恢复 |
| POST | `/api/ai-kbs/bindings` | 创建绑定 |
| DELETE | `/api/ai-kbs/bindings/{id}` | 删除绑定 |
| POST | `/api/ai-kbs/sync/pull` | 触发 pull |
| POST | `/api/ai-kbs/sync/push` | 触发 push |
| POST | `/api/ai-kbs/ask` | 问答(NDJSON) |
| POST | `/api/ai-kbs/sag/query` | 直接 SAG 查询 |
| POST | `/api/ai-kbs/auth/login` | 登录 |
| POST | `/api/ai-kbs/auth/logout` | 登出 |
| GET | `/api/ai-kbs/auth/status` | 当前登录态 |

---

## 5. 数据模型(摘要,详见 `require/05-data-model.md`)

| 表 | 用途 |
|----|------|
| `workspaces` | 工作空间元数据 |
| `spaces` | 文档空间(KB 等价物) |
| `documents` | 文档元数据(含 docCode / tree_path / 软删) |
| `document_versions` | 版本链(md5 / content / CAS base_version) |
| `chunks` | 文本块 + pgvector embedding |
| `sag_events` | SAG 事件 |
| `sag_entities` | SAG 实体 |
| `sag_event_entity_links` | 事件-实体关联 |
| `users` / `workspace_users` | 用户与角色 |
| `api_keys` | Scoped API Key |
| `sessions` | Agent 对话会话 |
| `audit_logs` | 操作审计日志 |

> 所有表 schema 与索引策略沿用 require/05-data-model.md,本设计不重复定义。

---

## 6. 部署架构

### 6.1 开发环境(Docker Compose)

```yaml
# docker-compose.dev.yml
services:
  postgres:
    image: postgres:15
    ports: ["5432:5432"]
    environment:
      POSTGRES_DB: aikbs
    volumes: ["pgdata:/var/lib/postgresql/data"]
  
  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.11.0
    environment:
      discovery.type: single-node
      xpack.security.enabled: "false"
    ports: ["9200:9200"]
  
  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]
  
  rabbitmq:
    image: rabbitmq:3.12-management
    ports: ["5672:5672", "15672:15672"]
  
  langfuse:
    image: langfuse/langfuse:latest
    environment:
      DATABASE_URL: postgresql://langfuse:langfuse@postgres:5432/langfuse
    ports: ["3000:3000"]
  
  sag-bridge:
    build: ./sag-bridge
    ports: ["8090:8090"]
    volumes: ["./data/lancedb:/data/lancedb"]
  
  docreader:
    build: ./docreader
    ports: ["50051:50051"]
  
  backend:
    build: ./backend
    ports: ["8080:8080"]
    depends_on: [postgres, elasticsearch, redis, rabbitmq, sag-bridge, docreader, langfuse]
    environment:
      AIKBS_DB_URL: postgres://aikbs:aikbs@postgres:5432/aikbs?sslmode=disable
      AIKBS_ES_URL: http://elasticsearch:9200
      AIKBS_REDIS_URL: redis://redis:6379
      AIKBS_RABBIT_URL: amqp://guest:guest@rabbitmq:5672
      AIKBS_SAG_URL: http://sag-bridge:8090
      AIKBS_DOCREADER_ADDR: docreader:50051
      AIKBS_LANGFUSE_URL: http://langfuse:3000
  
  mcp-server:
    build: ./mcp-server
    ports: ["8081:8081"]
    depends_on: [backend]
```

启动:`docker compose -f docker-compose.dev.yml up -d`(全栈 ≤ 10 分钟)

### 6.2 生产环境(Helm Chart)

```
helm/
├── Chart.yaml
├── values.yaml
├── values.prod.yaml.example
└── templates/
    ├── backend-deployment.yaml          # Deployment + Service + HPA
    ├── backend-configmap.yaml
    ├── backend-secret.yaml             # DB / ES / API Key 密文
    ├── mcp-server-deployment.yaml      # Deployment + Service + Ingress
    ├── sag-bridge-deployment.yaml
    ├── docreader-deployment.yaml
    ├── frontend-deployment.yaml        # Vue 3 SPA 静态资源 Deployment + Service
    ├── frontend-configmap.yaml         # nginx 配置(SPA 路由 fallback)
    ├── postgres-statefulset.yaml        # StatefulSet + PVC
    ├── elasticsearch-statefulset.yaml
    ├── redis-statefulset.yaml
    ├── rabbitmq-statefulset.yaml
    ├── langfuse-deployment.yaml
    ├── ingress.yaml                    # /api/ai-kbs → backend, /mcp → mcp-server, /* → frontend
    ├── serviceaccount.yaml
    ├── podmonitor.yaml                 # Prometheus podMonitor
    └── networkpolicy.yaml
```

**关键部署决策**:
- backend / mcp-server / sag-bridge / frontend 全部可水平扩展
- PG / ES / Redis / RabbitMQ 全部 StatefulSet + PVC,生产建议接外部托管服务
- Ingress: `/api/ai-kbs/*` → backend,`/mcp/*` → mcp-server,`/*` → frontend(SPA 路由 fallback 到 index.html)
- 资源建议:backend `1-4` 副本,每副本 `2 CPU / 4Gi`;mcp-server `1-2` 副本,每副本 `1 CPU / 2Gi`;frontend `1-2` 副本,每副本 `0.5 CPU / 256Mi`(静态资源 + nginx)
- HPA:CPU > 70% 或 QPS > 100 触发扩容
- frontend 镜像采用多阶段构建:Node 构建 + nginx 静态托管;CDN 缓存 index.html 与 chunk-*.js 提升首屏加载(NFR-FE-01)

### 6.3 灰度与回滚

| 策略 | 实现 |
|------|------|
| 蓝绿发布 | K8s `Deployment` + `Service` selector 双版本共存,Ingress 切流量 |
| 数据库迁移 | `migrations/` 顺序编号,带 up / down;CI 校验,生产由 migration operator 执行 |
| Schema 兼容 | frozen schema v1.0 字段集合锁;新增字段需 `version` 字段标注 |
| 回滚触发 | Langfuse 错误率 > 5% / 延迟 P95 > 2x baseline → 自动告警 + 手动回滚 Helm release |

---

## 7. 可观测性设计

### 7.1 Trace 传播

```text
Client(MCP / CLI / Web)
  │ traceparent: 00-<trace-id>-<span-id>-01
  ▼
ai-kbs-backend(trace_id 接收 + 生成 root span)
  │
  ├─ HTTP span(handler/documents.go:Read)
  ├─ DB span(pgx.Query)
  ├─ ES span(go-elasticsearch.Search)
  ├─ SAG span(sag-bridge.Query)
  ├─ LLM span(openai.ChatCompletion)
  └─ All spans reported to Langfuse via OTLP
```

### 7.2 关键指标

| 指标 | 来源 | 类型 |
|------|------|------|
| `aikbs_http_requests_total{path,status}` | backend middleware | Counter |
| `aikbs_http_request_duration_seconds{path}` | backend middleware | Histogram |
| `aikbs_search_engine_duration_seconds{engine=es/sag/hybrid}` | search_service | Histogram |
| `aikbs_llm_token_total{provider,model,type=input/output}` | llm provider | Counter |
| `aikbs_index_queue_depth` | RabbitMQ 管理 API | Gauge |
| `aikbs_parse_duration_seconds{doc_type}` | parse_worker | Histogram |
| `aikbs_cache_hit_total{kind}` | redis client | Counter |

Prometheus scrape → Grafana dashboard;Langfuse dashboard 默认开启。

---

## 8. 重要权衡(Trade-offs)

### 8.1 已决策的权衡

| 决策 | 取舍 | 理由 |
|------|------|------|
| fork WeKnora 而非自研 | 失去部分定制灵活性,获取 80% 复用 | 1 人月不可重建完整 RAG + Agent + RBAC + MCP |
| SAG 与 ES 并存(非替换) | 运维复杂度 × 1.5 | 不接受检索精度妥协 |
| CLI 命令集合独立设计(不镜像内部平台) | 失去迁移便利性,获取命名清晰 | 用户硬约束 + 公开仓库安全 |
| MCP server 独立 Python 进程 | 多一个进程,获取工具描述自动生成 + FastMCP 生态 | Go MCP 生态尚不成熟 |
| **Web UI 基于 WeKnora Vue 3 SPA fork + 白标化**(ADR-009) | 增加 1 名前端工程师 + ~7 人天改造 + Week 8 → 10-11 GA | 用户决策 2026-10-02;给非 Agent 用户浏览器入口;fork 比自研省 80% 工作量 |
| CLI `--json` envelope 强制 JSON 输出 | 失去自然可读性,获取 Agent 解析稳定 | frozen schema 是 Agent 友好度的核心 |
| 鉴权选 HMAC + JWT 双轨 | 实现复杂度 × 1.5 | HMAC 适合无状态(CLI/MCP),JWT 适合 Web/API |
| 数据模型沿用 docCode + CAS | 失去 Git 生态,获取细粒度权限与审计 | 文档场景 ≠ 代码场景 |
| Postgres `pgvector` 与 ES 并存 | 写入双写,获取事务一致性 + 检索性能 | PG 主存事务强,ES 检索强 |

### 8.2 显式延后的权衡

| 决策 | 何时再决策 |
|------|-----------|
| 多 LLM 路由(成本优化) | v2 |
| SAG 分布式(>100k 文档) | v1.1 |
| 数据源(飞书 / Notion / 语雀)+ Chrome Extension + 数据迁移工具 | v1.1(Phase 1.1);Web UI 已在 v1 GA |
| Web KG 可视化(D3 / Cytoscape)+ Web 管理后台(队列 / 审计 / worker pool) | v1.1 |
| OAuth2 + 远程 MCP 完整支持 | v1 已交付基础 OAuth2,高级 scope 控制在 v1.1 |
| SAG 与 ES 自动冷热分层 | v1.1 |

### 8.3 前端专项风险

| 风险 | 等级 | 缓解 |
|------|------|------|
| WeKnora fork 上游 breaking change 引入新公司专有 SDK | 中 | T1.23 白标化阶段强制清理;PR 强制 review;`make scrub-audit` 在 CI 阻断 |
| 首屏加载超 NFR-FE-01 (3s) | 中 | Vite code-splitting + CDN 缓存 + gzip/brotli;Playwright 性能 baseline |
| markdown 渲染 XSS 漏洞(NFR-FE-04) | 高 | marked + DOMPurify 双重清洗;Playwright 注入 `<script>` 测试 |
| NDJSON 流式解析断流(浏览器关闭 / 网络中断) | 中 | Ask.vue 自动重连 + 客户端断点续传(session_id);T2.19 验收 |
| Vue 3 + Vite 5 + Pinia 版本升级 breaking change | 低 | 锁定 major 版本;每半年评估 1 次 |

---

## 9. 运维与回滚

### 9.1 回滚点

| 改动 | 回滚方式 |
|------|----------|
| Backend 部署 | Helm `helm rollback <release> <revision>`(1 命令) |
| Frontend 部署 | 同 Helm rollback;镜像 tag 锁定允许 1 命令回滚到上一版本 |
| Frontend 缓存层 | CDN 缓存失效 `purge /index.html /chunk-*.js`(CDN API);前端 JS 失败 → 提示用户强刷(Ctrl+Shift+R) |
| 数据库迁移 | `migrations/<seq>_*.sql.down.sql`;迁移 operator 自动校验 |
| frozen schema v1.0 字段变更 | 不可逆(冻结);新增字段必须 `version: "1.1"` 标注 |
| SAG 索引数据 | LanceDB 目录 `/data/lancedb` 保留 7 天,可在 7 天内重放 indexer 重建 |
| ES 索引 | `PUT /ai-kbs-chunks` mapping 不可逆;新增字段用 `dynamic: true` |
| Langfuse 配置 | 自托管实例,可重新部署 |

### 9.2 故障应急

| 故障 | 应急 |
|------|------|
| WeKnora fork 升级引入 breaking change | 立即 `git revert`;修复 PR 走 hotfix 分支 |
| SAG 服务不可用 | 路由自动降级 RAG-only;错误率告警 |
| ES 不可用 | 路由降级到 PG 全文 + pgvector |
| PostgreSQL 主从切换 | backend 自动重连;5s 内恢复 |
| LLM provider 不可用 | 多 provider 降级链(DeepSeek → OpenAI → Ollama) |
| MCP server 不可用 | Claude Code 自动重连;CLI 不受影响 |
| 关键 schema 字段被改 | CI 阻断(grep frozen schema fields)+ PR 强制 review |

### 9.3 安全响应

- `SECURITY.md` 报告漏洞渠道
- 关键漏洞:72 小时内修复 + 通知所有 v1.0.x 用户
- CVE 编号 + 受影响版本范围在 release notes 中明示

---

## 10. 兼容性与迁移

### 10.1 v1 内部兼容

- API frozen schema v1.0:字段集合不再变更;新增字段需 `version: "1.1"` 字段标注
- frozen schema 不兼容变更必须走 `v2` 版本,且提供 `migration guide`

### 10.2 与 [内部 Wiki 系统] 的兼容性

- v1 **不交付迁移工具**(见 PRD §2.2)
- 老用户切换需手动 `ai-kbs init` + 重新 `bind` + `pull`,docCode 不会自动保留(与 [内部 Wiki 系统] 内部 docCode 不同源)
- 计划 v1.1 提供 `ai-kbs migrate <legacy-workspace>` 工具,读取 [内部 Wiki 系统] 旧版 conf/sync-state 并重建工作空间(需用户决策是否启动该工具开发)

### 10.3 与 WeKnora 上游兼容

- pin WeKnora v0.7.1(在 `backend/go.mod` 用 `replace` 指向 fork)
- 上游 v0.8.x 发布后,逐 PR 合入;critical security fix 立即 backport
- 关键改造点单独建仓(如 `ai-kbs/internal/agent` 自研包装层)

---

## 11. 附录:决策追溯(ADR)

> 与 PRD §3.3 共享;此处仅列技术决策摘要。

| ADR | 决策 | 关键证据 |
|-----|------|----------|
| ADR-001 | fork WeKnora | 复用率 80%;自研 12 周不可达 |
| ADR-002 | SAG 双引擎 | 论文 SOTA + 增量更新友好 |
| ADR-003 | CLI 独立设计 | 用户硬约束 + 公开仓库安全 |
| ADR-004 | CAS + docCode | 文档 ≠ 代码,Git 模型过重 |
| ADR-005 | MCP 一等公民 | Claude Code 等 Agent 默认支持 |
| ADR-006 | v1 GA = Backend + CLI + MCP + Web UI + Helm,Week 10-11 末 GA | 用户决策 2026-10-02;browser 用户入口 |
| ADR-007 | CLI 命令集合独立设计 | 用户硬约束 + 公开仓库安全 |
| ADR-009 | Web UI 基于 WeKnora Vue 3 SPA fork + 白标化,~7 人天前端 | 用户决策 2026-10-02;1 名前端工程师可完成 |

---

**design.md 待与 prd.md 一同进入需求收敛闸门,然后进入 implement.md 编写。**
