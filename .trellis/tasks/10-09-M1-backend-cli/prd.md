# M1 后端核心 + CLI 基础 + Frontend 脚手架 — 需求文档

> 父任务: `10-02-ai-kbs-mvp`
> 配套计划: `../10-02-ai-kbs-mvp/implement.md` §3 (Phase M1)
> 状态: planning
> 文档版本: v1.0 — 2026-10-09

---

## 1. 背景

M0 已完成基础设施 (CI / Docker / Helm / golangci / VitePress / Langfuse / Makefile)。M1 阶段把 WeKnora fork 改造为 AI-KBS 自有代码,在仓库内交付生产级后端核心 + CLI 工具集 + Frontend 脚手架。

M1 横跨三大模块,共 24 个子任务 (T1.1–T1.24)。每个子项的 输入 / 动作 / 输出 / 依赖 / 验收 / 风险 已在父 `implement.md §3` 中详细列出,本文档把"做什么 / 不做什么 / 怎么算做完"凝练成可验收的合同。

---

## 2. 范围 / 非范围

### 2.1 In-Scope (本任务交付)

| 模块 | 子任务 | 关键产出 |
|------|--------|----------|
| **Backend Core** | T1.1 – T1.9, T1.21 | 13 张核心表 schema; 13 个 repository; docCode 8位 base62; CAS 乐观锁; HMAC+JWT+API Key 三模鉴权; RBAC 4 级矩阵; 错误码 envelope v1.0; Langfuse OTLP 集成; Prometheus 6 类指标 |
| **CLI 工具集** | T1.10 – T1.20 | Cobra CLI 骨架; 5 级凭证链; envelope 解析 + text/json/ndjson 输出; 13 个子命令 (login/logout/init/status/bind/unbind/read/write/edit/version/move/resolve/pull/push); sync-state.json schema v1.0; SKILL.md + 6 个 reference |
| **Frontend 脚手架** | T1.22 – T1.24 | Vue 3 + Vite 5 + Pinia 2 + Vue Router 4 + TS 脚手架; 品牌白标化 (logo/色/i18n/CDN/SSO 替换); axios API client + Pinia auth store + Router 守卫 |

### 2.2 Out-of-Scope (本任务不做,留给 M2 / M3)

- 三层检索 (ES BM25/Dense/Hybrid) — M2
- WeKnora RAG / SAG agent runtime — M2
- MCP server 业务逻辑 — M2 (基础设施 M0 已交付)
- 完整前端视图 (knowledge CRUD / agent chat / dashboard) — M2
- GA 验收 / 性能压测 / 多平台 release — M3

---

## 3. 验收标准

### 3.1 必交付 (Acceptance) — Backend Core

- [ ] **AC-B1** `make migrate-up` 创建 13 张核心表 + ~25 索引 (T1.1)
- [ ] **AC-B2** `go test ./internal/repository/...` 全绿,覆盖率 ≥ 80% (T1.2)
- [ ] **AC-B3** docCode 生成器 100 万次 0 碰撞,100 并发协程 10 万次 0 重复 (T1.3)
- [ ] **AC-B4** CAS 100 并发同一文档:1 成功 + 99 收到 `ErrVersionConflict` (T1.4)
- [ ] **AC-B5** 鉴权中间件单元测试 ≥ 100 case (HMAC 签/时间窗/nonce 防重放/JWT 过期/API Key capability) (T1.5)
- [ ] **AC-B6** RBAC 4 角色 × 10 action 矩阵测试 40 case 全通过 (T1.6)
- [ ] **AC-B7** 所有错误响应包含 `{success, exit_code, error:{code,message}, data, request_id}` (T1.7)
- [ ] **AC-B8** 任意 HTTP 请求在 Langfuse dashboard 可见完整 span tree (T1.8)
- [ ] **AC-B9** `go test ./internal/auth ./internal/rbac ./internal/doc ./internal/repository` 覆盖率 ≥ 85% (T1.9)
- [ ] **AC-B10** `curl /metrics` 输出 6 类 Prometheus 指标,QPS/延迟/错误率/队列深度/LLM token/缓存命中率 (T1.21)

### 3.2 必交付 — CLI

- [ ] **AC-C1** `ai-kbs --version` 输出 `1.0.0`;`--help` 列出 13 个子命令 (T1.10)
- [ ] **AC-C2** 凭证 5 级优先级链测试全通过;`key_secret` 永不打印到 stdout (T1.11)
- [ ] **AC-C3** `--format json|text|ndjson` 三种输出 + 错误码透传 (T1.12)
- [ ] **AC-C4** login/init/status/bind/unbind 端到端在 dev 环境跑通 (T1.13)
- [ ] **AC-C5** read/write/edit 含 CAS 冲突时返回 exit 22 + `next_action` (T1.14)
- [ ] **AC-C6** version list/diff/rollback 端到端;rollback 产生新版本 (T1.15)
- [ ] **AC-C7** move/undelete/resolve 端到端 (T1.16)
- [ ] **AC-C8** SKILL.md 通过 Claude Code 实读执行 A–H 工作流 **8/8 全过** (T1.17)
- [ ] **AC-C9** sync-state.json JSON Schema 校验文件 + 1000 文件并发读写无损坏 (T1.18)
- [ ] **AC-C10** pull 1000 文件 < 60s (NFR-PERF-04);rename 路径不破坏引用;partial failure exit 23 (T1.19)
- [ ] **AC-C11** push CAS 冲突路由到 4 策略 (`manual/keep-mine/keep-theirs/both/auto`);残留冲突 exit 24 (T1.20)

