# 01 · 三源参考项目发现结果

## 1. losu-ai-wiki（公司内网，流程模板）

### 1.1 仓库结构

```
losu-ai-wiki/
├── losu-wiki-cli/        # Go CLI（Cobra）
│   ├── go/               # 实际代码目录
│   ├── skills/           # SKILL.md + references/（AI Agent 操作手册）
│   └── docs/             # 架构、命令参考、distribution 指南
├── losu-wiki-system/     # Java Spring Boot 3 后端
│   ├── src/main/java/com/szjlc/ai/doc/wiki/   # 自包含 wiki 模块（无公司依赖）
│   ├── src/main/java/com/szjlc/ai/doc/        # 其他 cloud profile 模块
│   ├── docker/           # 本地基础设施（MySQL + Redis）
│   └── docs/requirements # 需求与建表脚本
└── losu-wiki-web/        # Vue + Vite + Tailwind + Playwright e2e
```

### 1.2 核心能力盘点

#### 1.2.1 CLI 能力（losu-wiki-cli）

| 能力 | 命令 | 借鉴价值 |
|------|------|----------|
| 工作空间初始化 | `wiki init` | ✅ 完全借鉴 |
| 空间绑定 | `wiki bind/unbind` | ✅ 完全借鉴 |
| 双向同步 | `wiki pull/push` | ✅ 借鉴 + 增强（SAG 增量同步） |
| 单文档读写 | `wiki read/write/edit` | ✅ 借鉴 |
| 版本管理 | `wiki version list/diff/rollback` | ✅ 借鉴（CAS + git revert 语义） |
| 移动/重命名 | `wiki move` | ✅ 借鉴（docCode 不变） |
| 反删除 | `wiki undelete` | ✅ 借鉴 |
| 冲突解决 | `wiki resolve` | ✅ 借鉴（4 种策略 + auto） |
| 搜索 | `wiki search/grep/tree` | ✅ 借鉴 |
| 鉴权链 | HMAC-SHA256 / DirectToken / env | ✅ 借鉴 |
| 寻址体系 | 4 flag：url / space+doc / space+path | ✅ 借鉴 |
| AI 友好输出 | `--json` + `next_action` envelope | ✅ 完全借鉴 |
| 删除护栏 | >10 文件 / >5MB → `--yes` | ✅ 借鉴 |

#### 1.2.2 后端能力（losu-wiki-system）

| 能力 | 复用价值 |
|------|----------|
| Spring Boot 3 + MyBatis Plus | ❌ 不用 Java 栈 |
| Wiki 模块自包含设计 | ✅ 学习"自包含子模块"思路 |
| Repository 模式 | ❌ Go 用接口抽象即可 |
| ES/MySQL 双搜索后端 | ✅ 设计模式借鉴 |
| MySQL/OSS 双存储 | ✅ 设计模式借鉴 |
| 多 profile（local/dev/fat/uat/prod） | ✅ 设计模式借鉴 |
| 多认证模式（mock/sdk） | ✅ 设计模式借鉴 |

#### 1.2.3 前端能力（losu-wiki-web）

| 能力 | 复用价值 |
|------|----------|
| Vue + Vite + Tailwind | ✅ 沿用技术栈 |
| Playwright e2e | ✅ 沿用测试栈 |
| pnpm workspace | ✅ 沿用包管理 |

### 1.3 关键设计模式（必须保留）

1. **docCode 稳定标识**：8 位 base62，move/rename 不变，重命名后链接不失效
2. **CAS 乐观锁**：所有写操作携带 `baseVersion`，冲突即拒绝
3. **`--json` frozen schema v1.0**：所有命令默认输出 JSON，包含 `success / data / error / next_action / request_id`
4. **`next_action` 可执行修复建议**：错误响应携带 AI Agent 可直接执行的 `cmd` + `args` + `reason`
5. **SKILL.md 操作手册**：为 AI Agent 提供结构化操作流程，硬性纪律（HARD RULES）
6. **删除护栏三层**：`--prune` → 阈值检查 → `--yes` 强制
7. **冲突策略四元组**：manual / keep-mine / keep-theirs / both + auto（基于 md5 三路）
8. **版本回滚保留中间版本**：`rollback` 不删除中间历史，而是产生新版本（git revert 语义）
9. **失败聚合**：单文件失败不整体退出，汇总到 `sync_results[].errors[]`，exit 23
10. **优先级链**：CLI flag > 环境变量 > 凭证文件 > 默认值

