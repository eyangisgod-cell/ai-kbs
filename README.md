# AI-KBS（AI 知识库系统）

> **优先面向 AI 智能体的开源企业级知识库系统。**
> 面向 AI Agent 开发协作者的开源企业级知识库系统。

[![许可证](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](./LICENSE)
[![项目状态](https://img.shields.io/badge/status-pre--alpha-orange.svg)]()
[![后端](https://img.shields.io/badge/backend-forked%20from%20WeKnora-blueviolet.svg)](https://github.com/Tencent/WeKnora)

## 简介

**AI-KBS** 是一套从零构建的开源企业知识库系统，定位为 **AI 智能体的核心底层载体**。传统 Wiki 需要智能体爬取网页，普通 RAG 工具仅提供一层单薄的 REST 接口；而 AI-KBS 将 AI 智能体作为一等公民，具备以下能力：

- **以 MCP（模型上下文协议）作为主入口** — 通过命令 `ai-kbs mcp install --agent {claude-code|codex|cursor|gemini-cli}`，一键配置 Claude Code / Codex / Cursor / Gemini CLI。
- **带固定 JSON 信封格式的命令行工具** — 每条命令输出确定的 `next_action` 数据包，智能体可基于结果做分支判断，无需解析自然文本。
- **多跳检索** — 结构感知图（SAG）层配合 Elasticsearch + RAG，在 2WikiMultiHopQA、MuSiQue 这类多跳问答数据集上实现高 Recall@5 召回效果。
- **CAS + docCode 版本链** — 所有写入采用乐观锁，分配稳定的 8 位 Base62 标识；文档重命名不会破坏引用；回滚会生成新版本并完整保留历史。
- **工作空间 / 绑定 / 拉取 / 推送** — 本地目录与远程空间双向同步，支持 8 并发；使用体验和 Git 一致，但专为文档设计：支持细粒度 RBAC、审计日志，没有 Git LFS 的复杂负担。

## 核心特性

| 能力 | 提供内容 |
|---|---|
| **MCP 服务端** | stdio / SSE / HTTP 传输方式，OAuth2 权限域（`kbs:read` / `kbs:write` / `kbs:version` / `kbs:admin`） |
| **适配 AI 的命令行工具** | 约 20 条子命令；固定 `--json` 数据包规范 v1.0；程序内置 SKILL.md |
| **三层检索架构** | Elasticsearch（BM25 + 稠密向量 + 混合检索）+ RAG 框架（WeKnora）+ SAG（多跳检索）；由查询分类器为不同问题分配检索链路 |
| **文档增删改查 + 版本链** | CAS 乐观锁；回滚生成新版本，历史留存；软删除与恢复 |
| **工作空间同步** | 8 并发拉取/推送；依靠稳定 docCode 识别重命名；删除超过 10 个文件或大于 5MB 需要附加 `--yes` 确认 |
| **ReAct 智能体 + 问答** | 流式 NDJSON（`init / thinking / tool_call / tool_result / answer / complete`）；每条回答附带 `doc_code / 标题 / 引用片段 / 链接` 溯源信息 |
| **身份认证 + RBAC** | HMAC-SHA256（命令行/MCP）、JWT（网页/API）、带作用域的 API Key（嵌入式）；四级权限：所有者/管理员/贡献者/查看者；内置 IDOR、SSRF 防护 |
| **文档解析** | Markdown / PDF / Word / HTML / 纯文本（v1）；异步工作池 + 死信队列 |
| **对象存储** | 可插拔适配器：本地存储 / MinIO / S3 / OSS / TOS / KS3 / OBS |
| **可观测性** | 接入 Langfuse OTLP；遵循 W3C `traceparent` 链路透传；核心指标：QPS、延迟、Token 消耗、队列堆积 |
| **部署方案** | Docker Compose（开发环境，10 分钟内拉起）+ Helm Chart（生产环境，支持 HPA、健康检查、Ingress） |

## 快速开始

> 环境要求：Docker 24+ / Docker Compose v2 / Go 1.23+（自行编译） / Node 20+（前端）。

```bash
# 1. 拉取代码
git clone [https://github.com/eyangisgod-cell/ai-kbs.git](https://github.com/eyangisgod-cell/ai-kbs.git)
cd ai-kbs

# 2. 启动全套服务（PostgreSQL + Elasticsearch + Redis + RabbitMQ + Langfuse + 后端 + MCP 服务 + 前端）
docker compose -f docker-compose.dev.yml up -d

# 3. 安装 AI-KBS 命令行工具（单静态二进制文件）
#    源码路径 ./cli/；编译参数 CGO_ENABLED=0，支持 Linux / macOS / Windows，amd64 / arm64

# 4. 配置登录凭证
ai-kbs login --url [http://localhost:8080](http://localhost:8080)

# 5. 初始化工作空间并绑定知识库空间
mkdir docs && cd docs
ai-kbs init
ai-kbs bind --space dev
ai-kbs pull

# 6. 一键为 Claude Code 安装 MCP
ai-kbs mcp install --agent claude-code
```

完成第 6 步后重启 Claude Code，即可使用工具：
`ai_kbs_search`、`ai_kbs_read_doc`、`ai_kbs_ask`、`ai_kbs_create_doc`、`ai_kbs_update_doc`、
`ai_kbs_edit_doc`、`ai_kbs_list_versions`、`ai_kbs_rollback_version` 等。

## 架构

```
┌──────────────────────────────────────────────────────────────┐
│                     AI Agent 客户端层                         │
│     Claude Code   Codex   Cursor   Gemini CLI                │
└───────────────────────────┬──────────────────────────────────┘
                            │ MCP（stdio / SSE / HTTP / OAuth2）
                            ▼
┌──────────────────────────────────────────────────────────────┐
│              ai-kbs-mcp-server（Python，FastMCP）            │
└───────────────────────────┬──────────────────────────────────┘
                            │ HTTPS REST + NDJSON
                            ▼
┌──────────────────────────────────────────────────────────────┐
│          ai-kbs-backend（Go，基于 WeKnora 二次开发）         │
│  认证(HMAC/JWT/APIKey) | RBAC | 审计 | 链路追踪 | 限流       │
└────┬───────────┬───────────┬──────────────┬─────────────────┘
     ▼           ▼           ▼              ▼
PostgreSQL    Elastic     ai-kbs-        ai-kbs-
+pgvector    Search 8.x   sag-bridge     docreader
                          (SAG)         (gRPC)
```

完整架构图（包含存储、消息队列、可观测性、前端）请查看 [`docs/architecture.md`](./docs/architecture.md)（待完善）。

## 仓库目录结构

| 路径 | 用途 |
|---|---|
| `backend/` | Go 后端（基于 WeKnora v0.7.1 分支，模块名 `github.com/eyangisgod-cell/ai-kbs`） |
| `cli/` | Go 命令行程序 `ai-kbs`，内置 SKILL.md |
| `mcp-server/` | Python FastMCP 服务（stdio / SSE / HTTP，支持 OAuth2） |
| `sag-bridge/` | Python FastAPI 桥接服务，对外暴露 SAG 多跳检索能力 |
| `docreader/` | Python gRPC 文档解析服务（PDF / Word / Markdown / HTML / 纯文本） |
| `frontend/` | Vue3 单页应用（基于 WeKnora 前端分支，品牌重命名为 AI-KBS） |
| `helm/` | 生产环境 Helm Chart（K8s 部署） |
| `docs/` | 公开文档 |
| `require/` | 中文需求文档 |

## 文档

- 中文需求文档（PRD / 设计 / 任务）：[`require/`](./require/)
- 公开文档入口（待补全）：[`docs/`](./docs/)
- CLI 操作手册（SKILL）：打包在二进制内，源码路径 `cli/skills/SKILL.md`
- MCP 工具固定 Schema v1.0：随 MCP 服务端发布包附带 JSON Schema

## 参与贡献

欢迎提交 Issue 和 Pull Request。提交 PR 前，请阅读 [`AGENTS.md`](./AGENTS.md)，了解协作约定与 Trellis 多智能体流水线。

**强制规范：**

1. 仓库内所有源码、配置、注释、文档、提交信息中，**禁止出现内部平台名称**。CI 自动审计会拦截违规 PR。
2. 固定协议结构（`api.frozen.json`、`--json` 数据包、MCP 工具入参出参）版本锁定；修改必须升级版本号，并补充迁移说明。
3. Go 标识符使用小驼峰 / 大驼峰；CLI 参数使用短横线命名；环境变量前缀为 `AIKBS_*`；JSON 传输字段使用小驼峰。

## 许可证

本项目采用 **Apache License 2.0** 协议，详见 [`LICENSE`](./LICENSE)。

`backend/` 目录以及部分前端代码分支，衍生自 [WeKnora](https://github.com/Tencent/WeKnora)（同样为 Apache 2.0），在对应子目录保留原始版权声明。AI-KBS 为独立项目：重新设计产品名称、前端换皮，并重构命令行与配置规范，移除上游私有依赖痕迹。

如果你需要，我可以再单独输出一份精简中文版 README，适合直接替换仓库原版。