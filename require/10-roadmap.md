# 10 · 实施路线图

## 1. 总览

```
Phase 0 (2 周)        Phase 1 (6 周)         Phase 1.1 (4 周)        Phase 2 (持续)
基础设施搭建      →   核心能力交付      →   体验增强          →   生态扩展
fork WeKnora          CRUD/同步/版本/MCP     数据源/IM 接入      多 IM / 移动 / Cloud
                       ES + SAG 双引擎
```

## 2. Phase 0：基础设施（Week 1-2）

### 目标
搭建仓库、CI/CD、开发环境、基础设施依赖

### 任务清单

| # | 任务 | 估时 | 责任 |
|---|------|------|------|
| 0.1 | 仓库初始化（fork WeKnora + 改名 + LICENSE） | 0.5d | 全栈 |
| 0.2 | CI/CD 流水线（GitHub Actions / GitLab CI） | 1d | DevOps |
| 0.3 | Docker Compose 开发环境（PG + ES + Redis + SAG） | 1d | DevOps |
| 0.4 | Helm Chart 基础骨架 | 1d | DevOps |
| 0.5 | 代码规范工具配置（golangci-lint / ruff / eslint） | 0.5d | 全栈 |
| 0.6 | 文档站点骨架（VitePress / Docusaurus） | 1d | 全栈 |
| 0.7 | Langfuse 部署（Docker） | 0.5d | DevOps |
| 0.8 | 数据库迁移框架选型（沿用 WeKnora migrate） | 0.5d | 后端 |

### 交付物
- ✅ 仓库可编译
- ✅ `docker compose up` 一键启动所有依赖
- ✅ CI 在 PR 时跑 lint + test
- ✅ 文档站点可访问

## 3. Phase 1：核心能力（Week 3-8）

### 3.1 Week 3-4：后端基础 + 文档模型

#### 后端任务

| # | 任务 | 估时 |
|---|------|------|
| 1.1 | 数据库迁移：workspaces / spaces / documents / versions / chunks | 1d |
| 1.2 | 实体层：Repository pattern（接口 + PG 实现） | 1d |
| 1.3 | DocCode 生成器（8 位 base62） | 0.5d |
| 1.4 | CAS 乐观锁实现 | 0.5d |
| 1.5 | 鉴权：HMAC + JWT + API Key | 2d |
| 1.6 | RBAC 4 级矩阵 + 中间件 | 1d |
| 1.7 | 错误码体系 + envelope 输出 | 0.5d |
| 1.8 | Langfuse 集成（OTLP） | 0.5d |

#### 测试任务

| # | 任务 | 估时 |
|---|------|------|
| 1.9 | 单元测试：docCode / CAS / auth / RBAC | 1d |

### 3.2 Week 5：CLI 基础

| # | 任务 | 估时 |
|---|------|------|
| 2.1 | CLI 骨架（Cobra + global flags + config） | 1d |
| 2.2 | 凭证管理（priority chain + store） | 1d |
| 2.3 | envelope 输出（machine.go） | 0.5d |
| 2.4 | 命令：login / logout / init / status / bind / unbind | 1d |
| 2.5 | 命令：read / write / edit（CAS） | 1d |
| 2.6 | 命令：version list/diff/rollback | 0.5d |
| 2.7 | 命令：move / undelete / resolve | 0.5d |
| 2.8 | SKILL.md + references/ 编写 | 1d |

### 3.3 Week 6：同步引擎

| # | 任务 | 估时 |
|---|------|------|
| 3.1 | 工作空间配置加载（.ai-kbs/workspace.yaml） | 0.5d |
| 3.2 | sync-state.json 读写 | 0.5d |
| 3.3 | pull 实现（8 路并发 + rename 检测 + 删除护栏） | 2d |
| 3.4 | push 实现（CAS + 冲突策略） | 2d |
| 3.5 | 并发同步（8 路 errgroup + semaphore） | 0.5d |
| 3.6 | E2E 测试（Mock 后端 + 真后端） | 1d |

