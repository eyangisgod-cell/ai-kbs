# AI-KBS · 需求分析

> AI Knowledge Base System —— 基于 WeKnora、ES、SAG 的企业级 AI Agent 知识库开源项目需求文档。

## 项目愿景

构建一个**让 AI Agent 直接作为开发协作者**的企业级知识库系统：
- 让 **Claude Code + 本项目 CLI** 通过 MCP 工具自动获取项目需求、规范、约束
- 提供 **ES + SAG 双引擎** 检索，在多跳问答与结构化推理上达到 SOTA
- 提供 **完整的文档版本控制体系**（CAS 乐观锁、docCode 稳定标识、git revert 语义回滚）
- 提供 **AI-friendly 的 CLI 与 SKILL 操作手册**，让 Agent 与人类用户共享同一套操作语言

## 文档导航

| 文档 | 用途 |
|------|------|
| [00-context.md](./00-context.md) | 背景、问题陈述、约束与决策记录 |
| [01-discovery.md](./01-discovery.md) | 三源参考项目（ai-wiki / WeKnora / SAG）的发现与能力盘点 |
| [02-glossary.md](./02-glossary.md) | 统一词汇表（避免多源术语歧义） |
| [03-requirements.md](./03-requirements.md) | 功能 / 非功能需求清单与 v1 范围界定 |
| [04-architecture.md](./04-architecture.md) | 总体架构、模块划分、数据流 |
| [05-data-model.md](./05-data-model.md) | 核心实体、ER 图、版本/绑定/sync-state 状态机 |
| [06-cli-spec.md](./06-cli-spec.md) | CLI 命令规范（参考 wiki-cli 风格） |
| [07-mcp-tools.md](./07-mcp-tools.md) | MCP 工具清单（AI Agent 调用面） |
| [08-bdd-specs.md](./08-bdd-specs.md) | BDD Gherkin 行为场景 |
| [09-best-practices.md](./09-best-practices.md) | 安全 / 性能 / 可维护性 / 常见陷阱 |
| [10-roadmap.md](./10-roadmap.md) | 实施路线图与里程碑 |

## 快速决策摘要

| 维度 | 选择 | 一句话理由 |
|------|------|----------|
| 后端基座 | **WeKnora v0.7.1** | 唯一同时具备 RAG + ReAct Agent + Wiki Mode + 知识图谱 + RBAC + CLI + MCP + Langfuse 的开源项目 |
| 检索引擎 | **ES（BM25/Dense/Graph）+ SAG（事件-实体-动态超边）双引擎** | ES 适合常规语义检索；SAG 在多跳问答与结构化推理 SOTA；并存按 query 类型路由 |
| 文档解析 | **WeKnora docreader**（Python gRPC） | 已支持 10+ 格式（PDF/Word/Image/Excel/PPT/Markdown/HTML/EPUB/MHTML） |
| CLI | **Go + Cobra**（单二进制） | 性能 + 分发便捷 + 与 WeKnora 内核一致 |
| AI Agent 入口 | **MCP（stdio/SSE/HTTP）+ SKILL.md** | 与 Claude Code / Codex / Cursor 等通用 AI 工具开箱即用 |
| 文档版本 | **CAS 乐观锁 + docCode 稳定标识 + 完整版本链** | 借鉴 wiki 体系，避免 Git 复杂度 |
| 鉴权 | **HMAC-SHA256 / JWT 双轨 + 4 级 RBAC** | CLI 自动化用 HMAC；Web/API 用 JWT；前端用 scoped API Key |
| 部署 | **Docker Compose（开发）+ Helm（生产）** | 与 WeKnora 体系一致；扩展集成 ES + SAG |

## 阅读顺序建议

- **产品 / 业务**：00 → 01 → 03 → 10
- **架构师**：00 → 01 → 02 → 04 → 05
- **后端开发**：04 → 05 → 06 → 07
- **AI Agent 集成**：04 → 06 → 07 → 08
- **运维 / 安全**：04 → 05 → 09 → 10
