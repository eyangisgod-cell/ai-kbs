# 00 · 上下文与决策记录

## 1. 背景

### 1.1 现状

用户所在团队有一个**公司内部**的 Wiki 知识库系统（代号 `losu-ai-wiki`）：
- 三件套：CLI（Go）+ System 后端（Java Spring Boot）+ Web 前端（Vue）
- 完整支持文档生命周期、版本管理、批量同步、HMAC 鉴权、AI-friendly CLI
- 但**仅限内网使用**，无法作为开源项目二次分发

团队需要一个**对外开源**的同类项目，以满足：
1. 外部客户交付（无内网依赖）
2. 借鉴 losu-wiki 的成熟流程（CLI 批量同步、版本控制、SKILL 操作手册）
3. 集成业界更先进的 RAG + 知识图谱能力（不局限于经典 RAG）

### 1.2 候选参考项目

| 项目 | 角色 | 复用方式 |
|------|------|----------|
| **WeKnora v0.7.1** (`E:/my-project/WeKnora-main`) | **主基座** | 直接二次开发，剥离公司特有逻辑 |
| **SAG** (`E:/my-project/SAG`) | **检索增强层** | 集成其事件-实体-动态超边检索能力 |
| **losu-ai-wiki** (`E:/my-project/losu-ai-wiki`) | **流程模板** | 借鉴 CLI 设计 / 文档寻址 / 版本模型 / SKILL 手册范式 |

### 1.3 核心痛点

| 痛点 | 现有方案不足 | ai-kbs 的应对 |
|------|------------|--------------|
| AI Agent 无法直接获取项目需求/规范 | losu-wiki 仅做文档管理，无 Agent 调用面 | 提供 MCP server + SKILL.md，让 Claude Code 直接查询与写入 |
| 多跳问答准确率低 | 传统 RAG 在 2WikiMultiHopQA / MuSiQue 上召回率不足 | 集成 SAG（论文 SOTA）作为结构化推理层 |
| 文档与代码版本割裂 | losu-wiki 用乐观锁做冲突检测，但缺乏与代码仓库的关联 | 借鉴 losu-wiki 的 CAS 同步体系，新增 `docCode` 稳定标识 |
| CLI 输出不可被 Agent 解析 | 多数 CLI 输出散文文本 | 借鉴 losu-wiki 的 `--json` envelope + `next_action` 模式 |
| 检索引擎单点 | losu-wiki 单选 MySQL 或 ES | ES（粗排）+ SAG（结构化推理）双引擎并存 |

## 2. 问题陈述（Problem Statement）

> 能否构建一个开源的企业级 AI Agent 知识库系统，让 Claude Code 等 AI 工具通过 CLI + MCP 直接参与需求获取、规范查询、文档协作与版本管理，并在多跳业务问答与结构化推理任务上达到业界领先准确率？

## 3. 约束与前置条件

### 3.1 技术约束

| 类别 | 约束 |
|------|------|
| 后端语言 | 以 Go 为主（沿用 WeKnora 栈）；Python 用于 docreader 与 SAG 包装层 |
| LLM | OpenAI 兼容协议（支持 OpenAI / Anthropic / DeepSeek / Qwen / Ollama 等） |
| Embedding | BGE / GTE / OpenAI text-embedding / Ollama |
| 数据库 | PostgreSQL（主存，含 pgvector）+ Elasticsearch（检索副本）+ LanceDB / SQLite（SAG 本地副本） |
| 对象存储 | 本地 / MinIO / S3 / OSS（多实例，与 WeKnora 一致） |
| 部署 | Docker Compose（开发）+ Helm（生产） |
| 鉴权 | HMAC-SHA256（CLI 自动化）+ JWT（Web/API）+ scoped API Key（嵌入场景） |

### 3.2 业务约束

| 类别 | 约束 |
|------|------|
| 兼容性 | 必须支持 losu-wiki 已有的 CLI 工作流概念（init/bind/pull/push/version/diff/rollback），降低迁移成本 |
| 多租户 | 必须支持 workspace 隔离 + 4 级 RBAC（Owner / Admin / Contributor / Viewer） |
| 可观测 | 集成 Langfuse（替代 Jaeger），覆盖 RAG 流水线 + Agent ReAct + 文档解析 |
| 安全 | AES-256-GCM 静态加密 + SSRF 防护 + 凭据脱敏 + 沙箱执行（Agent Skills） |
| 国际化 | 默认中文 + 英文双语 README，CLI 支持中英混排输出 |

### 3.3 非目标（v1 明确不做）

| 项 | 不做理由 |
|----|---------|
| 飞书/Notion/语雀 数据源同步 | v1 聚焦核心 Wiki + 问答；数据源接入 v2 再做 |
| IM 通道（企微/飞书/Slack/Telegram） | 同上；架构预留接口但不交付 |
| Chrome Extension / 网站嵌入 | v2 计划 |
| 自研 Embedding / 自训模型 | 复用开源模型 |
| 多模态图像问答 | v1 仅文档问答；图像处理仅做 OCR 抽取进文本 |
| 移动端 Mini Program | v2 计划 |

