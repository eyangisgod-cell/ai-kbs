# AI-KBS Auto-Orchestrator — BDD Specs

> Companion to `_index.md`. All Gherkin scenarios live in this file; no separate `.feature` files are produced in the design phase.

Conventions:
- **Subject under test**: `trellis-auto-run-task` skill.
- **GIVEN** clauses seed state files and `implement.md` (using a fixture under `tests/fixtures/`).
- **WHEN** clauses invoke the skill (or `loop.py` directly for unit-speed tests).
- **THEN** clauses assert on `orchestration-state.json`, `git log`, and the orchestration log.

---

## Scenario 1 — Happy path: walk a clean plan to completion

```gherkin
Feature: Auto-orchestrator happy path

  Scenario: All task-blocks reach done
    Given an implement.md with 3 task-blocks T1.1, T1.2, T1.3 in source order
    And   orchestration-state.json is absent
    And   no sub-agent returns an error
    When  the skill trellis-auto-run-task is invoked
    Then  the skill dispatches trellis-implement for T1.1, waits for full result, then T1.2, then T1.3 — strictly in source order
    And   the skill dispatches trellis-check after each implement, waiting for full result before advancing
    And   the skill invokes trellis-update-spec between check and commit for each task-block
    And   the skill commits exactly once per task-block (3 commits total)
    And   orchestration-state.json records all three task-blocks in `completed[]`
    And   the skill exits with code 0 and summary lists 3 done / 0 failed / 0 skipped
```

---

## Scenario 2 — Implement sub-agent returns error, retry succeeds

```gherkin
  Scenario: Implement fails on first attempt, succeeds on retry
    Given implement.md contains task-block T1.5
    And   the trellis-implement Agent returns an error on attempt 1 for T1.5
    And   the trellis-implement Agent returns success on attempt 2 for T1.5
    When  the skill processes T1.5
    Then  the skill retries T1.5 automatically (attempt 2)
    And   T1.5 reaches `done` after the successful retry
    And   orchestration-state.json records attempt count = 2 for T1.5
    And   the orchestration log records the first-attempt error excerpt
```

---

## Scenario 13 — Branch-per-task: feature branch created, merged, deleted

```gherkin
Feature: Branch-per-task workflow

  Scenario: Each task-block uses a feature branch and merges back
    Given the current working branch is `require`
    And   T1.4 is the next eligible task-block
    When  the orchestrator processes T1.4
    Then  a feature branch `feature/T1.4` is created from `require` before the sub-agent runs
    And   the trellis-implement Agent works on `feature/T1.4`
    And   the commit lands on `feature/T1.4`
    And   the orchestrator switches back to `require`
    And   `feature/T1.4` is merged into `require` with `--no-ff`
    And   `feature/T1.4` is deleted after successful merge
    And   `require` now contains the merged T1.4 changes
```

---

## Scenario 14 — Up to 2-way parallel dispatch

```gherkin
Feature: Parallel dispatch (max 2)

  Scenario: Two independent task-blocks run in parallel
    Given implement.md has T1.1 and T1.2 as the next eligible blocks (both with empty 依赖)
    And   `in_flight` is currently empty
    When  the orchestrator advances
    Then  trellis-implement is dispatched for T1.1 on branch `feature/T1.1` — concurrent
    And   trellis-implement is dispatched for T1.2 on branch `feature/T1.2` — concurrent
    And   `in_flight` contains both records
    And   no third task-block is dispatched until one of them completes
```

---

## Scenario 15 — Dependency gates parallel dispatch

```gherkin
  Scenario: A dependent task-block waits for its dependency to finish
    Given T2.6 depends on T1.5
    And   T1.5 is in `in_flight` (not yet merged into `require`)
    When  the orchestrator scans for the next eligible block
    Then  T2.6 is NOT eligible and is NOT dispatched
    And   T2.6 becomes eligible only after T1.5 is merged into `require`
```

---

## Scenario 16 — Merge conflict on back-merge

```gherkin
  Scenario: Feature branch conflicts with `require`
    Given T0.2 was merged into `require` earlier
    And   T0.3 is now complete on `feature/T0.3` but conflicts with the current `require` tip
    When  the orchestrator attempts `git merge feature/T0.3`
    Then  the merge aborts cleanly
    And   AskUserQuestion is invoked with {Manual resolve, Abort loop}
    And   on Manual resolve, the orchestrator pauses and waits for the user to fix
```

---

## Scenario 3 — Exhausted retries → skip + advance

