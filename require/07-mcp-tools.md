# 07 · MCP 工具清单

> MCP server 暴露给 AI Agent（Claude Code / Codex / Cursor / Gemini CLI）的工具集合。所有工具输入/输出遵循 MCP 标准 + frozen schema v1.0。

## 1. MCP Server 配置

### 1.1 传输方式

| 传输 | 适用场景 | 默认端口 |
|------|----------|----------|
| stdio | 本地进程（Claude Code 直接调用） | — |
| SSE | 远程服务（HTTP 长连接） | 8081 |
| HTTP | REST + SSE 混合 | 8081 |

### 1.2 一键接入

```bash
# Claude Code（写入 .mcp.json）
ai-kbs mcp install --agent claude-code

# Codex
ai-kbs mcp install --agent codex

# Cursor
ai-kbs mcp install --agent cursor

# Gemini CLI
ai-kbs mcp install --agent gemini-cli
```

生成的 `.mcp.json` 示例：

```json
{
  "mcpServers": {
    "ai-kbs": {
      "command": "ai-kbs",
      "args": ["mcp", "serve", "--transport", "stdio"],
      "env": {
        "AIKBS_URL": "https://kb.example.com",
        "AIKBS_KEY_ID": "${AIKBS_KEY_ID}",
        "AIKBS_KEY_SECRET": "${AIKBS_KEY_SECRET}"
      }
    }
  }
}
```

## 2. 工具分层

### 2.1 Core 层（所有 Agent 默认可用）

| 工具 | 用途 |
|------|------|
| `ai_kbs_search` | 跨 space 关键词/语义搜索 |
| `ai_kbs_read_doc` | 读取单个文档（含 outline / section） |
| `ai_kbs_list_spaces` | 列出可用 space |
| `ai_kbs_get_doc_outline` | 获取文档大纲 |
| `ai_kbs_ask` | 自然语言提问（ReAct Agent） |

### 2.2 Write 层（需 write capability）

| 工具 | 用途 |
|------|------|
| `ai_kbs_create_doc` | 创建新文档 |
| `ai_kbs_update_doc` | 覆盖文档（CAS） |
| `ai_kbs_edit_doc` | 局部替换（CAS） |
| `ai_kbs_move_doc` | 移动/重命名 |
| `ai_kbs_delete_doc` | 软删除 |

### 2.3 Version 层（需 version capability）

| 工具 | 用途 |
|------|------|
| `ai_kbs_list_versions` | 列出版本 |
| `ai_kbs_diff_versions` | 版本对比 |
| `ai_kbs_rollback_version` | 回滚 |

### 2.4 SAG 层（多跳推理）

| 工具 | 用途 |
|------|------|
| `ai_kbs_sag_query` | 直接调用 SAG 推理（事件-实体） |
| `ai_kbs_sag_explain` | 解释 SAG 推理路径 |

### 2.5 Admin 层（需 admin capability）

| 工具 | 用途 |
|------|------|
| `ai_kbs_create_space` | 创建空间 |
| `ai_kbs_manage_binding` | 绑定管理 |
| `ai_kbs_get_audit_log` | 审计日志 |
| `ai_kbs_manage_api_keys` | API Key 管理 |

## 3. 工具详细规范

### 3.1 ai_kbs_search

**描述**：跨 space 关键词 + 向量混合搜索；返回 top-K 结果。

**输入 schema**：

```json
{
  "type": "object",
  "properties": {
    "query": { "type": "string", "description": "搜索关键词或自然语言查询" },
    "space_codes": { "type": "array", "items": { "type": "string" }, "description": "限定 space" },
    "path_prefix": { "type": "string", "description": "限定路径前缀" },
    "tags": { "type": "array", "items": { "type": "string" } },
    "top_k": { "type": "integer", "default": 10, "minimum": 1, "maximum": 50 },
    "engine": { "type": "string", "enum": ["auto", "es", "sag", "hybrid"], "default": "auto" }
  },
  "required": ["query"]
}
```

**输出 schema**：

```json
{
  "type": "object",
  "properties": {
    "engine": { "type": "string" },
    "total": { "type": "integer" },
    "items": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "doc_code": { "type": "string" },
          "space_code": { "type": "string" },
          "title": { "type": "string" },
          "path": { "type": "string" },
          "snippet": { "type": "string" },
          "score": { "type": "number" },
          "highlights": { "type": "object" },
          "engine": { "type": "string" }
        }
      }
    },
    "took_ms": { "type": "integer" }
  }
}
```