### 3.4 Week 7：检索 + Agent

| # | 任务 | 估时 |
|---|------|------|
| 4.1 | ES 索引设计 + mapping + IK 分词 | 1d |
| 4.2 | 文档解析 pipeline（docreader gRPC） | 1d |
| 4.3 | chunking（semantic / parent_child） | 1d |
| 4.4 | embedding pipeline（异步 + 缓存） | 0.5d |
| 4.5 | search service（BM25 + vector + hybrid） | 1d |
| 4.6 | SAG 集成：event/entity 提取 + 检索 | 1d |
| 4.7 | 检索路由（query classifier） | 0.5d |
| 4.8 | ReAct Agent 引擎（沿用 WeKnora） | 1d |
| 4.9 | ask 接口（NDJSON 流） | 1d |

### 3.5 Week 8：MCP + 集成测试

| # | 任务 | 估时 |
|---|------|------|
| 5.1 | MCP server 骨架（FastMCP + multi-transport） | 1d |
| 5.2 | MCP tools 实现（core + write + version + sag） | 2d |
| 5.3 | OAuth2 支持 | 1d |
| 5.4 | `ai-kbs mcp install` 子命令 | 0.5d |
| 5.5 | Claude Code / Codex 集成测试 | 1d |
| 5.6 | BDD 场景覆盖（pytest-bdd） | 1d |
| 5.7 | 整体集成测试 + 性能基线 | 1d |

### Phase 1 交付物（v1.0.0）

- ✅ 后端：CRUD / 版本 / 绑定 / 同步 / 检索 / Agent
- ✅ CLI：完整命令集 + SKILL.md
- ✅ MCP：core + write + version + sag tools + 一键接入
- ✅ 检索：ES + SAG 双引擎 + 路由
- ✅ 鉴权：HMAC + JWT + scoped API Key
- ✅ RBAC：4 级 + per-KB 所有权
- ✅ 文档：双语文档 + SKILL
- ✅ 部署：Docker Compose（开发） + Helm（生产基础）
- ✅ 测试：单元 ≥ 70% + BDD 48 个场景

## 4. Phase 1.1：体验增强（Week 9-12）

### 目标
扩展数据源、提升文档解析覆盖、增强 Web UI、补充迁移工具

### 任务清单

| # | 任务 | 估时 |
|---|------|------|
| 6.1 | 文档解析扩展：Excel / PPT / EPUB / MHTML | 1.5w |
| 6.2 | 图像 OCR（Tesseract / PaddleOCR） | 0.5w |
| 6.3 | 数据源：Feishu 同步（v1.1 选其一） | 1w |
| 6.4 | 通用 Webhook 接收 | 0.5w |
| 6.5 | Web UI：完整文档浏览 + 问答界面 | 1w |
| 6.6 | Web UI：知识图谱可视化（D3 / Cytoscape） | 0.5w |
| 6.7 | Web UI：管理后台（队列监控 / 审计日志） | 0.5w |
| 6.8 | `ai-kbs migrate from-losu-wiki` 工具 | 0.5w |
| 6.9 | Chrome Extension v0.1 | 1w |
| 6.10 | 性能优化 + 缓存 | 0.5w |

### Phase 1.1 交付物（v1.1.0）

- ✅ 多格式文档解析（12+ 格式）
- ✅ Feishu 同步
- ✅ Webhook 接入
- ✅ Web UI 完整功能
- ✅ 迁移工具
- ✅ Chrome Extension

## 5. Phase 2：生态扩展（Week 13+，持续）

### 候选模块（按用户反馈选）

| # | 模块 | 估时 |
|---|------|------|
| P2.1 | Notion / Yuque / RSS 数据源 | 各 1w |
| P2.2 | IM 集成：WeCom / Slack / Telegram / DingTalk | 各 1w |
| P2.3 | WeChat Mini Program | 2w |
| P2.4 | Embed Widget（网站嵌入 + secure-mode） | 1.5w |
| P2.5 | OCTX 导入导出（参考 SAG） | 1w |
| P2.6 | 多 LLM 路由（成本优化） | 1w |
| P2.7 | 文档协作（草稿-审核-发布工作流） | 2w |
| P2.8 | 多租户计费 / 配额 | 1w |
| P2.9 | WeKnora Cloud（托管 LLM + 解析） | 4w |

