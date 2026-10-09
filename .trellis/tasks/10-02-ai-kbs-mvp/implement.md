# AI-KBS v1 MVP — 实施计划(implement.md)

> 项目代号:**AI-KBS**(AI Knowledge Base System)
> 文档版本:v1.0 — 2026-10-02
> 范围:v1.0.0 GA(Backend + CLI + MCP + **Web UI** + Helm)
> 配套 PRD:`prd.md` / 设计:`design.md`(同目录)
> 验证闸门:本计划的所有任务执行前必须通过 §6 的"启动前检查"

---

## 1. 实施总览

### 1.1 任务树

```
M0 基础设施 (Week 1-2)
  ├─ T0.1 仓库初始化 + fork WeKnora
  ├─ T0.2 CI/CD 流水线(GitHub Actions)
  ├─ T0.3 Docker Compose 开发环境
  ├─ T0.4 Helm Chart 骨架
  ├─ T0.5 代码规范工具配置
  ├─ T0.6 文档站点骨架(VitePress)
  ├─ T0.7 Langfuse 部署 + 默认 dashboard
  ├─ T0.8 数据库迁移框架
  ├─ T0.9 根目录 Makefile + 统一命令入口
  └─ T0.10 fork WeKnora Vue 3 SPA → `frontend/`

M1 后端核心 + CLI 基础 + Frontend 脚手架 (Week 3-6)
  ├─ T1.1 数据库迁移:核心表
  ├─ T1.2 实体层:Repository pattern
  ├─ T1.3 docCode 生成器
  ├─ T1.4 CAS 乐观锁实现
  ├─ T1.5 鉴权:HMAC + JWT + API Key
  ├─ T1.6 RBAC 4 级矩阵
  ├─ T1.7 错误码体系 + envelope
  ├─ T1.8 Langfuse OTLP 集成
  ├─ T1.9 单元测试:core 包
  ├─ T1.10 CLI 骨架 + global flags + config
  ├─ T1.11 凭证管理(优先级链 + store)
  ├─ T1.12 CLI envelope 输出
  ├─ T1.13 CLI 命令:login/logout/init/status/bind/unbind
  ├─ T1.14 CLI 命令:read/write/edit
  ├─ T1.15 CLI 命令:version list/diff/rollback
  ├─ T1.16 CLI 命令:move/undelete/resolve
  ├─ T1.17 CLI SKILL.md + references/
  ├─ T1.18 sync-state.json schema v1.0 设计
  ├─ T1.19 CLI pull 命令实现
  ├─ T1.20 CLI push 命令实现
  ├─ T1.21 关键指标埋点(NFR-OBS-03)
  ├─ T1.22 frontend 项目脚手架(Vue 3 + Vite + Pinia)
  ├─ T1.23 frontend 白标化骨架(logo / 品牌色 / i18n)
  └─ T1.24 frontend 配置清洗 + API 客户端路径对齐

M2 检索 + Agent + MCP + Frontend 视图 (Week 7-8)
  ├─ T2.1 ES 索引设计 + mapping + IK 分词
  ├─ T2.2 docreader gRPC 集成
  ├─ T2.3 chunking 三种策略
  ├─ T2.4 embedding pipeline(异步 + 缓存)
  ├─ T2.5 search service(BM25 + vector + hybrid)
  ├─ T2.6 SAG bridge 集成:indexer + retriever
  ├─ T2.7 检索路由 query classifier
  ├─ T2.8 ReAct Agent 引擎 + 工具集
  ├─ T2.9 ask NDJSON 流接口
  ├─ T2.10 MCP server 骨架(FastMCP + transports)
  ├─ T2.11 MCP tools 实现(core + write + version + sag)
  ├─ T2.12 OAuth2 支持
  ├─ T2.13 ai-kbs mcp install 子命令
  ├─ T2.14 BDD 场景编写(48 个 Gherkin)
  ├─ T2.15 文档解析 worker pool + DLQ(FR-PARSE-03)
  ├─ T2.16 frontend Login 视图(标准 OAuth2 / 邮箱密码)
  ├─ T2.17 frontend Workspace + Spaces 视图
  ├─ T2.18 frontend Document 视图(markdown + outline + 版本)
  └─ T2.19 frontend Ask 视图(NDJSON 流式 + citations)

M3 v1.0.0 GA (Week 10-11)
  ├─ T3.1 BDD 场景覆盖(48 个场景)
  ├─ T3.2 Claude Code / Codex 集成测试
  ├─ T3.3 性能基线压测
  ├─ T3.4 整体集成测试 + 端到端冒烟
  ├─ T3.5 中英双语文档 + SKILL.md 定稿
  ├─ T3.6 Helm Chart K8s 测试集群部署验证(含 frontend)
  ├─ T3.7 release artifact 构建 + tag v1.0.0
  ├─ T3.8 脱敏审计脚本 + CI 阻断(关键)
  └─ T3.9 Playwright E2E + 视觉回归(FR-WEB-14/15)
```

### 1.2 依赖矩阵

```
T0.x  ─────┬─────▶ T1.x ─────┬─────▶ T2.x ─────┬─────▶ T3.x
(基础设施) │  (后端+CLI)    │  (检索+Agent+   │    (GA验收)
           │               │   MCP+Frontend) │
           ▼                ▼                  ▼
       T0.1 必须最先        T1.1 依赖 T0.1      T2.x 依赖 T1.5/1.6
       T0.9 依赖 T0.5      T1.10 依赖 T1.1     T2.10 依赖 T1.12
       T0.10 依赖 T0.1     T1.13-1.17 依赖 T1.10-1.12  T2.14 依赖 T1.13-1.20 + T2.10-2.13
                           T1.18 依赖 T1.13     T2.15 依赖 T2.2 + T0.3
                           T1.19-1.20 依赖 T1.18 T2.16-2.19 依赖 T1.22-1.24 + T2.10-2.13
                           T1.21 依赖 T0.7 + T1.8
                           T1.22-1.24 依赖 T0.10
```

### 1.3 团队配置(建议)

| 角色 | 人数 | 主要职责 |
|------|------|----------|
| 后端工程师 | 2 | backend(WeKnora fork)+ SAG bridge 集成 |
| CLI / MCP 工程师 | 1 | CLI + MCP server + SKILL.md |
| **前端工程师** | **1** | **Vue 3 SPA fork + 白标化(7 人天)+ 5 个核心视图** |
| DevOps | 0.5 | CI/CD + Docker Compose + Helm |
| QA | 0.5 | BDD + Playwright E2E + 性能压测 |

> 前端工程师在 Week 5 后段开始介入(M1 中段),等后端核心 API 稳定后再 fork 改造。
> 总计 62 个任务(10 M0 + 24 M1 + 19 M2 + 9 M3);详见 §9.3 资源评估。

---

## 2. Phase M0 — 基础设施(Week 1-2)

### T0.1 仓库初始化 + fork WeKnora

| 项 | 内容 |
|----|------|
| **输入** | 空白仓库 `github.com/eyangisgod-cell/ai-kbs`(已存在) |
| **动作** | (1) **复制 WeKnora 后端源码**:`cp -r e:/my-project/WeKnora-main/* ai-kbs/backend/`(WeKnora 主仓库根目录复制);(2) 改 `go.mod` module 路径为 `github.com/eyangisgod-cell/ai-kbs`;(3) 删除 WeKnora 公司 SKU 相关文件(认证 SDK / 监控 SDK / 内部配置);(4) 新建空目录:`cli/` `mcp-server/` `sag-bridge/` `helm/` `docs/`(占位,稍后填充);(5) 添加 `LICENSE`(Apache 2.0,与 WeKnora 一致)+ `README.md` + `CHANGELOG.md` + `.gitignore`(含 `.ai-kbs/` `*.bak` 等) |
| **复制源路径** | **`e:/my-project/WeKnora-main/`** → `ai-kbs/backend/`(直接复制 + 定制,**非 git submodule**) |
| **输出** | 仓库目录结构可编译;`.gitignore` 含 `.ai-kbs/` `*.bak` 等;`backend/go.mod` module 正确 |
| **依赖** | — |
| **验收** | `cd backend && go build ./...` 成功(编译 WeKnora fork);`cd backend && go mod tidy` 无报错;LICENSE 文件存在 |
| **风险** | WeKnora 内部 SKU 耦合未完全剥离 → T1.x 阶段持续剥离;复制源码后失去上游追踪 → 关键改造点单独记录 ADR |