### 3.3 必交付 — Frontend

- [ ] **AC-F1** `cd frontend && npm install && npm run dev` 起成功;`npm run build` 产出 dist;`npm run lint` 零警告 (T1.22)
- [ ] **AC-F2** `npm run build:analyze` 首屏 chunk < 200KB gzipped (NFR-FE-01) (T1.22)
- [ ] **AC-F3** `browserslist` 实际匹配 Chrome 100+/Firefox 100+/Safari 15+/Edge 100+ (NFR-FE-05) (T1.22)
- [ ] **AC-F4** 浏览器访问任意页面无公司专有字样;`make scrub-audit` 0 命中 (T1.23)
- [ ] **AC-F5** 登录后 `/api/ai-kbs/auth/status` 200 返回当前用户;Pinia 持有 token;路由守卫拦截未登录 (T1.24)

### 3.4 集成验收

- [ ] 24 个 commit 全部推送到 `feature/M1` (或合理子分支)
- [ ] `cd backend && golangci-lint run --config ./.golangci.yml ./...` 退出码 0
- [ ] `cd frontend && npm run lint && npm run type-check` 退出码 0
- [ ] `make help` 仍然列出全部 M0 + M1 target
- [ ] 父 `10-02-ai-kbs-mvp` 的 prd.md / design.md 中 T1.1–T1.24 各项状态从"未开始"标记为"已交付"
- [ ] 无任何 [内部 Wiki 系统] / 钉钉 / 飞书 / 企微 / 咕噜 / 上游内网名 出现在新增文件中 (CI scrub-audit 阻断)

---

## 4. 风险与缓解

| 风险 | 等级 | 缓解 |
|------|------|------|
| pgvector HNSW 索引在 PG 15 上需要 pgvector 0.5+ | 中 | docker-compose 镜像预装;`make migrate-up` 失败时显式提示 pgvector 版本 |
| pgx vs database/sql 选型 | 低 | 统一 pgx v5;新代码全部 pgx;旧 database/sql 代码逐步迁移 |
| HMAC 实现错位导致任意签名通过 | 高 | 单元测试覆盖正向 + 负向 ≥ 100 case;Fuzz testing 单独跑 |
| 24 子任务串行风险 (任一失败阻塞后续) | 中 | 任务间显式依赖图 (父 implement.md §3);子任务可并行派发 (trellis-implement + trellis-check) |
| Frontend 白标化不彻底残留公司字样 | 高 | `make scrub-audit` CI gate;T1.23 验收前必跑 |
| CLI SKILL.md 8/8 工作流不可达 | 中 | 验收需 Claude Code 实读执行 (非 Trellis 子代理);任一失败即返 rework |
| sync-state.json 1000 文件并发损坏 | 中 | 文件锁 + 原子 rename;JSON Schema 校验 + 单元测试故意损坏触发加载失败 |
| WeKnora fork 中已有 HMAC/RBAC/envelope 代码,需决策复用 vs 重写 | 中 | 默认复用并扩展;只在不满足 AI-KBS 验收阈值时重写;评估时点为 T1.5/T1.6/T1.7 启动前 |

---

## 5. 实施顺序 (高阶)

按依赖图大致分 3 阶段,可并行派发:

1. **Wave 1** (DB + repo + CAS + docCode) — T1.1, T1.2, T1.3, T1.4
2. **Wave 2** (auth + RBAC + envelope + OTLP + metrics) — T1.5, T1.6, T1.7, T1.8, T1.21
3. **Wave 3** (CLI 骨架 → 命令集 → sync → SKILL) — T1.10–T1.20 (CLI 串行依赖较多)
4. **Wave 4** (Frontend 脚手架 → 白标化 → API client) — T1.22, T1.23, T1.24

详细依赖见 `design.md` 拓扑图。

---

## 6. 完成定义 (Definition of Done)

- [ ] 24 个 commit 全部推送并 merge 到父分支
- [ ] `make migrate-up` 在 dev CI 跑通;`make test-unit` 覆盖率 ≥ 85% (core 包)
- [ ] `make scrub-audit` 0 命中
- [ ] 父 `10-02-ai-kbs-mvp` 的总览状态更新到 2/4 里程碑完成
- [ ] `.trellis/spec/backend/` 至少 3 个 spec 文件 (database / errors / observability) 被实际填充
- [ ] `.trellis/spec/frontend/` 至少 2 个 spec 文件被实际填充
- [ ] M1 task 归档到 `archive/2026-10/`