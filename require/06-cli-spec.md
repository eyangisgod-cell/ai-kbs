# 06 · CLI 命令规范

> 借鉴 losu-wiki-cli 的设计模式（init/bind/pull/push/version/docCode 寻址、`--json` envelope、SKILL 手册）。命令名改为 `ai-kbs` 前缀，输出 schema 重命名但保持 frozen。

## 1. 全局设计

### 1.1 命令前缀

- 可执行文件名：`ai-kbs`
- 子命令：`ai-kbs init`、`ai-kbs pull`、`ai-kbs ask` 等
- 别名：`aikbs`（短别名，v1.1 启用）

### 1.2 全局 flag

| Flag | 类型 | 默认 | 说明 |
|------|------|------|------|
| `--url <URL>` | string | — | 覆盖 ai-kbs 服务地址 |
| `--profile <NAME>` | string | — | 使用本地 profile（多 profile 切换） |
| `--format <FMT>` | string | `json` | `json` / `text` / `ndjson` |
| `--quiet` | bool | false | 仅输出关键值 |
| `--yes` | bool | false | 非交互确认（删除护栏时必填） |
| `--timeout <SEC>` | int | 30 | 请求超时 |
| `--debug` | bool | false | 调试日志 |

### 1.3 环境变量

| 变量 | 必填 | 默认 | 说明 |
|------|------|------|------|
| `AIKBS_URL` | 否* | — | 服务地址 |
| `AIKBS_KEY_ID` | 否 | — | HMAC Key ID（最高优先） |
| `AIKBS_KEY_SECRET` | 否 | — | HMAC Key Secret |
| `AIKBS_TOKEN` | 否 | — | JWT Token（仅直连后端） |
| `AIKBS_PROFILE` | 否 | — | 激活 profile |
| `AIKBS_WORKSPACE_FILE` | 否 | `.ai-kbs/workspace.yaml` | 工作空间配置路径 |
| `AIKBS_TIMEOUT` | 否 | `30` | 默认超时（秒） |
| `AIKBS_CONFIG_DIR` | 否 | `~/.ai-kbs` | 凭证目录 |
| `AIKBS_DEBUG` | 否 | — | 调试日志 |

### 1.4 凭证优先级链（auth/credential.go）

```
1. AIKBS_KEY_ID + AIKBS_KEY_SECRET     → HMAC 签名（headless/CI/云端注入）
2. ~/.ai-kbs/credentials: hmac         → HMAC（ai-kbs login 写入）
3. ~/.ai-kbs/credentials: token        → DirectToken（直连后端）
4. AIKBS_TOKEN                          → DirectToken（仅本地直连）
5. 无凭证                                → exit 10 (NotAuthenticated)
```

### 1.5 `--json` envelope（frozen schema v1.0）

#### 成功响应

```json
{
  "json_schema_version": "1.0",
  "success": true,
  "exit_code": 0,
  "data": { /* 业务 payload */ },
  "next_action": null,
  "request_id": "req-xxx"
}
```

#### 错误响应

```json
{
  "json_schema_version": "1.0",
  "success": false,
  "exit_code": 22,
  "error": {
    "code": "doc.version_conflict",
    "message": "文档版本冲突",
    "detail": { /* 可选，结构化上下文 */ }
  },
  "data": null,
  "next_action": {
    "cmd": "ai-kbs read",
    "args": ["--space", "dev", "--doc", "aB3kX9q2"],
    "reason": "版本冲突，先 read 获取最新 version 后重试",
    "strategies": ["keep-mine", "keep-theirs", "both"]
  },
  "request_id": "req-xxx"
}
```

**Envelope 字段说明**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `json_schema_version` | string | 固定 `"1.0"` |
| `success` | bool | 成功 `true` / 错误 `false` |
| `exit_code` | int | Unix 退出码 |
| `error` | object/null | `{code, message, detail?}`；成功时省略 |
| `data` | object/null | **成功时**业务 payload；**错误时恒为 null** |
| `next_action` | object/null | 可执行修复建议 |
| `request_id` | string | 后端 requestID |

