---
name: trellis-auto-run-task
description: Auto-run the AI-KBS MVP task list — for each T{N.M} block in implement.md, create feature/<task-id>, dispatch trellis-implement + trellis-check (blocking), update spec, commit, merge back to require, advance. Up to 2 task-blocks run in parallel. Resumable across sessions.
user-invocable: true
---

# trellis-auto-run-task

Walks `.trellis/tasks/10-02-ai-kbs-mvp/implement.md` end-to-end. For each remaining `### T{N.M}` block: creates a per-task feature branch, dispatches `trellis-implement` → `trellis-check` (blocking), runs `trellis-update-spec`, commits, merges back to `require`, and advances. Up to 2 independent task-blocks run in parallel. Loop ends when every block reaches a terminal state or the user aborts.

## Recursion Guard (MUST be first)

If invoked from inside a `trellis-implement` or `trellis-check` sub-agent context, exit immediately:

```
trellis-auto-run-task is a main-session skill; not invokable from sub-agent context.
```

This mirrors the guard in `.claude/agents/trellis-implement.md`.

## Workflow-State Check

If the active task's workflow-state is `[workflow-state:in_progress-inline]`, exit 1 with:
```
Auto-run is not supported in inline workflow-state; dispatch implement/check manually.
```

## Args

| Flag | Default | Meaning |
|---|---|---|
| `--dry-run` | off | Print plan; no sub-agents, no commits, no merges, no state mutation |
| `--max-retries N` | 2 | Per-task retry budget |
| `--max-parallel N` | 2 | Hard cap on in-flight task-blocks (1 or 2 only) |
| `--task-dir PATH` | `.trellis/tasks/10-02-ai-kbs-mvp` | Where implement.md + state live |
| `--base-branch NAME` | `require` | Branch to fork from and merge back to |

## Invocation Protocol (main-session driver)

The skill body executes this loop. Each numbered step is a real tool call in the main session; sub-agent calls (Agent tool) BLOCK until the sub-agent returns its single result message.

```
LOOP:
  load orchestration-state.json (init empty if absent)
  parse implement.md → list[TaskBlock]
  until every TaskBlock is in completed / failed / skipped:

    # Step 1 — refill the in-flight pool
    while len(state.in_flight) < config.max_parallel:
      next_block = pick_next_eligible(state, task_blocks)
        # eligible: not terminal, all deps in completed[]
        # tiebreak: earliest in implement.md source order
      if next_block is None: break
      # 1a. Branch-per-task
      Bash: git checkout {base_branch}
      Bash: git checkout -b feature/{next_block.id} {base_branch}
      # 1b. Record + persist
      state.in_flight.append({id: next_block.id, branch: feature/{next_block.id}, started_at: now})
      save(state)

    # Step 2 — wait for any in-flight sub-agent to return
    # Each in-flight task may be at any of the 4 sub-steps below.
    # We process them one at a time within the loop's outer turn (foreground Agent = blocking).
    for record in state.in_flight.copy():
      tb = task_blocks_by_id[record.id]
      attempt = state.attempts.get(tb.id, 1)

      # Step 2a — implement sub-agent (skip if already past this step on resume)
      if not state[tb.id].implement_done:
        Agent(
          subagent_type=trellis-implement,
          prompt=<see Dispatch Prompt Template below>
        )
        if Agent returns error:
          if attempt < config.max_retries:
            state.attempts[tb.id] = attempt + 1
            save(state); continue  # retry same step, same branch
          else:
            mark skipped, cleanup branch, remove from in_flight, save; continue

      # Step 2b — check sub-agent
      Agent(
        subagent_type=trellis-check,
        prompt=<see Check Prompt Template below>
      )
      if Agent returns blocking:
        AskUserQuestion({Fix & retry, Skip, Abort})

      # Step 2c — update spec
      Skill: trellis-update-spec

      # Step 2d — commit on feature branch
      Bash: git-agent commit --no-stage --intent "{tb.id} complete" --co-author "Claude Opus 4.7 <noreply@anthropic.com>"
      on failure: retry once with --free; if still fail, fallback to Skill: /git:commit

      # Step 2e — merge back to base
      Bash: git checkout {base_branch}
      Bash: git-agent merge feature/{tb.id} --no-ff -m "merge: {tb.id} complete"
      on conflict: AskUserQuestion({Manual resolve, Abort})
      Bash: git branch -d feature/{tb.id}

      # Step 2f — terminal transition
      state.completed.append(tb.id)
      state.in_flight = [r for r in state.in_flight if r.id != tb.id]
      delete state.attempts[tb.id]
      save(state)
      TaskUpdate: mark tb in_progress → completed

  emit Summary, exit
```

