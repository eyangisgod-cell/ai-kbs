# 03 · 需求清单与 v1 范围

## 1. 功能需求（Functional Requirements）

> 编号 FR-xxx。每条均含：用户故事、验收标准、优先级、对应来源。

### FR-001：本地工作空间管理
**用户故事**：作为开发者，我希望在一个本地目录中初始化工作空间，绑定远端 Space 的某个路径，然后双向同步文档。

**验收标准**：
- `ai-kbs init [path]` 在目标目录创建 `.ai-kbs/workspace.yaml`
- `ai-kbs bind <local> <space:path>` 建立绑定并写入 `workspace.yaml`
- `ai-kbs unbind <local>` 移除绑定，保留本地文件
- `ai-kbs status` 显示服务端登录态、工作空间、绑定清单、当前目录命中的绑定
- 工作空间配置向后兼容旧的 `.wiki/wiki.conf`（读取支持，写出仅用新格式）

**优先级**：P0（v1 必需）

**来源**：losu-wiki `wiki init/bind/unbind/status`

---

### FR-002：文档双向同步
**用户故事**：作为开发者，我希望在本地编辑文档后能安全地推送到远端，也能从远端拉取更新。

**验收标准**：
- `ai-kbs pull` 拉取当前目录命中绑定；支持 `--recursive`、`--prune`、`--dry-run`、`--exclude`（gitignore 语法）、`--binary=auto|skip|include`
- `ai-kbs push` 推送当前目录命中绑定；支持 `--recursive`、`--prune`、`--dry-run`、`--strategy auto|manual|keep-mine|keep-theirs|both`、`--exclude`、`--include`
- 8 路并发读写；删除护栏：>10 文件或 >5MB 单文件需 `--yes`
- `sync-state.json` 记录每个文件的 `{md5, version, docCode, remotePath}`；远端 rename 通过 docCode 识别
- Detached 文件不被 `--prune` 删除（避免误删已同步文件）
- 单文件失败聚合到 `sync_results[].errors[]`，整体 exit 23

**优先级**：P0

**来源**：losu-wiki `wiki pull/push`

---

### FR-003：单文档 CRUD
**用户故事**：作为开发者，我希望用 CLI 单条读写文档，无需双向同步。

**验收标准**：
- `ai-kbs read --url/--space+--doc/--space+--path` 读取文档
- 支持 `--content`、`--outline`、`--summary`、`--meta`、`--section`、`--lines`、`--offset`、`--version`
- `ai-kbs read --paths` 批量读取（8 路并发）
- `ai-kbs write` 创建/覆盖（CAS 乐观锁：必传 `--base-version`）
- `ai-kbs edit` 局部替换（CAS，必传 `--base-version`，支持 `--dry-run`、`--replace-all`）
- 所有写操作冲突时携带 `next_action` 修复建议

**优先级**：P0

**来源**：losu-wiki `wiki read/write/edit`

---

### FR-004：版本管理
**用户故事**：作为开发者，我希望查看文档历史版本、对比差异、回滚到旧版本。

**验收标准**：
- `ai-kbs version list --space --doc` 列出所有版本（透传后端版本项）
- `ai-kbs version diff --space --doc --from --to` 输出 unified diff + `{additions, deletions, changes}` 统计
- `ai-kbs version rollback --space --doc --to --message` 回滚（**git revert 语义**：产生新版本，中间版本全部保留）
- `version_content_unavailable` → exit 20；`rollback_target_lost` → exit 20

**优先级**：P0

**来源**：losu-wiki `wiki version list/diff/rollback`

---

### FR-005：移动 / 重命名 / 反删除
**用户故事**：作为开发者，我希望移动/重命名文档时链接不失效；能恢复误删的文档。

**验收标准**：
- `ai-kbs move --space --doc --parent/--title` 改父级或标题，**docCode 不变**
- `ai-kbs undelete --space --doc/--path` 恢复软删除（**不产生新版本**）
- 已存活文档调 undelete → 友好返回（exit 0）

**优先级**：P1

**来源**：losu-wiki `wiki move/undelete`

---

### FR-006：检索与浏览
**用户故事**：作为开发者，我希望能用关键词、正则、目录树浏览文档。

**验收标准**：
- `ai-kbs search <keyword>` 关键词搜索，支持 `--space`、`--path`、`--tags`、`--limit`、`--page`
- `ai-kbs grep <pattern> --space` 正则搜索，支持 `-n`、`-i`、`-C`、`--limit`
- `ai-kbs tree --space [--path] [--depth]` 目录树，支持分页游标
- `ai-kbs spaces` 列出可用空间（冒烟测试命令）
- 无结果时 exit 0（不报错）

**优先级**：P0