## 2. 寻址体系（4 flag，与 losu-wiki 一致）

| 优先级 | 形态 | 说明 |
|--------|------|------|
| 1（最高） | `--url <pageURL>` | 自包含页面 URL：`https://host/kb/space/{space}/page/{docCode}` |
| 2 | `--space <space> --doc <docCode>` | 用稳定标识 docCode 寻址（推荐，重命名不断链） |
| 3 | `--space <space> --path <path>` | 用人类可读路径（CLI 调 `/docs/resolve` 转 docCode） |

**互斥规则**：
- `--url`（自包含）与 `--space`/`--doc`/`--path` 同时出现 → exit 2
- `--doc` 与 `--path` 同时出现 → exit 2
- `--space` 需配合 `--doc` 或 `--path`，否则 exit 2

## 3. 命令清单

### 3.1 认证类

#### `ai-kbs login`

```bash
ai-kbs login --url https://kb.example.com           # 浏览器授权
ai-kbs login --url https://kb.example.com --paste   # 手动粘贴 KeyID/KeySecret
ai-kbs login status                                  # 查看登录态
```

#### `ai-kbs logout`

```bash
ai-kbs logout
ai-kbs logout --local-only
```

### 3.2 工作空间类

#### `ai-kbs init`

```bash
ai-kbs init [--url <URL>] [--space <code>] [<path>]
```

创建 `.ai-kbs/workspace.yaml`。已存在 → exit 21。

#### `ai-kbs status`

```bash
ai-kbs status [--show server|workspace|binding|all]
```

显示服务端登录态、工作空间、绑定、当前目录命中。

#### `ai-kbs bind`

```bash
ai-kbs bind <local> <remote> [--skip-verify]
# 例：ai-kbs bind docs dev:/api/
```

#### `ai-kbs unbind`

```bash
ai-kbs unbind <local>
```

### 3.3 同步类

#### `ai-kbs pull`

```bash
ai-kbs pull [<path>]
  [--recursive] [--dry-run] [--prune] [--yes]
  [--include <patterns>] [--exclude <patterns>] [--all-files]
  [--binary auto|skip|include]
```

8 路并发拉取；删除护栏：>10 文件 / >5MB 需 `--yes`；Detached 文件不被删除。

#### `ai-kbs push`

```bash
ai-kbs push [<path>]
  [--recursive] [--dry-run] [--prune] [--yes]
  [--strategy manual|keep-mine|keep-theirs|both|auto]
  [--include <patterns>] [--exclude <patterns>] [--all-files]
  [--binary auto|skip|include]
```

8 路并发推送；CAS 乐观锁；冲突按 `--strategy` 处理。

### 3.4 单文档类

#### `ai-kbs read`

```bash
ai-kbs read --space <s> [--doc <code>|--path <p>|--url <u>]
  [--content] [--outline] [--summary] [--meta]
  [--section <title>] [--lines <n>] [--offset <n>]
  [--version <n>]
ai-kbs read --paths <p1,p2,...> --space <s>   # 批量
```

#### `ai-kbs write`

```bash
ai-kbs write --space <s> [--doc <code>|--path <p>]
  [--content <text>|--file <path>]
  [--create] [--base-version <n>] [--message <msg>]
  [--strategy manual|keep-mine|keep-theirs|both]
ai-kbs write --space <s> --dir <path> [--path-prefix <p>]
  [--overwrite] [--dry-run]                        # 目录批量
```

#### `ai-kbs edit`

```bash
ai-kbs edit --space <s> [--doc <code>|--path <p>]
  --old <text> --new <text>
  [--base-version <n>] [--dry-run] [--replace-all]
```

### 3.5 版本与回收站

#### `ai-kbs version`