### 3.2 ai_kbs_read_doc

**输入**：

```json
{
  "type": "object",
  "properties": {
    "doc_code": { "type": "string" },
    "space_code": { "type": "string" },
    "version": { "type": "integer", "description": "0=当前激活版本" },
    "section": { "type": "string", "description": "按标题" },
    "lines": { "type": "integer", "default": 0 },
    "offset": { "type": "integer", "default": 0 },
    "format": { "type": "string", "enum": ["content", "outline", "summary", "meta"], "default": "content" }
  },
  "required": ["doc_code"]
}
```

**输出**：

```json
{
  "type": "object",
  "properties": {
    "doc_code": { "type": "string" },
    "space_code": { "type": "string" },
    "title": { "type": "string" },
    "version": { "type": "integer" },
    "md5": { "type": "string" },
    "content": { "type": "string" },
    "outline": { "type": "array" },
    "summary": { "type": "string" },
    "metadata": { "type": "object" },
    "url": { "type": "string" }
  }
}
```

### 3.3 ai_kbs_ask

**输入**：

```json
{
  "type": "object",
  "properties": {
    "question": { "type": "string" },
    "space_codes": { "type": "array", "items": { "type": "string" } },
    "session_id": { "type": "string" },
    "top_k": { "type": "integer", "default": 5 },
    "engine": { "type": "string", "enum": ["auto", "es", "sag", "hybrid"], "default": "auto" },
    "stream": { "type": "boolean", "default": true }
  },
  "required": ["question"]
}
```

**输出**（流式，逐 token / 事件）：

```json
{"type":"init","session_id":"sess-xxx","engine":"hybrid"}
{"type":"thinking","content":"The user asks about..."}
{"type":"tool_call","name":"ai_kbs_search","input":{...}}
{"type":"tool_result","name":"ai_kbs_search","output":{...}}
{"type":"answer","content":"...","citations":[{"doc_code":"aB3kX9q2","title":"...","quote":"..."}]}
{"type":"complete","done":true,"usage":{...}}
```

### 3.4 ai_kbs_create_doc

**输入**：

```json
{
  "type": "object",
  "properties": {
    "space_code": { "type": "string" },
    "title": { "type": "string" },
    "path": { "type": "string" },
    "content": { "type": "string" },
    "tags": { "type": "array", "items": { "type": "string" } },
    "parent_doc_code": { "type": "string" }
  },
  "required": ["space_code", "title", "path", "content"]
}
```

**输出**：

```json
{
  "type": "object",
  "properties": {
    "doc_code": { "type": "string" },
    "version": { "type": "integer" },
    "url": { "type": "string" }
  }
}
```

### 3.5 ai_kbs_update_doc

**输入**：

```json
{
  "type": "object",
  "properties": {
    "doc_code": { "type": "string" },
    "content": { "type": "string" },
    "base_version": { "type": "integer", "description": "CAS 基础版本" },
    "message": { "type": "string" }
  },
  "required": ["doc_code", "content", "base_version"]
}
```

**输出**：同 create。

**错误（22 VersionConflict）**：

```json
{
  "error": {
    "code": "doc.version_conflict",
    "message": "文档版本冲突"
  },
  "next_action": {
    "tool": "ai_kbs_read_doc",
    "args": { "doc_code": "aB3kX9q2" },
    "reason": "版本冲突，先 read 获取最新 version 后重试"
  }
}
```

### 3.6 ai_kbs_edit_doc

**输入**：

```json
{
  "type": "object",
  "properties": {
    "doc_code": { "type": "string" },
    "old_text": { "type": "string" },
    "new_text": { "type": "string" },
    "base_version": { "type": "integer" },
    "replace_all": { "type": "boolean", "default": false },
    "dry_run": { "type": "boolean", "default": false }
  },
  "required": ["doc_code", "old_text", "new_text", "base_version"]
}
```

### 3.7 ai_kbs_sag_query

**输入**：

```json
{
  "type": "object",
  "properties": {
    "question": { "type": "string" },
    "space_codes": { "type": "array", "items": { "type": "string" } },
    "top_k_events": { "type": "integer", "default": 5 },
    "include_explanation": { "type": "boolean", "default": false }
  },
  "required": ["question"]
}
```

