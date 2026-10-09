# AI-KBS Auto-Orchestrator — Architecture

Companion to `_index.md`. Component breakdown, data flow, state schema, and integration points.

---

## 1. System Overview

```
┌──────────────────────────────────────────────────────────────────────────┐
│ User  ──>  Main Claude session  ──>  Skill: trellis-auto-run-task         │
│                                              │                            │
│   ┌──────────────────────────────────────────┴────────────────────────┐  │
│   │ 1. parse_implement.md  →  list[TaskBlock]                         │  │
│   │ 2. load orchestration-state.json  →  OrchestrationState            │  │
│   │ 3. main loop (per task-block, blocking):                           │  │
│   │      a. TaskUpdate (in_progress)                                  │  │
│   │      b. Agent(subagent_type=trellis-implement, prompt)  ◄── BLOCK │  │
│   │      c. Agent(subagent_type=trellis-check,     prompt)  ◄── BLOCK │  │
│   │      d. Skill(trellis-update-spec)                       ◄── BLOCK │  │
│   │      e. Bash(git-agent commit …)                         ◄── BLOCK │  │
│   │      f. state.save (atomic)                                       │  │
│   │      g. TaskUpdate (completed | failed | skipped)                  │  │
│   │ 4. loop until all task-blocks terminal                            │  │
│   └───────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────┘
                                │
                                ▼  (sub-agent spawns in fresh context)
              ┌───────────────────────────────────────┐
              │ trellis-implement  Agent              │
              │   - reads own context (jsonl, prd…)  │
              │   - edits files                       │
              │   - returns single result message     │
              │   - recursion guard: no further spawn │
              └───────────────────────────────────────┘
              ┌───────────────────────────────────────┐
              │ trellis-check  Agent                  │
              │   - reads diff, prd, design           │
              │   - returns single verdict message    │
              └───────────────────────────────────────┘
```

---

## 2. Component Breakdown

### 2.1 Skill manifest: `.claude/skills/trellis-auto-run-task/SKILL.md`

**Purpose**: User-facing entry point. Invoked via the Skill tool.

**Frontmatter**:
```yaml
---
name: trellis-auto-run-task
description: Auto-run the AI-KBS MVP task list — dispatch trellis-implement + trellis-check per T{N.M} block in implement.md, update spec, commit, advance. Sequential, blocking, resumable.
user-invocable: true
---
```

**Body sections**:
1. **Recursion guard** — exit immediately if inside a sub-agent context.
2. **Workflow-state check** — exit 1 if `[workflow-state:in_progress-inline]`.
3. **Args** — `--dry-run`, `--max-retries N`, `--task-dir PATH` (default `.trellis/tasks/10-02-ai-kbs-mvp`), `--state PATH` (default `<task-dir>/orchestration-state.json`).
4. **Invocation protocol** — the 7-step loop described in `_index.md` §Detailed Design.
5. **Failure semantics table** — copied from `_index.md`.
6. **Exit codes** — 0 ok, 1 partial / inline-state, 2 hard error.

### 2.2 Parser: `scripts/parse_implement.py`

**Signature**:
```python
def parse_implement(md_path: str) -> list[TaskBlock]:
    """Pure function; no I/O side effects beyond reading md_path."""

@dataclass(frozen=True)
class TaskBlock:
    id: str           # "T1.1"
    title: str        # "DB 迁移:核心表"
    input: str        # verbatim from table
    action: str       # verbatim from table
    output: str       # verbatim from table
    dependency: str   # raw text "T0.1" or "T0.1 + T0.5"
    acceptance: str
    risk: str
```

**Regex**:
- Header: `re.compile(r"^###\s+(T(\d+)\.(\d+))\s+(.+?)\s*$", re.MULTILINE)`
- Table row: `re.compile(r"\|\s*\*\*(输入|动作|输出|依赖|验收|风险)\*\*\s*\|\s*(.+?)\s*\|")`

**Failure mode**: if any of the 6 required fields is missing on a block, raise `ParseError` with block id + missing field. Do **not** skip silently.

### 2.3 State store: `scripts/state.py`

**Schema** (`v1`):
```json
{
  "version": 1,
  "current_task": "T1.5",
  "completed": ["T0.1", "T0.2", "T0.3"],
  "failed": [
    { "id": "T1.7", "error": "check returned blocking: X", "attempts": 3 }
  ],
  "skipped": ["T2.10", "T2.11"],
  "last_update": "2026-10-07T12:34:56Z",
  "config": { "max_retries": 2, "dry_run": false }
}
```

