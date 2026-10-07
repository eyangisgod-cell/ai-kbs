"""Parse implement.md into a list of TaskBlock dataclasses.

Pure function — no I/O side effects beyond reading md_path. Idempotent.
Re-parsing the same file yields the same list.
"""
from __future__ import annotations

import re
from dataclasses import dataclass


HEADER_RE = re.compile(
    r"^###\s+(T(\d+)\.(\d+))\s+(.+?)\s*$",
    re.MULTILINE,
)

# Captures | **<field>** | <value> | where <field> ∈ {输入,动作,输出,依赖,验收,风险}
ROW_RE = re.compile(
    r"\|\s*\*\*(输入|动作|输出|依赖|验收|风险)\*\*\s*\|\s*(.+?)\s*\|",
    re.DOTALL,
)

REQUIRED_FIELDS = ("输入", "动作", "输出", "依赖", "验收")
OPTIONAL_FIELDS = ("风险",)


class ParseError(Exception):
    """Raised when a T-block is missing a required field."""


@dataclass(frozen=True)
class TaskBlock:
    id: str            # e.g. "T1.5"
    title: str
    section: str       # "T1"
    number: str        # "1.5"
    input: str
    action: str
    output: str
    dependency: str    # raw text; possibly empty
    acceptance: str
    risk: str
    line_start: int    # 0-based byte offset of the header

    def predecessors(self) -> list[str]:
        """Extract predecessor T-ids from the dependency field.

        Acceptable dependency shapes (raw text):
          ""                  → []
          "—"                 → []
          "T0.1"              → ["T0.1"]
          "T1.5 + T1.6"       → ["T1.5", "T1.6"]
          "T1.13-1.17 依赖 T1.10-1.12"  → range expansion (see below)
        """
        return _extract_dep_ids(self.dependency)


def _extract_dep_ids(raw: str) -> list[str]:
    """Extract T{major}.{n} ids from a dependency field.

    Handles ranges:
      "T0.1 + T0.10-0.12" → ["T0.1", "T0.10", "T0.11", "T0.12"]
      "T1.13-1.17"        → ["T1.13", "T1.14", ..., "T1.17"]

    Uses manual scanning because greedy \\d+ in finditer advances past
    "T0.10" → reads as "T0.1" + "0" then misses the rest.
    """
    if not raw or raw.strip() in {"", "—", "-"}:
        return []

    out: list[str] = []
    i = 0
    s = raw
    while i < len(s):
        idx = s.find("T", i)
        if idx == -1:
            break
        # Must be 'T' followed by digits
        j = idx + 1
        while j < len(s) and s[j].isdigit():
            j += 1
        if j == idx + 1:                       # 'T' with no digits — skip
            i = idx + 1
            continue
        major = int(s[idx + 1:j])
        # Expect '.'
        if j >= len(s) or s[j] != ".":
            i = j
            continue
        # Read the start number
        k = j + 1
        while k < len(s) and s[k].isdigit():
            k += 1
        if k == j + 1:                          # 'T0.' with no number
            i = k
            continue
        start = int(s[j + 1:k])

        # Look ahead for an optional "-\d+" range end
        end = start
        if k < len(s) and s[k] == "-":
            m = k + 1
            # Allow optional "<major>." prefix: "T0.10-0.12" or "T0.10-12"
            end_idx = s.find(".", m)
            if end_idx != -1 and end_idx - m < 5:
                # Check the run between m and end_idx is all digits
                if all(c.isdigit() for c in s[m:end_idx]):
                    m = end_idx + 1
            # Now read digits
            n = m
            while n < len(s) and s[n].isdigit():
                n += 1
            if n > m:
                end = int(s[m:n])
                k = n

        for num in range(start, end + 1):
            out.append(f"T{major}.{num}")

        i = k

    return sorted(set(out))


def parse_implement(md_path: str) -> list[TaskBlock]:
    with open(md_path, "r", encoding="utf-8") as f:
        text = f.read()

    blocks: list[TaskBlock] = []

    # Walk each T-block header and slice the section until the next header
    matches = list(HEADER_RE.finditer(text))
    for idx, m in enumerate(matches):
        block_id = m.group(1)
        section = f"T{m.group(2)}"
        number = f"{m.group(2)}.{m.group(3)}"
        title = m.group(4).strip()

        start = m.end()
        end = matches[idx + 1].start() if idx + 1 < len(matches) else len(text)
        section_text = text[start:end]

        # Extract the 6 required rows
        fields: dict[str, str] = {}
        for row in ROW_RE.finditer(section_text):
            fields[row.group(1)] = row.group(2).strip()

        missing = [f for f in REQUIRED_FIELDS if f not in fields]
        if missing:
            raise ParseError(
                f"{block_id} missing required field(s): {', '.join(missing)}"
            )

        blocks.append(
            TaskBlock(
                id=block_id,
                title=title,
                section=section,
                number=number,
                input=fields["输入"],
                action=fields["动作"],
                output=fields["输出"],
                dependency=fields["依赖"],
                acceptance=fields["验收"],
                risk=fields.get("风险", ""),
                line_start=m.start(),
            )
        )

    return blocks


if __name__ == "__main__":
    import json
    import sys

    md = sys.argv[1] if len(sys.argv) > 1 else ".trellis/tasks/10-02-ai-kbs-mvp/implement.md"
    parsed = parse_implement(md)
    out = [
        {
            "id": b.id,
            "title": b.title,
            "predecessors": b.predecessors(),
            "line_start": b.line_start,
        }
        for b in parsed
    ]
    print(json.dumps(out, ensure_ascii=False, indent=2))
