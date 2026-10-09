# M1 后端核心 + CLI 基础 + Frontend 脚手架 — 技术设计

> 配套需求: `prd.md` §3 验收标准
> 配套执行: `implement.md` 24 子项实施顺序
> 父设计: `../10-02-ai-kbs-mvp/design.md` (顶层架构,本文不重复)

本文档只覆盖 M1 特有的模块边界、模块间契约、和本阶段的关键技术决策。跨阶段架构 (三层检索 / Agent runtime / MCP) 见父 design.md。

---

## 1. 模块拓扑

```
┌─────────────────────────────────────────────────────────────────────┐
│                          CLI  (T1.10–T1.20)                          │
│  ┌─────────┐  ┌─────────────┐  ┌────────────┐  ┌───────────────┐   │
│  │  cmd/*  │→│ internal/cli │→│ internal/   │→│ internal/      │   │
│  │  (cobra)│  │ (per subcmd) │  │  sync      │  │ workspace     │   │
│  └─────────┘  └─────────────┘  └────────────┘  └───────────────┘   │
└──────────────┬──────────────────────────────────────────────────────┘
               │  HTTP (envelope v1.0)         │  本地文件
               ↓                              ↓
┌──────────────────────────┐    ┌──────────────────────────┐
│   Backend  (T1.1–T1.9, T1.21)   │  Create   │   local workspace   │
│  ┌─────────────────────────────────────────────────────┐           │
│  │           Handler  (Gin)              middleware/   │           │
│  │           ↑                            metrics.go    │           │
│  │  ┌────────┴──────────────────────────────────┐      │           │
│  │  │   internal/auth  +  rbac  +  errors       │      │           │
│  │  └────────┬──────────────────────────────────┘      │           │
│  │           ↓                                          │           │
│  │  ┌──────────────────────────────────────────────┐   │           │
│  │  │   internal/repository/*  ←  pgx v5           │   │           │
│  │  └────────┬─────────────────────────────────────┘   │   HTTPS /  │
│  │           ↓                                          │   W3C ctx  │
│  │  ┌──────────────────────────────────────────────┐   │           │
│  │  │   internal/database  (RunMigrationsWithOpts)  │   │           │
│  │  └────────┬─────────────────────────────────────┘   │           │
│  └───────────┼──────────────────────────────────────────┘           │
└──────────────┼──────────────────────────────────────────────────────┘
               ↓
┌──────────────────────────────────────────────────┐
│   PostgreSQL + Redis + Elasticsearch + RabbitMQ  │   (M0 cloud 部署)
└──────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                  Frontend  (T1.22–T1.24)                             │
│  ┌────────────────┐  ┌─────────────────┐  ┌──────────────────┐    │
│  │  src/api/*      │→│ src/stores/*     │→│ src/router/*     │    │
│  │  (axios + JWT)  │  │ (Pinia auth)    │  │ (Vue Router 4)   │    │
│  └────────────────┘  └─────────────────┘  └──────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 2. 模块边界 (一句话约束)

| 模块 | 责任 | 不允许 |
|------|------|--------|
| `internal/repository/*` | 纯 DB 访问;接收 ctx;返回 domain model + 错误类型 | 业务规则;HTTP 感知;事务管理 (除 repo 内的 atomic update) |
| `internal/auth/*` | 凭证解析 + 中间件注入 user 信息 | 业务逻辑;DB 直写 (除 api_key 表) |
| `internal/rbac/*` | `(role, resource, action) → bool`;middleware 拒绝 | 业务逻辑 |
| `internal/errors/*` | 命名空间常量 + envelope 写入 | 业务错误语义 |
| `internal/doc/{cas,doc_code}` | docCode 生成 + CAS 比对 | 任何 I/O (除生成器内的 DB 唯一性校验) |
| `internal/observe/*` | OTLP exporter + Prometheus metrics | 业务 span 字段 (由业务侧语义命名) |
| `internal/cli/*` | Cobra 命令绑定 + flag 解析 | 直连 DB / 直调 LLM (走 envelope) |
| `internal/sync/*` | pull/push 编排 + sync_state 维护 | 任何用户可见输出 (走 internal/output) |
| `internal/output/*` | envelope 解析 + text/json/ndjson 渲染 | 凭证 / 错误出处 |
| `src/api/*` | axios + JWT 注入 + endpoint 调用 | 业务逻辑 |
| `src/stores/*` | Pinia 状态 + 持久化 | HTTP 调用 (只调 src/api/*) |
| `src/router/*` | 路由表 + 守卫 | 业务组件 |

---

## 3. 模块间契约 (M1 关键)

### 3.1 Envelope v1.0 (frozen) — T1.7 ↔ T1.12

```json
{
  "success": true,
  "exit_code": 0,
  "data": <payload | null>,
  "error": null | { "code": "namespace.subname", "message": "..." },
  "request_id": "01J...",
  "trace_id": "<w3c-trace-id-32hex>"   // optional, only on OTLP path
}
```

- `success=false` 时 `exit_code` 必填;`data=null`;`error.code` 必填
- `success=true` 时 `error=null`;`exit_code=0`;`data` 必填
- frozen 字段集合禁止新增 (v1.0);扩展走 `meta` 子对象 (新)

### 3.2 CAS 接口 — T1.4

```go
// internal/doc/cas.go
func UpdateWithCAS(ctx context.Context, docCode string, baseVersion int64, newContent []byte) (newVersion int64, err error)
// err 类型: ErrVersionConflict{Current int64} | ErrNotFound | 其他
```

调用方：`internal/repository.Document.UpdateWithCAS` 是唯一入口;handler 必须把 `baseVersion` 从 `If-Match` header 透传。

### 3.3 鉴权中间件约定 — T1.5

请求头 → 鉴权方式 (按优先级):

| 头 | 方式 | 用途 |
|----|------|------|
| `Authorization: HMAC <keyId>,<sig>,<ts>,<nonce>` | HMAC 签名 | 服务间调用 |
| `Authorization: Bearer <jwt>` | JWT | 浏览器 SPA |
| `Authorization: APIKey <keyId>.<keySecret>` | API Key | CLI / 第三方 |

middleware 注入 `c.Set("user", *User)` + `c.Set("api_key_capabilities", []string{...})`。rbac guard 读 user.role。

### 3.4 CLI Envelope 输出 — T1.12

```go
// internal/output/machine.go
func Parse(r io.Reader) (*envelope.Envelope, error)

// internal/output/text.go
func RenderHuman(env *envelope.Envelope) (string, error)
```

CLI 错误时 exit code = `envelope.Exit_code`(默认 1)。machine 模式 (-o json) 严格输出 envelope 原始 JSON,text 模式调用 RenderHuman。

### 3.5 sync-state.json v1.0 schema — T1.18

```json
{
  "schema_version": "1.0",
  "bindings": {
    "<binding-name>": {
      "<relative-path>": {
        "md5": "<hex>",
        "version": <int>,
        "doc_code": "<8char-base62>",
        "remote_path": "<abs>",
        "last_sync_at": "<rfc3339>"
      }
    }
  }
}
```

读写用文件锁 + atomic write (.tmp + rename);JSON Schema 校验文件存在 `archCheck/sync-state.schema.json`。

---

## 4. 关键技术决策

### 4.1 pgx v5 (vs database/sql) — 影响 T1.2+

**决策:** 全 backend 新代码统一 pgx v5。

**理由:**
- 原生 pgx / ctx 传播好
- 支持 array / jsonb / COPY 协议
- 类型化 record (与 pgxquery 模板生成器对比 database/sql 字符串拼接更安全)

**WeKnora fork 兼容性:** 已有 database/sql 调用保持不动,只在新增代码用 pgx;逐步迁移到 M2/M3。

### 4.2 CAS 重试链 — T1.4 ↔ T1.20

CLI push 在 `ErrVersionConflict` 时:
- 默认 `--strategy=manual`:打印 next_action diff,exit 22
- `--strategy=keep-mine`:本地覆盖远端 (新 base_version)
- `--strategy=keep-theirs`:远端覆盖本地
- `--strategy=both`:本地+远端都保留 (新文件名)
- `--strategy=auto`:按文件大小/修改时间自动

CLI 不做自动重试超过 3 次 — 避免隐藏冲突。

### 4.3 错误码命名空间 — T1.7

```
auth.*        rbac.*       doc.*        binding.*
├ login.fail     (401)        ├ permission_denied  (403)  ├ not_found       (404)
├ token.expired  (401)        ├ ...                  ├ version_conflict (409)
└ ...                          └ ...                          └ ...

sync.*        search.*     sag.*        internal.*
├ partial_fail (207)        ├ ...                            └ ...
└ ...                        └ ...
```

每个 namespace 子名是 frozen 1 段。`data.frame` 在 envelope 中是 `data` (顶层),不在 error.code 里。

### 4.4 HMAC 时间窗 — T1.5

默认 ±300s;CLI / 服务间可覆盖 `HMAC_WINDOW` env。nonce 防重放通过 Redis `SETNX nonce:<keyId>:<nonce> 1 EX 600`。

### 4.5 Langfuse OTLP 双通道 — T1.8 + T1.21

- 业务 span → OTLP → Langfuse (T1.8)
- 基础设施指标 → Prometheus → Grafana (T1.21)
- 不重叠:OTLP 走 trace_id;Prometheus 负责 aggregate metrics
- 都从 `trace_id` 与外键关联

### 4.6 Frontend 状态持久化 — T1.24

- token / user: `localStorage` (持久跨会话)
- workspace.yaml: `~/.ai-kbs/workspace.yaml` (本地文件)
- Pinia store 不做 SSR;纯 CSR

---

## 5. 与父 design.md 的对齐

父 design.md §2 列出了 AI-KBS 顶层架构 (ES / 三层 / SAG / MCP)。M1 不涉及这些,只搭骨架。M2 才会接 Saga / 检索 / MCP 业务逻辑。

M1 阶段交付的模块将作为 M2 的接入点:
- `internal/repository/*` ← M2 的 ES / LanceDB 检索层会替换 Document repo
- `internal/auth/*` ← M2 的 MCP server 会复用 API Key 中间件
- `internal/observe/*` ← M2 的 LLM 调用会自动接入 OTLP

---

## 6. 待决策 (T1.x 启动时确认)

| 项 | 当前默认 | 决策依据 |
|----|----------|----------|
| WeKnora fork 中 HMAC 代码是否复用? | 默认复用并扩展 | T1.5 启动前 grep `internal/auth/hmac*`;若仅缺 RS256 则补,若架构差异大则重写 |
| 旧 `internal/database/migration.go` 与 T1.1 的关系? | T1.1 是 additive (新建表不破坏 WeKnora 原表) | T1.1 启动前读 `backend/migrations/versioned/000000_init.*` 决策 |
| Frontend 是否引入 Vitest 单测框架? | T1.22 引入 (M2 测试需要) | 与 M2 的 frontend 单测一并设计 |
| sync-state.json 是否支持 JSON5 (注释)? | 否,严格 JSON | T1.18 启动前决定 |