### T0.2 CI/CD 流水线

| 项 | 内容 |
|----|------|
| **输入** | `.github/workflows/` 空目录 |
| **动作** | 配置 4 个 workflow:`lint.yml`(golangci-lint + ruff + eslint)、`test.yml`(unit + integration)、`build.yml`(交叉编译 6 平台)、`release.yml`(tag 触发 release) |
| **输出** | 任何 PR 触发 lint + test;`v*` tag 触发 6 平台二进制 release |
| **依赖** | T0.1 |
| **验收** | 故意破坏 lint → CI 红色;创建 `v0.1.0-test` tag → release artifact 上传 |
| **风险** | runner 时间过长 → 加 build cache |

### T0.3 Docker Compose 开发环境

| 项 | 内容 |
|----|------|
| **输入** | `docker-compose.dev.yml` 占位 |
| **动作** | 服务清单:`postgres / elasticsearch / redis / rabbitmq / sag-bridge / docreader / backend / mcp-server / langfuse`;含健康检查 + depends_on;IK 分词插件预装 |
| **输出** | `docker compose -f docker-compose.dev.yml up -d` 全栈启动,各服务健康检查通过 |
| **依赖** | T0.1 + 各自 Dockerfile |
| **验收** | `curl http://localhost:8080/healthz` 200;`curl http://localhost:9200` ES 信息可见;`curl http://localhost:8090/healthz` SAG bridge OK |
| **风险** | ES 启动慢 → healthcheck 给 60s 宽限 |

### T0.4 Helm Chart 骨架

| 项 | 内容 |
|----|------|
| **输入** | `helm/` 空目录 |
| **动作** | 模板:`backend / mcp-server / sag-bridge / docreader`(Deployment);`postgres / elasticsearch / redis / rabbitmq`(StatefulSet + PVC);`langfuse`;`ingress / serviceaccount / podmonitor / networkpolicy` |
| **输出** | `helm install ai-kbs ./helm --dry-run` 通过;`helm template` 输出可读 |
| **依赖** | T0.1 |
| **验收** | `helm template` 渲染无错误;`values.yaml` 暴露所有可调参数 |
| **风险** | 镜像 tag 占位需后续替换 |

### T0.5 代码规范工具配置

| 项 | 内容 |
|----|------|
| **输入** | 各模块根目录 |
| **动作** | `.golangci.yml`(Go)+ `pyproject.toml [tool.ruff]`(Python)+ `.eslintrc.cjs`(前端预留,v1 不启用) |
| **输出** | `make lint` 在每个模块根目录成功 |
| **依赖** | T0.1 |
| **验收** | `golangci-lint run` 零警告;`ruff check` 零警告 |
| **风险** | WeKnora fork 默认风格可能不通过 lint → 全部加 `//nolint` 不接受,改为修正代码 |

### T0.9 根目录 Makefile 与统一命令入口

| 项 | 内容 |
|----|------|
| **输入** | T0.5 规范工具就绪 |
| **动作** | 仓库根 `Makefile` 暴露顶层命令:`make lint / test / test-unit / test-integration / bdd / bdd-mvp / cli-build / perf / helm-install / helm-uninstall / scrub-audit / migrate-up / migrate-down / dev-up / dev-down / docs-build`;各模块子 Makefile 由 T0.5 配置的工具驱动;命令幂等 |
| **输出** | `make help` 输出所有 target;`make cli-build` 在 macOS arm64 + Linux amd64 同时产出 |
| **依赖** | T0.5 |
| **验收** | `make help` 列出全部 target,每个目标运行成功且退出码 0 |
| **风险** | 跨平台 shell 差异(尤其 Windows)→ 通过 `.PHONY` + 显式 shell 声明规避 |

### T0.6 文档站点骨架

| 项 | 内容 |
|----|------|
| **输入** | `docs/` 空目录 |
| **动作** | VitePress 初始化;目录:`getting-started / architecture / api / cli / mcp / adr` |
| **输出** | `npm run docs:dev` 本地预览可访问 |
| **依赖** | T0.1 |
| **验收** | 首页 + 各 section 占位页可渲染 |
| **风险** | — |

### T0.7 Langfuse 部署

| 项 | 内容 |
|----|------|
| **输入** | Docker Compose 服务占位 |
| **动作** | `langfuse/langfuse:latest` 容器;接 PG;预置 API key;**预置默认 dashboard**:QPS / P50/P95/P99 延迟 / 错误率 / LLM token 用量 / 缓存命中率 / 队列深度 7 个 panel(对应 NFR-OBS-02/03) |
| **输出** | `http://localhost:3000` 可访问;默认 dashboard 7 panel 全部可见数据流;API key 写入 K8s Secret 占位 |
| **依赖** | T0.3 |
| **验收** | Langfuse 创建 project 后,backend 能上报 OTLP trace 并出现在 dashboard;任意一次 backend 调用在 dashboard 产生 span + metric |

### T0.8 数据库迁移框架

| 项 | 内容 |
|----|------|
| **输入** | WeKnora 迁移工具 |
| **动作** | 沿用 WeKnora 迁移机制(命令行 `migrate up / down / status`);`backend/migrations/` 目录 |
| **输出** | `make migrate-up` 在 dev 环境创建空 schema(无业务表) |
| **依赖** | T0.3 |
| **验收** | `migrate status` 显示 0 个 migration 文件;后续添加 `001_init.up.sql` 能成功 up |

### T0.10 fork WeKnora Vue 3 SPA → `frontend/`

| 项 | 内容 |
|----|------|
| **输入** | WeKnora v0.7.1 frontend 代码(本地源码 `e:/my-project/WeKnora-main/web/` 或类似路径) |
| **复制源路径** | **`e:/my-project/WeKnora-main/web/`** → `ai-kbs/frontend/`(直接复制 + 定制,**非 git submodule**) |
| **动作** | (1) **复制 WeKnora 前端源码**:`cp -r e:/my-project/WeKnora-main/web/* ai-kbs/frontend/`;(2) `package.json` name 改为 `ai-kbs-frontend`;(3) `vite.config.ts` 配置 base path + proxy + build chunks;(4) `tsconfig.json` paths;(5) `.eslintrc.cjs` 继承 T0.5 规范;(6) `.env.example` 添加 `VITE_AIKBS_API_BASE` |
| **输出** | `frontend/` 目录可独立 `npm run dev` 启动;但仍指向原 WeKnora 后端(白标化前的临时连接) |
| **依赖** | T0.1(WeKnora 后端 fork)+ T0.5(规范工具)+ `e:/my-project/WeKnora-main/web/` 源码已复制 |
| **验收** | `cd frontend && npm install && npm run dev` 启动成功,浏览器可访问原 WeKnora 前端界面(白标化前) |
| **风险** | WeKnora 前端可能依赖公司专有 SDK → T1.23 阶段统一清洗 |

---

## 3. Phase M1 — 后端核心 + CLI 基础(Week 3-5)

### T1.1 数据库迁移:核心表

| 项 | 内容 |
|----|------|
| **输入** | `require/05-data-model.md` §2.1-2.10 |
| **动作** | 迁移文件 `001_init.up.sql`:`workspaces / spaces / documents / document_versions / chunks / sag_events / sag_entities / sag_event_entity_links / users / workspace_users / api_keys / sessions / audit_logs`;配套 `.down.sql`;索引与 GIN 全部建好 |
| **输出** | `make migrate-up` 创建完整 schema;表数 13;索引数 ~25 |
| **依赖** | T0.8 |
| **验收** | 13 张表全部存在;`\d+ documents` 可见 HNSW / GIN / partial 索引 |
| **风险** | HNSW 索引在 PG 15 上需 pgvector 0.5+ → docker-compose 镜像预装 |

