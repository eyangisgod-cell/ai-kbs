# Quick Tour

A 5-minute walkthrough of AI-KBS's core flows.

## 1. Install the CLI

```bash
# Linux / macOS
curl -fsSL https://github.com/eyangisgod-cell/ai-kbs/releases/latest/download/ai-kbs-$(uname -s | tr A-Z a-z)-amd64 -o ai-kbs
chmod +x ai-kbs && sudo mv ai-kbs /usr/local/bin/

# Windows (PowerShell)
Invoke-WebRequest -Uri "https://github.com/eyangisgod-cell/ai-kbs/releases/latest/download/ai-kbs-windows-amd64.exe" -OutFile ai-kbs.exe
```

## 2. Initialize a workspace

```bash
mkdir my-kb && cd my-kb
ai-kbs init                # creates .ai-kbs/workspace.yaml
ai-kbs login --url https://kbs.example.com
ai-kbs bind dev:/api/      # binds the local dir to the remote Space "dev"
```

## 3. Push a document

```bash
echo "# Hello\nThis is my first doc." > hello.md
ai-kbs push                # uploads + chunks + indexes async
```

## 4. Ask a question

```bash
ai-kbs ask "what is the hello doc about?"
```

The CLI prints an NDJSON stream:

```jsonl
{"type":"init","session_id":"s_..."}
{"type":"thinking","thought":"single-hop retrieval"}
{"type":"tool_call","tool":"search","args":{"q":"hello doc"}}
{"type":"tool_result","engine":"es","hits":1}
{"type":"answer","text":"...","citations":[{"doc_code":"aB3xY9zQ","title":"hello","quote":"Hello"}]}
{"type":"complete"}
```

## 5. Connect an AI agent

```bash
ai-kbs mcp install --agent claude-code
```

This writes `.mcp.json` and (after a Claude Code restart) exposes tools like `ai_kbs_search`, `ai_kbs_read_doc`, `ai_kbs_ask`, `ai_kbs_create_doc`, `ai_kbs_update_doc`, and the SAG-layer tools.

## Next

- [Architecture overview](/architecture/)
- [CLI command reference](/cli/commands)
- [MCP tools reference](/mcp/tools)
