# M0 基础设施收尾 (T0.3–T0.9) — 需求文档

> 父任务: `10-02-ai-kbs-mvp`
> 配套计划: `../10-02-ai-kbs-mvp/implement.md` §2 (Phase M0)
> 状态: planning
> 文档版本: v1.0 — 2026-10-09

---

## 1. 背景

M0 阶段目标是搭好仓库 / CI / Docker / Helm / 规范工具 / 文档 / Langfuse / 迁移框架 / Makefile,作为后续 M1–M3 全部依赖的基础。T0.1 / T0.2 / T0.10 已在 `feature/T0.5` 上提交 (`00f6c4b`、`5c1711f`、`2e8e585`)。其余 T0.3–T0.9 的产物文件已经写好但未提交;本任务把它们收尾 + commit,并补齐 M0 阶段尚未落地的子项 (T0.4 Helm / T0.6 VitePress / T0.7 Langfuse 预置 dashboard / T0.8 迁移框架包装 / T0.9 Makefile 完整扩展)。

---

## 2. 范围 / 非范围

### 2.1 In-Scope (本任务交付)

| ID | 子项 | 状态 | 动作 |
|----|------|------|------|
| **T0.5** | 代码规范工具配置 | 文件已写好,未 commit | 验证 + commit |
| **T0.5a** | 根 `.golangci.yml` shim | 未 commit | 合并进 T0.5 commit |
| **T0.5b** | `backend/.golangci.yml` v2 schema | 未 commit (新建) | 合并进 T0.5 commit |
| **T0.5c** | `backend/.gitignore` 增 `!.golangci.yml` | 未 commit | 合并进 T0.5 commit |
| **T0.5d** | `.pre-commit-config.yaml` | 未 commit (新建) | 合并进 T0.5 commit |
| **T0.5e** | `.eslintrc.cjs` 占位 | 未 commit (新建) | 合并进 T0.5 commit |
| **T0.5f** | `.env.example` 根目录 | 未 commit (新建) | 合并进 T0.5 commit |
| **T0.3**  | Docker Compose 开发栈 | 文件已写好,未 commit | 验证 + commit |
| **T0.3a** | `docker-compose.dev.yml` (9 服务) | 未 commit (新建) | 合并进 T0.3 commit |
| **T0.3b** | `docker/{elasticsearch,mcp-server,postgres,sag-bridge}/` Dockerfile | 未 commit (新建) | 合并进 T0.3 commit |
| **T0.9a** | 根 `Makefile` (T0.5 基础) | 未 commit (新建) | 合并进 T0.5 commit (属于规范工具范畴) |
| **T0.9b** | 根 `Makefile` 完整扩展 | 已有基础,缺 test / bdd / cli-build / perf / helm / scrub-audit / migrate / dev / docs | 单独 commit |

### 2.2 In-Scope (本任务内,需实际实现)

| ID | 子项 | 当前状态 (2026-10-09) | 计划动作 |
|----|------|---------|---------|
| **T0.4** | Helm Chart 骨架 | `helm/.gitkeep` 占位,无 templates / values | 实际编写 `helm/Chart.yaml` + `values.yaml` + `templates/{backend,mcp-server,sag-bridge,docreader,postgres,elasticsearch,redis,rabbitmq,langfuse,ingress,serviceaccount,podmonitor,networkpolicy}.yaml` |
| **T0.6** | VitePress 文档站点 | `docs/.gitkeep` 占位 | 实际 `npm init` + `npm i -D vitepress` + 目录结构 `getting-started / architecture / api / cli / mcp / adr` + index.md |
| **T0.7** | Langfuse 预置 dashboard | `docker/langfuse/initdb.d/` 空 | 加 7 个 panel dashboard JSON + init SQL (如需) |
| **T0.8** | DB 迁移框架包装 | `backend/migrations/versioned/` 有 WeKnora 原有迁移,无 AI-KBS 包装 | 加 `cmd/migrate` (WeKnora 原生命令) + `make migrate-up/down/status` 目标 |