### T1.2 实体层:Repository pattern

| 项 | 内容 |
|----|------|
| **输入** | T1.1 表结构 |
| **动作** | `internal/repository/`:每个表 `xxx_repo.go`(接口 + PG 实现);错误类型统一(`ErrNotFound / ErrAlreadyExists / ErrVersionConflict`);DI 注入 |
| **输出** | 13 个 repo 文件;每个 repo 单元测试 ≥ 80% |
| **依赖** | T1.1 |
| **验收** | `go test ./internal/repository/...` 全绿;mock 测试可绕过 DB |
| **风险** | pgx vs database/sql 选型 → 统一 pgx(v5) |

### T1.3 docCode 生成器

| 项 | 内容 |
|----|------|
| **输入** | — |
| **动作** | `internal/doc/doc_code.go`:8 位 base62 字符集 `0-9A-Za-z`;生成器 `NewDocCode()`;唯一性校验(查询 DB);碰撞重试 ≤3 次;Snowflake 备用方案(若 base62 碰撞率 > 0.01%) |
| **输出** | 单文件 + 单元测试(碰撞测试 + 唯一性测试 + 并发测试) |
| **依赖** | T1.2 |
| **验收** | 100 万次生成,0 碰撞;并发 100 协程生成 10 万个,0 重复 |
| **风险** | 碰撞率高 → 切换 Snowflake(11 char) |

### T1.4 CAS 乐观锁实现

| 项 | 内容 |
|----|------|
| **输入** | T1.2 documents repo |
| **动作** | `internal/doc/cas.go`:`UpdateWithCAS(ctx, docCode, baseVersion, newContent) error`;若 `current_version != base_version` → 返回 `ErrVersionConflict{Current: N}` |
| **输出** | 函数 + 单元测试(正常 / 冲突 / 并发竞争) |
| **依赖** | T1.2 |
| **验收** | 100 并发请求同一文档,1 成功 99 返回 conflict;后续重试链 OK |

### T1.5 鉴权:HMAC + JWT + API Key

| 项 | 内容 |
|----|------|
| **输入** | `design.md` §2.6 |
| **动作** | `internal/auth/hmac.go`(签名字符串 + 验签);`internal/auth/jwt.go`(HS256/RS256);`internal/auth/api_key.go`(bcrypt 哈希 + capability 校验);`internal/auth/middleware.go`(Gin middleware,按请求头自动选鉴权方式) |
| **输出** | 鉴权中间件 + handler `/api/ai-kbs/auth/login / logout / status` |
| **依赖** | T1.2(users repo) |
| **验收** | 单元测试覆盖 HMAC 签名 / 时间戳窗口 / nonce 防重放 / JWT 过期 / API Key capability 校验 |
| **风险** | HMAC 实现错位导致任意签名通过 → 单元测试覆盖正向 + 负向 100+ case |

### T1.6 RBAC 4 级矩阵

| 项 | 内容 |
|----|------|
| **输入** | `design.md` §2.7 |
| **动作** | `internal/rbac/matrix.go`:定义 `(role, resource, action) → bool`;`internal/rbac/guard.go`:Gin middleware 注入 user role,失败返回 `11 PermissionDenied` |
| **输出** | 矩阵文件 + guard middleware;每个 handler 在路由上声明所需角色 |
| **依赖** | T1.5(auth middleware 必须先注入 user 信息) |
| **验收** | 单元测试覆盖 4 角色 × 10 action 矩阵,40 个 case;中间件在 owner/admin/contributor/viewer 下分别通过/拒绝 |

### T1.7 错误码体系 + envelope

| 项 | 内容 |
|----|------|
| **输入** | `require/02-glossary.md` §5 + §6 |
| **动作** | `internal/errors/`:命名空间常量(`auth.* / rbac.* / doc.* / binding.* / sync.* / search.* / sag.* / internal.*`);统一 envelope `WriteJSON(c, status, envelope)`;frozen schema v1.0 |
| **输出** | 错误码常量 + envelope helper;handler 全部切换 |
| **依赖** | T1.2 |
| **验收** | 任意错误响应都包含 `{success:false, exit_code, error:{code,message}, data:null, request_id}` |

### T1.8 Langfuse OTLP 集成

| 项 | 内容 |
|----|------|
| **输入** | T0.7 Langfuse 已部署 |
| **动作** | `internal/observe/`:OTLP exporter → Langfuse;trace_id 从 HTTP header `traceparent` 读取或新建;W3C tracecontext 传播 |
| **输出** | 任何 handler 自动产生 root span;DB / ES / SAG / LLM 调用产生 child span |
| **依赖** | T0.7 |
| **验收** | 任意 HTTP 请求在 Langfuse dashboard 可见完整 span tree |

### T1.9 单元测试:core 包

| 项 | 内容 |
|----|------|
| **输入** | T1.2 - T1.8 完成 |
| **动作** | `go test ./internal/auth/... ./internal/rbac/... ./internal/doc/... ./internal/repository/...`;覆盖率目标 ≥ 85% |
| **输出** | 测试报告 |
| **依赖** | T1.2 - T1.8 |
| **验收** | coverage ≥ 85%;`make test` 全绿 |

### T1.10 CLI 骨架 + global flags + config

| 项 | 内容 |
|----|------|
| **输入** | `require/06-cli-spec.md` §1.1 - 1.3 |
| **动作** | `cli/cmd/ai-kbs/main.go` + `cli/internal/cli/root.go`:Cobra root + global flags;`cli/internal/config/`:加载 `~/.ai-kbs/config.yaml` + 环境变量 |
| **输出** | `ai-kbs --help` 输出所有 global flags + 子命令占位 |
| **依赖** | T0.1(Go module) |
| **验收** | `go build -o ai-kbs ./cmd/ai-kbs` 成功;`./ai-kbs --version` 输出 `1.0.0` |

### T1.11 凭证管理(优先级链 + store)

| 项 | 内容 |
|----|------|
| **输入** | `require/06-cli-spec.md` §1.4 |
| **动作** | `cli/internal/auth/credential.go`:5 级优先级链解析;`cli/internal/auth/store.go`:凭证文件 `~/.ai-kbs/credentials`(TOML);AES-256-GCM 加密 key_secret |
| **输出** | 函数 `ResolveCredential(ctx) (Credential, error)` + 单元测试 |
| **依赖** | T1.10 |
| **验收** | 5 级优先级测试(每个级别都正确解析);凭证文件可读写;key_secret 永不打印到 stdout |

### T1.12 CLI envelope 输出

| 项 | 内容 |
|----|------|
| **输入** | T1.7 backend envelope |
| **动作** | `cli/internal/output/machine.go`:解析 backend 响应到 envelope;`cli/internal/output/text.go`:人类可读渲染;`--format json|text|ndjson` 切换 |
| **输出** | envelope 解析器 + text renderer;错误时 exit code 透传 |
| **依赖** | T1.10 |
| **验收** | 单元测试覆盖成功 / 错误分支;`--format text` 输出可读;`--format json` 严格匹配 frozen schema |

### T1.13 CLI 命令:login/logout/init/status/bind/unbind

| 项 | 内容 |
|----|------|
| **输入** | `require/06-cli-spec.md` §3.1 - 3.2 |
| **动作** | `cli/internal/cli/login.go`(浏览器授权 + 手动粘贴两种模式);`logout.go`;`init.go`(生成 `.ai-kbs/workspace.yaml`);`status.go`;`bind.go` / `unbind.go` |
| **输出** | 5 个子命令;每个都有 `--help` 输出 |
| **依赖** | T1.11(凭证) + T1.12(envelope) + T1.10 |
| **验收** | `ai-kbs login --url ...` 在 dev 环境成功登录;`ai-kbs init` 生成 workspace.yaml;`ai-kbs bind docs dev:/api/` 写入绑定 |

