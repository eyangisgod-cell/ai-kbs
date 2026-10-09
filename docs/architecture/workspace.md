# Workspace / Sync Model

The local-to-remote sync model is the backbone of CLI usage.

## Concepts

- **Workspace** — a local directory holding `.ai-kbs/workspace.yaml` + `.ai-kbs/sync-state.json`
- **Space** — a remote named KB; one workspace can be bound to many Spaces
- **docCode** — 8-character base62 stable identifier; survives rename / move
- **CAS** — writes carry `base_version`; conflicts return `next_action`
- **Versions** — every write appends a new version; rollback creates a new version on top

## Bind

```bash
ai-kbs bind dev:/api/    # local dir → remote Space "dev", path "/api/"
```

`bind` writes the binding into `.ai-kbs/workspace.yaml` and seeds `sync-state.json`.

## Pull / Push

```bash
ai-kbs pull    # remote → local; respects .ai-kbs/sync-state.json
ai-kbs push    # local → remote; 8-way concurrent
```

Both are idempotent. `sync-state.json` records the per-file mapping:

```json
{
  "version": 1,
  "files": {
    "docs/intro.md": {
      "md5": "...",
      "version": 7,
      "docCode": "aB3xY9zQ",
      "remotePath": "/api/docs/intro.md"
    }
  }
}
```

## Conflict resolution

When `push` finds the remote is at a newer version, the CLI returns `next_action.strategies` so a human (or agent) can choose:

- `keep-mine` — overwrite remote (you accept their changes are lost)
- `keep-theirs` — discard local (you accept your changes are lost)
- `both` — keep both copies; remote renamed to a conflict suffix
- `auto` — three-way merge on plain-text; otherwise `both`

Or resolve non-interactively: `ai-kbs resolve --strategy both path/to/doc.md`.