**来源**：losu-wiki `wiki search/grep/tree/spaces`

---

### FR-007：冲突解决
**用户故事**：作为开发者，我希望本地与远端版本冲突时能选择保留哪一方。

**验收标准**：
- `ai-kbs resolve --space --doc --base-version --strategy` 单文档解决
- `ai-kbs resolve --batch --space --strategy` 批量解决空间内所有冲突
- 支持策略：`manual`（默认，写 `.THEIRS.md` 并排）/ `keep-mine` / `keep-theirs` / `both` / `auto`（push 时三路分层）
- 残留冲突 → exit 24 + `next_action.strategies` 列表

**优先级**：P1

**来源**：losu-wiki `wiki resolve`

---

### FR-008：鉴权与凭证管理
**用户故事**：作为开发者/AI Agent，我希望通过多种方式安全地认证到 ai-kbs。

**验收标准**：
- `ai-kbs login` 通过浏览器授权或手动粘贴 KeyID/KeySecret
- 凭证优先级链：`AIKBS_KEY_ID+AIKBS_KEY_SECRET`（HMAC）> `~/.ai-kbs/credentials`（HMAC 或 DirectToken）> `AIKBS_TOKEN`（JWT/Token，仅直连后端）
- `ai-kbs logout [--local-only]` 退出
- `ai-kbs login status` 显示当前登录用户、spaces、过期时间
- Token / KeySecret 永不打印到 stdout / 日志 / AI 对话
- AES-256-GCM 加密存储 API Key

**优先级**：P0

**来源**：losu-wiki `wiki login/logout` + WeKnora scoped API Key

---

### FR-009：AI Agent MCP 接口
**用户故事**：作为 AI Agent（Claude Code / Codex / Cursor），我希望通过 MCP 协议调用 ai-kbs 的核心能力。

**验收标准**：
- MCP server 多传输支持：stdio / SSE / HTTP
- MCP tools 清单：
  - `ai_kbs_search`：跨 space 搜索
  - `ai_kbs_read_doc`：读取文档（含 outline / section）
  - `ai_kbs_ask`：自然语言提问（触发 ReAct Agent）
  - `ai_kbs_write_doc`：创建/覆盖文档
  - `ai_kbs_edit_doc`：局部替换
  - `ai_kbs_version_list/diff/rollback`：版本管理
  - `ai_kbs_list_spaces`：列出空间
  - `ai_kbs_sag_query`：调用 SAG 推理（多跳问答）
- Tool 输入/输出遵循 MCP 标准 + frozen schema v1.0
- OAuth2 支持（远程 MCP）
- 一键接入：`ai-kbs mcp install --agent claude-code` 自动写 `.mcp.json`

**优先级**：P0

**来源**：WeKnora MCP server + SAG MCP 集成经验

---

### FR-010：SKILL 操作手册
**用户故事**：作为 AI Agent，我希望阅读结构化的 SKILL.md 手册，知道如何安全地操作 ai-kbs。

**验收标准**：
- `cli/skills/SKILL.md` + `cli/skills/references/*.md` 提供完整操作手册
- HARD RULES 包括：
  - 所有命令必带 `--json`
  - edit/write 必先 read 拿 base-version
  - 冲突重试 ≤2 次
  - Token / KeySecret 不进 AI 对话
  - `--prune` 不可逆，需 `--yes`
- 意图 → 命令决策矩阵（与 losu-wiki 一致）
- 典型工作流 A-G：A 读取 / B 编辑 / C 创建 / D 覆盖 / E 绑定同步 / F 回滚 / G 恢复

**优先级**：P0

**来源**：losu-wiki SKILL 模式

---

### FR-011：自然语言问答（Agent）
**用户故事**：作为业务人员，我希望用自然语言提问，得到基于文档的准确答案与引用。

**验收标准**：
- Web UI / CLI `ai-kbs ask <question>` 提供对话式问答
- 简单查询走 RAG（ES + pgvector hybrid search）
- 多跳查询自动路由到 SAG（事件-实体推理）
- 答案附引用（点击跳转到原文档原文）
- 会话持久化（session_id 跨多轮）
- 支持引用 KB（用户可指定某个或多个 space）
- Langfuse 追踪完整推理链

**优先级**：P0

**来源**：WeKnora ReAct Agent + SAG 多跳推理

---

### FR-012：双引擎检索路由
**用户故事**：作为系统，我需要根据查询特征智能选择 ES 或 SAG。

**验收标准**：
- 查询分类器（轻量 LLM 调用 + 规则）：
  - 关键词类、单跳 → ES
  - 多跳、实体类、关系类 → SAG
  - 不确定 → 并行召回 + 重排融合
