# CLI

`ai-kbs` is a single static Go binary with no runtime dependencies.

## Distribution

Download a release artifact for your OS + arch:

| OS | Arch | File |
|----|------|------|
| Linux | amd64 | `ai-kbs-linux-amd64` |
| Linux | arm64 | `ai-kbs-linux-arm64` |
| macOS | amd64 | `ai-kbs-darwin-amd64` |
| macOS | arm64 | `ai-kbs-darwin-arm64` |
| Windows | amd64 | `ai-kbs-windows-amd64.exe` |
| Windows | arm64 | `ai-kbs-windows-arm64.exe` |

Or install via `go install`:

```bash
go install github.com/eyangisgod-cell/ai-kbs/cli/cmd/ai-kbs@latest
```

## Global flags

```
--url <url>            Backend URL (or use $AIKBS_URL)
--profile <name>       Profile from ~/.ai-kbs/config.yaml (default: "default")
--format <fmt>         Output: text | json | ndjson (default: text)
--quiet                Suppress non-essential output
--yes                  Skip confirmation prompts
--timeout <dur>        Per-request timeout (default: 30s)
--debug                Enable debug logging
```

## Environment variables

| Variable | Maps to |
|----------|---------|
| `AIKBS_URL` | `--url` |
| `AIKBS_KEY_ID` | HMAC key id |
| `AIKBS_KEY_SECRET` | HMAC key secret |
| `AIKBS_TOKEN` | JWT or scoped API key |
| `AIKBS_PROFILE` | `--profile` |
| `AIKBS_WORKSPACE_FILE` | path to `workspace.yaml` |
| `AIKBS_TIMEOUT` | `--timeout` |
| `AIKBS_CONFIG_DIR` | directory holding `~/.ai-kbs/*` (default: `$HOME/.ai-kbs`) |
| `AIKBS_DEBUG` | `--debug` |

## Credential resolution

Precedence (first wins):

1. `$AIKBS_KEY_ID` + `$AIKBS_KEY_SECRET` (HMAC)
2. `~/.ai-kbs/credentials:hmac`
3. `~/.ai-kbs/credentials:token`
4. `$AIKBS_TOKEN`
5. exit 10

## Commands (excerpt)

| Command | Purpose |
|---------|---------|
| `ai-kbs login` / `logout` | Browser OAuth2 or paste a token |
| `ai-kbs init` | Create `.ai-kbs/workspace.yaml` |
| `ai-kbs status` | Show current workspace + sync state |
| `ai-kbs bind` / `unbind` | Bind a local dir to a remote Space |
| `ai-kbs pull` / `push` | Sync local ↔ remote (8-way concurrent) |
| `ai-kbs read` / `write` / `edit` | Single-document operations (CAS) |
| `ai-kbs search` / `grep` / `tree` | Browse |
| `ai-kbs spaces` | List visible Spaces |
| `ai-kbs version` | List / diff / rollback versions |
| `ai-kbs move` / `undelete` / `resolve` | Move / restore / CAS conflict resolution |
| `ai-kbs ask` | NDJSON streaming Agent answer |
| `ai-kbs mcp install` | Write `.mcp.json` for an AI agent |

See [Install](/cli/install) and [Commands](/cli/commands) for details.