```gherkin
  Scenario: Implement fails N+1 times, task-block is skipped
    Given implement.md contains task-block T2.10
    And   config.max_retries = 2
    And   the trellis-implement Agent returns an error on attempts 1, 2, and 3 for T2.10
    When  the skill processes T2.10
    Then  the skill stops retrying after attempt 3
    And   T2.10 is recorded in `skipped[]` (per default config; not `failed[]`)
    And   the orchestration log contains the final error excerpt
    And   the loop advances to T2.11
    And   the user is notified via a visible TaskUpdate message ("T2.10 SKIPPED: <error>")
```

---

## Scenario 4 — Check sub-agent flags blocking issue, user chooses Skip

```gherkin
  Scenario: Check fails, user picks Skip via AskUserQuestion
    Given T1.7 has been implemented successfully
    When  the trellis-check Agent returns a blocking issue for T1.7
    Then  the skill invokes AskUserQuestion with options {Fix & retry, Skip, Abort loop}
    And   when the user selects Skip, T1.7 is recorded in `skipped[]`
    And   the loop advances to T1.8
```

---

## Scenario 5 — Resume from mid-loop state

```gherkin
  Scenario: Skill resumes after interruption
    Given orchestration-state.json exists with `current_task = T1.4`, `completed = [T1.1, T1.2, T1.3]`
    When  the skill is re-invoked
    Then  the skill does NOT re-dispatch T1.1 / T1.2 / T1.3
    And   the skill resumes at T1.4 (inclusive — T1.4 is re-attempted)
    And   the loop continues normally until all remaining blocks reach terminal state
```

---

## Scenario 6 — Dependency marked as blocked

```gherkin
  Scenario: A task-block whose dependency is in failed/skipped is marked blocked
    Given T2.6 depends on T1.5
    And   T1.5 is in `failed[]` from a prior run
    When  the skill reaches T2.6
    Then  T2.6 is recorded in `skipped[]` with reason "dependency T1.5 is failed/skipped"
    And   the loop continues to T2.7
```

---

## Scenario 7 — Recursion guard inside sub-agent

```gherkin
  Scenario: Skill invoked from a sub-agent context exits immediately
    Given the active agent is trellis-implement or trellis-check (sub-agent context)
    When  the skill trellis-auto-run-task is invoked
    Then  the skill exits immediately with the message
          "trellis-auto-run-task is a main-session skill; not invokable from sub-agent context."
    And   no state mutation occurs
```

---

## Scenario 8 — Dry-run produces plan, no side effects

```gherkin
  Scenario: --dry-run walks the plan without dispatching
    Given orchestration-state.json is absent
    And   implement.md contains 5 task-blocks
    When  the skill is invoked with --dry-run
    Then  no sub-agent is dispatched
    And   no commit is performed
    And   the skill prints a dispatch plan for each of the 5 task-blocks
    And   orchestration-state.json is NOT created (or remains untouched)
    And   the skill exits with code 0
```

---

## Scenario 9 — State write atomicity

```gherkin
  Scenario: Concurrent skill invocation does not corrupt state
    Given two parallel invocations of the skill race on the same state file
    When  both attempt to save state simultaneously
    Then  the final state file is consistent (no torn JSON)
    And   every task-block either appears in `completed[]` / `failed[]` / `skipped[]` exactly once across both runs
    And   the implementation uses temp-file + os.replace for atomicity
```

---

## Scenario 10 — Commit failure fallback to `/git:commit`

```gherkin
  Scenario: git-agent commit fails, fallback to /git:commit skill
    Given a task-block has been implemented + checked successfully
    When  git-agent commit returns an error
    Then  the skill retries once with --free flag
    And   on second failure, invokes the /git:commit skill as fallback
    And   if the fallback also fails, the task-block is recorded in `failed[]`
```

---

## Scenario 11 — Hard error in state persistence aborts the loop

```gherkin
  Scenario: State file cannot be written
    Given the state file path is read-only
    When  the skill attempts to persist state after a successful task-block
    Then  the skill aborts immediately with exit code 2
    And   the user sees the persistence error verbatim
```

---

## Scenario 12 — Skill respects the workflow-state guard

```gherkin
  Scenario: Workflow-state says inline mode, skill declines
    Given the active task's workflow-state is `[workflow-state:in_progress-inline]`
    When  the skill trellis-auto-run-task is invoked
    Then  the skill exits with code 1 and message
          "Auto-run is not supported in inline workflow-state; dispatch implement/check manually."
```