**Atomic write**:
```python
def save(state: OrchestrationState, path: str) -> None:
    tmp = path + ".tmp"
    with open(tmp, "w", encoding="utf-8") as f:
        json.dump(asdict(state), f, ensure_ascii=False, indent=2)
        f.flush(); os.fsync(f.fileno())
    os.replace(tmp, path)  # atomic on POSIX; best-effort on Windows
```

**Migration**: if `version` ≠ current, refuse to load and require explicit migration. (v1 is the only version shipped; future bumps add a `_migrate(v1→v2)` function.)

### 2.4 Loop driver: `scripts/loop.py`

**Public API**:
```python
def run(state_path: str,
        md_path: str,
        task_dir: str,
        config: LoopConfig) -> Summary:
    """Drives the loop. Returns Summary; does NOT raise on per-task failure."""

@dataclass
class LoopConfig:
    max_retries: int = 2
    dry_run: bool = False

@dataclass
class Summary:
    done: list[str]
    failed: list[tuple[str, str]]   # (id, error)
    skipped: list[str]
    total: int
```

**The `loop.py` module is intentionally tool-agnostic**: it returns structured `DispatchPlan` objects (not actual Agent calls) so it can be unit-tested without a live LLM. The SKILL.md body is the bridge that maps `DispatchPlan` items to actual `Agent` tool invocations.

### 2.5 Dispatch plan object

```python
@dataclass
class DispatchPlan:
    task_block: TaskBlock
    attempt: int                       # 1..max_retries
    steps: list[Literal["implement", "check", "update_spec", "commit"]]
    prompt_prefix: str                 # begins with "Active task: <task_dir>"
```

The SKILL.md body iterates `DispatchPlan.steps` and invokes the matching tool. The mapping:

| step | tool | args |
|---|---|---|
| `implement` | `Agent` | `subagent_type=trellis-implement`, `prompt=<prompt_prefix + TASK-BLOCK body>` |
| `check` | `Agent` | `subagent_type=trellis-check`, `prompt=describe-diff-scope + task-block` |
| `update_spec` | `Skill` | `trellis-update-spec` |
| `commit` | `Bash` | `git-agent commit --no-stage --intent "<task-id> complete" --co-author "Claude Opus 4.7 <noreply@anthropic.com>"` |

### 2.6 Dispatch prompt template

```
Active task: <task_dir>

TASK-BLOCK: {task_block.id} — {task_block.title}
ATTEMPT: {attempt} / {max_retries}

INPUT (verbatim from implement.md):
{task_block.input}

ACTION (verbatim from implement.md):
{task_block.action}

OUTPUT EXPECTED:
{task_block.output}

DEPENDENCIES (already satisfied):
{task_block.dependency}

ACCEPTANCE CRITERIA:
{task_block.acceptance}

DISPATCHED BY: trellis-auto-run-task (do not auto-commit — the orchestrator commits).
RETURN: list of files changed + test results + any blocker. Single message.
```

---

## 3. Data Flow

```
implement.md ──parse──> list[TaskBlock]
                       │
state.json   ──load──> OrchestrationState
                       │
                       ▼
                  ┌────────────────────────────────────────────────┐
                  │ while there is capacity (in_flight < max_parallel):
                  │   pick next eligible TaskBlock:                 │
                  │     - not in completed / failed / skipped       │
                  │     - all predecessors in completed[]            │
                  │     - append to in_flight, persist state         │
                  │   if no eligible block (all blocked by deps),    │
                  │     wait for the next in_flight to finish        │
                  │                                                  │
                  │ wait for any in_flight sub-agent to return       │
                  │ on return:                                       │
                  │   - success ─► continue (commit + merge)          │
                  │   - failure ─► retry / skip per config           │
                  │   - remove from in_flight, persist state         │
                  │                                                  │
                  │ loop until all blocks terminal                   │
                  └────────────────────────────────────────────────┘
                       │
                       ▼
                OrchestrationState (saved)
```

Per-task control flow (the "execute_plan" for a single in-flight task-block):