### T1.14 CLI 命令:read/write/edit

| 项 | 内容 |
|----|------|
| **输入** | `require/06-cli-spec.md` §3.4 |
| **动作** | `cli/internal/cli/read.go`(4 flag 寻址 + content/outline/summary/meta/section);`write.go`(CAS + --create + --dir 批量);`edit.go`(局部替换 + --dry-run + --replace-all) |
| **输出** | 3 个子命令 |
| **依赖** | T1.13 + T1.4(CAS) |
| **验收** | 单文档 CRUD 端到端测试通过;CAS 冲突时返回 22 + next_action |

### T1.15 CLI 命令:version list/diff/rollback

| 项 | 内容 |
|----|------|
| **输入** | `require/06-cli-spec.md` §3.5 |
| **动作** | `cli/internal/cli/version.go`:三个子命令 list/diff/rollback;rollback 调用 backend `/documents/{docCode}/rollback` |
| **输出** | 1 个父命令 + 3 个子命令 |
| **依赖** | T1.13 + backend version endpoint(T1.2 repo 已有) |
| **验收** | 文档版本列表 / diff / 回滚端到端测试;rollback 产生新版本,中间版本保留 |

### T1.16 CLI 命令:move/undelete/resolve

| 项 | 内容 |
|----|------|
| **输入** | `require/06-cli-spec.md` §3.5 - 3.6 |
| **动作** | `move.go`(docCode 不变);`undelete.go`(不产生新版本);`resolve.go`(单文档 + 批量,4 策略) |
| **输出** | 3 个子命令 |
| **依赖** | T1.13 |
| **验收** | 移动 / 软删 / 恢复 / 冲突解决端到端测试 |

### T1.17 CLI SKILL.md + references/

| 项 | 内容 |
|----|------|
| **输入** | `require/06-cli-spec.md` §6 + 已交付命令集 |
| **动作** | `cli/skills/SKILL.md`(HARD RULES + 意图决策矩阵 + 典型工作流 A-H);`references/{auth,workspace,sync,conflict,version,mcp}.md` |
| **输出** | SKILL.md + 6 个 reference 子文档 |
| **依赖** | T1.13 - T1.16 |
| **验收** | SKILL.md 通过 Claude Code 实际读取 + 执行 A-H 工作流 **8/8 全部成功**(明确阈值:任一失败 ≥ 1 即视为未通过) |

### T1.18 sync-state.json schema v1.0 设计

| 项 | 内容 |
|----|------|
| **输入** | PRD §3.2 sync-state 模式 + T1.13 bind 命令输出格式 |
| **动作** | `cli/internal/workspace/sync_state.go`:frozen schema v1.0 定义(每个 binding 下文件 → `{md5, version, docCode, remotePath, lastSyncAt}`);读写校验;并发安全(文件锁 / atomic rename) |
| **输出** | sync-state.json schema 文档 + Go 类型定义 + JSON Schema 校验文件;`make schema-validate` 包含该 schema |
| **依赖** | T1.13(bind 已落地)+ T1.7(envelope schema 已锁) |
| **验收** | 单元测试覆盖并发读写 1000 文件;JSON Schema 校验文件生成成功;故意损坏 schema → 加载失败 |

### T1.19 CLI pull 命令实现

| 项 | 内容 |
|----|------|
| **输入** | T1.18 sync-state schema + T1.14 backend GET endpoints |
| **动作** | `cli/internal/cli/pull.go` + `cli/internal/sync/pull.go`:8 路并发(`errgroup + semaphore`);rename 检测(docCode 比对,rename 不破坏本地引用);删除护栏(`>10` 文件或 `>5MB` 单文件 → 需 `--yes`);detached 文件护栏;`--prune` `--dry-run` `--include/--exclude`(gitignore 语法)`--binary auto|skip|include` |
| **输出** | pull 子命令 + 单元测试 + 1000 文件端到端 benchmark |
| **依赖** | T1.13 + T1.14 + T1.18 |
| **验收** | 1000 文件 pull < 60s(NFR-PERF-04);rename 路径不破坏引用;护栏触发准确;partial failure 聚合到 `sync_results[].errors[]`,整体 exit 23 |

### T1.20 CLI push 命令实现

| 项 | 内容 |
|----|------|
| **输入** | T1.18 + T1.14 + T1.4 CAS |
| **动作** | `cli/internal/cli/push.go` + `cli/internal/sync/push.go`:8 路并发;CAS(`--base-version`);冲突策略 4 种(`manual / keep-mine / keep-theirs / both / auto`);`--prune`(软删远端,可 undelete)`--dry-run` `--include/--exclude` |
| **输出** | push 子命令 + 单元测试覆盖所有策略 |
| **依赖** | T1.13 + T1.14 + T1.4 + T1.18 + T1.19(pull 共享 sync_state) |
| **验收** | push 1000 文件 + CAS 冲突正确路由到 `--strategy`;`--prune` 软删可 undelete;残留冲突 → exit 24 |

### T1.21 关键指标埋点(覆盖 NFR-OBS-03)

| 项 | 内容 |
|----|------|
| **输入** | NFR-OBS-03 指标清单(QPS / 延迟 / 错误率 / 队列深度 / LLM token / 缓存命中率) |
| **动作** | `backend/internal/observe/metrics.go`:Prometheus metrics 注册;`backend/internal/middleware/metrics.go`:HTTP / DB / ES / SAG / LLM 调用处埋点;`backend/internal/handler/metrics.go`:`/metrics` endpoint;Langfuse OTLP 与 Prometheus 双通道 |
| **输出** | 6 类核心指标 + Grafana dashboard JSON(对应 T0.7 Langfuse panel) |
| **依赖** | T0.7(Langfuse 已部署)+ T1.8(OTLP 框架) |
| **验收** | `curl /metrics` 输出 6 类指标;Grafana dashboard 加载成功;压力测试下指标实时更新 |

### T1.22 frontend 项目脚手架(Vue 3 + Vite + Pinia)

| 项 | 内容 |
|----|------|
| **输入** | T0.10 frontend 目录 |
| **动作** | `frontend/`:Vue 3 + Vite 5 + Pinia 2 + Vue Router 4 + TypeScript;`package.json` 锁定依赖;ESLint + Prettier 继承 T0.5 规范;`vite.config.ts` 配置 proxy + build chunks;`.env.example` 暴露 `VITE_AIKBS_API_BASE`;`tsconfig.json` 配置 paths aliases;**`browserslist` 字段覆盖 NFR-FE-05**:Chrome 100+ / Firefox 100+ / Safari 15+ / Edge 100+;**Vite code-splitting 配置覆盖 NFR-FE-01**:chunkedVendor / chunkedCommon 拆分,首屏加载目标 < 3s |
| **输出** | 项目可独立 `npm run dev`;TS 类型检查通过;ESLint 零警告;浏览器兼容性自动测试 |
| **依赖** | T0.10 + T0.5 |
| **验收** | `cd frontend && npm install && npm run dev` 启动成功;`npm run build` 产出 dist;`npm run lint` 零警告;**`npm run build:analyze` 体积报告:首屏 chunk < 200KB gzipped**;browserslist 实际匹配上述 4 个浏览器 |

### T1.23 frontend 白标化骨架(品牌 / i18n / 第三方 SDK 替换)

| 项 | 内容 |
|----|------|
| **输入** | T1.22 脚手架 |
| **动作** | (1) 替换 logo / favicon / OG 分享图为 AI-KBS 资产;(2) CSS 品牌色变量统一;(3) `i18n/zh-CN.ts` / `en-US.ts` 清洗公司专有字符串;(4) 移除公司专有 SSO / 监控 SDK / 内部 CDN / 私有 npm 源;(5) `AppHeader` / `AppFooter` 文案替换 |
| **输出** | 浏览器加载前端显示 AI-KBS 品牌;无公司专有字样;脱敏审计脚本扫描通过 |
| **依赖** | T1.22 + T3.8 脱敏审计脚本(预先运行) |
| **验收** | 浏览器访问前端任意页面,无公司专有字样可见;`make scrub-audit` 0 命中;视觉与 WeKnora 原版一致(白标化是外观替换,不是 UI 重设计) |