## 6. 里程碑（Milestones）

| Milestone | 日期 | 验收 |
|-----------|------|------|
| **M0**：仓库 + CI + 开发环境就绪 | Week 2 末 | `docker compose up` 一键启动；CI 绿 |
| **M1**：后端核心 + CLI 基础 | Week 5 末 | 文档 CRUD + 版本 + 鉴权 + RBAC 全通；CLI 8 路并发同步 |
| **M2**：检索 + Agent | Week 7 末 | ES + SAG 双引擎；Claude Code 通过 MCP 调用 `ai_kbs_ask` 成功 |
| **M3**：v1.0.0 GA | Week 8 末 | 48 个 BDD 场景全过；性能基线达标；文档齐备 |
| **M4**：v1.1.0 数据源 + Web | Week 12 末 | Feishu 同步；Web UI GA；Chrome Extension |
| **M5**：v2.0.0 生态 | Week 24 末 | 多 IM + 多数据源 + 嵌入 |

## 7. 团队配置（建议）

| 角色 | 人数 | 主要职责 |
|------|------|----------|
| 后端工程师 | 2 | Go 后端、DB、检索 |
| 前端工程师 | 1 | Web UI（Vue） |
| AI/ML 工程师 | 1 | SAG 集成、Agent、embedding pipeline |
| DevOps / SRE | 0.5 | CI/CD、K8s、可观测 |
| 产品 / QA | 0.5 | SKILL 文档、BDD 场景、E2E 测试 |

**总计**：~5 人 / 12 周完成 v1.1。

## 8. 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| WeKnora 上游 breaking change | 高 | Pin 版本；fork 长期维护；监控 WeKnora release |
| SAG 论文复现误差 | 中 | 用官方 CLI 验证；与论文作者社区保持联系 |
| 中文分词（IK）效果不佳 | 中 | 备选：HanLP、Jieba；用户可配置 |
| 多 LLM provider 兼容 | 中 | 沿用 WeKnora 抽象；每 provider 独立测试 |
| Agent Skills 沙箱安全 | 高 | 用成熟方案（gVisor / Firecracker）；红蓝对抗测试 |
| 单机部署性能瓶颈 | 低 | v1 定位中小团队；v2 加分布式 SAG |
| 用户迁移成本（losu-wiki → ai-kbs） | 中 | 提供迁移工具；保留 CLI 兼容层（部分命令） |

## 9. 关键绩效指标（KPI）

### Phase 1 结束（v1.0.0）

| 指标 | 目标 |
|------|------|
| 单元测试覆盖率 | ≥ 70% |
| BDD 场景数 | ≥ 48 |
| 集成测试通过率 | 100% |
| E2E 测试通过率 | 100% |
| 启动时间（CLI） | < 200ms |
| 文档检索 P95 | < 200ms |
| 问答 P95（不含 LLM） | < 2s |
| 1000 文件 pull 耗时 | < 60s |

### Phase 1.1 结束（v1.1.0）

| 指标 | 目标 |
|------|------|
| 文档格式支持 | ≥ 12 种 |
| 数据源 | ≥ 1 种（Feishu） |
| Web UI 主要功能 | 100% 完成 |
| Chrome Extension | 发布 |

### Phase 2 结束（v2.0.0）

| 指标 | 目标 |
|------|------|
| IM 通道 | ≥ 4 种 |
| 数据源 | ≥ 4 种 |
| 月活用户（自部署） | ≥ 100 个组织 |
| GitHub stars | ≥ 1k |

## 10. 开放问题（v1 内必须决策）

