# AI-KBS Auto-Orchestrator Design — _index

## Context

**Problem.** `.trellis/tasks/10-02-ai-kbs-mvp/implement.md` enumerates **62 deliverable tasks** (T0.1~T3.9) for the AI-KBS v1 MVP. The current state is **2/62 complete** (T0.1, T0.10 — the WeKnora fork). Each remaining task requires the same multi-step ritual: dispatch `trellis-implement` → dispatch `trellis-check` → update spec → commit. Manually running this for 60+ tasks inside a single session is impractical (context exhaustion, manual error). A repeatable orchestration primitive is required.

**Goal.** A single Claude Code **skill** (`trellis-auto-run-task`) that, when invoked, walks the remaining `### T{N.M}` blocks in `implement.md` and, for each one, dispatches the implement+check+commit loop in the main session until the task is fully complete (including git commit), then advances to the next task. Loop ends only when every remaining task reaches `done` (committed) or is `skipped` after exhausting retries.

**Constraints.**
1. **Branch-per-task**: every `task-block` is developed on its own branch `feature/<task-id>` forked from the current working branch (`require` in this repo). After check passes, the branch is merged back to `require` and the feature branch is deleted. Sub-agent work always happens on the feature branch.
2. **Up to 2 parallel dispatches**: independent task-blocks (no dependency between them) may be dispatched concurrently — **maximum 2 at a time**. Dependent task-blocks are dispatched serially. Each dispatched sub-agent still must complete fully (return its result) before the next dispatch decision; the orchestrator pools up to 2 in-flight at any moment.
3. State must be **durable across session interruptions** (crash, `/exit`, context exhaustion).
4. Failure handling: per-task **retry** (default N=2) then **skip** with notification, never silent.
5. The skill must not violate Trellis's "main session only dispatches" rule from the active workflow-state block.
6. The skill must not auto-push to remote, auto-create PRs, or touch any other branch besides `require` and `feature/<task-id>`.
7. Reuses existing primitives (`trellis-implement`, `trellis-check`, `trellis-update-spec`, `task.py`, `git-agent commit`, `git-agent merge`) — does **not** reimplement them.

**Non-goals.**
- Parallel/async multi-task execution (user contract is sequential blocking dispatch).
- Auto-PR creation, auto-merge to `main`, auto-tag, auto-release.
- Modifying `implement.md` task definitions (the skill reads, never writes the source plan).

---

## Discovery Results

### Codebase evidence

| Source | Finding |
|---|---|
| `.trellis/tasks/10-02-ai-kbs-mvp/implement.md` | 62 `### T{N.M}` blocks, each followed by a markdown table with `输入/动作/输出/依赖/验收/风险` fields. Current completion: only T0.1, T0.10 are merged (commits `2e8e585`, `ca13ccb`). |
| `.trellis/workflow.md` `[workflow-state:in_progress]` | "Main-session default: dispatch implement/check sub-agents. … Dispatch prompt starts with `Active task: <task path from task.py current>`." |
| `.claude/agents/trellis-implement.md` | Agent has a **recursion guard** — when running as `trellis-implement`, do **not** spawn another `trellis-implement` or `trellis-check`. Dispatch is **main session only**. |
| `.claude/agents/trellis-check.md` (trellis-check skill) | Step 1: `git diff --name-only HEAD` + `git status`. Step 2: read prd/design/implement. Step 3: run project checks. |
| `.trellis/scripts/task.py` | `task.py current`, `task.py list --mine --status <s>`, `task.py finish` available. |
| `.claude/skills/trellis-channel/` | Exposes async channel-based multi-agent (workers, inbox, durable event log). **Alternative** for true async dispatch — but user contract says "main session waits for each sub-agent", so the simpler Agent tool is preferred. |
| `.trellis/scripts/get_context.py` | Loads task context (`prd → design → implement`). Reused, not reimplemented. |
| `.claude/skills/trellis-update-spec/SKILL.md` | Phase 3.3 spec-update skill; called between check and commit. |

### External / vendor context

- **Claude Code Agent tool** (`Agent` / `Task`): synchronous sub-agent dispatch. The child agent runs to completion in its own context window and returns a single message. **This is the user's contract: blocking, full result before next dispatch.**
- **Trellis workflow** is the active harness; the skill must speak its dialect.

