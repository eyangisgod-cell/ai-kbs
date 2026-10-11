# MCP Install for AI Agents

One command per supported agent.

## Claude Code

```bash
ai-kbs mcp install --agent claude-code
ai-kbs mcp install --agent claude-code --api-key ak_live_xxx --scopes kbs:read,kbs:write
```

Writes `.mcp.json` in the current project (or `~/.config/claude/mcp.json` for user-wide) and prints the restart command.

```json
{
  "mcpServers": {
    "ai-kbs": {
      "command": "ai-kbs",
      "args": ["mcp", "serve", "--transport", "stdio"],
      "env": {
        "AIKBS_URL": "https://kbs.example.com",
        "AIKBS_TOKEN": "ak_live_xxx"
      }
    }
  }
}
```

## Codex

```bash
ai-kbs mcp install --agent codex
```

Writes `~/.codex/config.toml`:

```toml
[mcp_servers.ai-kbs]
command = "ai-kbs"
args = ["mcp", "serve", "--transport", "stdio"]

[mcp_servers.ai-kbs.env]
AIKBS_URL = "https://kbs.example.com"
AIKBS_TOKEN = "ak_live_xxx"
```

## Cursor

```bash
ai-kbs mcp install --agent cursor
```

Writes `~/.cursor/mcp.json` (same shape as Claude Code).

## Gemini CLI

```bash
ai-kbs mcp install --agent gemini-cli
```

Writes `~/.gemini/settings.json`.

## Verifying the install

After restarting the agent, send a probe:

```text
List the available ai-kbs tools, then call ai_kbs_list_spaces and report the result.
```

If the agent reports the tool list and the call succeeds, the install is complete.

## Updating credentials

```bash
ai-kbs mcp install --agent claude-code --api-key ak_live_yyy --scopes kbs:read,kbs:write,kbs:version
```

This overwrites the existing entry. Restart the agent.
