"""Unit tests for trellis-auto-run-task scripts.

Run from the skill directory:

    cd .claude/skills/trellis-auto-run-task
    python -m unittest discover tests

Covers:
  - parse_implement: header + table extraction, dependency range expansion
  - state: atomic save/load, version refusal
  - loop: eligibility, predecessor gating, in_flight pool cap
"""
from __future__ import annotations

import json
import os
import sys
import tempfile
import unittest

# Make scripts/ importable when running from the skill root
HERE = os.path.dirname(os.path.abspath(__file__))
SKILL_ROOT = os.path.dirname(HERE)
SCRIPTS = os.path.join(SKILL_ROOT, "scripts")
sys.path.insert(0, SCRIPTS)

from parse_implement import parse_implement, TaskBlock, ParseError  # noqa: E402
from state import OrchestrationState, save, load, LoopConfig, SCHEMA_VERSION  # noqa: E402
import loop as loop_mod  # noqa: E402


SAMPLE_MD = """\
### T0.1 Foo
| 项 | 内容 |
|----|------|
| **输入** | x |
| **动作** | do foo |
| **输出** | y |
| **依赖** | — |
| **验收** | builds |
| **风险** | none |

### T0.2 Bar
| **输入** | a |
| **动作** | do bar |
| **输出** | b |
| **依赖** | T0.1 |
| **验收** | tested |
| **风险** | low |

### T1.1 Baz
| **输入** | p |
| **动作** | do baz |
| **输出** | q |
| **依赖** | T0.1 + T0.2 |
| **验收** | e2e |
| **风险** | med |

### T1.5 Range
| **输入** | r |
| **动作** | do range |
| **输出** | s |
| **依赖** | T0.1 + T0.10-0.12 |
| **验收** | ok |
| **风险** | low |
"""


class ParseImplementTests(unittest.TestCase):
    def test_parses_all_blocks(self):
        blocks = parse_implement.__wrapped__ if hasattr(parse_implement, "__wrapped__") else parse_implement
        # Use the public function on a tempfile
        with tempfile.NamedTemporaryFile("w", suffix=".md", delete=False, encoding="utf-8") as f:
            f.write(SAMPLE_MD)
            path = f.name
        try:
            blocks = parse_implement(path)
        finally:
            os.unlink(path)
        self.assertEqual(len(blocks), 4)
        self.assertEqual([b.id for b in blocks], ["T0.1", "T0.2", "T1.1", "T1.5"])

    def test_fields_extracted(self):
        with tempfile.NamedTemporaryFile("w", suffix=".md", delete=False, encoding="utf-8") as f:
            f.write(SAMPLE_MD)
            path = f.name
        try:
            blocks = parse_implement(path)
        finally:
            os.unlink(path)
        self.assertEqual(blocks[0].title, "Foo")
        self.assertEqual(blocks[0].action, "do foo")
        self.assertEqual(blocks[0].acceptance, "builds")
        self.assertEqual(blocks[1].predecessors(), ["T0.1"])
        self.assertEqual(blocks[2].predecessors(), ["T0.1", "T0.2"])

    def test_dependency_range_expansion(self):
        with tempfile.NamedTemporaryFile("w", suffix=".md", delete=False, encoding="utf-8") as f:
            f.write(SAMPLE_MD)
            path = f.name
        try:
            blocks = parse_implement(path)
        finally:
            os.unlink(path)
        # T1.5 has deps "T0.1 + T0.10-0.12" → ["T0.1", "T0.10", "T0.11", "T0.12"]
        self.assertEqual(blocks[3].predecessors(), ["T0.1", "T0.10", "T0.11", "T0.12"])

    def test_missing_field_raises(self):
        bad = SAMPLE_MD.replace("| **动作** | do foo |\n", "")
        with tempfile.NamedTemporaryFile("w", suffix=".md", delete=False, encoding="utf-8") as f:
            f.write(bad)
            path = f.name
        try:
            with self.assertRaises(ParseError):
                parse_implement(path)
        finally:
            os.unlink(path)


class StateTests(unittest.TestCase):
    def test_round_trip(self):
        s = OrchestrationState(current_task="T1.5", completed=["T0.1", "T0.2"])
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "state.json")
            save(s, p)
            loaded = load(p)
        self.assertEqual(loaded.current_task, "T1.5")
        self.assertEqual(loaded.completed, ["T0.1", "T0.2"])
        self.assertEqual(loaded.version, SCHEMA_VERSION)

    def test_refuses_unknown_version(self):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "state.json")
            with open(p, "w") as f:
                json.dump({"version": 999, "completed": []}, f)
            with self.assertRaises(ValueError):
                load(p)

    def test_atomic_write_no_partial(self):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "state.json")
            s = OrchestrationState(completed=["T0.1"])
            save(s, p)
            self.assertTrue(os.path.exists(p))
            self.assertFalse(os.path.exists(p + ".tmp"))


class LoopTests(unittest.TestCase):
    def _blocks(self):
        with tempfile.NamedTemporaryFile("w", suffix=".md", delete=False, encoding="utf-8") as f:
            f.write(SAMPLE_MD)
            path = f.name
        try:
            return parse_implement(path)
        finally:
            os.unlink(path)

    def test_eligibility_gated_by_predecessors(self):
        blocks = self._blocks()
        state = OrchestrationState()
        # Initially, only T0.1 (no predecessors) is eligible
        next_block = loop_mod.pick_next_eligible(blocks, state)
        self.assertEqual(next_block.id, "T0.1")
        # After T0.1 is completed, T0.2 becomes eligible
        state.completed.append("T0.1")
        next_block = loop_mod.pick_next_eligible(blocks, state)
        self.assertEqual(next_block.id, "T0.2")
        # After T0.1 + T0.2, T1.1 becomes eligible
        state.completed.append("T0.2")
        next_block = loop_mod.pick_next_eligible(blocks, state)
        self.assertEqual(next_block.id, "T1.1")

    def test_in_flight_blocks_not_re_picked(self):
        blocks = self._blocks()
        state = OrchestrationState(in_flight=[
            __import__("state").InFlightRecord(id="T0.1", branch="feature/T0.1", started_at="now")
        ])
        # T0.1 is in_flight → not eligible; nothing else has no-predecessor besides it
        self.assertIsNone(loop_mod.pick_next_eligible(blocks, state))

    def test_max_parallel_caps_pool(self):
        blocks = self._blocks()
        # Add several independent blocks (no predecessors)
        state = OrchestrationState()
        # T0.1 only initially. With max_parallel=2, after dispatching T0.1 there is still capacity.
        # But the next block (T0.2) needs T0.1, which is in_flight → not eligible.
        # So refill should stop at 1.
        plans = loop_mod.build_dispatch_plans(blocks, state, LoopConfig(max_parallel=2))
        # Only one plan: branch_checkout for T0.1
        self.assertEqual(len(plans), 1)
        self.assertEqual(plans[0].task_block.id, "T0.1")
        self.assertEqual(plans[0].step, "branch_checkout")


if __name__ == "__main__":
    unittest.main()