### T1.24 frontend API 客户端路径对齐 + 基础 store

| 项 | 内容 |
|----|------|
| **输入** | T1.23 + backend 已稳定的 endpoint(`/api/ai-kbs/auth/status`, `/api/ai-kbs/spaces`) |
| **动作** | (1) `src/api/client.ts`:axios 实例 baseURL=`/api/ai-kbs`,JWT 注入 interceptor;(2) `src/api/auth.ts` / `spaces.ts` 实现;(3) `src/stores/auth.ts` Pinia store:token + user + role;(4) `src/router/index.ts` Vue Router 4 配置 + 路由守卫(JWT 校验) |
| **输出** | API 客户端可调用 backend;登录态持久化;未登录跳转 /login |
| **依赖** | T1.5(auth backend)+ T1.23 |
| **验收** | 浏览器登录后,`/api/ai-kbs/auth/status` 200 返回当前用户;Pinia store 持有 token;路由守卫拦截未登录访问 |

---

## 4. Phase M2 — 检索 + Agent + MCP(Week 6-7)

---

## 4. Phase M2 — 检索 + Agent + MCP(Week 6-7)

### T2.1 ES 索引设计 + mapping + IK 分词

| 项 | 内容 |
|----|------|
| **输入** | `require/05-data-model.md` §5 |
| **动作** | `internal/repository/search_es.go`:两个索引 `ai-kbs-chunks` + `ai-kbs-docs`;mapping 含 IK 分词;启动时自动创建(若不存在) |
| **输出** | 索引创建 + 单元测试 |
| **依赖** | T0.3(ES 容器) |
| **验收** | `curl localhost:9200/ai-kbs-chunks` 返回 mapping;`curl localhost:9200/ai-kbs-chunks/_analyze` 中文分词正确 |

### T2.2 docreader gRPC 集成

| 项 | 内容 |
|----|------|
| **输入** | WeKnora docreader proto |
| **动作** | `internal/parse/docreader_client.go`:gRPC client;5 种格式(PDF/Word/HTML/Txt/Markdown)的解析调用 |
| **输出** | client + 单元测试(mock server) |
| **依赖** | T0.3(docreader 容器) |
| **验收** | 上传 PDF / Word / MD 各 1 个,解析返回 chunks 数组,长度合理 |

### T2.3 chunking 三种策略

| 项 | 内容 |
|----|------|
| **输入** | T2.2 chunks |
| **动作** | `internal/parse/chunker.go`:策略 `semantic`(段落+句子边界)/ `parent_child`(父-子双层)/ `fixed`(固定窗口);每种可配置 size + overlap |
| **输出** | chunker + 单元测试(3 策略各 ≥ 10 case) |
| **依赖** | T2.2 |
| **验收** | 同一份长文档,3 策略产生的 chunk 数与平均大小符合预期 |

### T2.4 embedding pipeline

| 项 | 内容 |
|----|------|
| **输入** | T2.3 chunks |
| **动作** | `internal/embedding/`:4 个 provider(BGE / OpenAI / GTE / Ollama);异步任务(MQ)批量 embedding;embedding 缓存(Redis);失败重试 ≤3 次 |
| **输出** | embedder + MQ worker + 单元测试 |
| **依赖** | T2.3 + T0.3(RabbitMQ) |
| **验收** | 1000 chunks 批量 embedding < 60s(BGE on GPU / Ollama);Redis 命中后 < 50ms |

### T2.5 search service(BM25 + vector + hybrid)

| 项 | 内容 |
|----|------|
| **输入** | T2.1 + T2.4 |
| **动作** | `internal/service/search_service.go`:BM25 / Dense / Hybrid 三种模式;返回 `engine` 字段 |
| **输出** | service + 单元测试 |
| **依赖** | T2.1 + T2.4 |
| **验收** | 100w chunks 索引上,hybrid 查询 P95 < 200ms(NFR-PERF-01) |

### T2.6 SAG bridge 集成

| 项 | 内容 |
|----|------|
| **输入** | SAG 论文实现(本地源码 `e:/my-project/SAG/`) |
| **复制源路径** | **`e:/my-project/SAG/`** → `ai-kbs/sag-bridge/sag_src/`(复制核心算法,Python 重写包装层) |
| **动作** | (1) **复制 SAG 核心代码**:`cp -r e:/my-project/SAG/core/* ai-kbs/sag-bridge/sag_src/`;(2) 在 `sag-bridge/sag_bridge/` 下用 Python 写 FastAPI 包装层:`indexer.py`(chunk → event + entity)、`retriever.py`(query → events)、`explainer.py`(推理路径解释);(3) 用 LanceDB 替代 SAG 默认存储(SQLite 可作轻量备选);(4) PG 同步任务:同步 `sag_events` + `sag_entities` + `sag_event_entity_links` 表 |
| **输出** | bridge 服务 + 单元测试;LanceDB + PG 双写;`/events/append` `/events/search` `/events/explain` 3 个 HTTP endpoint |
| **依赖** | T2.3(chunks) + T2.4(embedding) + `e:/my-project/SAG/` 源码已复制 |
| **验收** | `curl -X POST sag-bridge:8090/events/append -d ...` 成功;`curl -X POST .../search` 返回 events + entities;PG `sag_*` 表与 LanceDB 数据一致 |

### T2.7 检索路由 query classifier

| 项 | 内容 |
|----|------|
| **输入** | T2.5 + T2.6 |
| **动作** | `internal/repository/rag_router.go`:规则 + 轻量 LLM 调用;分发到 ES / SAG / Hybrid |
| **输出** | router + 单元测试 |
| **依赖** | T2.5 + T2.6 |
| **验收** | 100 测试查询,路由准确率 ≥ 85%(人工标注) |

### T2.8 ReAct Agent 引擎 + 工具集

| 项 | 内容 |
|----|------|
| **输入** | WeKnora agent 引擎骨架 |
| **动作** | `internal/agent/engine.go` 沿用 WeKnora;`tools/`:`search / read_doc / sag_query / web_search / final_answer` 全部指向 ai-kbs backend |
| **输出** | engine + 5 个 tool |
| **依赖** | T2.5 + T2.6 |
| **验收** | `ask("退款规则")` 触发 search → read_doc → final_answer 完整 ReAct 链 |

### T2.9 ask NDJSON 流接口

| 项 | 内容 |
|----|------|
| **输入** | T2.8 |
| **动作** | `internal/handler/ask.go`:WebSocket / SSE 输出 NDJSON;事件类型:`init / thinking / tool_call / tool_result / answer / complete` |
| **输出** | NDJSON handler |
| **依赖** | T2.8 |
| **验收** | `curl -N .../ask` 实时输出 6 类事件;citations 字段含 doc_code |

### T2.10 MCP server 骨架

| 项 | 内容 |
|----|------|
| **输入** | FastMCP 框架 |
| **动作** | `mcp-server/ai_kbs_mcp/main.py`:FastMCP 入口;`transports/{stdio,sse,http}.py` |
| **输出** | MCP server 可启动(`ai-kbs mcp serve --transport stdio`) |
| **依赖** | T0.3 + T1.5(auth) |
| **验收** | stdio 模式下,echo 测试工具可被 Claude Code 调用 |

### T2.11 MCP tools 实现

| 项 | 内容 |
|----|------|
| **输入** | `require/07-mcp-tools.md` §2-3 |
| **动作** | `tools/`:14 个工具(core 5 + write 5 + version 3 + sag 2;admin 工具 v1.1);每个工具 frozen schema v1.0 + capability 校验 |
| **输出** | 14 个 MCP tool |
| **依赖** | T2.10 + backend 全部 endpoint |
| **验收** | Claude Code 通过 MCP 调用所有 14 个工具,全部成功 |