## 2. WeKnora（开源基座）

### 2.1 仓库结构

```
WeKnora-main/
├── cli/                  # Go CLI（agent-first wire contract）
│   ├── internal/         # 实现
│   ├── AGENTS.md         # AI Agent 接线约定
│   ├── README.md         # 人类用户手册
│   └── skills/           # CLI 内置 skills
├── internal/             # Go 后端
│   ├── agent/            # ReAct Agent 引擎 + 工具注册
│   ├── application/      # 应用层
│   ├── container/        # 依赖注入容器
│   ├── handler/          # HTTP handler
│   ├── mcp/              # MCP 服务端（multi-transport）
│   ├── models/           # 领域模型
│   └── ...
├── mcp-server/           # Python MCP server（独立进程）
├── docreader/            # Python 文档解析（gRPC）
├── frontend/             # Web UI
├── docker/               # 容器化
├── helm/                 # K8s 部署
├── skills/               # Agent Skills
├── migrations/           # DB 迁移
└── config/               # 配置
```

### 2.2 核心能力盘点

| 模块 | 能力 | 复用方式 |
|------|------|----------|
| RAG 检索 | BM25 + Dense + GraphRAG + parent-child + HNSW pgvector | ✅ 沿用，扩展 ES/SAG 路由 |
| ReAct Agent | 多步推理 + 工具调用 + MCP 集成 | ✅ 沿用 |
| Wiki Mode | Agent 自动生成结构化 Markdown wiki | ✅ 沿用，增强 SAG 事件-实体提取 |
| 数据源 | 飞书/Notion/语雀/RSS | v2 启用 |
| 文档格式 | PDF/Word/MD/HTML/EPUB/MHTML/Image/CSV/Excel/PPT/JSON | ✅ 沿用 docreader |
| 多 LLM | OpenAI/Claude/DeepSeek/Qwen/Volcengine/Gemini/MiniMax/NVIDIA/Novita/Ollama | ✅ 沿用 |
| Embedding | Ollama/BGE/GTE/Zhipu/OpenAI | ✅ 沿用 |
| 向量库 | pgvector/ES/OpenSearch/Milvus/Weaviate/Qdrant/Doris/Tencent | ✅ 沿用，扩展 SAG |
| 对象存储 | Local/MinIO/S3/OSS/TOS/KS3/OBS | ✅ 沿用 |
| IM 集成 | WeCom/Feishu/Lark/QQBot/Slack/Telegram/DingTalk/Mattermost/WeChat/Yunzhijia | v2 启用 |
| RBAC | 4 级 + per-KB 所有权 + 审计日志 | ✅ 沿用 |
| 鉴权 | Scoped API Key + Principal model + AES-256-GCM | ✅ 沿用 |
| Observability | Langfuse（OTLP/OTel + W3C traceparent） | ✅ 沿用 |
| 任务管理 | MQ 异步 + per-stage worker pool + per-model 并发调控 + DLQ | ✅ 沿用 |
| CLI | agent-first wire contract（success→stdout / error→stderr） | ✅ 借鉴 + 改写为 ai-kbs |
| MCP server | multi-transport（stdio/SSE/HTTP）+ OAuth | ✅ 沿用 |

### 2.3 关键代码位置（用于二次开发）

| 文件 | 用途 | 改造点 |
|------|------|--------|
| `internal/agent/engine.go` | ReAct 主循环 | 注入 SAG 工具 |
| `internal/agent/tools/` | 内置工具 | 新增 `sag_query` 工具 |
| `internal/handler/` | HTTP handlers | 新增 `/api/ai-kbs/*` 路由 |
| `internal/container/` | DI 容器 | 注册 SAG client |
| `mcp-server/main.py` | MCP server | 新增 ai-kbs tools |
| `cli/internal/` | CLI 实现 | 全部重写为 ai-kbs CLI |
| `docker/docker-compose.yml` | 容器编排 | 集成 ES + SAG 服务 |

### 2.4 注意点

- WeKnora CLI 用 `cobra` + Go；可借鉴其 wire contract，但不复用其命令名
- WeKnora mcp-server 是 Python 独立进程；本项目保留但新增 ai-kbs 专用工具
- WeKnora docreader 用 gRPC；保留此协议便于扩展

## 3. SAG（检索增强）

### 3.1 架构原理（论文摘要）

```
chunk → 1 个事件（event）+ 多个实体（entity）
event ↔ entity → 1 条潜在超边（hyperedge）
查询时：找到 seed 实体 → SQL 联接共享实体的事件 → 动态生成超边
```

