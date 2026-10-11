# AI-KBS v1 MVP — 需求文档(PRD)

> 项目代号:**AI-KBS**(AI Knowledge Base System)
> 仓库:https://github.com/eyangisgod-cell/ai-kbs
> 文档版本:v1.0 — 2026-10-02
> 状态:brainstorm 阶段 — 等待用户对 v1 范围边界的最终决策

---

## 1. 背景与目标

### 1.1 背景

当前企业知识库工作流存在三类痛点:

1. **AI Agent 无法直接接入**。 主流知识库(开源/商业)对 AI Agent 不友好:要么没有 MCP 接口,要么 CLI 输出不可被 Agent 解析,Agent 只能把知识库当 HTTP 文本源暴力抓取。
2. **多跳问答召回率不足**。 传统 BM25 / Dense RAG 在 2WikiMultiHopQA、MuSiQue 等多跳基准上 Recall@5 仍偏低;纯 GraphRAG 构建成本高、增量更新困难。
3. **文档与代码协作割裂**。 文档需要细粒度权限、审计、版本回滚;Git 仓库模型对文档场景过重(Git LFS / partial clone / submodule 复杂度过剩)。

团队已经掌握了**企业级 Wiki 系统的成熟范式**(CAS 乐观锁 + 稳定标识 + workspace/bind 同步模型 + AI-friendly wire contract),并通过 Go CLI + Python 后端 + SKILL 操作手册的形式在内部验证。但该内部系统无法对外开源,需要构建一个**对等开源**的 AI 知识库系统。

### 1.2 项目定位

> **AI-KBS 是面向 AI Agent 开发协作者的开源企业级知识库系统。**

差异化定位:

| 维度 | 传统 Wiki | 传统 RAG 工具 | **AI-KBS** |
|------|----------|--------------|------------|
| AI Agent 接入 | 无 / 弱 | 仅有 REST | **MCP 一等公民**(stdio/SSE/HTTP/OAuth2) |
| CLI 输出 | 散文文本 | 不可解析 | **frozen JSON envelope + next_action** |
| 检索引擎 | 单选 | RAG-only | **ES + RAG + SAG 三层并存**(WeKnora RAG 框架完整保留作降级路径,SAG 作多跳增强) |
| 版本管理 | Git / 无 | 无 | **CAS + docCode 稳定标识 + rollback 保留中间版本** |
| 协作模式 | 单人 / 团队 | 无 | **workspace + bind + pull/push 同步模型** |
| SKILL 手册 | 无 | 无 | **随项目分发,Agent 可直接读取** |

### 1.3 项目目标(v1 GA)

| # | 目标 | 度量 |
|---|------|------|
| G1 | 让 Claude Code / Codex / Cursor 通过一行 MCP 配置即可接入 | `ai-kbs mcp install` 一条命令完成 |
| G2 | 多跳问答准确率达到业界 SOTA | 在 2WikiMultiHopQA / MuSiQue 子集上 Recall@5 ≥ WeKnora 基线 +5pp |
| G3 | AI Agent 可双向操作文档(读 / 写 / 编辑 / 版本) | MCP 工具集覆盖 CRUD + 版本 + 绑定 |
| G4 | 单二进制 CLI 跨平台分发 | Linux / macOS / Windows × amd64 / arm64,无运行时依赖 |
| G5 | 中小团队可自部署 | Docker Compose 一键启动 ≤ 10 分钟 |

---

## 2. 范围 / 非范围

### 2.1 v1 GA In-Scope(P0 必交付)

> v1 GA = 面向 AI Agent 的**最小可独立生产化版本**。详见 §6 里程碑 M3。

- 后端核心:文档 CRUD / 版本链 / workspace-bind 同步 / **ES + RAG + SAG 三层检索** / ReAct Agent / MCP server
- 检索架构(明确三层并存):
  - **ES**:BM25 + Dense + Hybrid 全文检索引擎(底层基础设施)
  - **RAG**:WeKnora 完整 RAG 框架(单引擎 RAG-only 路径,**完整保留作降级路径**)
  - **SAG**:事件 - 实体 - 动态超边多跳推理(多跳增强)
  - **Query Classifier**:按 query 类型路由 → ES-RAG / SAG / 融合
  - 三层并存,SAG 不可用时自动降级到 RAG-only(ES + pgvector)
- 文档解析:Markdown / PDF / Word / HTML / Txt(其余格式 v1.1)
- CLI:认证 / 工作空间 / 同步 / 单文档 / 版本 / 检索 / 问答 / MCP 共 ~20 个子命令,所有命令默认输出 JSON envelope
- **Web UI**:基于 WeKnora v0.7.1 Vue 3 SPA fork,**白标化改造**(logo / 品牌色 / 产品名 / SSO → 标准 OAuth2 / 第三方 SDK / API 路径对齐);用户可在浏览器查看 CLI 同步上传的文档 / 提问(SAG 多跳) / 管理空间;覆盖 5 个核心视图(workspace / document / ask / spaces / login)
- 鉴权:HMAC-SHA256(CLI 自动化)+ JWT(Web / API)+ Scoped API Key(嵌入 / 自动化)
- RBAC:Owner / Admin / Contributor / Viewer 四级 + per-Space 所有权
- 可观测:Langfuse 集成(OTLP)+ 关键指标埋点
- 部署:Docker Compose(开发)+ Helm Chart(生产基础)+ Frontend 静态资源 / SPA
- 文档:中英双语 README + CLI SKILL.md + 公开 docs/

### 2.2 v1 GA Out-of-Scope(明确不做)