- 双引擎超时独立（ES 3s / SAG 5s），互不影响
- 响应中标注 `engine` 字段（`es` / `sag` / `hybrid`）
- 提供 A/B 评估接口（管理员后台）

**优先级**：P0

**来源**：本项目创新点（基于 ADR-002）

---

### FR-013：文档多格式解析
**用户故事**：作为用户，我希望上传 PDF/Word/Excel/PPT/图片 后能直接问答。

**验收标准**：
- 支持格式：PDF / Word / Txt / Markdown / HTML / EPUB / MHTML / Images(OCR) / CSV / Excel / PPT / JSON
- 解析走 WeKnora docreader（gRPC）
- 异步解析任务（MQ 异步 + worker pool）
- 解析进度可见（langfuse-style timeline）
- 失败重试 + DLQ
- 单文件 ≤ 100MB（可配）

**优先级**：P0

**来源**：WeKnora docreader

---

### FR-014：多租户与 RBAC
**用户故事**：作为管理员，我希望给不同用户分配不同空间的不同角色。

**验收标准**：
- 4 级角色矩阵：Owner / Admin / Contributor / Viewer
- 每用户 × 空间一个角色
- Per-KB 所有权（每个 KB 有 owner 字段）
- 跨 workspace 超管（可选）
- 工作空间成员管理（邀请 / 移除）
- 工作空间审计日志（每次写操作记录）
- self-service workspace 创建（gated / tenantless 两种模式）

**优先级**：P0

**来源**：WeKnora RBAC

---

### FR-015：可观测性
**用户故事**：作为运维/开发，我希望看到完整的请求链路与系统状态。

**验收标准**：
- 集成 Langfuse（OTLP/OTel + W3C traceparent 传播）
- 覆盖：RAG 检索 / Agent ReAct / 文档解析 / MCP 调用 / CLI 调用
- 系统管理后台：运行时任务队列监控（队列深度、worker pool、失败任务、手动重试）
- Per-model 并发调控仪表盘
- 文档解析 trace timeline（langfuse-style span tree）
- Prompt 缓存命中率观测

**优先级**：P1

**来源**：WeKnora observability

---

### FR-016：CLI 输出 AI-Friendly
**用户故事**：作为 AI Agent，我希望 CLI 输出严格遵循 frozen JSON schema，便于程序解析。

**验收标准**：
- 所有命令默认 `--json` 输出（除非显式 `--format text`）
- 成功 envelope：`{json_schema_version, success, exit_code, data, next_action, request_id}`
- 错误 envelope：`{json_schema_version, success, exit_code, error:{code, message}, data:null, next_action, request_id}`
- 退出码语义稳定（同 02-glossary.md 退出码表）
- `next_action` 字段包含 `{cmd, args, reason, strategies?, conflict_refs?}`，AI 可直接 shell-execute
- 错误分支 `data` 恒为 `null`，避免 Agent 误解析
- NDJSON 流（chat/ask）每行一个事件 `{type, ...}`

**优先级**：P0

**来源**：losu-wiki frozen schema v1.0 + WeKnora wire contract

---

### FR-017：从 losu-wiki 迁移
**用户故事**：作为 losu-wiki 老用户，我希望把我的工作空间和数据迁移到 ai-kbs。

**验收标准**：
- `ai-kbs migrate from-losu-wiki <workspace-path>` 一键迁移
- 读取 `.wiki/wiki.conf` 和 `.wiki/sync-state.json`
- docCode 直接保留
- 文档内容走远端复制（API）
- 同步状态写入新格式 `.ai-kbs/sync-state.json`
- 生成迁移报告（成功数 / 失败数 / 跳过数）

**优先级**：P2（v1 末期）

**来源**：本项目特有需求

---

## 2. 非功能需求（NFR）

### NFR-001：性能

| 项 | 目标 |
|----|------|
| 文档检索（ES，10w 文档） | P95 < 200ms |
| 多跳问答（SAG） | P95 < 2s |
| 文档写入（含解析） | P95 < 5s（不含 embedding/索引） |
| 双向同步（1000 文件） | < 60s（8 路并发） |
| CLI 启动 | < 200ms |
| MCP 工具调用 | P95 < 100ms（不含 LLM） |

### NFR-002：可用性

- 核心 API 可用性 ≥ 99.5%（自部署场景）
- 故障恢复：单后端实例崩溃后 30s 内自动重启
- 优雅降级：ES 不可用时降级到 PostgreSQL 全文搜索；SAG 不可用时降级到 RAG-only

### NFR-003：安全