## Dispatch Prompt Template (trellis-implement)

```
Active task: {task_dir}

TASK-BLOCK: {tb.id} — {tb.title}
ATTEMPT: {attempt} / {config.max_retries}

You are working on branch `feature/{tb.id}` (already checked out by the orchestrator).
Do NOT switch branches. Do NOT commit. Do NOT merge. Do NOT push.
Return a single message: files changed + test results + any blocker.

INPUT (verbatim from implement.md):
{tb.input}

ACTION (verbatim from implement.md):
{tb.action}

OUTPUT EXPECTED:
{tb.output}

DEPENDENCIES (already merged into require):
{tb.dependency}

ACCEPTANCE CRITERIA:
{tb.acceptance}
```

## Check Prompt Template (trellis-check)

```
Active task: {task_dir}

TASK-BLOCK: {tb.id} — {tb.title}

Verify the recent diff against the acceptance criteria. The diff is on branch `feature/{tb.id}`.

ACCEPTANCE CRITERIA:
{tb.acceptance}

INPUT: {tb.input}
ACTION (the intended change): {tb.action}

Return one of:
  PASS — acceptance criteria met, safe to commit + merge
  BLOCKING: <issue> — must fix before commit
  WARN: <issue> — non-blocking, caller decides

Do NOT switch branches. Do NOT commit.
```

## Failure Semantics

| Event | Action |
|---|---|
| Implement Agent error | Retry up to `max_retries` on same branch. Exhausted → mark `skipped`, `git branch -D feature/<id>`, advance. |
| Check Agent blocking | AskUserQuestion {Fix & retry, Skip, Abort}. Default on no answer (60s) → Skip. |
| Update-spec error | Surface verbatim; AskUserQuestion. |
| Commit error | Retry once with `--free`. Still failing → fallback to `/git:commit`. Still failing → `failed[]`. |
| Merge conflict | AskUserQuestion {Manual resolve, Abort}. Pauses loop. |
| State write error | **Hard abort** (exit 2). |
| Branch checkout error (e.g., dirty tree) | AskUserQuestion {Stash & retry, Abort}. |

## Exit Codes

- `0` — all task-blocks reached `done`
- `1` — partial (some `failed`/`skipped`), or workflow-state guard tripped
- `2` — hard error (state persistence, uncaught)

## Stop / Output

Final summary:
```
SUMMARY
  done:     T0.2 T0.3 ... (count)
  failed:   T1.7 (error excerpt) ... (count)
  skipped:  T2.10 ... (count)
  in_flight at exit: [T1.4, T2.6]  (only present if interrupted)
  total:    62
  exit:     0
```

## Dependencies

- Python 3.10+ for the helper scripts (`scripts/parse_implement.py`, `scripts/state.py`, `scripts/loop.py`).
- `trellis channel` is **not** required — the orchestrator uses synchronous Agent tool calls.
- `git-agent commit` and `git-agent merge` per `.trellis/scripts/hooks/git-agent`.

## Files in this skill

- `SKILL.md` (this file) — main-session protocol
- `scripts/parse_implement.py` — pure parser for `### T{N.M}` blocks
- `scripts/state.py` — atomic load/save for `orchestration-state.json`
- `scripts/loop.py` — eligibility + dispatch-plan builder (testable; the SKILL.md executes the plans)
- `tests/test_*.py` — unit tests