### T2.12 OAuth2 支持

| 项 | 内容 |
|----|------|
| **输入** | T2.10 |
| **动作** | `mcp-server/ai_kbs_mcp/oauth.py`:OAuth2 client + PKCE;scope 校验;token 存 `~/.ai-kbs/mcp-tokens.json` |
| **输出** | OAuth2 流程 |
| **依赖** | T2.10 |
| **验收** | `ai-kbs mcp install --agent claude-code --remote https://mcp.example.com` 走完浏览器授权流程 |

### T2.13 ai-kbs mcp install 子命令

| 项 | 内容 |
|----|------|
| **输入** | T1.10 CLI 框架 |
| **动作** | `cli/internal/cli/mcp.go`:`mcp install` 自动写 `.mcp.json` / `.cursor/mcp.json` 等;`mcp serve` 启动 MCP server(可选独立进程或内嵌) |
| **输出** | 2 个子命令 |
| **依赖** | T2.10 + T2.11 |
| **验收** | 在 Claude Code / Cursor 项目中,`ai-kbs mcp install` 一步接入,重启后工具可见 |

### T2.14 BDD 场景编写(48 个 Gherkin)

| 项 | 内容 |
|----|------|
| **输入** | PRD §6 验收场景草案 + `require/08-bdd-specs.md`(若已有) |
| **动作** | `tests/bdd/features/` 编写 48 个 `.feature` 文件,覆盖:认证(6)/ 工作空间(6)/ 同步(8)/ 单文档(8)/ 版本(5)/ 检索(5)/ 问答(5)/ MCP(5);每个 Feature 含 1-3 个 Scenario;对应 step 实现(GoDog / pytest-bdd) |
| **输出** | 48 个 Gherkin Feature 文件 + 双栈 step 实现;`make bdd-mvp` 在本地可跑通 |
| **依赖** | T1.13-T1.20(CLI 命令集)+ T2.10-T2.13(MCP tools) |
| **验收** | 48 个 Feature 文件全部存在且能加载;`make bdd-mvp` 在 M2 末尾跑通(用于 T3.1 验收的"全绿"前提) |

### T2.15 文档解析 worker pool + DLQ(覆盖 FR-PARSE-03)

| 项 | 内容 |
|----|------|
| **输入** | T2.2 docreader client + T2.3 chunking |
| **动作** | `backend/internal/parse/worker.go`:RabbitMQ 消费者(队列 `aikbs.doc.parse`)+ worker pool(N=4 可配);成功 → 写 PG chunks + 投递 `aikbs.doc.index`;失败 → retry × 3 指数退避;最终失败 → DLQ `aikbs.doc.parse.dlq`;admin API `/api/ai-kbs/admin/parse/retry` + `/api/ai-kbs/admin/parse/skip` 用于手动干预 |
| **输出** | worker + DLQ + admin retry/skip endpoint;单元测试覆盖 retry 路径 |
| **依赖** | T2.2 + T0.3(RabbitMQ) |
| **验收** | 故意注入解析失败 → 3 次 retry 后入 DLQ;admin API 重试成功;100 文件批量解析 < 5s(NFR-PERF-03) |

### T2.16 frontend Login 视图(标准 OAuth2 / 邮箱密码)

| 项 | 内容 |
|----|------|
| **输入** | T1.24 + T1.5(auth backend) |
| **动作** | `src/views/Login.vue`:登录表单(邮箱密码);OAuth2 PKCE 按钮(Google / GitHub);`src/views/SsoCallback.vue` 接收 OAuth2 code → 换 token;移除公司专有 SSO 按钮;登录失败显示错误码 + 文案 |
| **输出** | 浏览器登录页 + OAuth2 流程全通 |
| **依赖** | T1.24 + T2.12(OAuth2 backend) |
| **验收** | 邮箱密码登录 + OAuth2 流程均成功;JWT 写入 localStorage + Pinia;错误响应按 envelope schema v1.0 显示 |

### T2.17 frontend Workspace + Spaces 视图

| 项 | 内容 |
|----|------|
| **输入** | T2.16 + backend spaces endpoint |
| **动作** | `src/views/Workspace.vue`:列出所有可用 space,显示 code / name / 当前角色 / doc_count;点击进入 Space 详情;`src/views/Spaces.vue`(冒烟测试):纯列表,简单 GET 验证 |
| **输出** | 浏览器显示空间列表;点击 space 进入树形浏览 |
| **依赖** | T2.16 + T1.24 |
| **验收** | 浏览器登录后跳转 `/workspace`,看到所有 space;点击 space 进入 document 视图;无 doc 的 space 显示空状态 |

### T2.18 frontend Document 视图(markdown + outline + 版本)

| 项 | 内容 |
|----|------|
| **输入** | T2.17 + backend document/version endpoint |
| **动作** | `src/views/Document.vue`:左侧 outline + 右侧 markdown 渲染(marked + DOMPurify);顶部版本下拉(`ai_kbs_list_versions`)+ 版本 diff;点击 outline 项跳转锚点;markdown 内引用自动转 citation chip(点击跳转引用源);**CSS 响应式:NFR-FE-06 验收**:桌面 ≥ 768px 双栏布局,移动端 ≥ 375px 单栏折叠 outline |
| **输出** | 用户在浏览器看到 CLI push 的文档;markdown 渲染 + 版本切换 + outline 导航 |
| **依赖** | T2.17 + T2.11(MCP tools / backend endpoints) |
| **验收** | 浏览器打开某 space 下任一文档,显示 markdown;outline 跳转正确;版本切换看到旧版本;**NFR-FE-04 XSS 测试(注入 `<script>` / `<img onerror>` / `<svg/onload>` 各 1 case)不执行**;**NFR-FE-02 文档浏览 P95 < 500ms**(Playwright performance API 采集);**NFR-FE-06 响应式:375px / 768px / 1280px 三档截图比对通过** |

### T2.19 frontend Ask 视图(NDJSON 流式 + citations)

| 项 | 内容 |
|----|------|
| **输入** | T2.18 + T2.9(ask NDJSON)+ T2.6(SAG) |
| **动作** | `src/views/Ask.vue`:输入框 + 历史会话列表;流式渲染 NDJSON 事件(`init / thinking / tool_call / tool_result / answer / complete`);`engine: "sag"` 时高亮多跳推理路径;citation chip 可点击跳转到 Document 视图对应锚点;session_id 持久化 |
| **输出** | 用户在浏览器提问,实时看到流式答案 + 引用 |
| **依赖** | T2.18 + T2.9 + T2.6 |
| **验收** | 浏览器提问走 SAG multi-hop 时,显示 `engine: sag` 标识 + top-K events 推理路径;引用点击跳转正确;长会话 session_id 跨多轮保持 |

---

## 5. Phase M3 — v1.0.0 GA(Week 8)

### T3.1 BDD 场景全量回归(48 个)

| 项 | 内容 |
|----|------|
| **输入** | T2.14 已编写的 48 个 Feature + T1.x/T2.x 全部 |
| **动作** | 在全栈 v1 build 上跑完整 BDD 套件;失败场景定位并修复(非新增场景,新增归属 T2.14 阶段);输出失败分析报告 |
| **输出** | `make bdd` 全绿;失败定位闭环 |
| **依赖** | T2.14 + T1.x + T2.x 全部 |
| **验收** | 48/48 场景通过;无 `pending` / `skipped` 状态(除非显式标记为 v1.1) |

### T3.2 Claude Code / Codex 集成测试

| 项 | 内容 |
|----|------|
| **输入** | T2.13 MCP install + T3.1 BDD |
| **动作** | 在 Claude Code / Codex 真实环境中执行 A-H 八个典型工作流(读 / 编辑 / 创建 / 覆盖 / 绑定同步 / 回滚 / 恢复 / 提问) |
| **输出** | 集成测试报告(截图 + 日志) |
| **依赖** | T2.13 + T3.1 |
| **验收** | 8 个工作流全部端到端成功;脱敏审计脚本确认输出无内部平台专有字符串 |

