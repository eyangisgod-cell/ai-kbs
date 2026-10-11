# ADR-0003: CLI Command Set Independently Designed

- **Status:** Accepted
- **Date:** 2026-10-02

## Context

The team has previously built a similar system internally. The internal CLI is well-designed but carries platform-specific naming and configuration that the open-source project must not inherit.

## Decision

The AI-KBS CLI command set is **independently designed**. The verb names (`init / bind / pull / push / read / write / search / ask / mcp`) follow common engineering conventions, but the flag set, subcommand structure, error envelope, and exit codes are AI-KBS's own.

## Consequences

Easier:
- No upstream-internal naming leaks into the public codebase
- The CLI surface is documented and stable from day one

Harder:
- Engineers familiar with the internal CLI will need to re-learn the mapping

## Alternatives considered

1. **Adopt internal CLI verbatim.** Rejected: violates ADR-0007.
2. **Adopt a popular OSS CLI style (e.g. `gh`, `kubectl`).** Considered; we kept a unique style to avoid misleading users who assume the semantics match.