```bash
ai-kbs version list  --space <s> --doc <code>
ai-kbs version diff  --space <s> --doc <code> --from <n> --to <n>
ai-kbs version rollback --space <s> --doc <code> --to <n> --message <msg>
```

回滚 = git revert 语义（产生新版本，中间版本保留）。

#### `ai-kbs move`

```bash
ai-kbs move --space <s> --doc <code> [--parent <code>|--title <t>]
```

docCode 不变。

#### `ai-kbs undelete`

```bash
ai-kbs undelete --space <s> [--doc <code>|--path <p>]
```

不产生新版本。

### 3.6 冲突解决

#### `ai-kbs resolve`

```bash
ai-kbs resolve --space <s> --doc <code> --base-version <n> --strategy <s>
ai-kbs resolve --batch --space <s> --strategy <s>
```

### 3.7 检索与浏览

#### `ai-kbs search`

```bash
ai-kbs search <keyword>
  [--space <s1,s2>] [--path <prefix>] [--tags <t1,t2>]
  [--limit <n>] [--page <n>]
```

#### `ai-kbs grep`

```bash
ai-kbs grep <pattern> --space <s>
  [--path <prefix>] [-n] [-i] [-C <n>] [--limit <n>]
```

#### `ai-kbs tree`

```bash
ai-kbs tree --space <s> [--path <p>] [--depth <n>]
```

#### `ai-kbs spaces`

```bash
ai-kbs spaces
```

冒烟测试命令。

### 3.8 问答类

#### `ai-kbs ask`

```bash
ai-kbs ask <question>
  [--space <s1,s2>] [--session <id>] [--top-k <n>]
  [--engine auto|es|sag|hybrid]            # 强制路由
```

NDJSON 流：`init` / `thinking` / `tool_call` / `answer` / `complete`。

### 3.9 MCP 类

#### `ai-kbs mcp install`

```bash
ai-kbs mcp install --agent claude-code [--scope user|project]
ai-kbs mcp install --agent codex [--scope user|project]
ai-kbs mcp install --agent cursor
ai-kbs mcp install --agent gemini-cli
```

自动写入 `.mcp.json` / `.cursor/mcp.json` 等。

#### `ai-kbs mcp serve`

```bash
ai-kbs mcp serve [--transport stdio|sse|http] [--port <n>]
```

启动 MCP server。

### 3.10 迁移类

#### `ai-kbs migrate from-losu-wiki`

```bash
ai-kbs migrate from-losu-wiki <workspace-path> [--target-space <s>]
```

读取 `.wiki/wiki.conf` + `.wiki/sync-state.json`，生成新工作空间配置 + 触发完整 pull。

## 4. NDJSON 流（chat / ask）

`ai-kbs ask` 与 `ai-kbs mcp serve` 在 `--format ndjson` 下输出每行一个事件：

```jsonl
{"type":"init","session_id":"sess-xxx","profile":"prod","engine":"hybrid"}
{"type":"thinking","content":"The user is asking about..."}
{"type":"tool_call","name":"ai_kbs_search","input":{"query":"退款规则","top_k":5}}
{"type":"tool_result","name":"ai_kbs_search","output":{"items":[...]}}
{"type":"answer","content":"根据文档，单笔退款上限为 5000 元。","citations":[{"docCode":"aB3kX9q2","title":"退款规则"}]}
{"type":"complete","done":true,"usage":{"input_tokens":1234,"output_tokens":567}}
```

## 5. 退出码（与 02-glossary.md 一致）

| 码 | 符号 | 含义 |
|----|------|------|
| 0 | OK | 成功 |
| 1 | General | 通用错误 |
| 2 | Usage | 参数错误 |
| 10 | NotAuthenticated | 未认证 |
| 11 | PermissionDenied | 无权限 |
| 20 | NotFound | 不存在 |
| 21 | AlreadyExists | 已存在 |
| 22 | VersionConflict | 版本冲突 |
| 23 | PartialSuccess | 部分成功 |
| 24 | ConflictsRemain | 冲突残留 |
| 25 | ManifestStale | sync-state 过期 |
| 30 | NetworkError | 网络错误 |
| 31 | Timeout | 超时 |
| 32 | RateLimited | 限流 |
| 40 | PatternNotFound | 模式未匹配 |
| 41 | NoChanges | 无变更 |