### T3.3 性能基线压测

| 项 | 内容 |
|----|------|
| **输入** | 全栈 v1 build |
| **动作** | wrk / k6 压测:`/api/ai-kbs/search` P95 / `pull` 1000 文件耗时 / `ask` P95(不含 LLM)/ CLI 启动时间 |
| **输出** | 性能报告,所有指标达标 |
| **依赖** | T1.x + T2.x + 测试数据集(1000 文档) |
| **验收** | NFR-PERF-01..06 全达标 |

### T3.4 整体集成测试 + 端到端冒烟

| 项 | 内容 |
|----|------|
| **输入** | 全栈 v1 build + 测试数据集 |
| **动作** | `tests/integration/`:mock 后端 + 真后端双模式;`tests/e2e/`:Bash 脚本走完整用户旅程(注册→建工作空间→上传→检索→问答→同步→回滚) |
| **输出** | 集成测试报告 |
| **依赖** | T3.1 + T3.2 |
| **验收** | mock 与真后端双模式全过;E2E 冒烟脚本成功 |

### T3.5 中英双语文档 + SKILL.md 定稿 + Frozen Schema v1.0 发布

| 项 | 内容 |
|----|------|
| **输入** | 所有交付物 |
| **动作** | (1) README.md(中英)+ docs/{getting-started,architecture,api,cli,mcp,adr}/;(2) SKILL.md 定稿 + 6 个 references;(3) CHANGELOG v1.0.0;(4) **Frozen Schema v1.0 正式发布**:在 `docs/api/frozen-schema-v1.0.md` 完整定义 CLI envelope + 14 个 MCP tool input/output schema + REST 21 个 endpoint schema;字段集合不再变更;schema 校验脚本随二进制 release |
| **输出** | 完整文档站点可访问;Frozen Schema v1.0 文档 + JSON Schema 文件(JSON Schema 2020-12 语法);`make schema-validate` 命令在 CI 阻断 schema 漂移 |
| **依赖** | T3.1 - T3.4 |
| **验收** | VitePress 构建零警告;SKILL.md 8 个工作流全部可在 Claude Code 真实跑通;**Frozen Schema v1.0 文档生成 + 14 个 MCP tool schema + 21 个 REST schema 全部就绪**;`make schema-validate` 通过 |

### T3.6 Helm Chart K8s 部署验证

| 项 | 内容 |
|----|------|
| **输入** | helm/ + 全栈镜像 |
| **动作** | 在 K8s 测试集群(kind / minikube / 自有集群)`helm install` 完整部署;Ingress 切流 |
| **输出** | 部署报告(资源 / 副本 / 健康检查) |
| **依赖** | T0.4 + T3.1 |
| **验收** | 所有 pod Ready;Ingress 200;扩缩容触发正常 |

### T3.7 release artifact 构建 + tag v1.0.0 + 跨平台矩阵验证

| 项 | 内容 |
|----|------|
| **输入** | T0.2 CI + 全栈 v1 build + T3.5 Frozen Schema |
| **动作** | (1) 创建 `v1.0.0` tag;(2) CI 自动构建 6 平台二进制(linux/darwin/windows × amd64/arm64)+ 3 个 Python wheel + 2 个 Docker 镜像;(3) **跨平台矩阵验证**:每个平台至少跑 1 个冒烟测试(`ai-kbs --version` + `ai-kbs login --help`),通过 matrix strategy 并行;(4) 发 GitHub Release |
| **输出** | release page + 11 个 artifact + SHA256SUMS + 6 平台验证日志 |
| **依赖** | T3.1 - T3.6 + T0.2(matrix strategy 配置) |
| **验收** | release artifact 可下载;SHA256 校验通过;6 平台冒烟测试全部通过;安装即用 |

### T3.8 脱敏审计脚本 + CI 阻断

| 项 | 内容 |
|----|------|
| **输入** | 整个仓库 |
| **动作** | `scripts/scrub-audit.sh`:正则匹配内部平台专有字符串前缀 + [内部 Wiki 系统] 平台专有名称;命中即 exit 1;接入 CI lint workflow |
| **输出** | 审计脚本 + CI 集成 |
| **依赖** | T0.2(CI 框架) |
| **验收** | 在 PR 中故意加入内部平台专有字符串 → CI 红;`make scrub-audit` 在主分支上零命中 |

### T3.9 Playwright E2E + 视觉回归(覆盖 FR-WEB-14/15)

| 项 | 内容 |
|----|------|
| **输入** | T2.16-T2.19 全部 frontend 视图已就绪 |
| **动作** | `frontend/tests/e2e/`:`auth.spec.ts`(登录 / 失败 / OAuth2)/ `workspace.spec.ts`(浏览空间列表 + 树形)/ `document.spec.ts`(文档渲染 + outline + 版本 + XSS 防护)/ `ask.spec.ts`(流式渲染 + citations 跳转 + SAG 高亮);Playwright 截图比对基线入库(`tests/e2e/snapshots/`);CI 矩阵覆盖 Chromium + Firefox + WebKit |
| **输出** | 4 个 E2E spec + 截图基线;`make e2e` 命令全过 |
| **依赖** | T2.16 + T2.17 + T2.18 + T2.19 |
| **验收** | 4 个 spec 全过;视觉回归 0 偏差(故意改 logo 颜色 → 测试失败);3 浏览器矩阵全绿;NFR-FE-01/02/03 性能指标采集 |

---

## 6. 启动前检查(Before `task.py start`)

> 这些检查必须在 `task.py start` 之前全部通过。

### 6.1 文档完整性

- [x] `prd.md` 存在,无未解决的开放问题
- [x] `design.md` 存在,覆盖架构 + 数据流 + 部署 + 权衡
- [x] `implement.md` 存在,所有任务有输入/输出/依赖/验收
- [x] `implement.jsonl` ≥ 1 条真实条目(非 seed-only)
- [x] `check.jsonl` ≥ 1 条真实条目(非 seed-only)

### 6.2 范围锁定

- [x] v1.0.0 GA 范围 = Backend + CLI + MCP + Helm(无 Web UI)— 用户已确认
- [x] out-of-scope = Web UI / 数据源 / Chrome Extension / 多格式解析扩展 / migrate 工具 — 已划入 v1.1

### 6.3 脱敏审计

- [x] PRD/design/implement 中无 [内部 Wiki 系统] 平台专有字样(由脱敏审计脚本 T3.8 在 CI 中强制)
- [x] 脱敏审计脚本(T3.8)将在 CI 中阻断类似字符串

### 6.4 验收标准已定义

- [x] 48 个 BDD 场景来自 `require/08-bdd-specs.md`
- [x] 性能基线 NFR-PERF-01..06 量化
- [x] 准确率基线 NFR(2WikiMultiHopQA +5pp / MuSiQue +5pp)
- [x] 兼容性矩阵 6 平台 + frozen schema v1.0

### 6.5 风险与回退已识别

- [x] WeKnora fork breaking change → pin 版本 + hotfix 分支
- [x] SAG 不可用 → 自动降级 RAG-only
- [x] ES 不可用 → 自动降级 PG 全文
- [x] LLM provider 不可用 → 多 provider 降级链

### 6.6 用户最终批准(下一步)

> 当前阶段:用户已锁定 v1 范围 + PRD 反馈。本文档待用户对最终规划总结做整体确认,然后才可 `task.py start`。

### 6.7 Frontend 启动前检查(Option B 增量)

- [ ] **Frontend Vitest 单元测试覆盖率 ≥ 70%**(`frontend/tests/unit/` 跑通)
- [ ] **Frontend ESLint + Prettier 零警告**(`cd frontend && npm run lint`)
- [ ] **Playwright E2E baseline 已提交**(`frontend/tests/e2e/snapshots/` 入库,4 个 spec)
- [ ] **白标化视觉回归 baseline 已提交**(所有页面截图比对基线)
- [ ] **Frontend 体积报告 OK**(首屏 chunk < 200KB gzipped,`npm run build:analyze`)
- [ ] **NFR-FE-01/02/03 性能 baseline 已采集**(Playwright performance API)
- [ ] **NFR-FE-04 XSS 测试用例已入库**(3 case:`<script>` / `<img onerror>` / `<svg/onload>`)
- [ ] **NFR-FE-05 浏览器矩阵已验证**(Chromium + Firefox + WebKit 三浏览器 Playwright 通过)