```
tb ──► attempt = 1
   │
   ├─► git checkout -b feature/<tb.id> require       ◄── branch-per-task
   │
   ├─► Agent(trellis-implement, prompt)              ◄── BLOCK until sub-agent returns
   │     ├─ success ─► continue
   │     └─ error ──► if attempt < max_retries: attempt++; loop; else mark failed/skipped
   │                  (cleanup: git checkout require && git branch -D feature/<tb.id>)
   │
   ├─► Agent(trellis-check, prompt-diff)              ◄── BLOCK
   │     ├─ pass ─► continue
   │     └─ blocking ─► AskUserQuestion({Fix&retry, Skip, Abort})
   │
   ├─► Skill(trellis-update-spec)                     ◄── BLOCK
   │
   ├─► Bash(git-agent commit …)                       ◄── BLOCK   (commit on feature branch)
   │     ├─ success ─► continue
   │     └─ failure ─► retry once with --free; if still fail, fallback to /git:commit
   │
   ├─► git checkout require
   ├─► git-agent merge feature/<tb.id> --no-ff       ◄── merge back
   │     ├─ success ─► git branch -d feature/<tb.id>
   │     └─ conflict ─► AskUserQuestion({Manual resolve, Abort})
   │
   └─► mark done, persist state, remove from in_flight
```

---

## 4. State Schema (canonical, also in `scripts/state.py`)

```python
@dataclass
class OrchestrationState:
    version: int = 1
    current_task: Optional[str] = None       # in-progress task-block id (resumable marker)
    in_flight: list[InFlightRecord] = field(default_factory=list)  # up to max_parallel
    completed: list[str] = field(default_factory=list)
    failed: list[FailedRecord] = field(default_factory=list)
    skipped: list[str] = field(default_factory=list)
    last_update: str = ""                    # ISO8601 UTC
    config: LoopConfig = field(default_factory=LoopConfig)

@dataclass
class InFlightRecord:
    id: str                                  # task-block id, e.g. "T1.5"
    branch: str                              # "feature/T1.5"
    started_at: str                          # ISO8601 UTC

@dataclass
class FailedRecord:
    id: str
    error: str
    attempts: int
    branch: str                              # leftover feature branch, cleaned up on skip

@dataclass
class LoopConfig:
    max_retries: int = 2
    dry_run: bool = False
    base_branch: str = "require"             # current working branch; all features fork from here
    max_parallel: int = 2                    # user constraint; not configurable above 2
```

JSON-on-disk layout mirrors the dataclass. `version: 1` is mandatory; absent version → refuse load.

---

## 5. Integration Points

| External | How integrated | Failure handling |
|---|---|---|
| `.claude/agents/trellis-implement.md` | Sub-agent type `trellis-implement` via Agent tool | Sub-agent self-exempt (recursion guard). |
| `.claude/agents/trellis-check.md` (Agent form) | Sub-agent type `trellis-check` via Agent tool | Same. |
| `.claude/skills/trellis-update-spec/SKILL.md` | Skill tool invocation | If update-spec skill errors, surface error verbatim and ask user (Fix & retry / Skip / Abort). |
| `git-agent commit` (per `.trellis/scripts/hooks/git-agent`) | Bash invocation | Retry with `--free`; fallback to `/git:commit`; then `failed[]`. |
| `.trellis/scripts/task.py` | Read-only via `task.py current` to print progress | Not blocking. |
| `.trellis/workspace/jimmy/orchestration.log` | Append-only log for post-mortem | If write fails, log to stderr but continue (don't abort). |

---

## 6. Concurrency & Ordering

- **Up to 2-way parallelism**: independent task-blocks (whose `依赖` from `implement.md` is empty or already satisfied by `completed[]`) may be in-flight concurrently, up to `max_parallel = 2`. Dependent task-blocks wait until their dependencies are merged into `require`.
- **Per-task serial**: within a single task-block, the four steps (implement → check → update-spec → commit+merge) are strictly serial. Each sub-agent must complete fully (return its result) before the next step of that same task begins.
- **Source-order tiebreaker**: when more than 2 task-blocks become eligible at the same instant, the ones appearing earlier in `implement.md` win.
- **Resumed in-flight tasks are re-attempted**: `in_flight[]` is the resumable marker; on resume, each in-flight task-block is re-dispatched (not skipped). Feature branches that still exist on disk for in-flight tasks are re-checked-out.
- **Branch isolation**: sub-agents always work on `feature/<task-id>`. The main session `require` branch is only touched by the orchestrator (for merges). Sub-agents never switch the main branch.

---

## 7. Testing Strategy (high-level)

- **Unit** (`tests/test_parse_implement.py`, `tests/test_state.py`): pure-function coverage.
- **Integration** (`tests/test_loop.py`): stub `Agent` and `Skill` invocations; assert `DispatchPlan` sequences and state transitions.
- **E2E**: manual on `T0.2` first (smallest task-block), then `pnpm dev` smoke (not in scope here).
- **Dry-run smoke**: always run `--dry-run` once before any live loop to print the plan.
