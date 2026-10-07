"""Atomic load/save for orchestration-state.json.

Schema v1 — see docs/plans/2026-10-07-ai-kbs-auto-orchestrator-design/_index.md §Glossary
and architecture.md §4.
"""
from __future__ import annotations

import json
import os
from dataclasses import asdict, dataclass, field
from datetime import datetime, timezone


SCHEMA_VERSION = 1


@dataclass
class InFlightRecord:
    id: str               # task-block id, e.g. "T1.5"
    branch: str           # "feature/T1.5"
    started_at: str       # ISO8601 UTC
    step: str = "implement"  # implement | check | update_spec | commit | merge
    attempts: int = 1


@dataclass
class FailedRecord:
    id: str
    error: str
    attempts: int
    branch: str


@dataclass
class LoopConfig:
    max_retries: int = 2
    dry_run: bool = False
    base_branch: str = "require"
    max_parallel: int = 2


@dataclass
class OrchestrationState:
    version: int = SCHEMA_VERSION
    current_task: str | None = None
    in_flight: list[InFlightRecord] = field(default_factory=list)
    completed: list[str] = field(default_factory=list)
    failed: list[FailedRecord] = field(default_factory=list)
    skipped: list[str] = field(default_factory=list)
    last_update: str = ""
    config: LoopConfig = field(default_factory=LoopConfig)

    def is_terminal(self, task_id: str) -> bool:
        return (
            task_id in self.completed
            or any(f.id == task_id for f in self.failed)
            or task_id in self.skipped
        )

    def to_dict(self) -> dict:
        d = asdict(self)
        return d

    @classmethod
    def from_dict(cls, d: dict) -> "OrchestrationState":
        if d.get("version") != SCHEMA_VERSION:
            raise ValueError(
                f"Unsupported state version: {d.get('version')!r} "
                f"(expected {SCHEMA_VERSION}). Refusing to load."
            )
        return cls(
            version=d.get("version", SCHEMA_VERSION),
            current_task=d.get("current_task"),
            in_flight=[InFlightRecord(**r) for r in d.get("in_flight", [])],
            completed=list(d.get("completed", [])),
            failed=[FailedRecord(**r) for r in d.get("failed", [])],
            skipped=list(d.get("skipped", [])),
            last_update=d.get("last_update", ""),
            config=LoopConfig(**d.get("config", {})),
        )


def now_iso() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def save(state: OrchestrationState, path: str) -> None:
    """Atomic write — temp file + os.replace."""
    state.last_update = now_iso()
    tmp = path + ".tmp"
    with open(tmp, "w", encoding="utf-8") as f:
        json.dump(state.to_dict(), f, ensure_ascii=False, indent=2)
        f.flush()
        os.fsync(f.fileno())
    os.replace(tmp, path)


def load(path: str) -> OrchestrationState:
    if not os.path.exists(path):
        return OrchestrationState()
    with open(path, "r", encoding="utf-8") as f:
        d = json.load(f)
    return OrchestrationState.from_dict(d)


if __name__ == "__main__":
    import sys

    path = sys.argv[1] if len(sys.argv) > 1 else "orchestration-state.json"
    s = load(path)
    print(json.dumps(s.to_dict(), ensure_ascii=False, indent=2))