---

## 7. 关键接口契约(摘要)

### 7.1 Backend REST(同 `design.md` §4.3)

### 7.2 CLI envelope(frozen schema v1.0)

(同 `design.md` §4.1)

### 7.3 MCP tool input/output schema

(同 `require/07-mcp-tools.md`)

### 7.4 SAG bridge HTTP

```text
POST /events/append        # 异步索引新 chunk
POST /events/search        # query → events
POST /events/explain       # 解释推理路径
GET  /healthz              # 健康检查
```

### 7.5 docreader gRPC

(同 `design.md` §2.5.1)

---

## 8. 验证命令清单

> 每个任务完成后,运行对应的验证命令。

| 阶段 | 命令 | 期望 |
|------|------|------|
| M0 完 | `docker compose -f docker-compose.dev.yml up -d && sleep 60 && curl localhost:8080/healthz` | 200(含 frontend container) |
| M0 完 | `make lint` | 0 warning(Go + Python + Vue 全栈) |
| M1 完 | `make test-unit` | coverage ≥ 85%(core 包)+ frontend vitest ≥ 70% |
| M1 完 | `make cli-build` | 6 平台二进制产物 |
| M1 完 | `cd frontend && npm run build` | dist 产物 + 体积检查 |
| M2 完 | `make bdd-mvp` | 14 个 BDD 场景全过 |
| M2 完 | Claude Code E2E | 8 个工作流全成功 |
| M2 完 | `cd frontend && npm run dev` | 5 个视图均可点;无控制台错误 |
| M3 完 | `make bdd` | 48 个 BDD 场景全过 |
| M3 完 | `make perf` | NFR-PERF-01..06 + NFR-FE-01..03 全达标 |
| M3 完 | `make helm-install` | K8s 部署成功(含 frontend Deployment + Ingress) |
| M3 完 | `make scrub-audit` | 0 命中(含 frontend 目录) |
| M3 完 | `make e2e` | Playwright 4 个 spec 全过 + 视觉回归 0 偏差 |

---

## 9. 高风险文件与回退点

### 9.1 高风险文件

| 文件 | 风险 | 回退 |
|------|------|------|
| `backend/internal/auth/hmac.go` | HMAC 实现错位导致任意签名通过 | 单元测试覆盖 100+ case;PR 强制 2 review |
| `backend/internal/errors/envelope.go` | frozen schema v1.0 envelope 任何字段错改 → CLI/MCP/SDK 全链路解析失败 | `make schema-validate` 在 CI 阻断;字段集合添加须走 v2 schema 升级 |
| `backend/internal/repository/rag_router.go` | 路由错误导致检索精度退化 | A/B 评估接口 + 离线回归集 |
| `cli/internal/sync/pull.go` | 并发同步产生脏数据 | `sync-state.json` 事务化 + 失败回滚 |
| `cli/internal/sync/push.go` | 8 路并发 push + CAS 冲突策略选错 → 误覆盖远端 | push 单元测试覆盖所有 `--strategy`;`--prune` 强制 `--yes`;detached 文件护栏 |
| `mcp-server/ai_kbs_mcp/tools/*.py` | 14 个 MCP tool 的 frozen schema v1.0 校验错 → Agent 解析失败 | 每个 tool 启动时 schema 自校验;`make schema-validate` 比对 release artifact schema |
| `migrations/001_init.up.sql` | 表结构错 → 无法 up/down | dev 环境反复 up/down 测试;CI 校验 |
| `cli/skills/SKILL.md` | HARD RULES 错误 → Agent 误操作 | Claude Code 真实跑通 A-H 工作流(8/8 必过) |
| `frontend/src/views/Document.vue` | markdown 渲染 + XSS 漏洞(NFR-FE-04) | marked + DOMPurify 双重清洗;Playwright 注入 `<script>` / `<img onerror>` / `<svg/onload>` 三 case 测试 |
| `frontend/src/api/client.ts` | API 路径前缀未对齐 → 404 雪崩 | T1.24 路径统一 `/api/ai-kbs`;`make e2e` 全 spec 验证所有 endpoint 可达 |
| `frontend/src/views/Login.vue` + `SsoCallback.vue` | OAuth2 PKCE 流程错位 → 远程 MCP 登录失败 | OAuth2 标准库 + 单元测试覆盖所有 grant type;真实浏览器 E2E |
| `frontend/src/views/Ask.vue` | NDJSON 流式解析断流(浏览器关闭 / 网络中断) → 用户看到残缺答案 | 自动重连 + session_id 持久化 + 客户端断点续传;T2.19 验收 + T3.9 E2E |

### 9.2 关键回退点

- 任何 T1.x 阶段改动 → 重置 `migrate down && migrate up`
- frozen schema v1.0 字段变更 → 不可逆,必须升级 schema 版本
- Helm chart 重大变更 → 保留上一版本 release;新 chart 走 dry-run + canary
- 脱敏审计失败 → PR 阻断,不允许合并

### 9.3 团队配置回顾(针对 reviewer 反馈 + Option B Web UI 加入)

- **5 FTE × 10-11 周 = 50-55 person-weeks ≈ 350-385 person-days**
  - 后端 2 + CLI/MCP 1 + **前端 1(Week 5 后段开始介入)** + DevOps 0.5 + QA 0.5
- **63 个任务 × 平均 1.5 day/任务 ≈ 95 person-days(净编码)**
- 余量 ≈ 255-290 person-days 用于:BDD 编写(T2.14,48 场景)/ Playwright E2E(T3.9)/ 调试与返工 / 文档 / 测试 / 跨平台 matrix / Frontend 白标化验证
- **结论**:**Week 10-11 GA 可达**,关键路径:
  - 前端工程师 Week 5 后段入职(等 T1.5 auth + T1.7 envelope 稳定后开始 fork 改造)
  - Frontend 白标化 7 人天(T1.23)+ 视图开发 4 个任务 × 1.5 天 ≈ 13 人天(Week 7-9)
  - BDD 编写从 M2 早期并行启动(T2.14)
  - 若 M1 阶段有 > 3 天返工,需把 M3 顺延 0.5-1 周并相应调整 v1.1 启动时间

---

## 10. 后续检查清单

> `task.py start` 之后,Phase 2 启动前应做的最后一轮检查:

- [ ] 所有任务责任人已分配
- [ ] 团队对 design.md 中的 8.x 权衡已达成共识
- [ ] 分支策略确定(主分支:`main`;开发分支:`feature/<task-id>`;hotfix:`hotfix/<desc>`)
- [ ] 根 Makefile(T0.9)就绪 + 顶层命令全部可跑
- [ ] CI 全绿(lint + test + build + bdd-mvp + e2e)
- [ ] 脱敏审计 0 命中(含 frontend 目录)
- [ ] BDD 冒烟(T2.14 完成后跑 5 个核心场景)全过
- [ ] Frozen Schema v1.0 文档(T3.5)草稿可访问
- [ ] 团队已读过 PRD + design + implement
- [ ] **Frontend 工程师已入职**(Week 5 后段介入;1 名熟练 Vue 3 + Vite + Pinia)
- [ ] **白标化视觉回归 baseline 已提交**(所有页面截图比对基线)
- [ ] **Playwright 4 个 spec 在 CI 全过**(登录 / 浏览 / 提问 / 引用跳转)
- [ ] **5 个视图在 dev 环境全部可点**(Login / Workspace / Document / Ask / Spaces)

---

**implement.md 与 prd.md / design.md 一同构成 v1.0.0 GA 的完整规划三件套,等待用户最终批准。**
