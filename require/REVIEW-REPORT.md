# AI-KBS 方案可行性评审报告

> 评审对象：`e:\my-project\my-open-source\ai-kbs\require\README.md` 及关联 11 份文档
> 评审依据：本地实际代码（WeKnora v0.7.1 / SAG v1.8.6 / losu-ai-wiki 文档与 SKILL 模式）
> 评审人：Claude（基于代码扫描与文档分析）
> 评审结论：**整体方案可行**，核心技术选型真实可靠，需对若干细节做风险对冲

---

## 0. 一句话结论

| 维度 | 判定 | 说明 |
|------|------|------|
| **三源整合可行性** | ✅ 可行 | WeKnora 是真实可 fork 的 Go 主基座；SAG 是真实已开源的事件-实体 SOTA 检索；losu-ai-wiki 仅借鉴流程范式（无版权风险） |
| **架构设计** | ✅ 合理 | 后端 Go + Python 双层 + MCP 入口 + ES/SAG 双引擎，匹配业界主流模式 |
| **功能取舍** | ⚠️ 需收敛 | 当前 v1 P0+部分过载，建议明确"v1 必交付 vs v1.1/v2"以避免延期 |
| **实施性** | ⚠️ 偏激进 | 12 周 / 5 人完成 WeKnora fork + SAG 集成 + 19 命令 CLI + MCP + 迁移工具，估时需 ±30% buffer |
| **losu-ai-wiki 仅参考借鉴** | ✅ 边界清晰 | 只借鉴"设计模式 + 协议范式"，不复制源码；建议补充"反向证明"——文档明确"未复用 losu-ai-wiki 任何源文件"声明 |

---

## 1. 三源参考项目代码扫描结果

### 1.1 WeKnora v0.7.1（开源 MIT-like Tencent License）

**仓库实情**：`E:\my-project\WeKnora-main`，v0.7.1（VERSION 文件确认）

| 验证项 | 实测结果 | 文档描述是否匹配 |
|--------|---------|----------------|
| Go 后端 | `internal/` 下 27 个子包（agent / application / handler / mcp / container / searchutil / tracing ...） | ✅ 匹配 |
| ReAct Agent | `internal/agent/engine.go` 730 行，注入 Langfuse tracing、memory container、token estimator | ✅ 匹配 |
| 工具系统 | `internal/agent/tools/` 含 grep_chunks / database_query / faq_snippet / data_analysis / json_repair / get_document_info 等 20+ 工具 | ✅ 匹配 |
| Skills 系统 | `internal/agent/skills/` 含 loader.go / manager.go / skill.go，含 Progressive Disclosure 模式 | ✅ 匹配 |
| MCP | `mcp-server/` 单进程 Python（main.py + weknora_mcp_server.py），非 multi-transport | ⚠️ 文档"multi-transport"需核实——本机所见是单一可执行；可能含 SSE/HTTP 多种 transport 但不是子项目分离 |
| Docreader | `docreader/` Python gRPC，含 parser / splitter / proto / client / server | ✅ 匹配 |
| CLI | `cli/cmd/`（非 `cli/internal/cli/`），含 cmd/agent / api / auth / chat / chunk / config / doc / kb / search / wiki 等子命令，约 30+ 命令 | ⚠️ 文档中描述 "cli/internal/cli/" 路径与实际不符——实际在 `cli/cmd/<group>/<cmd>.go` |
| 鉴权 | `internal/auth/` + `internal/router/routes_auth_tenant.go` + scoped API Key | ✅ 匹配 |
| RBAC | `internal/handler/rbac.go` + `internal/router/rbac_selfcheck_test.go` | ✅ 匹配 |
| Observability | `internal/tracing/langfuse/` + `internal/agent/langfuse_test.go` | ✅ 匹配 |
| 部署 | `docker/docker-compose.yml` + `helm/` + `migrations/`（mysql/paradedb/sqlite/versioned） | ✅ 匹配 |
| 数据源/IM | `internal/datasource/` + `internal/im/`，已实现飞书/Notion/语雀/RSS/WeCom/Slack 等 | ✅ 匹配（v2 启用策略成立） |

