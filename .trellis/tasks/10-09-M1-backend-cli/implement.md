# M1 后端核心 + CLI 基础 + Frontend 脚手架 — 执行计划

> 配套需求: `prd.md` §3 验收标准
> 配套设计: `design.md` 模块边界 + 契约
> 父执行: `../10-02-ai-kbs-mvp/implement.md` §3 (T1.1–T1.24 详细定义)

---

## 1. Wave 划分 (依赖驱动的并行机会)

| Wave | 子任务 | 关系 | 可并行派发 |
|------|--------|------|------------|
| **W1: DB 基础** | T1.1 (DB 迁移), T1.2 (Repo), T1.3 (docCode), T1.4 (CAS) | 严格串行 (T1.1 → T1.2 → T1.3/T1.4) | ❌ 内部串行 |
| **W2: 鉴权 + 可观测** | T1.5 (HMAC+JWT+APIKey), T1.6 (RBAC), T1.7 (errors+envelope), T1.8 (Langfuse OTLP), T1.21 (Prometheus metrics) | T1.5 → T1.6; T1.7 与 T1.8 独立;T1.21 依赖 T1.8 | ✅ W2 内可 2-3 并发 |
| **W3: CLI 工具集** | T1.10 (CLI 骨架) → T1.11 (凭证) → T1.12 (envelope) → T1.13 (login 等) → T1.14 (read/write/edit) → T1.15 (version) → T1.16 (move/resolve) → T1.17 (SKILL.md) / T1.18 (sync-state schema) → T1.19 (pull) → T1.20 (push) | 严格链式 | ❌ 内部串行;可与 W4 并行 |
| **W4: Frontend 脚手架** | T1.22 (Vue 3 + Vite) → T1.23 (白标化) → T1.24 (API client + Pinia) | 严格链式 | ❌ 内部串行;可与 W3 并行 |
| **W5: 测试封板** | T1.9 (core 包单测 ≥ 85%) | 依赖 W1 + W2 全部完成 | ❌ 串行 |

**派发顺序:** W1 → (W3 与 W4 并行,W2 在 W1 完成后插入) → W5 收尾。

---

## 2. 每个子任务实施模板

每个 T1.x 落地时遵循 5 步:

```
1. 读 父 implement.md §3 此 T1.x 段 (输入/动作/输出/依赖/验收/风险)
2. 读 design.md 此 T1.x 相关契约章节
3. dispatch trellis-implement Agent:
     prompt 开头: "Active task: .trellis/tasks/10-09-M1-backend-cli"
     然后: T1.x 标题 + 父 implement.md 摘要 + 设计契约 + 验收阈值
4. implement Agent 自己 commit (per-T1.x 1 commit)
5. dispatch trellis-check Agent 验证 (AC + 契约 + scrub-audit + golangci-lint)
6. 如有 fix:implement Agent 修,check Agent 重跑,直到 PASS
```

---

## 3. 验证命令清单 (Phase 3.1 + 3.4 收尾)

| AC 维度 | 命令 | 期望 |
|---------|------|------|
| 后端编译 | `cd backend && go build ./...` | exit 0 |
| 后端 vet | `cd backend && go vet ./...` | exit 0 |
| 后端 lint | `cd backend && golangci-lint run --config ./.golangci.yml ./...` | exit 0 |
| 单测 | `cd backend && go test -count=1 -short ./...` | exit 0 |
| 覆盖率 (core) | `cd backend && go test -coverprofile=cov.out ./internal/auth ./internal/rbac ./internal/doc ./internal/repository && go tool cover -func=cov.out \| tail -1` | ≥ 85% |
| 迁移 | `make migrate-up` (依赖 dev-up) | 13 表 + ~25 索引创建 |
| 集成测试 | `make test-integration` (需要 `INTEGRATION=1`) | exit 0 |
| CLI 编译 | `make cli-build` | 产出 `./bin/clai` |
| CLI --version | `./bin/clai --version` | 输出 `1.0.0` |
| CLI help | `./bin/clai --help` | 列出 13 子命令 |
| Frontend 类型检查 | `cd frontend && npm run type-check` | exit 0 |
| Frontend lint | `cd frontend && npm run lint` | exit 0 |
| Frontend build | `cd frontend && npm run build` | 产出 dist/ |
| Frontend analyze | `cd frontend && npm run build:analyze` | 首屏 chunk < 200KB gzipped |
| Scrub 审计 | `make scrub-audit` | 0 命中 |

---

## 4. Commit 策略

| 维度 | 规则 |
|------|------|
| 每 T1.x 1 commit | `feat(backend): T1.x <title>` 或 `feat(cli): T1.x <title>` 或 `feat(frontend): T1.x <title>` |
| 不 squash | 每个 T1.x 独立 commit,可独立回滚 |
| 不破坏主线 | 所有 commit 在 `feature/M1` 分支,merge 时 fast-forward 或 PR 单 squash |
| spec 文件 | `.trellis/spec/backend/*.md` 的 fill 算 T1.07/T1.21 commit,不单独算 |
| 包 commit | W5 收尾时补一个 `chore(backend): M1 closeout — golangci clean + coverage report` 总结 |

---

## 5. 风险与回滚点 (per Wave)

| Wave | 风险 | 回滚策略 |
|------|------|----------|
| W1 | 13 表 schema 变更破坏 WeKnora fork 已有迁移 | T1.1 用 additive 设计 (建表不破坏);迁移失败 `make migrate-down` 回退;WeKnora 原 000000_init 不动 |
| W2 | HMAC 实现错位 (高危) | T1.5 单元测试覆盖 ≥ 100 case,任何 fail 不进 commit;若 CI 失败整体回滚 W2 |
| W3 | CLI SKILL.md 8/8 工作流不可达 | T1.17 验收需 Claude Code 实读,任一工作流失败返 rework;若反复 rework,降级为 6/8 工作流并 ADR 记录原因 |
| W4 | Frontend 白标化不彻底 (高危) | T1.23 后立即跑 `make scrub-audit`;1 命中即修复直到 0 命中 |
| W5 | 覆盖率不达标 | T1.9 验收阈值 85% 强;不足则补测试,不降阈值 |

---

## 6. Review 门控 (trellis-check 必跑项)

每个 T1.x commit 后:
- spec compliance (与父 implement.md §3 输入/动作/输出/验收一致)
- 契约一致 (envelope schema / CAS 接口 / RBAC 矩阵)
- golangci-lint / go vet / go test (后端) 或 npm run lint (前端)
- scrub-audit (新增文件 0 命中)
- commit message 符合 feat(backend|cli|frontend): T1.x <title>

---

## 7. 完成定义 (Definition of Done) — 拉取自 prd.md §6

- 24 个 commit 全部推送并 merge 到父分支
- `make migrate-up` 在 dev CI 跑通;`make test-unit` 覆盖率 ≥ 85% (core 包)
- `make scrub-audit` 0 命中
- 父 `10-02-ai-kbs-mvp` 总览状态更新到 2/4 里程碑完成
- `.trellis/spec/backend/` 至少 3 个 spec 文件 (database / errors / observability) 被实际填充
- `.trellis/spec/frontend/` 至少 2 个 spec 文件被实际填充
- M1 task 归档到 `archive/2026-10/`