## 6. SKILL 操作手册（SKILL.md）

CLI 携带 `cli/skills/SKILL.md`，为 AI Agent 提供结构化操作流程：

### 6.1 HARD RULES

1. **始终带 `--format json`**（默认就是 json，但显式声明更稳）
2. **读 `next_action`**：错误响应携带可执行修复建议，按其 `cmd` + `args` 重试
3. **edit 必先 read**：edit 需 `--base-version`，必须先 `read` 拿当前 version
4. **write 覆盖必先 read**：覆盖已有文档需 CAS，先 `read` 拿 version
5. **冲突重试 ≤2 次**：exit 22 最多重试 2 次，超过则提示用户
6. **Token 不进对话**：永不打印 KeySecret / JWT 到 stdout / 日志
7. **`--prune` 不可恢复**：`pull --prune` 删本地（无回收站），`push --prune` 软删远端（可 undelete）；>10 文件/>5MB 须 `--yes`
8. **目录批量默认 create-only**：`write --dir` 默认跳过既有文档（不覆盖），覆盖需 `--overwrite`

### 6.2 意图 → 命令决策矩阵

| 用户意图 | 命令 |
|----------|------|
| 读某文档（给 URL 或 space+path） | `read` |
| 模糊查找 | `search`（关键词）/ `grep`（正则） |
| 局部替换 | `read` → `edit`（先 `--dry-run` 再正式） |
| 创建新文档 | `write --create` |
| 覆盖已有文档 | `read` → `write --base-version` |
| 浏览目录 | `tree` / `spaces` |
| 整目录双向同步 | `init` → `bind` → `pull` / `push` |
| 看历史/对比/回滚 | `version list` / `diff` / `rollback` |
| 移动/重命名 | `move`（docCode 不变） |
| 恢复已删 | `undelete` |
| 解决冲突 | `resolve`（或 `push --strategy`） |
| 自然语言提问 | `ask` |

### 6.3 典型工作流

- **A 读取**：有 URL/space+path → `read`；只有关键词 → `search` 看 score/snippet → `read`
- **B 编辑**：`read` 取 version → `edit --dry-run` 预览（确认 1 匹配）→ `edit` 正式；exit 22 → 重 read 重试
- **C 创建**：`write --create`；exit 21 → 去 `--create` 改覆盖
- **D 覆盖**：`read` 取 version → `write --base-version`；exit 22 → 重试
- **E 绑定同步**：`init` → `bind` → `pull` → 本地编辑 → `push --dry-run` → `push`；冲突 → `pull` 后 `push --strategy` 或 `resolve`
- **F 回滚**：`version list` → `version diff` → `version rollback` → `pull`
- **G 恢复**：`undelete` → `pull`
- **H 提问**：`ask <question>`；多轮自动接续；显式 `--session` 跨多轮

## 7. 命令分类：绑定 vs 非绑定

- **绑定命令**（需先 `init`+`bind`）：`init` / `status` / `bind` / `unbind` / `pull` / `push`
- **非绑定命令**（直接寻址远端，无需 init+bind）：`read` / `write` / `edit` / `search` / `grep` / `tree` / `spaces` / `version` / `move` / `undelete` / `resolve` / `ask`
- **认证**：`login` / `logout`
- **MCP**：`mcp install` / `mcp serve`
- **迁移**：`migrate`

> 歧义消除：要**某个文档的最新内容**（不落本地）→ `read`；要**对齐一整个本地绑定目录 ← 远端**（批量、更新 sync-state）→ `pull`。
