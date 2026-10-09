<!-- ai-kbs:WORKFLOW-ROUTING:START -->
# ai-kbs 项目工作流路由约束

本项目使用 **Trellis 作为骨架**，外挂 Superpowers/dev-orchestrator 填补缺口。
所有 AI 会话必须按以下规则路由 skill，避免选错。

## 工作流骨架（Trellis）
- 规划 → `trellis-brainstorm`
- 实施 → `trellis-implement`（sub-agent）或 `trellis-before-dev`（inline）
- 验证 → `trellis-check`
- 调试 → `trellis-break-loop`
- 并行 → `trellis-channel`
- 归档 → `/trellis:finish-work`

## 显式外挂（强制）
| 场景 | 必须调用的 skill | 理由 |
|------|------------------|------|
| TDD 强制 | `/superpowers:test-driven-development` | Trellis 无原生 TDD |
| 代码评审 | `/superpowers:requesting-code-review` | Trellis 无 PR Review skill |
| 架构评审 | `dev-orchestrator/04-expert-review` | 多专家并行对抗评审 |
| 一页纸概要 | `dev-orchestrator/01-spec-summary` | 28KB PRD 给领导拍板用 |
| 决策矩阵 | `dev-orchestrator/02-decision-capture` | 多方案拍板记录 |
| 深度根因 | `/superpowers:systematic-debugging` | BUG 修复前必须 |

## Skill 冲突优先级
当多个 skill 适用同一意图时，按以下顺序选择：
1. **Trellis 内部**（保持工作流一致性）
2. **Superpowers**（基础纪律：TDD / 评审 / 调试）
3. **dev-orchestrator**（多端/一页纸/决策矩阵）

禁止：跳过 TDD 直接写实现；不经验证提交 PR；未走 expert-review 做架构变更。

## 单条规则
1. **写新代码前必先有测试**（TDD 红 → 绿 → 重构）
2. **PR 创建前必经 code-review**
3. **BUG 修复后必走 systematic-debugging + trellis-break-loop**
4. **涉及架构决策必走 expert-review（5 专家并行 spawn）**
5. **PRD 写完后必先 01-spec-summary 压一页纸给领导拍板**，再进实施
<!-- ai-kbs:WORKFLOW-ROUTING:END -->

<!-- ai-kbs:STAGE-COMMANDS:START -->
# ai-kbs 完整阶段命令流

## Phase 1: Plan（status=`planning`）

| 触发场景 | 命令 | 产物 |
|---|---|---|
| 需求复杂 / 新功能 | `/trellis-brainstorm` | prd.md + design.md + implement.md |
| PRD 写完 | dev-orchestrator/skills/01-spec-summary | one-pager.md |
| 多方案拍板 | dev-orchestrator/skills/02-decision-capture | decision-matrix.md |
| 架构变更 | dev-orchestrator/skills/04-expert-review | review-report.md |
| 启动任务 | `python .trellis/scripts/task.py start <task>` | status → in_progress |

## Phase 2: Execute（status=`in_progress`）

| 触发场景 | 命令 | 产物 |
|---|---|---|
| 首次写代码 | `/superpowers:test-driven-development` | failing test |
| 写实现 | `trellis-implement`（sub-agent）或 `trellis-before-dev`（inline） | code |
| 质量验证 | `trellis-check` | check-report.md |
| 并行任务 | `/trellis:channel spawn` | parallel workers |
| 回归监控 | `/loop 5m trellis-check` | watch report |
| PR 创建前 | `/superpowers:requesting-code-review` | review-report |

## Phase 3: Finish（status=`in_progress` 收尾）

| 触发场景 | 命令 | 产物 |
|---|---|---|
| BUG 复发 | `/superpowers:systematic-debugging` → `trellis-break-loop` | root-cause.md + prevention.md |
| 学到新约定 | `trellis-update-spec` | spec/*.md 更新 |
| 归档完成 | `/trellis:finish-work` | commit + archive |

## Trellis 与 Superpowers 协作规则

### 不冲突（顺序正确）

```
TDD (Superpowers 红) → Implement (Trellis) → Check (Trellis) → Review (Superpowers)
                                       ↑
                                  没有顺序歧义
```

### 冲突场景与解决

| 场景 | 冲突 | 解决 |
|---|---|---|
| Plan 阶段 | `trellis-brainstorm` vs `superpowers:brainstorming` | 选 Trellis（保持工作流一致）|
| 写代码 | TDD vs 直接 implement | **TDD 先行**（AGENTS.md 单条规则 1）|
| 验证 | `trellis-check` vs `superpowers:code-review` | check 先（自动），review 后（手动）|
| Debug | `trellis-break-loop` vs `superpowers:systematic-debugging` | systematic 先（方法论），loop 后（深度）|

### 单条规则

1. **写代码前先 TDD**（红→绿→重构）
2. **顺序固定**：`trellis-implement` → `trellis-check` → `/superpowers:requesting-code-review`
3. **debug 顺序固定**：`/superpowers:systematic-debugging` → `trellis-break-loop`
4. **架构变更必走 expert-review**（dev-orchestrator）
5. **PRD 写完后必先压一页纸给领导拍板**

## 监控与自动提交

| 场景 | 工具 |
|---|---|
| CI 看护 | `/loop 5m` "查 trellis-check 状态" |
| 长构建 | `ScheduleWakeup(270, "查 build")` |
| 测试通过自动 commit | `/trellis:finish-work`（内置验证 + commit）|
| 失败拦截 | `.git/hooks/pre-commit` + `trellis-check` |
<!-- ai-kbs:STAGE-COMMANDS:END -->

<!-- TRELLIS:START -->
# Trellis Instructions

These instructions are for AI assistants working in this project.

This project is managed by Trellis. The working knowledge you need lives under `.trellis/`:

- `.trellis/workflow.md` — development phases, when to create tasks, skill routing
- `.trellis/spec/` — package- and layer-scoped coding guidelines (read before writing code in a given layer)
- `.trellis/workspace/` — per-developer journals and session traces
- `.trellis/tasks/` — active and archived tasks (PRDs, research, jsonl context)

If a Trellis command is available on your platform (e.g. `/trellis:finish-work`, `/trellis:continue`), prefer it over manual steps. Not every platform exposes every command.

If you're using Codex or another agent-capable tool, additional project-scoped helpers may live in:
- `.agents/skills/` — reusable Trellis skills
- `.codex/agents/` — optional custom subagents

Managed by Trellis. Edits outside this block are preserved; edits inside may be overwritten by a future `trellis update`.

<!-- TRELLIS:END -->
