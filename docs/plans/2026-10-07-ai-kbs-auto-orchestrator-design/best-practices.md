# AI-KBS Auto-Orchestrator — Best Practices

Companion to `_index.md`. Security, performance, code quality, and common pitfalls for the `trellis-auto-run-task` skill.

---

## 1. Security

### 1.1 No auto-merge, no auto-push, no tag mutation

The skill commits to the **current working branch only** (`require` in the current repo). It MUST NOT:
- run `git push`
- run `git merge`
- create or move git tags
- touch `main` even if the user is on it

The commit command uses `git-agent commit` with the standard `--no-stage --intent "<task-id> complete"` shape; no `--push`/`--merge`/`--tag` flags are ever passed.

### 1.2 No destructive ops without AskUserQuestion

The skill may invoke `AskUserQuestion` only when:
- the check sub-agent flags a **blocking** issue, OR
- a task-block has exhausted retries.

It MUST NOT use `AskUserQuestion` for routine choices (e.g., "retry vs skip on first failure" — that is decided by config).

### 1.3 State file lives under `.trellis/`, not the repo root

The state file is at `<task-dir>/orchestration-state.json` (default `.trellis/tasks/10-02-ai-kbs-mvp/orchestration-state.json`). This:
- keeps it under Trellis's owned namespace
- excludes it from any user-visible artifact
- pairs cleanly with `.gitignore` patterns for `.trellis/orchestration*`

### 1.4 Dispatch prompts never embed secrets

The parsed `implement.md` action strings reference secrets only indirectly (e.g., "configure Langfuse OTLP"). The dispatch prompt template echoes the action verbatim. Operators are responsible for ensuring `implement.md` does not contain plaintext secrets; the skill does not redact (it has no way to know what is sensitive).

---

## 2. Performance

### 2.1 Sub-agent context cost

Each `Agent` invocation spins up a fresh context window. Budget: roughly 50-150k tokens per sub-agent (the implement.md table + task-block body + relevant spec). Across 60 task-blocks, total context spend is ~6-9M tokens — well within the orchestrator's budget **provided the main session context stays compact**.

The orchestrator therefore:
- does NOT re-read the full `implement.md` between tasks (it keeps the parsed `list[TaskBlock]` in memory)
- does NOT re-read task `prd.md` / `design.md` (delegated to the sub-agent)
- uses compact dispatch prompts (table fields only, not the surrounding prose)

### 2.2 State write cost

`orchestration-state.json` is < 10 KB for 62 tasks. Atomic write is `temp + fsync + rename`. Cost is negligible; performed after every transition is correct.

### 2.3 Loop throughput target

Goal: **5-10 task-blocks per hour** end-to-end (implement + test + check + commit). Realistic lower bound: 3/hour for backend-heavy blocks (T1.x, T2.x); upper bound: 15/hour for small config blocks (T0.x). Total ETA for 60 remaining: **6-20 hours wall clock**, possibly split across multiple sessions via resumability.

---

## 3. Code Quality

### 3.1 Module boundaries

| Module | Responsibility | Lines (target) |
|---|---|---|
| `SKILL.md` | User-facing protocol, recursion guard, dispatch prompt template | ≤ 150 |
| `scripts/parse_implement.py` | Pure parser | ≤ 120 |
| `scripts/state.py` | Load/save, atomic write | ≤ 100 |
| `scripts/loop.py` | Plan builder + summary | ≤ 200 |
| `tests/test_*.py` | Unit tests | — |

Total ≤ 600 lines (excluding tests). Skill body is short because the orchestration logic lives in `loop.py` and is invoked from `SKILL.md` via `Bash` tool.

### 3.2 Type discipline

- All public functions carry type hints.
- `TaskBlock`, `OrchestrationState`, `LoopConfig`, `DispatchPlan`, `Summary` are `frozen=True` dataclasses.
- No `Any`, no `# type: ignore` without an inline justification.

### 3.3 No silent failure

Every error path:
- writes to `orchestration.log`
- persists state with the failure recorded
- surfaces to the user via TaskUpdate message