| 项 | 不做理由 | 计划 |
|----|---------|------|
| Web 知识图谱可视化(D3 / Cytoscape) | v1 聚焦核心浏览/问答;KG 可视化为体验增强 | v1.1 |
| Web 管理后台(队列监控 / 审计日志 / worker pool) | v1 通过 CLI + Langfuse dashboard 覆盖运维 | v1.1 |
| 数据源迁移工具(从 [参考项目 A] 导入) | v1 不交付迁移工具;新工作空间从零创建 | v1.1 |
| 飞书 / Notion / 语雀 数据源同步 | 聚焦核心 Wiki + 问答;数据源接入会引入 OAuth 复杂度 | v1.1 |
| IM 通道(企微 / 飞书 / Slack / Telegram / DingTalk) | 架构预留 webhook 入口但不交付具体实现 | v2 |
| Chrome Extension / 网站嵌入 widget | 与 AI Agent 优先战略偏离;业务模型需另定 | v2 |
| 自研 Embedding / 自训模型 | 复用开源模型(BGE / GTE / OpenAI / Ollama) | 不在路线图 |
| 多模态图像问答 | v1 仅文档问答;图像处理仅做 OCR 进文本 | v2 |
| 移动端 Mini Program | 与 v1 定位不符 | v2 |
| 工作流引擎(草稿 - 审核 - 发布) | v1 用 CAS + RBAC 已覆盖;工作流属 v2 | v2 |

### 2.3 部署形态

- **开发环境**:Docker Compose 单机起 PostgreSQL + Elasticsearch + Redis + RabbitMQ + LanceDB + Langfuse + ai-kbs-backend + mcp-server + ai-kbs-frontend
- **生产环境**:Helm Chart 部署到 Kubernetes;支持 HPA / 健康检查 / 灰度;Frontend 走 Ingress SPA 路由 + nginx 静态资源
- **混合云**:Backend 部署在私有网络,MCP Server + Frontend 通过 HTTPS + OAuth2 暴露公网

---

## 3. 确认事实(Confirmed Facts)

> 来自 `require/00-context.md` / `04-architecture.md` / `05-data-model.md` / `10-roadmap.md`,以及本项目提示词。

### 3.1 技术选型(已锁)

| 层 | 选型 | 备注 |
|----|------|------|
| 后端基座 | WeKnora v0.7.1 fork | RAG + ReAct Agent + Wiki Mode + 知识图谱 + RBAC + Langfuse 全部沿用 |
| 检索增强 | SAG(论文实现) | 事件 - 实体 - 动态超边;与 ES 并存路由 |
| 文档解析 | WeKnora docreader(Python gRPC) | 沿用,支持 12+ 格式 |
| 后端语言 | Go(沿用 WeKnora) | CLI 同语言,降低维护成本 |
| MCP server | Python(FastMCP) | 生态成熟,工具描述自动生成 |
| CLI | Go + Cobra | 单二进制静态编译,CGO_ENABLED=0 |
| LLM Provider | OpenAI 兼容协议 | OpenAI / Anthropic / DeepSeek / Qwen / Ollama |
| Embedding | BGE / GTE / OpenAI / Ollama | 默认 BGE-large-zh-v1.5 |
| 主数据库 | PostgreSQL 15+ | 含 pgvector 扩展 |
| 检索副本 | Elasticsearch 8.x | IK 中文分词 |
| 缓存 | Redis 7+ | 会话 / 限流 / 缓存 |
| 异步 | RabbitMQ 3.12+ | 文档解析 / 索引任务 |
| SAG 存储 | LanceDB(默认) | SQLite 可作为轻量替代 |
| 对象存储 | Local / MinIO / S3 / OSS / TOS / KS3 / OBS | 可插拔适配器 |
| 可观测 | Langfuse(OTLP) | 覆盖 RAG / Agent / 解析 / MCP / CLI |
| 部署 | Docker Compose(dev)+ Helm(prod) | — |
| 鉴权 | HMAC-SHA256 + JWT + Scoped API Key | 详见 FR-AUTH |

### 3.2 关键设计模式(沿用,概念可保留名称)

