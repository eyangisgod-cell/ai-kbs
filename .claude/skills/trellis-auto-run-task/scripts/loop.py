"""Loop driver — eligibility + dispatch-plan builder for trellis-auto-run-task.

Pure logic. Does NOT invoke any Agent / Bash / Skill — that is the SKILL.md's job
in the main session. This module is the testable decision engine.
"""
from __future__ import annotations

from dataclasses import dataclass
from typing import Literal

from parse_implement import TaskBlock
from state import OrchestrationState, LoopConfig, InFlightRecord


StepName = Literal["branch_checkout", "implement", "check", "update_spec", "commit", "merge", "branch_cleanup"]


@dataclass(frozen=True)
class DispatchPlan:
    task_block: TaskBlock
    in_flight_idx: int          # index into state.in_flight
    step: StepName
    attempt: int                # 1..max_retries
    branch: str


@dataclass(frozen=True)
class Summary:
    done: list[str]
    failed: list[tuple[str, str]]
    skipped: list[str]
    in_flight: list[str]
    total: int


def is_eligible(tb: TaskBlock, state: OrchestrationState) -> bool:
    """A task-block is eligible iff:
      - not already terminal (completed / failed / skipped / in_flight)
      - all predecessors are in completed[]
    """
    if state.is_terminal(tb.id):
        return False
    if any(r.id == tb.id for r in state.in_flight):
        return False
    for pred in tb.predecessors():
        if pred not in state.completed:
            return False
    return True


def pick_next_eligible(
    blocks: list[TaskBlock], state: OrchestrationState
) -> TaskBlock | None:
    """First eligible block in source-file order."""
    for tb in blocks:
        if is_eligible(tb, state):
            return tb
    return None


def build_dispatch_plans(
    blocks: list[TaskBlock], state: OrchestrationState, config: LoopConfig
) -> list[DispatchPlan]:
    """One plan per currently-in-flight task-block.

    Each plan says which step to execute next for that task-block. The SKILL.md
    consumes these plans and turns them into Agent / Skill / Bash tool calls.

    Plans are emitted in in_flight order (oldest first) so retries/continuations
    are deterministic.
    """
    plans: list[DispatchPlan] = []
    by_id = {tb.id: tb for tb in blocks}

    for idx, rec in enumerate(state.in_flight):
        tb = by_id.get(rec.id)
        if tb is None:
            # Stale record (e.g., implement.md was edited and this id vanished)
            continue

        branch = rec.branch

        # Resume logic: if any step is "completed" for this task-block, advance.
        # The SKILL.md tracks per-task step completion in state.attempts + branch
        # state; here we only know the in_flight step, so we trust rec.step.
        step: StepName = _next_step(rec.step, rec.attempts)
        plans.append(
            DispatchPlan(
                task_block=tb,
                in_flight_idx=idx,
                step=step,
                attempt=rec.attempts,
                branch=branch,
            )
        )

    # Refill: dispatch new eligible blocks if pool has capacity
    # We track in_flight length directly (already mutated below).
    while len(state.in_flight) < config.max_parallel:
        tb = pick_next_eligible(blocks, state)
        if tb is None:
            break
        branch = f"feature/{tb.id}"
        new_rec = InFlightRecord(
            id=tb.id, branch=branch, started_at="", step="branch_checkout", attempts=1
        )
        plans.append(
            DispatchPlan(
                task_block=tb,
                in_flight_idx=-1,           # not yet in state
                step="branch_checkout",
                attempt=1,
                branch=branch,
            )
        )
        # Mark as virtually in_flight for subsequent refill iterations
        state.in_flight.append(new_rec)

    return plans


def _next_step(current: str, attempt: int) -> StepName:
    """Map (current_step, attempt) → next step.

    Note: 'attempt' here is the per-task retry counter for the *current* step.
    When the SKILL.md sees a step error, it increments attempt and re-plans with
    the same step.
    """
    order: list[StepName] = [
        "branch_checkout",
        "implement",
        "check",
        "update_spec",
        "commit",
        "merge",
        "branch_cleanup",
    ]
    try:
        idx = order.index(current)  # type: ignore[arg-type]
    except ValueError:
        return "branch_checkout"
    if idx + 1 < len(order):
        return order[idx + 1]  # type: ignore[return-value]
    return "branch_cleanup"


def summarize(blocks: list[TaskBlock], state: OrchestrationState) -> Summary:
    return Summary(
        done=list(state.completed),
        failed=[(f.id, f.error) for f in state.failed],
        skipped=list(state.skipped),
        in_flight=[r.id for r in state.in_flight],
        total=len(blocks),
    )


if __name__ == "__main__":
    import sys
    from parse_implement import parse_implement
    from state import load

    md = sys.argv[1] if len(sys.argv) > 1 else ".trellis/tasks/10-02-ai-kbs-mvp/implement.md"
    state_path = sys.argv[2] if len(sys.argv) > 2 else "orchestration-state.json"

    blocks = parse_implement(md)
    state = load(state_path)

    plans = build_dispatch_plans(blocks, state, state.config)
    print(f"eligible pool size: {len(plans)}")
    for p in plans:
        print(f"  {p.task_block.id:8} step={p.step:14} attempt={p.attempt} branch={p.branch}")

    s = summarize(blocks, state)
    print(
        f"\nSUMMARY done={len(s.done)} failed={len(s.failed)} "
        f"skipped={len(s.skipped)} in_flight={len(s.in_flight)} total={s.total}"
    )