- 静态加密：AES-256-GCM（API Key、对象存储凭据）
- 传输加密：TLS 1.2+（外部）/ mTLS（内部 gRPC）
- SSRF 防护：所有 URL 导入与数据源同步走 SSRF-safe HTTP client
- 凭据脱敏：API Key / Token 不出现在日志与错误响应中
- 沙箱执行：Agent Skills 在受限容器内执行
- IDOR 防护：所有写操作校验资源所属 workspace + 角色
- RBAC 矩阵在中间件统一拦截，不依赖业务代码自觉

### NFR-004：可维护性

- 代码风格统一（Go: golangci-lint；Python: ruff）
- 单元测试覆盖率 ≥ 70%（核心模块 ≥ 85%）
- 集成测试：Mock 后端 + 真后端双模式
- E2E 测试：Playwright（Web）+ Bash（CLI）
- 每个 internal 子包独立可测

### NFR-005：可扩展性

- 检索后端可插拔（pgvector / ES / OpenSearch / Milvus / Weaviate）
- LLM Provider 可插拔（20+ provider）
- Embedding 可插拔（BGE / GTE / OpenAI / Ollama / Zhipu）
- 对象存储可插拔（Local / MinIO / S3 / OSS / TOS / KS3 / OBS）
- 检索路由可扩展（未来加 GraphRAG / ColBERT / HyDE）

### NFR-006：可观测

- 所有外部调用携带 `trace_id` + `span_id`
- Langfuse dashboard 默认启用
- 关键指标埋点：QPS / 延迟 / 错误率 / 队列深度 / LLM token 用量
- 文档解析 trace timeline 可视化

### NFR-007：国际化

- 默认中文 + 英文双语 README / SKILL / 错误消息
- CLI 错误消息中英混排可读
- Web UI 支持中英切换（v1 仅中文，v2 加英文）

### NFR-008：兼容性

- CLI 兼容主流操作系统（Linux / macOS / Windows）+ 主流架构（amd64 / arm64）
- 单二进制静态编译（CGO_ENABLED=0），无运行时依赖
- API 向后兼容：frozen schema v1.0 + version field

### NFR-009：合规

- 凭据不留痕（不在日志、不在响应）
- 操作审计日志（每写操作记录 actor / action / target / timestamp）
- 数据导出/导入支持（v2，遵循 OCTX 格式参考 SAG）

## 3. v1 范围界定（In-Scope vs Out-of-Scope）

### v1 必交付（P0）

| 模块 | 包含 |
|------|------|
| 后端核心 | 文档 CRUD / 版本 / 绑定 / 同步 / 检索 / Agent / MCP |
| 检索 | ES + SAG 双引擎并存路由 |
| 解析 | PDF / Word / Markdown / HTML（其他格式 v1.1） |
| CLI | 完整命令集 + SKILL.md |
| Web | 基础文档浏览 + 问答界面 + 空间管理 |
| 鉴权 | HMAC + JWT + scoped API Key |
| RBAC | 4 级 + per-KB 所有权 |
| 可观测 | Langfuse 集成 |
| 部署 | Docker Compose（开发） + Helm（生产基础） |

### v1.1 增量（P1）

| 模块 | 包含 |
|------|------|
| 解析 | Excel / PPT / 图片 OCR / EPUB / MHTML |
| 数据源 | Feishu 同步（v1.1 选其一） |
| IM | Webhook 通用接口（不接入具体 IM） |
| 浏览器扩展 | Chrome Extension v0.1 |
| 移动端 | 仅响应式 Web |

### v2 计划（P2）

| 模块 | 包含 |
|------|------|
| 多 IM 集成 | WeCom / Slack / Telegram / DingTalk |
| 多数据源 | Notion / Yuque / RSS / GitHub |
| 网站嵌入 | Embed widget + secure-mode token exchange |
| 移动 App | WeChat Mini Program |
| WeKnora Cloud | 托管 LLM + 托管解析 |

## 4. 验收标准（v1 整体）

- [ ] 100 个测试文档的 KB 上，能正确完成 CRUD / 版本 / 绑定 / 同步
- [ ] 1000 文件的双向同步 < 60s
- [ ] Claude Code 通过 MCP 调用 `ai_kbs_read_doc` / `ai_kbs_ask` / `ai_kbs_write_doc` 均能成功
- [ ] SKILL.md 中所有典型工作流（A-G）端到端跑通
- [ ] 在 HotpotQA / 2WikiMultiHopQA 子集（100 题）上问答准确率 ≥ WeKnora 基线 +5pp
- [ ] 单元测试覆盖率 ≥ 70%
- [ ] 至少 3 种 LLM provider（OpenAI / DeepSeek / Ollama）+ 2 种 Embedding（OpenAI / BGE）集成测试通过
- [ ] Docker Compose 单机部署 < 10 分钟启动完成
- [ ] Helm Chart 在 K8s 测试集群部署成功