| # | 问题 | 决策点 | 当前默认 |
|---|------|--------|----------|
| Q1 | 是否在 v1 内支持 PG-only 部署（无 ES）？ | Week 3 | 否，必须 ES（搜索质量） |
| Q2 | SAG 默认后端：LanceDB 还是 SQLite？ | Week 6 | LanceDB（性能 + 扩展性） |
| Q3 | embedding 模型默认值？ | Week 6 | BGE-large-zh-v1.5（中文场景） |
| Q4 | LLM 默认值？ | Week 6 | DeepSeek（成本 + 中文友好） |
| Q5 | sync-state schema 是否 v1 就稳定？ | Week 5 | 是，frozen schema v1.0 |
| Q6 | MCP server 是 Go 还是 Python？ | Week 8 | Python（FastMCP 生态成熟） |

## 11. 持续运营

### 11.1 监控仪表盘

- Langfuse：RAG / Agent 推理链路
- Prometheus + Grafana：基础设施
- ai-kbs 自带 admin UI：运行时队列 / worker pool / 审计日志

### 11.2 社区运营

- GitHub Issues / Discussions：用户支持
- Discord / 飞书群：开发者交流
- 月度发布：CHANGELOG + 升级指南
- 季度 AMA：roadmap 同步

### 11.3 安全响应

- SECURITY.md：报告漏洞的渠道
- 关键漏洞：72 小时内修复 + 通知
- CVE 编号：受影响版本号明确标注
- 定期渗透测试（年 1 次）

---

## 附录 A：依赖矩阵

| 组件 | 版本 | 用途 |
|------|------|------|
| Go | 1.22+ | 后端 + CLI |
| Python | 3.11+ | MCP server / docreader / SAG bridge |
| Node | 20+ | Web UI（构建期） |
| PostgreSQL | 15+ | 主存 |
| Elasticsearch | 8.x | 检索 |
| Redis | 7+ | 缓存 / 会话 |
| RabbitMQ | 3.12+ | 异步任务 |
| Langfuse | latest | 可观测 |
| LanceDB | latest | SAG 本地存储 |
| Cobra | 1.8+ | CLI 框架 |
| FastMCP | latest | MCP server 框架 |
| gRPC | 1.60+ | docreader 协议 |
| pgvector | 0.5+ | 向量扩展 |
| IK Analyzer | 8.x | 中文分词 |

## 附录 B：仓库布局（最终）

```
ai-kbs/
├── README.md                      # 项目入口
├── LICENSE                        # MIT
├── CHANGELOG.md
├── CLAUDE.md                      # 给 Claude Code 的项目指南
├── docker-compose.yml             # 开发环境
├── docker-compose.dev.yml
├── helm/                          # K8s chart
├── docs/                          # 公开文档
│   ├── README.md
│   ├── getting-started/
│   ├── architecture/
│   ├── api/
│   ├── cli/
│   ├── mcp/
│   └── adr/                       # 架构决策记录
├── backend/                       # Go 后端（forked from WeKnora）
│   ├── cmd/
│   ├── internal/
│   ├── migrations/
│   ├── config/
│   ├── go.mod
│   └── README.md
├── cli/                           # Go CLI（独立项目）
│   ├── cmd/
│   ├── internal/
│   ├── skills/                    # SKILL.md
│   ├── go.mod
│   └── README.md
├── mcp-server/                    # Python MCP server
│   ├── ai_kbs_mcp/
│   ├── pyproject.toml
│   └── README.md
├── docreader/                     # Python 文档解析（forked from WeKnora）
│   ├── parser/
│   ├── splitter/
│   ├── proto/
│   └── pyproject.toml
├── sag-bridge/                    # Python SAG 集成
│   ├── sag_bridge/
│   ├── pyproject.toml
│   └── README.md
├── frontend/                      # Vue Web UI（forked from WeKnora）
│   ├── src/
│   ├── package.json
│   └── README.md
├── scripts/                       # 工具脚本
│   ├── seed.sh
│   ├── reindex.sh
│   └── migrate.sh
└── tests/                         # 集成 / E2E
    ├── integration/
    ├── e2e/
    └── bdd/
```