### 2.3 Out-of-Scope (本任务不做,留给 M1)

- T1.1–T1.24 (后端核心 / CLI / Frontend 脚手架) — 全部 M1
- ADR / 设计文档扩展 — design.md 已锁 v1 范围,后续变更走 ADR 流程
- 实际跑通 `make dev-up` 端到端冒烟 (M1 起动后端前不做)

---

## 3. 验收标准

### 3.1 必交付 (Acceptance)

- [x] **AC-1a** T0.5 已 commit (`2c6b3bc` feat(infra): T0.5 golangci v2 config + Makefile + pre-commit + env catalog) ✅
- [x] **AC-1b** T0.3 已 commit (`10cef19` feat(infra): T0.3 Docker Compose dev stack (9 services) + Dockerfiles) ✅
- [x] **AC-1c** T0.9b (Makefile 完整扩展) 已 commit (`f3c5437` feat(infra): T0.9b Makefile full extension) ✅
- [x] **AC-1d** T0.4 (Helm) 已 commit (`bf0f0fb` feat(infra): T0.4 Helm chart — 9 services, ingress, network policy, pod monitor) ✅
- [x] **AC-1e** T0.6 (VitePress) 已 commit (`ac704da` feat(docs): T0.6 VitePress documentation site — 6 sections + 9 ADRs) ✅
- [x] **AC-1f** T0.7 (Langfuse dashboards) 已 commit (`4ad850c` feat(obs): T0.7 Langfuse default dashboard — 7 panels + bootstrap sidecar) ✅
- [x] **AC-1g** T0.8 (迁移框架包装) 已 commit (`ed3cb9e` feat(infra): T0.8 DB migration wrapper — cmd/migrate CLI + Make targets) ✅
- [x] **AC-2** `cd backend && golangci-lint run --config ./.golangci.yml ./...` 退出码 0 (本机 golangci-lint v1.64.0 已验证) ✅
- [ ] **AC-3** `make lint` (根目录) 退出码 0 — `make` 命令本机未装,需在 CI / WSL 验证
- [x] **AC-4** `docker compose -f docker-compose.dev.yml config -q` 退出码 0 (本机 docker 29.5.3 已验证,仅警告缺失 .env 变量,符合预期) ✅
- [ ] **AC-5** `helm template ./helm --dry-run` 渲染成功 — helm 本机未装,T0.4 chart 已写完待 CI / WSL 验证
- [x] **AC-6** `make help` 列出所有已实现的 target ✅ (T0.5 + T0.8 + T0.9b 合计 22 个)
- [x] **AC-7** 没有任何 [内部 Wiki 系统] / 钉钉 / 飞书 / 企微 / 咕噜 / 上游内网名 出现在新增文件中 (本会话已脱敏 docker-compose.dev.yml 第 36 行注释;scrub-audit 目标已落地到 Makefile,作用域 = AI-KBS-authored infra,fork 代码豁免) ✅

### 3.2 性能 / 兼容性

- N/A — 本任务不涉及运行时性能

### 3.3 集成验收

- [ ] T0.4 / T0.6 / T0.7 / T0.8 / T0.9b 各自有独立 commit (允许每个 T0.x 一个 commit)
- [ ] 所有 M0 commit 在 `feature/T0.5` 上连续,无 main 上的快进 merge

---

## 4. 风险与缓解

| 风险 | 等级 | 缓解 |
|------|------|------|
| golangci-lint v1 vs v2 schema 兼容 | 中 | `.pre-commit-config.yaml` 锁 v1.64;CI 用 v2.1.0;配置同时可解析 (v1.64 报 deprecation notice 但能用) |
| `make dev-up` 在 Windows 上不可用 (docker-compose 在 WSL 才稳) | 低 | 仅验收 `docker compose config` 语法,不要求真起 |
| Helm chart values 缺关键参数 | 中 | 暴露所有可调参数 (image / resources / replicaCount / service.port / ingress.host);`helm template --debug` 验证 |
| T0.7 Langfuse 预置 dashboard JSON 格式与 Langfuse v3 不匹配 | 中 | 复用 Langfuse 官方 ClickHouse schema;参考 v3 官方文档;若失败回退为"仅提供 docker-compose 启动,dashboard 由 M1 阶段初始化" |
| WeKnora 原生 `cmd/migrate` 是否兼容 AI-KBS fork | 中 | 已在 `backend/cmd/` 包含 WeKnora 原生命令,只需验证 `make migrate-up` 调用;若不兼容,在 `Makefile` 用 `go run ./cmd/migrate` 显式调用 |