### 3.2 与传统 RAG/GraphRAG 的差异

| 维度 | 传统 RAG | GraphRAG | SAG |
|------|----------|----------|-----|
| 索引 | chunk 向量 | chunk + 三元组 + 全局图 | chunk + 事件 + 实体 + 事件-实体关联 |
| 检索 | 向量相似度 | 图遍历 | SQL 联接 + 向量/全文种子 |
| 增量更新 | 易 | 难（需重建图） | 易（追加事件和实体） |
| 多跳 | 弱 | 强但代价高 | 强且代价低 |
| 论文 SOTA | 基线 | 在 HotpotQA 上部分指标强 | **三项基准全部 SOTA** |

### 3.3 集成方式

**架构选型**：
- SAG 作为 Python 服务（FastAPI + LanceDB / SQLite 本地副本），独立进程
- ai-kbs 后端通过 HTTP 调用 SAG API
- 数据流：doc 写入 → docreader 解析 → chunk → 触发 SAG 索引（事件 + 实体提取） → 双写 PostgreSQL + ES + SAG
- 查询时：粗排用 ES → top-N → 精排 + 结构化推理用 SAG → 事件 → 原文回溯

**集成点**：

| 集成点 | 说明 |
|--------|------|
| 索引同步 | doc 写入 PostgreSQL 后，异步任务触发 SAG 索引 |
| 检索路由 | query 进来后根据类型路由：单纯语义 → ES；多跳/实体类 → SAG |
| 引用追溯 | SAG 返回的 event 携带 chunk_id，可直接关联到原文档 |
| 评估 | 提供 SAG vs ES 的 A/B 评估接口 |

### 3.4 部署形态

```yaml
# docker-compose 扩展
services:
  sag-api:
    image: ai-kbs/sag:latest
    ports: ["8001:8001"]
    volumes:
      - sag_data:/data
    environment:
      - SAG_EMBEDDING_URL=${EMBEDDING_URL}
      - SAG_LLM_URL=${LLM_URL}
```

### 3.5 关键参考

- SAG 论文：https://arxiv.org/abs/2606.15971
- SAG 官方 CLI：`@zleap-ai/sag-cli`（Node），提供 `sag agent connect claude-code` 一键接入
- SAG Benchmarks：HotpotQA / 2WikiMultiHopQA / MuSiQue

## 4. 三源融合架构示意

```
                       Claude Code / Codex / Cursor
                                  │
                                  ▼ MCP (stdio/SSE/HTTP)
                          ┌──────────────┐
                          │  ai-kbs      │
                          │  MCP Server  │
                          └──────┬───────┘
                                 │
                                 ▼ REST / gRPC
                          ┌──────────────┐
                          │  ai-kbs      │      ┌─────────┐
                          │  Backend     │ ◀── │   SAG   │
                          │  (Go, forked│      │  (Python)│
                          │   from      │      └────┬────┘
                          │   WeKnora)  │           │
                          └──┬────┬──┬──┘           ▼
                             │    │  │         LanceDB/SQLite
              ┌──────────────┘    │  └──────────────┐
              ▼                   ▼                 ▼
       PostgreSQL         Elasticsearch      Object Storage
       (主存 + pgvector)   (检索副本)         (Local/S3/OSS)

                       ─────────── CLI (Go) ───────────
                                 ▲
                                 │ HMAC + JWT
                                 │
                            Human / CI / Agent
```

## 5. 与 losu-wiki 兼容性矩阵

| 维度 | losu-wiki | ai-kbs | 兼容策略 |
|------|-----------|--------|----------|
| CLI 命令名 | `wiki` | `ai-kbs` | 不重名（提供别名支持过渡） |
| 工作空间配置 | `.wiki/wiki.conf` | `.ai-kbs/workspace.yaml` | 不兼容（迁移工具） |
| sync-state | `.wiki/sync-state.json` | `.ai-kbs/sync-state.json` | schema 重写 |
| docCode | 8 位 base62 | 8 位 base62 | **完全兼容** |
| `--json` envelope | frozen schema v1.0 | frozen schema v1.0 | 字段重命名（`data.data` → `data.items` 等） |
| 鉴权链 | HMAC/Token | HMAC/JWT/scoped API Key | 兼容 HMAC + 新增 JWT |
| RBAC | 简单角色 | 4 级 + scoped | 增强 |
| 搜索后端 | MySQL/ES | ES + SAG | 增强 |