**关键发现**：
1. **License 是 Tencent 自定义但类似 MIT**，需在 fork 时明确 LICENSE 文件归属；
2. **CLI 路径不一致**——`04-architecture.md` 写 `cli/cmd/ai-kbs/main.go` 是对的，但 `04-architecture.md §2.5` 又写 `internal/cli/` 是错的，应统一为 `cli/cmd/<group>/<cmd>.go` 模式；
3. **MCP server 是单 Python 进程**，不是多进程分模块；"multi-transport"指的是同一进程支持多种 transport，而非子项目分离；
4. **WeKnora 已有自己的 wiki 存储模型**（`agent/prompts_wiki.go` + `router/routes_wiki.go` + `router/router_wiki_test.go`），fork 后**要剥离** WeKnora 的 wiki 实现，替换为 ai-kbs 自有 docCode + version 链——这是 fork 工作的隐藏成本。

**适配成本估算**：
- fork + 模块替换 + 剥离 wiki：3-5 人天
- 改造点（参考 01-discovery.md §2.3 表）：每个 0.5-2 人天，共约 8-12 人天

### 1.2 SAG v1.8.6（开源 MIT）

**仓库实情**：`E:\my-project\SAG`，v1.8.6（SAG/README.md 徽章确认）

| 验证项 | 实测结果 | 文档描述是否匹配 |
|--------|---------|----------------|
| 架构原理 | `chunk → event + entities`，`query-time dynamic hyperedge`（SQL 联接） | ✅ 完全匹配 |
| Python 包 | `zleap-sag` v1.8.6（PyPI） | ✅ 匹配 |
| API 服务 | `apps/api/sag_api/`，含 db / jobs / mcp / generation / services / tools / octx / runtime | ✅ 匹配 |
| Agent | `apps/api/sag_agent/`（loop / runtime / tools / store / types） | ✅ 匹配 |
| MCP 支持 | `apps/api/sag_api/mcp/` + `@zleap-ai/sag-cli` 提供 `sag agent connect claude-code` | ✅ 匹配 |
| 部署形态 | `compose.yaml` + `compose.postgres.yaml` + `compose.dify.yaml` | ✅ 匹配 |
| OCTX 导入导出 | `apps/api/sag_api/octx/`，含 conflict handling + integrity validation | ✅ 匹配（可作 v2 数据迁移参考） |
| 论文 SOTA | HotpotQA / 2WikiMultiHopQA / MuSiQue 上 Recall@5 +6.79pp / F1 +4.33pp | ✅ 匹配 |

**关键发现**：
1. **SAG 本身已支持 MCP**——这意味着 ai-kbs 可以让 SAG 单独作为一个 MCP server 被 Claude Code 接入，**不一定需要 ai-kbs 自建 `ai_kbs_sag_query` 工具**。建议简化：ai-kbs 暴露 `ai_kbs_search` 即可（在 router 内部决定走 ES 还是 SAG），用户也可单独挂载 SAG MCP。
2. **SAG 文档/事件抽取依赖 LLM**——若 ai-kbs 部署环境无强 LLM，SAG 索引会失败。这是隐性依赖，文档应增加降级策略（SAG 不可用时降级到 ES）。
3. **SAG 默认 LanceDB / SQLite**，从单机起步；扩展到 PostgreSQL/pgvector 由 SAG 自己提供路径。ai-kbs 集成层（`internal/runtime/`）已抽象好，集成成本约 2-3 人天。
4. **Dify 集成已就绪**——`compose.dify.yaml` 表明 SAG 已支持 Dify workflow 嵌入；ai-kbs 可以借此省去自建 Agent orchestration。

**适配成本估算**：
- 独立部署 SAG + HTTP 客户端封装：1-2 人天
- 索引同步 MQ 任务：2-3 人天
- 检索路由（query classifier）：1 人天
- A/B 评估面板：1 人天
- 总计：~6 人天

### 1.3 losu-ai-wiki（公司内网，仅参考借鉴）

**仓库实情**：`E:\my-project\losu-ai-wiki`，三件套：CLI（Go）+ System（Java Spring Boot）+ Web（Vue）