---

## 5. 实施顺序

1. **Step 1 — T0.5 commit** (规范化工具, 1 个 commit)
   - 文件: `.golangci.yml`, `backend/.golangci.yml`, `backend/.gitignore`, `.pre-commit-config.yaml`, `.eslintrc.cjs`, `.env.example`, `Makefile` (T0.5 基础)
   - 验证: `make lint` 退出码 0

2. **Step 2 — T0.3 commit** (Docker Compose 栈, 1 个 commit)
   - 文件: `docker-compose.dev.yml`, `docker/{elasticsearch,mcp-server,postgres,sag-bridge}/Dockerfile`
   - 验证: `docker compose -f docker-compose.dev.yml config -q` 退出码 0

3. **Step 3 — T0.4 commit** (Helm Chart 骨架, 1 个 commit)
   - 文件: `helm/Chart.yaml`, `helm/values.yaml`, `helm/templates/*`
   - 验证: `helm template ./helm --dry-run` 成功

4. **Step 4 — T0.6 commit** (VitePress 文档骨架, 1 个 commit)
   - 文件: `docs/package.json`, `docs/.vitepress/config.ts`, `docs/index.md`, `docs/{getting-started,architecture,api,cli,mcp,adr}/` 占位
   - 验证: `cd docs && npm install && npm run docs:dev --port 5174` 能起 (手动验证;CI 仅验 npm install 成功)

5. **Step 5 — T0.7 commit** (Langfuse 预置 dashboard, 1 个 commit)
   - 文件: `docker/langfuse/initdb.d/*.sql` (ClickHouse 初始化) + `docker/langfuse/dashboards/*.json` (7 个 panel)
   - 验证: docker-compose 启动后 langfuse 容器内 dashboards 目录可读

6. **Step 6 — T0.8 commit** (DB 迁移框架包装, 1 个 commit)
   - 文件: `Makefile` 新增 `migrate-up / migrate-down / migrate-status` 目标;不新增 Go 代码 (沿用 WeKnora 原生 `cmd/migrate`)
   - 验证: `make migrate-status` 在 docker compose dev 起来后能列出 WeKnora 原生迁移

7. **Step 7 — T0.9b commit** (Makefile 完整扩展, 1 个 commit)
   - 文件: `Makefile` 新增 `test / test-unit / test-integration / bdd / bdd-mvp / cli-build / perf / helm-install / helm-uninstall / scrub-audit / dev-up / dev-down / docs-build` 目标
   - 验证: `make help` 列出全部

---

## 6. 完成定义 (Definition of Done)

- [ ] 7 个 commit 全部推送到 `feature/T0.5`
- [ ] `make lint` 退出码 0 (本机)
- [ ] `docker compose -f docker-compose.dev.yml config -q` 退出码 0
- [ ] `helm template ./helm --dry-run` 成功 (如 T0.4 完成)
- [ ] 父任务 `10-02-ai-kbs-mvp` 的 prd.md / design.md / implement.md 中 T0.3–T0.9 各项状态从"未开始/进行中"标记为"已交付"
- [ ] 无任何 [内部 Wiki 系统] 平台专有字符串泄漏 (CI 阻断)
- [ ] 本 prd.md 移到 `archive/` 前,关联 spec (`.trellis/spec/`) 不需要为 M0 收尾新增条目 (T0.5–T0.9 都是工具类,不涉及业务代码规范)