There is no `except Exception: pass`.

### 3.4 Idempotent re-parsing

`parse_implement(md_path)` is a pure function: same input → same output, no I/O side effects. Re-running the skill never re-dispatches already-`done` task-blocks because state persistence is the source of truth, not the parse result.

### 3.5 Logging discipline

- One line per state transition to `.trellis/workspace/jimmy/orchestration.log` (append-only).
- Format: `<iso8601> <event> <task_block_id> <detail>`.
- Never log full file contents (just paths).
- Never log secret-like content (assume `implement.md` is sensitive-adjacent; only log IDs + summaries).

---

## 4. Common Pitfalls

### 4.1 Recursion loop

> Pitfall: the orchestrator dispatches `trellis-implement`, which (mistakenly) tries to dispatch the orchestrator again.

**Mitigation**: every sub-agent (`trellis-implement`, `trellis-check`) carries its own recursion guard. The orchestrator skill ALSO carries a guard (exits if invoked from a sub-agent context). Both sides enforce; one alone is insufficient.

### 4.2 State torn-write on crash

> Pitfall: skill crashes mid-write, leaving a half-written JSON that fails to parse on resume.

**Mitigation**: atomic write (`temp + os.replace`) + version field. On `json.JSONDecodeError`, refuse load and surface to user; do **not** auto-recover (the safest default).

### 4.3 Stale `current_task` after resume

> Pitfall: state file says `current_task = T1.5` but the work for T1.5 actually completed (commit landed) before the crash; resuming re-dispatches T1.5 and creates duplicate work.

**Mitigation**: on resume, the orchestrator inspects `git log --oneline -1` and compares to the persisted `last_commit_for_task` (added in `state` schema v1). If they match → mark T1.5 `done` and advance without re-dispatch. If they don't match → re-dispatch T1.5 (the safe default).

### 4.4 Implement.md edits mid-loop

> Pitfall: user edits `implement.md` while the loop is running; the re-parse picks up new task-blocks that weren't in the original plan.

**Mitigation**: the orchestrator parses `implement.md` **once** at invocation start. Mid-loop re-parse is not performed. New task-blocks require a fresh skill invocation. This is the simplest correctness guarantee.

### 4.5 Branch drift

> Pitfall: user checks out a different branch mid-loop; commits land on the wrong branch.

**Mitigation**: at invocation start, capture `git rev-parse --abbrev-ref HEAD` into `state.config.branch`. On every commit, re-check the branch and refuse to proceed if it changed. Surface to user via `AskUserQuestion`.

### 4.6 Silent skip on exhausted retries

> Pitfall: implement fails 3 times, the skill silently skips T1.5 and moves on; user discovers later.

**Mitigation**: every skip / failure writes a visible `TaskUpdate` message ("T1.5 SKIPPED: <error>"). The skill's final summary enumerates `done / failed / skipped` explicitly. Exit code 1 on any skip/fail (not 0).

### 4.7 Over-eager AskUserQuestion

> Pitfall: the skill prompts the user every few minutes, defeating the point of automation.

**Mitigation**: `AskUserQuestion` is invoked only on (a) check-blocking or (b) exhausted retries. Routine failures auto-retry up to `max_retries`. Default `max_retries = 2` (two retries = three attempts total).

---

## 5. Observability Checklist

- [x] Per-task transition logged to `orchestration.log`.
- [x] Per-task in-progress / completed marker on the visible TaskCreate list.
- [x] Final summary printed at exit (counts + lists).
- [x] State file inspectable by `cat orchestration-state.json`.
- [x] Dry-run mode for plan inspection.

---

## 6. Future Extensions (out of scope for v1)

- Parallel dispatch via `trellis channel` async workers — would violate user's blocking contract, listed for completeness only.
- Auto-PR creation per task-block — explicitly forbidden by `_index.md` §Non-goals.
- Spec-driven task-block completion (mark T-block done when `prd.md` acceptance criteria are detected in the diff) — could replace the binary done/fail model but is YAGNI for v1.
- Migration / schema bump for `OrchestrationState` v2 — listed only when needed.