| 模式 | 含义 | 来源附录 |
|------|------|----------|
| Workspace | 本地目录,持 `.ai-kbs/workspace.yaml` + `sync-state.json` | 见 [附录 A.1](#附录-a概念参考来源映射) |
| Space | 远端的命名空间(KB 等价物),文档按 Space 隔离 | 见 [附录 A.1](#附录-a概念参考来源映射) |
| docCode | 8 位 base62 稳定标识,move / rename 不变 | 见 [附录 A.1](#附录-a概念参考来源映射) |
| CAS 乐观锁 | 写入时携带 `base_version`,冲突返回 next_action | 见 [附录 A.1](#附录-a概念参考来源映射) |
| 版本链 | 每次写入新增 version,中间版本全部保留 | 见 [附录 A.1](#附录-a概念参考来源映射) |
| workspace/bind 同步 | 本地目录 ↔ 远端 space:path 双向同步 | 见 [附录 A.1](#附录-a概念参考来源映射) |
| rollback = revert 语义 | 回滚产生新版本,中间版本不删 | 见 [附录 A.1](#附录-a概念参考来源映射) |
| Event / Entity / Hyperedge | SAG 论文核心数据结构 | 来源:参考 SAG 论文 |
| RAG / Hybrid | 通用检索增强生成;Hybrid = BM25 + Dense 重排 | 通用术语 |

### 3.3 已采纳的关键决策(ADR)

| ID | 决策 | 状态 |
|----|------|------|
| ADR-001 | 基座选择 WeKnora 而非自研(降低 80% 重叠工作量) | ✅ Accepted |
| ADR-002 | 检索层引入 SAG 而非自研 GraphRAG(增量更新友好) | ✅ Accepted |
| ADR-003 | CLI 借鉴 [参考项目 A] 的成熟模式,**命名与命令结构去耦合** | ✅ Accepted |
| ADR-004 | 版本管理采用 CAS + docCode 而非 Git(权限与审计模型更贴合) | ✅ Accepted |
| ADR-005 | MCP 作为 AI Agent 标准入口(Claude Code / Codex / Cursor / Gemini CLI) | ✅ Accepted |
| ADR-006 | **v1 GA 范围 = Backend + CLI + MCP + Web UI + Helm** — Week 10-11 末 GA;Web UI 基于 WeKnora Vue 3 SPA fork + 白标化 | ✅ Resolved 2026-10-02 |
| ADR-007 | **CLI 命令集合独立设计** — 子命令分组 / flag 集合 / envelope schema 均不照搬内部平台 | ✅ Resolved 2026-10-02(用户硬约束) |
| ADR-008 | **数据迁移工具(从 [参考项目 A] 导入)推迟到 v1.1** — v1 用户从零 init 工作空间 | ✅ Resolved 2026-10-02(与 ADR-006 一致) |
| ADR-009 | **Web UI 基于 WeKnora Vue 3 SPA fork + 白标化改造**(logo / 品牌色 / 产品名 → AI-KBS;移除公司专有 SSO / 监控 SDK / CDN / 配置;API 路径对齐 /api/ai-kbs);总工作量 ~7 人天;1 名熟练 Vue 工程师可完成 | ✅ Resolved 2026-10-02 |

### 3.4 关键约束(已锁,不可破)

- 仓库**不出现任何** `[内部 Wiki 系统]` 平台专有字样;命名 / 配置 / 命令 / 代码均不得照搬该平台
- Go 标识符:`camelCase / PascalCase`;CLI flag:`kebab-case`;环境变量:`AIKBS_*`;JSON 字段:`camelCase`
- frozen schema v1.0:`--json` envelope 字段集合已锁(`json_schema_version / success / exit_code / error / data / next_action / request_id`)
- 退出码表与错误码命名空间(`auth.* / rbac.* / doc.* / binding.* / sync.* / search.* / sag.* / internal.*`)已锁

---

## 4. 功能需求

> 编号 FR-xxx。优先级 P0(v1 必交付)/ P1(v1.1)/ P2(未来)。

### FR-WK:WeKnora 集成层

| # | 需求 | 优先级 |
|---|------|--------|
| FR-WK-01 | fork WeKnora v0.7.1,剥离内部 SKU 化逻辑(鉴权 SDK / 计量 / 计费等) | P0 |
| FR-WK-02 | 替换其 internal/auth 模块为 ai-kbs 自研鉴权(HMAC + JWT + API Key) | P0 |
| FR-WK-03 | 替换其 internal/agent 为 ReAct Agent,集成 SAG 检索工具 | P0 |
| FR-WK-04 | 替换其存储后端为可插拔 adapter(Local / MinIO / S3 / OSS / TOS / KS3 / OBS) | P0 |
| FR-WK-05 | 替换其 web/frontend 路由前缀,避免与 WeKnora 命名冲突 | P0 |

### FR-SAG:双引擎检索

| # | 需求 | 优先级 |
|---|------|--------|
| FR-SAG-01 | 集成 SAG 作为独立服务(FastAPI + LanceDB) | P0 |
| FR-SAG-02 | chunk → event + entity 自动抽取(SAG indexer,异步) | P0 |
| FR-SAG-03 | query → events 检索,查询时动态生成超边 | P0 |
| FR-SAG-04 | 双引擎超时独立(ES 3s / SAG 5s) | P0 |
| FR-SAG-05 | query classifier(LLM-call + 规则):关键词单跳 → ES;多跳/实体 → SAG;不确定 → 融合 | P0 |
| FR-SAG-06 | 检索结果融合:并行召回 + cross-encoder 重排 | P0 |
| FR-SAG-07 | 响应中标注 `engine` 字段(`es` / `sag` / `hybrid`) | P0 |
| FR-SAG-08 | A/B 评估接口(管理员后台):召回率 / F1 / 延迟 / 引用准确率 | P1 |
| FR-SAG-09 | SAG 文档量 > 100k 时触发分片与冷热分层 | P1 |
| FR-SAG-10 | 嵌入模型可配置(BGE / GTE / OpenAI / Ollama),维度可调 | P0 |

### FR-CLI:自研 CLI(Go + Cobra)

> **命名原则**:命令动词参考业界惯例(init / bind / pull / push / read / write / search / ask),但子命令集合、flag 命名、错误响应 envelope **不复用 [内部 Wiki 系统] 的内部命令结构**。详见 [附录 A.2](#附录-a概念参考来源映射)。

| # | 需求 | 优先级 |
|---|------|--------|
| FR-CLI-01 | 全局 flag:`--url / --profile / --format / --quiet / --yes / --timeout / --debug` | P0 |
| FR-CLI-02 | 环境变量:`AIKBS_URL / AIKBS_KEY_ID / AIKBS_KEY_SECRET / AIKBS_TOKEN / AIKBS_PROFILE / AIKBS_WORKSPACE_FILE / AIKBS_TIMEOUT / AIKBS_CONFIG_DIR / AIKBS_DEBUG` | P0 |
| FR-CLI-03 | 凭证优先级链:`AIKBS_KEY_*` env → `~/.ai-kbs/credentials:hmac` → `~/.ai-kbs/credentials:token` → `AIKBS_TOKEN` → exit 10 | P0 |
| FR-CLI-04 | `--json` envelope(frozen schema v1.0):成功 / 错误分支,错误时 `data: null`,必有 `next_action` | P0 |
| FR-CLI-05 | 子命令:login / logout / init / status / bind / unbind / pull / push / read / write / edit / search / grep / tree / spaces / version / move / undelete / resolve / ask / mcp | P0 |
| FR-CLI-06 | 寻址 4 flag:`--url`(自包含页面 URL)> `--space + --doc`(docCode)> `--space + --path`(路径解析) | P0 |
| FR-CLI-07 | 互斥规则:`--url` 与 `--space/--doc/--path` 互斥;`--doc` 与 `--path` 互斥 | P0 |
| FR-CLI-08 | 8 路并发 pull / push;`errgroup + semaphore` | P0 |
| FR-CLI-09 | 删除护栏:`>10` 文件或 `>5MB` 单文件需 `--yes`;Detached 文件不被 prune | P0 |
| FR-CLI-10 | CAS:`write --base-version`、`edit --base-version`;冲突 → `next_action.strategies` 列表 | P0 |
| FR-CLI-11 | `sync-state.json` schema v1.0:frozen,记录每文件 `{md5, version, docCode, remotePath}` | P0 |
| FR-CLI-12 | rename 检测:基于 docCode 稳定标识,rename 不破坏本地引用 | P0 |
| FR-CLI-13 | NDJSON 流(chat / ask):`init / thinking / tool_call / tool_result / answer / complete` | P0 |
| FR-CLI-14 | 退出码:0/1/2/10/11/20/21/22/23/24/25/30/31/32/40/41 已锁 | P0 |
| FR-CLI-15 | 单二进制静态编译,跨平台(linux/darwin/windows × amd64/arm64) | P0 |
| FR-CLI-16 | SKILL.md + references/{auth,workspace,sync,conflict,version,mcp}.md 随二进制分发 | P0 |

### FR-MCP:MCP 工具集(AI Agent 入口)

| # | 需求 | 优先级 |
|---|------|--------|
| FR-MCP-01 | MCP server 多传输:stdio / SSE(8081)/ HTTP(8081) | P0 |
| FR-MCP-02 | OAuth2 支持(远程 MCP,scope:`kbs:read / kbs:write / kbs:version / kbs:admin`) | P0 |
| FR-MCP-03 | Core 层工具:`ai_kbs_search / ai_kbs_read_doc / ai_kbs_ask / ai_kbs_list_spaces / ai_kbs_get_doc_outline` | P0 |
| FR-MCP-04 | Write 层工具(capability 校验):`ai_kbs_create_doc / ai_kbs_update_doc / ai_kbs_edit_doc / ai_kbs_move_doc / ai_kbs_delete_doc` | P0 |
| FR-MCP-05 | Version 层工具:`ai_kbs_list_versions / ai_kbs_diff_versions / ai_kbs_rollback_version` | P0 |
| FR-MCP-06 | SAG 层工具:`ai_kbs_sag_query / ai_kbs_sag_explain` | P0 |
| FR-MCP-07 | Admin 层工具:`ai_kbs_create_space / ai_kbs_manage_binding / ai_kbs_get_audit_log / ai_kbs_manage_api_keys` | P1 |
| FR-MCP-08 | 一键接入:`ai-kbs mcp install --agent {claude-code / codex / cursor / gemini-cli}` 自动写 `.mcp.json` 等 | P0 |
| FR-MCP-09 | 工具 schema frozen v1.0,输入输出类型严格化,标注版本号 | P0 |
| FR-MCP-10 | 限流:单 API key 默认 60 req/min,可配 | P0 |
| FR-MCP-11 | 大文档读取:`read_doc` ≤ 5MB;超大走 `lines + offset` 分页 | P0 |
| FR-MCP-12 | 危险操作支持 `dry_run` 预览模式(`delete_doc / rollback_version`) | P0 |
| FR-MCP-13 | Token / API Key 不出现在工具输入输出 | P0 |

### FR-AUTH:鉴权 + RBAC

| # | 需求 | 优先级 |
|---|------|--------|
| FR-AUTH-01 | HMAC-SHA256 签名(CLI / MCP / API):`AIKBS_KEY_ID + signature(secret, ts, nonce, body)` | P0 |
| FR-AUTH-02 | JWT 验签(Web / API):HS256 / RS256 可切换 | P0 |
| FR-AUTH-03 | Scoped API Key:capability + per-space 限制 + 过期时间 | P0 |
| FR-AUTH-04 | 4 级 RBAC:Owner / Admin / Contributor / Viewer | P0 |
| FR-AUTH-05 | 每用户 × 空间一个角色(workspace_users 表) | P0 |
| FR-AUTH-06 | 跨 workspace 超管(is_super_admin 字段) | P1 |
| FR-AUTH-07 | workspace 邀请 / 移除成员 | P0 |
| FR-AUTH-08 | 操作审计日志(audit_logs):每次写操作记录 actor / action / target / request_id | P0 |
| FR-AUTH-09 | AES-256-GCM 静态加密(API Key / 对象存储凭据) | P0 |
| FR-AUTH-10 | SSRF 防护:URL 导入与数据源同步走 SSRF-safe HTTP client | P0 |
| FR-AUTH-11 | IDOR 防护:所有写操作校验资源所属 workspace + 角色 | P0 |
| FR-AUTH-12 | 凭据脱敏:Token / KeySecret 不出现在日志 / 错误响应 / AI 对话 | P0 |

### FR-AGENT:ReAct Agent + 问答

| # | 需求 | 优先级 |
|---|------|--------|
| FR-AGENT-01 | ReAct Agent 引擎沿用 WeKnora,工具集替换为 ai-kbs 自研 | P0 |
| FR-AGENT-02 | NDJSON 流式输出:`init / thinking / tool_call / tool_result / answer / complete` | P0 |
| FR-AGENT-03 | 答案附引用:每条引用含 `doc_code / title / quote / url` | P0 |
| FR-AGENT-04 | 会话持久化(session_id 跨多轮);支持 `kb_ids` 限定引用空间 | P0 |
| FR-AGENT-05 | LLM Provider 可插拔:OpenAI / Anthropic / DeepSeek / Qwen / Ollama | P0 |
| FR-AGENT-06 | 多 LLM 路由(成本 / 延迟 / 能力维度),v1 默认单 provider,v2 多路由 | P2 |
| FR-AGENT-07 | Agent Skills 沙箱执行(受限容器 / 进程隔离);沙箱执行结果回传 | P1 |

### FR-OBS:可观测性

| # | 需求 | 优先级 |
|---|------|--------|
| FR-OBS-01 | Langfuse 集成(OTLP / OpenTelemetry),覆盖 RAG / Agent / 解析 / MCP / CLI | P0 |
| FR-OBS-02 | 关键指标埋点:QPS / 延迟 / 错误率 / 队列深度 / LLM token 用量 / 缓存命中率 | P0 |
| FR-OBS-03 | 文档解析 trace timeline 可视化(langfuse span tree) | P1 |
| FR-OBS-04 | 管理后台:运行时任务队列 / worker pool / 失败任务 / 手动重试 | P1 |
| FR-OBS-05 | Per-model 并发调控仪表盘 | P1 |

### FR-PARSE:文档解析

| # | 需求 | 优先级 |
|---|------|--------|
| FR-PARSE-01 | docreader gRPC 接口:PDF / Word / Txt / Markdown / HTML(v1);Excel / PPT / EPUB / MHTML / CSV / JSON / Image-OCR(v1.1) | P0(v1 子集)/ P1(v1.1) |
| FR-PARSE-02 | chunking:语义 / 父-子 / 固定三种策略可配 | P0 |
| FR-PARSE-03 | 异步解析 + worker pool + 失败重试 + DLQ | P0 |
| FR-PARSE-04 | 单文件 ≤ 100MB(可配) | P0 |

### FR-WEB:Web UI(WeKnora Vue 3 SPA fork + 白标化)

| # | 需求 | 优先级 |
|---|------|--------|
| FR-WEB-01 | **fork WeKnora v0.7.1 Vue 3 SPA → `frontend/`**,剥离公司专有样式(logo / 品牌色 / 产品名 / SSO / 监控 SDK / 内部 CDN / 配置),统一白标为 "AI-KBS" | P0 |
| FR-WEB-02 | **品牌白标化**:logo / favicon / 品牌色 / 顶部 / 页脚 / OG 分享图全部替换为 AI-KBS 资产 | P0 |
| FR-WEB-03 | **i18n 字符串清洗**:中英双语包,移除公司专有名称 / 功能名 / 联系方式 | P0 |
| FR-WEB-04 | **登录认证**:公司专有 SSO(钉钉 / 飞书 / 企微)→ 标准 OAuth2(Google / GitHub / 邮箱密码)+ JWT 接收 | P0 |
| FR-WEB-05 | **API 客户端路径对齐**:baseURL `/api` → `/api/ai-kbs`;所有 fetch/axios 调用统一前缀 | P0 |
| FR-WEB-06 | **第三方 SDK 替换**:公司内部监控 → 移除 / 改用 Langfuse Web SDK;公司 CDN → jsDelivr / unpkg | P0 |
| FR-WEB-07 | **配置清洗**:`.env.example` / `vite.config.ts` / `package.json` 移除公司专有 flag / 私有 npm 源 | P0 |
| FR-WEB-08 | **Workspace 视图**:空间列表 + 当前用户的角色 + doc_count | P0 |
| FR-WEB-09 | **Document 视图**:目录树 + 文档内容 + outline + 版本历史 + 元数据 | P0 |
| FR-WEB-10 | **Ask 视图**:输入框 + NDJSON 流式渲染 + citations 可点击跳转 + session_id 跨多轮 | P0 |
| FR-WEB-11 | **Spaces 视图**(冒烟测试):列出可用 space 及当前用户角色 | P0 |
| FR-WEB-12 | **Web UI 可视化** CLI push 的文档:`GET /api/ai-kbs/spaces/{code}/tree` + `GET /api/ai-kbs/documents/{docCode}` | P0 |
| FR-WEB-13 | **Web UI 提问走 SAG**:NDJSON 流显示 `engine: "sag"` 时高亮多跳推理路径 | P0 |
| FR-WEB-14 | Playwright E2E:登录 / 浏览 / 提问 / 引用跳转 / 移动端响应式 | P0 |
| FR-WEB-15 | 视觉回归测试:截图比对所有页面,确保白标化无遗漏 | P0 |
| FR-WEB-16 | KG 可视化(D3 / Cytoscape) | P1(v1.1) |
| FR-WEB-17 | 管理后台(队列监控 / 审计日志 / worker pool / 手动重试) | P1(v1.1) |
| FR-WEB-18 | Web UI i18n:仅中文(v2 加英文) | P1(v1.1)/ v2 双语 |

---

## 5. 非功能需求(NFR)

| 类别 | 编号 | 目标 |
|------|------|------|
| 性能 | NFR-PERF-01 | 文档检索(ES,10w 文档)P95 < 200ms |
| 性能 | NFR-PERF-02 | 多跳问答(SAG)P95 < 2s(不含 LLM) |
| 性能 | NFR-PERF-03 | 文档写入(含解析)P95 < 5s(不含 embedding / 索引) |
| 性能 | NFR-PERF-04 | 双向同步(1000 文件)< 60s(8 路并发) |
| 性能 | NFR-PERF-05 | CLI 启动 < 200ms |
| 性能 | NFR-PERF-06 | MCP 工具调用 P95 < 100ms(不含 LLM) |
| 可用性 | NFR-AVAIL-01 | 核心 API 可用性 ≥ 99.5%(自部署) |
| 可用性 | NFR-AVAIL-02 | 单后端实例崩溃后 30s 内自动重启 |
| 可用性 | NFR-AVAIL-03 | 优雅降级:ES 不可用 → PG 全文;SAG 不可用 → RAG-only |
| 安全 | NFR-SEC-01 | 静态加密:AES-256-GCM |
| 安全 | NFR-SEC-02 | 传输加密:TLS 1.2+(外部)/ mTLS(内部 gRPC) |
| 安全 | NFR-SEC-03 | SSRF 防护(URL 导入与数据源) |
| 安全 | NFR-SEC-04 | 凭据脱敏(日志 / 错误响应 / AI 对话) |
| 安全 | NFR-SEC-05 | Agent Skills 沙箱执行(gVisor / Firecracker) |
| 安全 | NFR-SEC-06 | IDOR 防护(中间件统一拦截) |
| 可维护 | NFR-MAINT-01 | 单元测试覆盖率 ≥ 70%(核心模块 ≥ 85%) |
| 可维护 | NFR-MAINT-02 | golangci-lint(Go)/ ruff(Python)/ eslint(前端)统一风格 |
| 可维护 | NFR-MAINT-03 | Mock 后端 + 真后端双模式集成测试 |
| 可维护 | NFR-MAINT-04 | Playwright(Web)+ Bash(CLI)E2E 测试 |
| 可扩展 | NFR-EXT-01 | 检索后端可插拔(pgvector / ES / OpenSearch / Milvus / Weaviate) |
| 可扩展 | NFR-EXT-02 | LLM Provider 可插拔(20+ provider) |
| 可扩展 | NFR-EXT-03 | Embedding 可插拔(BGE / GTE / OpenAI / Ollama) |
| 可扩展 | NFR-EXT-04 | 对象存储可插拔(Local / MinIO / S3 / OSS / TOS / KS3 / OBS) |
| 可扩展 | NFR-EXT-05 | 检索路由可扩展(GraphRAG / ColBERT / HyDE 为预留位) |
| 可观测 | NFR-OBS-01 | 所有外部调用携带 `trace_id + span_id`(W3C traceparent) |
| 可观测 | NFR-OBS-02 | Langfuse dashboard 默认启用 |
| 可观测 | NFR-OBS-03 | 关键指标埋点(QPS / 延迟 / 错误率 / 队列深度 / token 用量) |
| 国际化 | NFR-I18N-01 | 默认中文 + 英文双语 README / SKILL / 错误消息 |
| 国际化 | NFR-I18N-02 | CLI 错误消息中英混排可读 |
| 国际化 | NFR-I18N-03 | Web UI v1 仅中文;v1.1 起加英文双语(Option B 已锁定 Web UI 必交付) |
| 兼容性 | NFR-COMPAT-01 | CLI 跨 OS(Linux / macOS / Windows)× 架构(amd64 / arm64) |
| 兼容性 | NFR-COMPAT-02 | 单二进制静态编译(CGO_ENABLED=0) |
| 兼容性 | NFR-COMPAT-03 | API frozen schema v1.0,字段 `version` 标注版本号 |
| 前端性能 | NFR-FE-01 | Web UI 首屏加载 < 3s(冷启动,无缓存) |
| 前端性能 | NFR-FE-02 | 文档浏览 P95 < 500ms(API + 渲染) |
| 前端性能 | NFR-FE-03 | 提问流式首个 token < 1s |
| 前端安全 | NFR-FE-04 | XSS 防护:Vue 模板默认转义 + DOMPurify 清洗 markdown 渲染 |
| 前端兼容 | NFR-FE-05 | 浏览器支持:Chrome 100+ / Firefox 100+ / Safari 15+ / Edge 100+ |
| 前端兼容 | NFR-FE-06 | 响应式:≥ 768px 桌面 + ≥ 375px 移动浏览器 |
| 合规 | NFR-COMP-01 | 凭据不留痕 |
| 合规 | NFR-COMP-02 | 操作审计日志(每写操作) |
| 合规 | NFR-COMP-03 | 数据导出 / 导入(OCTX 格式,v2) |

---

## 6. 验收标准(Acceptance Criteria)

> 验收以 Gherkin 风格描述;完整 BDD 场景集见 `require/08-bdd-specs.md`(v1 内覆盖 ≥ 48 个场景)。

### 6.1 功能验收(v1 GA)

```gherkin
Feature: AI Agent 通过 MCP 一键接入

  Scenario: Claude Code 接入 ai-kbs
    Given 用户执行 "ai-kbs mcp install --agent claude-code"
    When 系统写入 ".mcp.json" 并重启 Claude Code
    Then Claude Code 可调用 ai_kbs_search / ai_kbs_read_doc / ai_kbs_ask / ai_kbs_write_doc / ai_kbs_edit_doc
    And 调用响应符合 MCP 协议 + frozen schema v1.0

Feature: 多跳问答路由到 SAG

  Scenario: 多跳问题自动走 SAG 引擎
    Given 用户提问 "A 公司收购 B 公司后,C 产品线整合情况如何?"
    When ai-kbs ask 接收问题
    Then query classifier 判定为 "multi_hop"
    And 检索走 SAG 引擎,响应中 engine="sag"
    And 答案附引用,每条含 doc_code / title / quote

Feature: CLI 双向同步

  Scenario: pull 1000 个文档
    Given 用户在绑定目录下执行 "ai-kbs pull"
    When 系统并发 8 路拉取
    Then 1000 文件 < 60s 完成
    And sync-state.json 写入本地映射
    And 失败聚合到 sync_results[].errors[],整体 exit 23

Feature: CAS 乐观锁冲突解决

  Scenario: write --base-version 冲突
    Given 文档版本 v=7
    When 用户提交 "ai-kbs write --base-version 5" (v=7)
    Then 系统返回 22 VersionConflict
    And next_action.strategies = ["keep-mine", "keep-theirs", "both", "auto"]
    And 携带 req-uuid 便于审计

Feature: HMAC 鉴权链

  Scenario: CLI 优先使用 env HMAC 凭证
    Given env 注入 AIKBS_KEY_ID / AIKBS_KEY_SECRET
    When 用户执行任意写命令
    Then 系统使用 env 凭证(优先级高于 ~/.ai-kbs/credentials)
    And 请求头携带 X-AIKBS-KeyId / X-AIKBS-Timestamp / X-AIKBS-Nonce / X-AIKBS-Signature
```

### 6.2 性能验收(NFR 对应)

| ID | 验收命令 | 目标 |
|----|---------|------|
| NFR-PERF-01 | `wrk -t4 -c50 -d30s http://localhost:8080/api/ai-kbs/search` | P95 < 200ms |
| NFR-PERF-04 | `time ai-kbs pull` in 1000-file workspace | < 60s |
| NFR-PERF-05 | `time ai-kbs --help` | < 200ms |
| NFR-PERF-06 | MCP tool call 直连压测 | P95 < 100ms |

### 6.3 兼容性验收

- [ ] 在 Linux/macOS/Windows × amd64/arm64 全 6 个目标平台上,`ai-kbs --version` 输出一致
- [ ] `api_kbs_search` MCP tool 的输入 schema 与 frozen v1.0 字节级一致(JSON Schema 校验)
- [ ] frozen schema v1.0 在 release notes 中明示,字段集合不再变更

### 6.4 集成验收

- [ ] Claude Code / Codex / Cursor 三种 Agent 通过 `mcp install` 接入并完成读 / 写 / 编辑端到端测试
- [ ] 3 种 LLM provider(OpenAI / DeepSeek / Ollama)+ 2 种 Embedding(OpenAI / BGE)集成测试通过
- [ ] Docker Compose 单机部署 ≤ 10 分钟启动完成
- [ ] Helm Chart 在 K8s 测试集群部署成功
- [ ] **Web UI**:用户在浏览器登录后,看到 CLI push 的文档(同源数据);点击文档显示 markdown + 版本历史;提问走 SAG 显示多跳引用
- [ ] **Playwright E2E**:登录 / 浏览 / 提问 / 引用跳转 4 个核心场景全过;视觉回归截图比对 100% 命中

### 6.5 准确率验收

- [ ] 在 2WikiMultiHopQA 子集(100 题)上 Recall@5 ≥ WeKnora 基线 +5pp
- [ ] 在 MuSiQue 子集(100 题)上 Recall@5 ≥ WeKnora 基线 +5pp
- [ ] A/B 评估接口可对比 ES-only / SAG-only / Hybrid 三种路由的准确率与延迟

---

## 7. 里程碑(参考 `require/10-roadmap.md`)

| Milestone | 日期(目标) | 验收 | v1 GA 范围 |
|-----------|-----------|------|-----------|
| **M0**:仓库 + CI + 开发环境 | Week 2 末 | `docker compose up` 一键启动(含 frontend 容器);CI 绿 | ✅ |
| **M1**:后端核心 + CLI 基础 + Frontend 脚手架 | Week 6 末 | 文档 CRUD + 版本 + 鉴权 + RBAC 全通;CLI 8 路并发同步;Frontend fork 初始化 + 白标化骨架就绪 | ✅ |
| **M2**:检索 + Agent + MCP + Frontend 视图 | Week 8 末 | ES + SAG 双引擎;Claude Code 通过 MCP 调用 `ai_kbs_ask` 成功;Web UI 5 个核心视图全可点 | ✅ |
| **M3**:**v1.0.0 GA** | **Week 10-11 末** | 48 个 BDD 场景全过;性能基线达标;中英双语文档 + SKILL.md 齐备;Helm chart 可部署;Playwright E2E + 视觉回归全过 | ✅ |
| **M4**:v1.1.0 增强 | Week 14 末 | KG 可视化 + Web 管理后台 + Feishu 同步 + 多格式解析 + 迁移工具 | ⏸ v1.1 |
| **M5**:v2.0.0 生态 | Week 24 末 | 多 IM + 多数据源 + 嵌入 widget | ⏸ v2 |

> 注:本 PRD 聚焦 v1.0.0 GA(M3,Week 10-11 末)。v1.1 / v2 在本文档 §2.2 中已划入 Out-of-Scope。Frontend 改造从 M1 中段(M1.5)开始介入,等后端核心 API 稳定后再 fork 改造。

---

## 8. 风险与开放问题

### 8.1 风险登记

| 风险 | 等级 | 缓解 |
|------|------|------|
| WeKnora 上游 breaking change | 高 | pin 版本(v0.7.1);fork 长期维护;监控 release notes;关键改造点单独建仓 |
| SAG 论文复现误差 | 中 | 用官方 CLI 验证;与论文社区保持联系;提供 A/B 评估接口 |
| 中文分词(IK)效果不佳 | 中 | 备选 HanLP / Jieba;用户可配 |
| 多 LLM provider 兼容 | 中 | 沿用 WeKnora 抽象;每 provider 独立测试 |
| 单机部署性能瓶颈 | 低 | v1 定位中小团队;v2 分布式 SAG |
| [内部 Wiki 系统] 平台专有命令 / 配置 / 命名泄漏到公开仓库 | 高 | 严格脱敏审计 + PR 检查脚本(模式匹配 [内部 Wiki 系统] 平台专有字符串前缀);CI 阻断 |
| AI Agent 学习成本(SKILL.md 必须随二进制分发) | 中 | SKILL.md + references/{auth,workspace,sync,conflict,version,mcp}.md 全部随二进制 release artifact 分发 |

### 8.2 开放问题(等用户决策)

> ADR-006/007/008/009 已在 2026-10-02 由用户锁定。下列 Q4-Q8 为**默认决策**,用户在最终审查时可一并确认或推翻。

| # | 问题 | 选项 | 当前默认 |
|---|------|------|---------|
| Q4 | **SAG 默认可选后端** | A. LanceDB(默认);B. SQLite | A |
| Q5 | **embedding 模型默认值** | A. BGE-large-zh-v1.5;B. multilingual-e5-large | A |
| Q6 | **LLM 默认 provider** | A. DeepSeek(成本友好);B. OpenAI(gpt-4o-mini) | A |
| Q7 | **是否支持 PG-only 部署(无 ES)** | A. 否(必须 ES);B. 是(用户自由选择) | A |
| Q8 | **MCP server 进程边界** | A. 独立 Python 进程;B. Go 内嵌(沿用 WeKnora) | A |

---

## 9. 附录 A:概念参考来源映射

> **本附录仅供内部设计追溯,不暴露任何内部项目名称到公开仓库。**

### 9.1 工程概念映射

| ai-kbs 采用概念 | 通用含义 | 来源附录 |
|----------------|---------|----------|
| Workspace | 本地工作目录 + 绑定配置 + sync-state | 工程惯例(Git workspace / IDE workspace) |
| Space | 远端的命名空间(KB 等价物) | 通用术语 |
| docCode | 8 位 base62 稳定标识 | 工程模式(Snowflake / ULID 的精简版) |
| CAS 乐观锁 | base_version + 冲突时 next_action | 通用数据库模式 |
| 版本链 | 每次写入新增 version,中间版本保留 | Git commit chain 的简化版 |
| workspace/bind 同步 | 本地目录 ↔ 远端 双向同步 | 工程惯例(VSCode workspace / Git submodule) |
| rollback = revert 语义 | 不删中间版本 | Git revert 的工程惯例 |
| RAG / Hybrid | 检索增强生成 | 通用 AI 术语 |
| Event / Entity / Hyperedge | SAG 论文数据结构 | 参考论文 *"SAG: Structure-Aware Graph for Multi-hop Reasoning"* |
| KG / Knowledge Graph | 文档中提取的实体 - 关系图 | 通用 AI 术语 |

### 9.2 工程模式来源说明

> ai-kbs 借鉴了**已发表论文 / 公开开源项目 / 通用工程惯例**中的成熟设计模式。本附录为内部追溯使用,不对外公开仓库暴露。

**主要借鉴来源**:
- WeKnora v0.7.1(Apache 2.0 开源):作为后端基座,沿用其 RAG / ReAct Agent / Wiki Mode / 知识图谱 / RBAC / MCP / Langfuse 设计
- SAG 论文(*"SAG: Structure-Aware Graph for Multi-hop Reasoning"*):作为多跳检索增强层
- Git / Snowflake / ULID / VSCode workspace 等通用工程模式:作为本地工作空间与同步模型的设计参考

**借鉴方式**:仅借鉴**概念模型与设计模式**,不复用任何项目特有的命名 / 命令 / 配置 / 代码段。所有 ai-kbs 的命名、命令集合、配置 schema 均为本项目独立设计。

### 9.3 命名独立设计原则

- 所有 CLI 命令名(`init / bind / pull / push / read / write / ...`)虽与通用工程惯例(Git / IDE)有重合,但 ai-kbs 的**flag 命名、参数结构、错误响应 schema、子命令嵌套关系**均为独立设计
- 所有数据模型字段(`docCode / baseVersion / treePath / ...`)虽与通用工程模式有重合,但 ai-kbs 的**表结构、索引策略、状态机**均为独立设计
- 所有错误码命名空间(`auth.* / rbac.* / doc.* / ...`)为 ai-kbs 独立命名

---

**PRD 已完成范围锁定与需求收敛,Q1-Q3 已并入 ADR-006/007/008。等待用户对整体规划总结做最终批准。**