| 验证项 | 实测结果 | 可借鉴度 |
|--------|---------|---------|
| CLI 模块 | `losu-wiki-cli/go/internal/` 含 cli / workspace / auth / resolve / config / update / output 七组 | 借鉴"模块切分"和"package 边界" |
| SKILL 手册 | `losu-wiki-cli/skills/SKILL.md` v1.3.0 + 18 个 references/*.md | **完全可借鉴**（这是公开的 SKILL 范式，与具体实现无关） |
| CLI 命令集 | 19 个命令：read/write/edit/search/grep/tree/spaces/login/logout/init/status/bind/unbind/pull/push/version/move/undelete/resolve | 借鉴"命令集覆盖范围" |
| 鉴权链 | HMAC / DirectToken / env 优先级链 | 借鉴"4 级优先级链"设计模式 |
| 寻址体系 | 4 flag：url / space+doc / space+path | 借鉴 |
| 输出协议 | `--json` frozen schema v1.0 + `next_action` envelope | 借鉴 |
| 删除护栏 | >10 文件 / >5MB → `--yes` | 借鉴 |
| 冲突策略 | manual / keep-mine / keep-theirs / both / auto | 借鉴 |
| 同步引擎 | sync-state.json + rename_detect + 8 路并发 | 借鉴"engine 切分"模式 |

**重要约束**：
1. **禁止复制 `losu-ai-wiki/losu-wiki-cli/go/internal/**/*.go` 源码**——这是公司内部代码，不能进 ai-kbs 公开仓库。
2. **可借鉴的部分**：
   - SKILL.md 文档结构（这不是源码，是设计文档）
   - CLI 命令范式（init/bind/pull/push/version/move/undelete/resolve 的工作流思路）
   - 输出 envelope schema 设计（这是协议设计，不受代码版权约束）
   - 鉴权链优先级（设计模式）
   - sync-state.json 数据结构（设计模式）
3. **借鉴的边界把控**：
   - ✅ 借鉴 = 读 SKILL.md / docs/specs/*.md 学习"做什么 / 为什么"
   - ❌ 不可借鉴 = 复制 internal/cli/*.go、internal/workspace/*.go 的实现代码
   - **审计建议**：在 `docs/adr/ADR-003-借鉴而非复用.md` 中加入"未复用源码声明"+ git commit 钩子检测（自动扫描 losu 私有代码指纹）。
4. **SKILL.md v1.3.0 是公开可读的范式**——这是"事实标准"层，可以以"参考开源文档范式"名义引用。

---

## 2. 架构合理性评估

### 2.1 总体架构（04-architecture.md §1）

```
Agent → MCP Server → Backend (Go, fork WeKnora) → PG + ES + SAG + Object Storage
```

**评估**：✅ 经典分层，无架构级问题。
- MCP 作为入口 = 标准做法
- Backend 解耦 = 好
- 双引擎并存 = 创新但有合理性

**潜在问题**：
- **Backend → SAG 的同步链路过深**：PG 主写 → MQ → Index Worker → SAG 索引 → ES 索引。任意一环失败都会导致"已写但检索不到"。文档缺少一致性模型说明（最终一致 vs 强一致？）。
- **建议补充**：在 `04-architecture.md §3.4` 增加"索引一致性"小节，明确"PG 主写成功后 ES/SAG 异步索引，N 秒内可见，失败有 DLQ 重试 + 审计告警"。

### 2.2 模块划分（04-architecture.md §2）

**评估**：✅ 模块粒度合理（与 WeKnora 内部对齐）。
- 后端：`handler/service/repository/agent/auth/rbac/doc/sync` 与 WeKnora `internal/handler` `internal/application/service` `internal/application/repository` 一一对应
- CLI：`cli/resolve/workspace/auth/output/errors` 与 losu-wiki 对应

**潜在问题**：
- **CLI 包结构与文档不一致**——`04-architecture.md §2.5` 写 `cli/internal/cli/root.go`，但实际 WeKnora 是 `cli/cmd/root.go + cli/cmd/<group>/`。建议统一为 `cli/cmd/ai-kbs/<group>/<cmd>.go` 模式（与 WeKnora 对齐）。
- **MCP server 写为独立 Python 项目**——但 WeKnora 的 MCP server 是 Python 单文件。ai-kbs 的 mcp-server 拆 5 个 transport 子模块（stdio/sse/http/oauth/client）是否过度？建议参考 WeKnora 的 weknora_mcp_server.py 单文件结构，控制在 1000-2000 行总规模。

### 2.3 数据流（04-architecture.md §3）

**评估**：✅ 双流（写 + 查）合理。
- 写入流：MCP → Backend → 3 个 sink（PG / MQ / Object Storage）→ 异步索引
- 查询流：MCP → Backend → 路由（ES / Hybrid / SAG）→ Fusion → NDJSON stream

**潜在问题**：
- **NDJSON 流未给出字段定义**——`04-architecture.md §3.2` 提到 `thinking | tool_call | answer | complete`，但与 WeKnora 的 wire contract 是否一致？建议在 `06-cli-spec.md` 同步定义 NDJSON schema v1.0。
- **Result Fusion 算法未指定**——`04-architecture.md §4.2` 提到 RRF 或 Cross-encoder，但未选型。建议 v1 用 RRF（无依赖），Cross-encoder 作为 v1.1 增强。

### 2.4 关键技术决策（04-architecture.md §4）

**评估**：✅ 决策合理。
- Go 后端（沿用 WeKnora）= 迁移成本最低
- ES + SAG 双引擎 = SOTA 与成熟并取
- CAS + docCode = 避开 Git 复杂度

**未决项**：
- **Q1-Q6 在 `10-roadmap.md §10` 已列**，需要在 Week 3 / Week 6 决策；建议提前在 Phase 0 收口。

---

## 3. 功能取舍评估

### 3.1 v1 P0（03-requirements.md §3）

| FR | 标题 | 取舍建议 |
|----|------|---------|
| FR-001 | 本地工作空间管理 | ✅ 必做（losu-wiki 验证成熟） |
| FR-002 | 文档双向同步 | ✅ 必做（核心卖点） |
| FR-003 | 单文档 CRUD | ✅ 必做 |
| FR-004 | 版本管理 | ✅ 必做（核心卖点） |
| FR-005 | 移动/重命名/反删除 | ⚠️ 建议降至 v1.1（P1）—— 不影响核心故事线 |
| FR-008 | 鉴权与凭证管理 | ✅ 必做（HMAC + JWT） |
| FR-009 | MCP 接口 | ✅ 必做（核心卖点） |
| FR-010 | SKILL 手册 | ✅ 必做（差异化武器） |
| FR-011 | 自然语言问答 | ✅ 必做（核心卖点） |
| FR-012 | 双引擎检索路由 | ⚠️ 建议 v1 仅做"ES + SAG 并存 + 手动路由"，自动路由推迟到 v1.1 |
| FR-013 | 文档多格式解析 | ⚠️ v1 仅 PDF/Word/Markdown/HTML，其他推迟到 v1.1（与 §3 表格一致） |
| FR-014 | 多租户与 RBAC | ✅ 必做（沿用 WeKnora） |
| FR-015 | 可观测性 | ⚠️ 建议 v1 仅 Langfuse trace，Grafana 推迟到 v1.1 |
| FR-016 | CLI 输出 AI-Friendly | ✅ 必做（核心卖点） |
| FR-017 | 从 losu-wiki 迁移 | ⚠️ P2 标记合理——`10-roadmap.md §4 (6.8)` 已放 v1.1 |

**调整后的 v1 必交付（精简版）**：
- 文档模型 + 版本链 + docCode + CAS
- CLI（init/bind/pull/push/read/version）+ SKILL
- MCP（search/read/write/edit/ask + version 三个）
- 检索：ES（默认）+ SAG（手动路由，简单查询由用户/Agent 选）
- 鉴权：HMAC + JWT
- RBAC：4 级
- 解析：4 种核心格式
- 部署：Docker Compose

### 3.2 移除/推迟项建议

| 项 | 当前定位 | 建议 |
|----|---------|------|
| FR-005 move/undelete | P1 | 保留 P1，进入 v1.1 |
| FR-012 自动 query classifier | P0 | 拆为 v1.1（v1 仅 ES-only 默认）|
| FR-015 Grafana | 隐含 P0 | 推迟到 v1.1 |
| FR-017 迁移工具 | P2 | 推迟到 v1.1（与 10-roadmap §6.8 一致）|

### 3.3 "借鉴而非复用"的合规边界

**新增 ADR-006 建议**：在 `00-context.md` 增加"借鉴合规声明"：

> **ADR-006：losu-ai-wiki 借鉴合规边界**
>
> **决策**：ai-kbs CLI 设计借鉴 losu-wiki CLI 的"工作流范式、命令命名灵感、输出协议格式"，但**不复制任何源文件**，不重用任何 go 文件内容。
>
> **审计机制**：
> 1. CI 阶段扫描 `cli/` 与 losu-wiki CLI 的源码指纹（字符串 / 包名 / 函数名）相似度 > 30% 触发告警；
> 2. LICENSE 中明确声明 "no source code from losu-ai-wiki is reused"；
> 3. `docs/adr/ADR-006-losu-wiki-borrowing.md` 记录"借鉴 vs 复用"对照表。
>
> **可借鉴清单**（不含任何 losu 私有实现）：
> - 工作空间管理 init/bind/unbind/pull/push 命令范式（业界 GitOps 通用）
> - `wiki read/write/edit` 的 CAS 乐观锁模式（业界通用）
> - `--json` envelope + `next_action` 字段（业界通用协议）
> - 删除护栏阈值设计（业界通用最佳实践）
> - SKILL.md 操作手册结构（openai/Claude Code 通用文档范式）

---

## 4. 实施性评估

### 4.1 资源估算

**`10-roadmap.md §7` 计划**：5 人 / 12 周完成 v1.1

**评审意见**：⚠️ 偏激进

| 模块 | 文档估算 | 评审调整 | 理由 |
|------|---------|---------|------|
| Phase 0（基础设施）| 6.5 人天 | 8 人天 | +1.5 (Langfuse 本地化 + Helm 测试集群) |
| Phase 1.1（后端 + CLI 基础） | 17.5 人天 | 22 人天 | +4.5 (WeKnora fork 剥离 wiki 改造) |
| Phase 1.2（CLI 基础）| 7.5 人天 | 9 人天 | +1.5 (losu-wiki 范式 SKILL 编写) |
| Phase 1.3（同步引擎） | 7 人天 | 9 人天 | +2 (rename heuristic 调优) |
| Phase 1.4（检索 + Agent） | 8 人天 | 11 人天 | +3 (SAG 集成 + MQ 同步链) |
| Phase 1.5（MCP + 集成） | 7.5 人天 | 10 人天 | +2.5 (MCP OAuth + BDD 场景) |
| Phase 1.1（增强）| 7.5 人天 | 9 人天 | +1.5 (Web UI 调研) |
| **总计** | **~60 人天** | **~78 人天** | +30% buffer |

按 5 人 / 5 工作日/人 = **约 3.2 周 buffer** → 原 12 周计划建议延长到 **14-15 周**，或**精简 v1 scope**（按 §3.1 调整建议）。

### 4.2 关键路径与依赖

```
Phase 0 (fork + docker-compose)
   ↓