## 4. 关键决策记录（ADR）

### ADR-001：基座选择 WeKnora 而非自研

**状态**：✅ Accepted

**背景**：用户希望"重新开发一套全新的 AI agent 知识库开源项目"。

**决策**：基于 WeKnora v0.7.1 二次开发，剥离公司 SKU 化逻辑。

**理由**：
1. WeKnora 已具备完整 RAG + ReAct Agent + Wiki Mode + 知识图谱 + RBAC + CLI + MCP + Langfuse，开源最完整
2. 自研将导致 80% 工作量与 WeKnora 重叠
3. WeKnora 模块化程度高（每个 internal 子模块都可独立替换）

**风险**：
- WeKnora 内部某些模块可能与公司耦合（如 auth-sdk）；需仔细剥离
- WeKnora 升级节奏可能影响本项目跟进

**对策**：
- 通过 fork + 模块替换，而非完整 vendor
- 关键改造点单独建仓（`ai-kbs/internal/agent` 包装 SAG）

### ADR-002：检索层引入 SAG 而非自研 GraphRAG

**状态**：✅ Accepted

**背景**：传统 GraphRAG 构建成本高、增量更新困难。

**决策**：集成 SAG（事件-实体-查询时动态超边）作为结构化推理层。

**理由**：
1. SAG 在 HotpotQA / 2WikiMultiHopQA / MuSiQue 上 Recall@5 / F1 全面 SOTA（论文数据 +6.79/+4.33 pp）
2. SAG 增量更新友好：新 chunk 直接添加事件和实体，无需重建全局图
3. SAG 独立于 RAG 服务运行，避免双系统维护

**风险**：
- SAG 是新架构，社区生态不如 GraphRAG 成熟
- SAG 的动态超边在极大数据集上的性能未充分验证

**对策**：
- 与 ES 检索并存（双引擎），按 query 类型路由
- 文档量 > 100k 时触发分片与冷热分层

### ADR-003：CLI 借鉴 losu-wiki 而非复用其代码

**状态**：✅ Accepted

**背景**：losu-wiki CLI 流程成熟，但耦合 Spring Boot 后端。

**决策**：独立开发 Go CLI，借鉴其设计模式（init/bind/pull/push/version/docCode 寻址、--json envelope、SKILL 操作手册）。

**理由**：
1. losu-wiki CLI 的 `wiki login` / `wiki pull` / `wiki push` / `wiki version` 等交互范式已经过内网验证
2. 借鉴而非复用可避免 Java 后端的强耦合
3. 输出协议可保持兼容：`ai-kbs read/write/edit/pull/push/version` 的 `--json` schema 与 losu-wiki 类似但字段重命名

**风险**：
- 借鉴而非复用意味着失去一些边角特性
- 与 losu-wiki CLI 的命令不一致，迁移用户需学习新命令

**对策**：
- 提供 `ai-kbs migrate-from losu-wiki` 子命令（仅迁移 docCode → 本地 docID 映射）
- SKILL.md 提供平移对照表

### ADR-004：版本管理采用 CAS + docCode 而非 Git 集成

**状态**：✅ Accepted

**背景**：losu-wiki 使用 CAS 乐观锁 + docCode 稳定标识。

**决策**：沿用此模式，不引入 Git 作为文档后端。

**理由**：
1. 文档与代码的协作模式不同：文档需要细粒度权限与审计，Git 仓库模型过重
2. CAS + docCode 已能覆盖 95% 协作场景（创建/编辑/移动/版本回滚）
3. 避开 Git LFS、partial clone、submodule 等复杂概念

**风险**：
- 大文档（>10MB）二进制版本管理压力大
- 缺少 Git 的 PR/MR 工作流（Code Review）

**对策**：
- 大文件走 OSS 直传 + md5 校验，不入版本链
- 后续可叠加 "草稿-审核-发布" 工作流（v2）

### ADR-005：MCP 作为 AI Agent 标准入口

**状态**：✅ Accepted

**背景**：Claude Code / Codex / Cursor 等 AI 工具普遍支持 MCP。

**决策**：ai-kbs 同时提供 MCP server（stdio/SSE/HTTP）+ REST API + CLI + Web UI；AI Agent 优先通过 MCP 调用。

**理由**：
1. MCP 已是事实标准；Claude Code 通过 `.mcp.json` 一行配置即可接入
2. MCP 工具描述自动生成，无需手写 SDK 绑定
3. 与未来 AI 工具兼容性最佳

**风险**：
- MCP 协议仍在演进（v0.x）；需关注 breaking change
- MCP 工具粒度过粗/过细都不好

**对策**：
- MCP 工具分两层：core（read/write/search/agent-query）+ extended（version/binding/admin）
- 工具输入/输出严格遵循 frozen schema，标注版本号