**输出**：

```json
{
  "type": "object",
  "properties": {
    "events": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "event_id": { "type": "string" },
          "chunk_id": { "type": "string" },
          "doc_code": { "type": "string" },
          "event_text": { "type": "string" },
          "score": { "type": "number" },
          "linked_entities": { "type": "array" }
        }
      }
    },
    "entities": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "entity_id": { "type": "string" },
          "name": { "type": "string" },
          "type": { "type": "string" },
          "score": { "type": "number" }
        }
      }
    },
    "hyperedges_count": { "type": "integer" },
    "explanation": { "type": "string" }
  }
}
```

### 3.8 ai_kbs_list_versions

**输入**：

```json
{
  "type": "object",
  "properties": {
    "doc_code": { "type": "string" }
  },
  "required": ["doc_code"]
}
```

**输出**：

```json
{
  "type": "object",
  "properties": {
    "doc_code": { "type": "string" },
    "active_version": { "type": "integer" },
    "items": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "version": { "type": "integer" },
          "status": { "type": "string", "enum": ["active", "archived"] },
          "md5": { "type": "string" },
          "author": { "type": "string" },
          "message": { "type": "string" },
          "created_at": { "type": "string", "format": "date-time" }
        }
      }
    }
  }
}
```

### 3.9 ai_kbs_rollback_version

**输入**：

```json
{
  "type": "object",
  "properties": {
    "doc_code": { "type": "string" },
    "target_version": { "type": "integer" },
    "message": { "type": "string" }
  },
  "required": ["doc_code", "target_version"]
}
```

### 3.10 ai_kbs_list_spaces

**输入**：无（或 `{}`）

**输出**：

```json
{
  "type": "object",
  "properties": {
    "items": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "code": { "type": "string" },
          "name": { "type": "string" },
          "role": { "type": "string", "enum": ["owner", "admin", "contributor", "viewer"] },
          "doc_count": { "type": "integer" }
        }
      }
    }
  }
}
```

## 4. 错误处理

所有工具的错误响应遵循统一 envelope：

```json
{
  "error": {
    "code": "<namespace.code>",
    "message": "人类可读消息",
    "detail": { /* 可选结构化上下文 */ }
  },
  "next_action": {
    "tool": "<suggested_tool>",
    "args": { /* 建议参数 */ },
    "reason": "<为什么>"
  }
}
```

错误码命名空间（与 02-glossary.md 一致）：

| Namespace | 例子 |
|-----------|------|
| `auth.*` | `auth.unauthenticated`、`auth.invalid_token` |
| `rbac.*` | `rbac.permission_denied`、`rbac.workspace_not_found` |
| `doc.*` | `doc.not_found`、`doc.version_conflict`、`doc.already_exists` |
| `binding.*` | `binding.not_found`、`binding.overlap` |
| `sync.*` | `sync.partial_success`、`sync.conflicts_remain` |
| `search.*` | `search.engine_error`、`search.timeout` |
| `sag.*` | `sag.indexing_failed`、`sag.query_timeout` |
| `internal.*` | `internal.error`、`internal.unavailable` |

## 5. OAuth2 支持（远程 MCP）

远程部署的 MCP server 支持 OAuth2：

```bash
# 用户首次访问
ai-kbs mcp install --agent claude-code --remote https://mcp.example.com

# 触发 OAuth 流程
# 浏览器打开授权页 → 登录 → 回调 → 自动写 .mcp.json
```

OAuth scopes：

| Scope | 包含能力 |
|-------|----------|
| `kbs:read` | search / read_doc / list_spaces |
| `kbs:write` | create / update / edit / move / delete |
| `kbs:version` | version.* 工具 |
| `kbs:admin` | admin.* 工具 |

## 6. 安全约束

- 所有工具调用携带 `request_id`，AI Agent 应在调用时传入 `client_request_id` 便于追踪
- 工具调用限流：单 API key 默认 60 req/min，可配
- 大文档读取：`read_doc` 单次 ≤ 5MB；超大文档走 `lines` + `offset` 分页
- 危险操作（`delete_doc` / `rollback_version`）返回 `dry_run` 预览模式
- Token / API Key 不出现在工具输入/输出