Phase 1.1 (PG schema + 鉴权)
   ↓
Phase 1.2 (CLI 基础 8 命令) ←── 关键路径（决定 demo 是否能跑）
   ↓
Phase 1.3 (同步引擎)
   ↓
Phase 1.4 (ES + SAG) ←── 关键路径（决定多跳问答 SOTA）
   ↓
Phase 1.5 (MCP)
   ↓
GA v1.0.0
```

**并行机会**：
- Web UI 可以在 Phase 1.2 后并行启动（不阻塞后端）
- WeKnora fork 剥离可以由独立工程师在 Phase 0-1.1 持续做
- SAG 集成可以与 ES 集成并行（两路工程师）

### 4.3 风险与缓解

| 风险 | 评级 | 已有缓解 | 评审补充 |
|------|------|---------|---------|
| WeKnora 上游 breaking change | 高 | Pin + fork 维护 | ✅ 已合理 |
| SAG 论文复现误差 | 中 | 用官方 CLI 验证 | ⚠️ 建议增加 SAG 集成回归测试集（10 个固定 query） |
| 中文分词效果 | 中 | 备选 HanLP / Jieba | ✅ 已合理 |
| **losu-wiki 借鉴合规** | **新风险** | 无 | ⚠️ 需新增 ADR-006 + 自动审计 |
| **fork WeKnora 剥离 wiki 工作量低估** | 新风险 | 无 | ⚠️ 预留 2 周剥离 buffer |
| **MCP server 单 vs 多进程** | 新风险 | 无 | ⚠️ 先实现单文件 2000 行版本，v1.1 再分模块 |
| SAG 部署失败导致整体不可用 | 中 | 缺 | ⚠️ 必须在路由层实现"ES-only fallback"，不让 SAG 单点阻塞 |

---

## 5. 实施路径建议（调整后）

### 5.1 调整后的里程碑

| 阶段 | 时间 | 关键交付 |
|------|------|---------|
| Phase 0 | Week 1-2 | fork + license + docker-compose + ADR-006 |
| Phase 1.1 | Week 3-4 | PG schema + 鉴权 + WeKnora wiki 剥离 |
| Phase 1.2 | Week 5-6 | CLI 8 命令 + SKILL.md + envelope schema v1.0 |
| Phase 1.3 | Week 7-8 | 同步引擎（pull/push） |
| Phase 1.4 | Week 9-10 | ES + SAG（默认 ES，SAG 可选）+ Agent |
| Phase 1.5 | Week 11-12 | MCP server + 集成测试 + BDD |
| Phase 1.6 | Week 13 | v1.0 GA + 文档 |
| Phase 1.7 | Week 14-17 | v1.1（Web UI / move/undelete / 自动路由 / Grafana） |

**总计**：v1.0 在 13 周 / v1.1 在 17 周（原计划 12 周 v1.1 → 调整后 17 周）

### 5.2 立即可执行的下一步

1. **补 ADR-006**：losu-wiki 借鉴合规声明（1 人天）
2. **修文档小问题**：列 `04-architecture.md` 中 CLI 路径与 WeKnora 实际不一致（0.5 人天）
3. **起 fork 仓库**：从 WeKnora v0.7.1 fork + 改名 + 加 LICENSE（0.5 人天）
4. **决策 Q1-Q6**：在 Week 1 末前决策完（影响 §3 调整）
5. **MVP 切片**：先做"无 ES + 无 Web 的 CLI + Backend + SAG-only"作为 Week 4 的 demo 验证，验证三源整合可行性。

---

## 6. 待解决问题（评审中识别，需在 Phase 0 决策）

| # | 问题 | 默认答案 | 决策时点 |
|---|------|---------|---------|
| Q7 | 是否在 v1 内做 WeKnora wiki 全部剥离？ | 是（fork 后立刻做） | Phase 0 |
| Q8 | MCP server 单文件 vs 多模块？ | 单文件起步（2000 行内），按需拆 | Phase 0 |
| Q9 | SAG 不可用时降级到？ | ES-only（保留 RAG 核心） | Phase 1.4 |
| Q10 | 是否需要"无 SAG 单引擎"部署模式？ | 是（适配中小团队） | Phase 1.4 |
| Q11 | losu-wiki 借鉴的"参照 vs 复用"如何审计？ | CI 字符串相似度扫描 + LICENSE 声明 | Phase 0 |
| Q12 | 是否在 v1 内支持多租户？ | 是（沿用 WeKnora） | Phase 1.1 |
| Q13 | 是否需要 v0.9 内测版？ | 建议加（Week 11 内测，Week 13 GA） | Phase 1.5 |
| Q14 | Helm Chart 复杂程度？ | 基础（与 WeKnora 一致），高级配置推迟 | Phase 0 |

---

## 7. 总结

### 7.1 评审结论

**方案整体可行，技术选型真实可靠，架构合理，文档完整度高。**

具体优势：
- ✅ WeKnora v0.7.1 是真实可 fork 的 Go 主基座，模块化程度满足二次开发
- ✅ SAG v1.8.6 是真实的 SOTA 检索引擎，集成路径明确
- ✅ losu-ai-wiki 的设计模式（SKILL / 寻址 / envelope / 鉴权链 / CAS）可在不复制源码的前提下完全借鉴
- ✅ 架构分层（MCP / Backend / PG+ES+SAG / Object Storage）符合业界最佳实践
- ✅ 12 周 roadmap 节奏对 5 人团队可行但偏紧，建议加 30% buffer

### 7.2 必须解决的问题（阻塞 GA）

1. **ADR-006：losu-wiki 借鉴合规声明**（必须）
2. **CLI 路径与 WeKnora 实际对齐**（必须）
3. **v1 scope 精简**（建议）——按 §3.1 调整，自动 query classifier 推迟到 v1.1
4. **MCP server 实现策略**（建议）——先单文件，按需分模块
5. **ES-only fallback**（必须）——SAG 不可用时不能阻塞整体

### 7.3 不影响 GA 的优化项

- Helm Chart 高级配置 → v2
- OCTX 导入导出 → v2
- 多 IM 集成 → v2
- WeChat Mini Program → v2

---

## 附录 A：本次评审扫描的代码与文件清单

### A.1 WeKnora（公开 MIT-like Tencent License，已验证）

扫描路径：
- `e:\my-project\WeKnora-main\VERSION` → 0.7.1
- `e:\my-project\WeKnora-main\go.mod` → module github.com/Tencent/WeKnora; go 1.26.0
- `e:\my-project\WeKnora-main\LICENSE` → Tencent 自定义许可
- `e:\my-project\WeKnora-main\internal\` → 27 个子包
- `e:\my-project\WeKnora-main\internal\agent\engine.go` → 730 行
- `e:\my-project\WeKnora-main\internal\agent\tools\` → 20+ 工具
- `e:\my-project\WeKnora-main\internal\agent\skills\` → Progressive Disclosure
- `e:\my-project\WeKnora-main\internal\container\` → DI 容器
- `e:\my-project\WeKnora-main\internal\router\` → HTTP 路由
- `e:\my-project\WeKnora-main\internal\mcp\` → MCP client/manager/oauth
- `e:\my-project\WeKnora-main\cli\cmd\` → CLI 子命令（agent/api/chat/doc/kb/search/wiki 等）
- `e:\my-project\WeKnora-main\mcp-server\` → Python MCP 单进程
- `e:\my-project\WeKnora-main\docreader\` → Python gRPC 文档解析
- `e:\my-project\WeKnora-main\migrations\` → mysql/paradedb/sqlite/versioned

### A.2 SAG（公开 MIT，已验证）

扫描路径：
- `e:\my-project\SAG\README.md` → v1.8.6（PyPI 徽章确认）
- `e:\my-project\SAG\LICENSE` → MIT
- `e:\my-project\SAG\apps\api\sag_api\` → API 服务（db / jobs / mcp / generation / services / tools / octx / runtime / sag）
- `e:\my-project\SAG\apps\api\sag_agent\` → SAG 内部 Agent（loop / runtime / tools / store / types）
- `e:\my-project\SAG\compose.yaml` → 部署（API + Web）
- `e:\my-project\SAG\compose.dify.yaml` → Dify 集成
- `e:\my-project\SAG\compose.postgres.yaml` → Postgres 可选后端
- `e:\my-project\SAG\docs\` → 文档（assets / sag-cli.en.md / dify-integration / sag-cli.md）

### A.3 losu-ai-wiki（公司私有，未读源码，仅读 SKILL.md 设计文档）

扫描路径（仅文档与文件清单）：
- `e:\my-project\losu-ai-wiki\losu-wiki-cli\go\` → CLI 包目录结构（cli / workspace / auth / resolve / config / update / output 七组）
- `e:\my-project\losu-ai-wiki\losu-wiki-cli\skills\SKILL.md` → v1.3.0 设计文档（已读）
- `e:\my-project\losu-ai-wiki\losu-wiki-cli\skills\references\*.md` → 18 个 reference（搜索/移动/绑定/恢复/绘图/读取/登录/初始化/解决/版本/写入/同步/技能/登出/树/grep/编辑/升级/空间）

**未读源码文件**（遵循"仅参考借鉴"约束）：
- `losu-ai-wiki\losu-wiki-cli\go\internal\**\*.go`（全部）
- `losu-ai-wiki\losu-wiki-system\src\**\*.java`（未扫描）
- `losu-ai-wiki\losu-wiki-web\src\**`（未扫描）

---

> 文档结束 | 建议将此报告归档至 `e:\my-project\my-open-source\ai-kbs\require\REVIEW-REPORT.md` 并据此迭代 00-context.md（新增 ADR-006）和 10-roadmap.md（调整里程碑与缓冲）