---

## Glossary

Reconciled vocabulary across sub-agent contexts. **Canonical labels** (used throughout design files):

| Concept | Canonical label | Rejected variants |
|---|---|---|
| Skill under design | **`trellis-auto-run-task`** | "orchestrator", "auto-runner", "task loop", "编排技能" |
| One unit of work in `implement.md` | **`task-block`** (e.g., `T1.1`) | "subtask", "step", "ticket" (overloaded) |
| Per-task state record | **`task-block-state`** | "job", "work-item" |
| Outer orchestration loop | **`auto-run-loop`** | "pipeline", "controller" |
| Sub-agent call inside the loop | **`dispatch-step`** | "spawn", "kick-off" |
| Persistent state file | **`orchestration-state.json`** | "checkpoint", "ledger" |
| Trellis source-of-truth task list | **`implement.md`** (read-only) | "plan", "todo list" |
| Sub-agent type for implementation | **`trellis-implement`** | "implement agent" |
| Sub-agent type for verification | **`trellis-check`** | "check agent", "reviewer" |

---

## Requirements

### Functional

- **F1.** Read `.trellis/tasks/10-02-ai-kbs-mvp/implement.md`, parse every `### T{N.M}` block + its following markdown table.
- **F2.** For each `task-block`, in source-file order (which already encodes the dependency chain M0→M3):
  1. Resolve `依赖` from `implement.md` table; if any predecessor is in `failed[]`/`skipped[]`, mark current as `blocked` and surface to user before continuing.
  2. **Create/checkout the feature branch** `feature/<task-id>` from `require` (no fast-forward; fresh branch per task-block).
  3. Dispatch a **`trellis-implement`** Agent with a prompt that starts `Active task: <task path>` and embeds the parsed task-block input. Sub-agent does all work on the feature branch.
  4. Wait for the implement Agent to **exit fully** (block until result message arrives). Each dispatched sub-agent must return before the next dispatch decision.
  5. Dispatch a **`trellis-check`** Agent on the resulting diff (still on feature branch).
  6. Wait for the check Agent to **exit fully**.
  7. Run the **`trellis-update-spec`** skill (Phase 3.3 of Trellis workflow).
  8. Commit on the feature branch (`git-agent commit …` or fallback `/git:commit` skill).
  9. **Merge feature branch back to `require`**: `git-agent merge feature/<task-id> --no-ff -m "merge: <task-id> complete"`. Delete the feature branch after successful merge.
  10. Mark `task-block` `done` in state, persist state, advance.
- **F3.** **Failure handling**: implement or check returns `error` → retry up to `MAX_RETRIES` (default 2). If still failing → mark `failed` or `skipped` per skill config, persist, surface to user, advance.
- **F4.** **Resumability**: every transition persists `orchestration-state.json`. On skill (re)invocation, load state and resume from `current_task`. No re-processing of `done` blocks.
- **F5.** **Visible progress**: maintain a TaskCreate list, one task per `task-block`, marked in_progress / completed as the loop advances.
- **F5b.** **Up to 2-way parallelism**: the orchestrator maintains a small pool of in-flight sub-agents (max 2). Independent task-blocks (whose `依赖` from `implement.md` is empty or already satisfied) are dispatched concurrently. Dependent task-blocks wait for their dependencies to merge. Each individual sub-agent is still blocking from the orchestrator's perspective — the parallelism is in *which* task-blocks are simultaneously in-flight, not in concurrent sub-agents for the same task.
- **F6.** **Stop conditions** (any one):
  - All `task-block`s reach terminal state (`done` | `failed` | `skipped`).
  - User aborts (skill exits with summary).
  - Hard error in state persistence (abort, surface error).
- **F7.** **Skill self-exemption**: while inside a `trellis-implement` or `trellis-check` sub-agent, the orchestration skill is **not** invokable (recursion guard). Mirrors the existing guard in `trellis-implement.md`.

### Non-functional

