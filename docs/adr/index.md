# Architecture Decision Records

Significant architectural decisions, with their context, options, and consequences.

| ID | Title | Status |
|----|-------|--------|
| [ADR-0001](/adr/0001-fork-weknora) | Fork WeKnora v0.7.1 as the backend base | Accepted |
| [ADR-0002](/adr/0002-sag-as-multi-hop) | Add SAG as a multi-hop retrieval layer | Accepted |
| [ADR-0003](/adr/0003-cli-independence) | CLI command set independently designed | Accepted |
| [ADR-0004](/adr/0004-cas-instead-of-git) | CAS + docCode for versioning (not Git) | Accepted |
| [ADR-0005](/adr/0005-mcp-as-agent-entry) | MCP as the canonical AI agent entry | Accepted |
| [ADR-0006](/adr/0006-v1-includes-web) | v1 GA includes Web UI (forked from WeKnora Vue 3 SPA) | Accepted |
| [ADR-0007](/adr/0007-no-internal-naming) | No upstream-internal naming in the public codebase | Accepted |
| [ADR-0008](/adr/0008-no-migration-tool-v1) | Data-migration tool deferred to v1.1 | Accepted |
| [ADR-0009](/adr/0009-vue3-spa-fork) | Web UI based on WeKnora Vue 3 SPA fork + white-label | Accepted |

## Format

Each ADR follows the structure:

- **Context** — what forces drove the decision
- **Decision** — what we chose
- **Consequences** — what becomes easier, what becomes harder
- **Alternatives considered** — what we rejected and why

ADRs are immutable once accepted. Superseded decisions get a new ADR that links to the old one.