- **NF1.** Skill body + script ≤ 400 lines total (lean, testable).
- **NF2.** State writes are atomic (write to temp + rename).
- **NF3.** Dry-run mode: `--dry-run` flag walks the loop, prints dispatch plan, makes **no** sub-agent calls and **no** commits.
- **NF4.** Logs every transition to `.trellis/workspace/jimmy/orchestration.log` (append-only) for post-mortem.

---

## Rationale

**Why a separate skill and not a one-off script?** The orchestration runs **inside a Claude Code session**, and the user wants it driven by the AI dispatcher (`Agent` tool), not by an unattended cron. A skill exposes a natural user-facing entry point, integrates with the workflow-state block, and respects the Trellis "main session dispatches" rule.

**Why serial blocking dispatch?** The user's explicit contract — "每个 sub-agent 必须完整完成才能退出并结果给主会话,再重新派新的 sub-agent". The simpler Agent tool satisfies this directly. `trellis channel`'s async worker model would be over-engineered and harder to debug per-task failures.

**Why persistent state, not just session memory?** The 62-task loop will run across many turns; context will compact and respawn. Without a JSON file, a single crash loses hours of work. The cost of one JSON write per task is trivial.

**Why reuse `trellis-update-spec` and `git-agent commit` rather than inline them?** The skill must not become a re-implementation of Trellis primitives; otherwise upgrades to those primitives will silently miss it.

**Alternatives rejected** (recorded so the evaluator can flag if my reasoning is wrong):

| Alternative | Reject reason |
|---|---|
| `trellis channel` async workers | Over-engineered for serial blocking contract; harder per-task failure inspection. |
| One mega sub-agent that loops internally | Violates Trellis "main session only dispatches" rule; loses resumability across turns. |
| Cron-driven shell script that calls `task.py` | No AI judgment on per-task failure; cannot adapt sub-agent prompts to task-block content. |
| `/goal` wrapper around brainstorming | Out of scope — there is no committed design target yet. |

---

## Detailed Design

### Architecture (one image)

```
┌────────────────────────────────────────────────────────────────┐
│ Main session                                                   │
│   ↓ invokes via Skill tool                                     │
│ ┌──────────────────────────────────────────────────────────┐   │
│ │ trellis-auto-run-task  (this skill)                      │   │
│ │   1. Load orchestration-state.json (or init empty)       │   │
│ │   2. Parse implement.md → list[T1.1..T3.9]               │   │
│ │   3. For each task-block (in source order, skipping done)│   │
│ │        a. dispatch trellis-implement  (Agent, blocking)  │──┼──┐
│ │        b. dispatch trellis-check      (Agent, blocking)  │──┼──┤
│ │        c. invoke trellis-update-spec  (Skill)            │   │  │
│ │        d. git-agent commit                              │   │  │
│ │        e. persist state, advance                         │   │  │
│ │   4. Loop until all terminal                             │   │  │
│ │   5. Emit final summary, exit                           │   │  │
│ └──────────────────────────────────────────────────────────┘   │
└────────────────────────────────────────────────────────────────┘
                              │ sub-agent context (fresh)
                              ▼
                ┌─────────────────────────────┐
                │ trellis-implement  or       │
                │ trellis-check (Agent)       │
                │ - reads its own context     │
                │ - returns single message    │
                └─────────────────────────────┘
```

### Component breakdown

**1. Skill manifest** — `.claude/skills/trellis-auto-run-task/SKILL.md`
- Frontmatter: `name`, `description`, `user-invocable: true`.
- Body: invocation protocol, recursion guard, dispatch prompt template, failure semantics, exit codes.

**2. Parser** — `.claude/skills/trellis-auto-run-task/scripts/parse_implement.py`
- Pure function `parse_implement(md_path) -> list[TaskBlock]`.
- Identifies `^### T(\d+)\.(\d+)\s+(.+)$` headers, extracts following 6-row markdown table into `{id, title, input, action, output, dependency, acceptance, risk}`.
- Idempotent: re-parsing yields the same list (no side effects).

**3. State store** — `.claude/skills/trellis-auto-run-task/scripts/state.py`
- `load(path) -> OrchestrationState`
- `save(state, path)` — atomic write (`temp + os.replace`).
- `OrchestrationState`: `{ version: 1, current_task: str|None, completed: [str], failed: [{id, error, retries}], skipped: [str], last_update: iso8601, config: {max_retries, dry_run} }`.

**4. Dispatch prompt template** — embedded in SKILL.md
```
Active task: .trellis/tasks/10-02-ai-kbs-mvp

TASK-BLOCK: {task_id} — {title}
INPUT: {input}
ACTION (verbatim from implement.md):
{action}
DEPENDENCIES: {dependency}
ACCEPTANCE: {acceptance}

Dispatched by trellis-auto-run-task (attempt {attempt}/{max_retries}).
Context files to load: implement.jsonl entries, prd.md, design.md, implement.md.
After implementing, return: files changed + test results + any blockers. Do NOT commit.
```

**5. Loop driver** — `.claude/skills/trellis-auto-run-task/scripts/loop.py`
- `run(state_path, md_path, task_dir, config) -> Summary`
- Pulls the loop body out of the SKILL.md into a testable module so the skill body stays short.

**6. Skill body** — short, mostly delegates to `loop.py` via `Bash` tool calls.

### Dispatch protocol (per task-block)

| Step | Tool | Args | Block? | Recursion-safe? |
|---|---|---|---|---|
| 1 | TaskUpdate | mark in_progress | — | yes |
| 2 | Agent | `subagent_type=trellis-implement`, prompt starts with `Active task: …` | YES | yes (sub-agent self-exempt) |
| 3 | Agent | `subagent_type=trellis-check`, prompt describes diff scope | YES | yes |
| 4 | Skill | `trellis-update-spec` | YES | yes |
| 5 | Bash | `git-agent commit …` (or `/git:commit` fallback) | YES | yes |
| 6 | State write | atomic save | — | yes |
| 7 | TaskUpdate | mark completed | — | yes |

### Failure semantics

- **Implement sub-agent returns error / non-zero / "blocked" marker** → retry up to `MAX_RETRIES`. Each retry increases `attempt` in dispatch prompt. After exhausting retries → mark `failed` with error excerpt, persist, **advance** (do not block the loop forever on one task).
- **Check sub-agent flags blocking issues** → retry the check (not implement) up to 2 times. If still blocking → surface to user via `AskUserQuestion` once: **{Fix & retry, Skip, Abort loop}**. Default on no answer in 60s → Skip.
- **Commit fails** → retry once with `--free` flag (per git-agent convention). Still failing → mark `failed`, surface.
- **State write fails** → hard abort (cannot guarantee resumability).

### Resumability contract

- State written **after every transition** (not just at task end).
- On invocation: if state file exists and `current_task` is set, resume from `current_task` (inclusive — re-attempt the in-progress one).
- Re-parsing `implement.md` is safe: new `task-block`s (if added later) are picked up automatically; existing `done` ones are skipped.

### Recursion guard

Mirroring `trellis-implement.md` §Recursion Guard:

> If invoked from inside a `trellis-implement` or `trellis-check` sub-agent context, the skill **exits immediately** with message `trellis-auto-run-task is a main-session skill; not invokable from sub-agent context.` This prevents infinite dispatch loops.

### Dry-run mode

`--dry-run` prints for each remaining task-block:
- `T{id} — {title}`
- `deps: {ok | blocked by [T1.1, T1.2]}`
- `dispatch plan: trellis-implement → trellis-check → update-spec → commit`
- `would retry up to {max_retries} on failure`

No sub-agent calls, no state mutation beyond a single "last dry-run" timestamp.

### Stop / exit

After loop ends, skill emits:
```
SUMMARY:
  done:    T0.2 T0.3 … (count)
  failed:  T1.5 (error excerpt) … (count)
  skipped: T2.10 … (count)
  total:   62
```

Exit code 0 if all done; 1 if any failed/skipped; 2 on hard error.

---

## Design Documents

Companion files in this folder:

- [`bdd-specs.md`](./bdd-specs.md) — 12 Gherkin scenarios covering happy path, retry, skip, resume, recursion guard, dry-run, blocked-dependency, hard-error.
- [`architecture.md`](./architecture.md) — Component breakdown, data flow, state schema, integration points.
- [`best-practices.md`](./best-practices.md) — Atomic writes, recursion guards, idempotent re-parsing, observability.
