# 08 · BDD Gherkin 行为场景

> BDD（Behavior-Driven Development）规范，使用 Gherkin 语法描述关键场景。
> 测试框架：Python `pytest-bdd`（后端 + CLI）+ JS `jest-cucumber`（Web）。

## Feature: 工作空间与绑定

### Scenario: 初始化本地工作空间

```gherkin
Feature: Workspace Initialization

  Scenario: 成功初始化
    Given 用户在空目录 /tmp/proj 中
    And 服务端 ai-kbs.example.com 可访问
    And 用户已通过 `ai-kbs login` 认证
    When 用户执行 `ai-kbs init --url https://ai-kbs.example.com --space dev`
    Then 退出码为 0
    And .ai-kbs/workspace.yaml 文件存在
    And workspace.yaml 中 wiki_url = "https://ai-kbs.example.com"
    And workspace.yaml 中 default_space = "dev"

  Scenario: 工作空间已存在
    Given 目录已包含 .ai-kbs/workspace.yaml
    When 用户执行 `ai-kbs init`
    Then 退出码为 21
    And 错误码 = "workspace.already_exists"

  Scenario: 未认证
    Given 用户未登录
    When 用户执行 `ai-kbs init`
    Then 退出码为 10
    And 错误码 = "auth.unauthenticated"
```

### Scenario: 绑定本地目录到远端

```gherkin
  Scenario: 成功绑定
    Given 工作空间已初始化
    And space "dev" 存在
    When 用户执行 `ai-kbs bind docs "dev:/api/"`
    Then 退出码为 0
    And workspace.yaml 包含 binding {local: "docs", remote: "dev:/api/"}

  Scenario: 远端 space 不存在
    When 用户执行 `ai-kbs bind docs "ghost:/api/"`
    Then 退出码为 20
    And 错误码 = "space.not_found"

  Scenario: 父子重叠
    Given 已存在 binding docs → dev:/api/
    When 用户执行 `ai-kbs bind docs/sub "dev:/api/sub/"`
    Then 退出码为 2
    And 错误信息包含 "父子重叠"
```

## Feature: 单文档读取与编辑

### Scenario: 用 docCode 读取文档

```gherkin
  Scenario: 成功读取
    Given space "dev" 中存在 docCode "aB3kX9q2"
    When 用户执行 `ai-kbs read --space dev --doc aB3kX9q2 --format json`
    Then 退出码为 0
    And response.data.docCode = "aB3kX9q2"
    And response.data.content 包含文档正文
    And response.data.version = 7

  Scenario: 读取不存在的 docCode
    When 用户执行 `ai-kbs read --space dev --doc nonexistent --format json`
    Then 退出码为 20
    And 错误码 = "doc.not_found"
```

### Scenario: 创建新文档

```gherkin
  Scenario: 成功创建
    When 用户执行 `ai-kbs write --space dev --path /docs/new.md --content "# New" --create`
    Then 退出码为 0
    And response.data.created = true
    And response.data.docCode 匹配 "^[a-zA-Z0-9]{8}$"
    And response.data.version = 1

  Scenario: 文档已存在但未带 --create
    Given space "dev" 中已存在 /docs/existing.md
    When 用户执行 `ai-kbs write --space dev --path /docs/existing.md --content "..."`
    Then 退出码为 21
    And 错误码 = "doc.already_exists"
    And next_action.strategies 包含 "with_base_version"

  Scenario: 覆盖已有文档（CAS）
    Given 文档 /docs/existing.md 当前 version = 5
    When 用户执行 `ai-kbs write --space dev --path /docs/existing.md --content "updated" --base-version 5`
    Then 退出码为 0
    And response.data.version = 6
    And response.data.created = false

  Scenario: 版本冲突
    Given 文档当前 version = 5，但用户用 --base-version 4
    When 用户执行 `ai-kbs write --space dev --path /docs/existing.md --content "..." --base-version 4`
    Then 退出码为 22
    And 错误码 = "doc.version_conflict"
    And next_action.cmd = "ai-kbs read"
    And next_action.args 包含 ["--space", "dev", "--doc", "<docCode>"]
```

### Scenario: 局部编辑

```gherkin
  Scenario: 成功替换 1 处
    Given 文档当前内容包含 "TODO"
    And 当前 version = 5
    When 用户执行 `ai-kbs edit --space dev --doc aB3kX9q2 --old "TODO" --new "DONE" --base-version 5`
    Then 退出码为 0
    And response.data.replacedCount = 1
    And response.data.version = 6

  Scenario: 0 匹配
    Given 文档当前内容不包含 "NOTEXIST"
    When 用户执行 `ai-kbs edit --space dev --doc aB3kX9q2 --old "NOTEXIST" --new "X" --base-version 5`
    Then 退出码为 40
    And 错误码 = "edit.pattern_not_found"

  Scenario: 多处匹配但未带 --replace-all
    Given 文档包含 3 处 "TODO"
    When 用户执行 `ai-kbs edit --space dev --doc aB3kX9q2 --old "TODO" --new "DONE" --base-version 5`
    Then 退出码为 40
    And 错误码 = "edit.multiple_matches"

  Scenario: dry-run 预览
    When 用户执行 `ai-kbs edit ... --dry-run`
    Then 退出码为 0
    And response.data.dryRun = true
    And response.data.diff 包含 unified diff 文本
    And response.data.version = 5  # 未变更
```

## Feature: 版本管理

### Scenario: 列出文档所有版本

```gherkin
  Scenario: 列出 3 个版本
    Given 文档有版本 1, 2, 3
    When 用户执行 `ai-kbs version list --space dev --doc aB3kX9q2`
    Then 退出码为 0
    And response.data.active_version = 3
    And response.data.items 长度 = 3
    And items[0].version = 1
    And items[2].version = 3
```

### Scenario: 版本对比

```gherkin
  Scenario: 对比 v1 和 v3
    When 用户执行 `ai-kbs version diff --space dev --doc aB3kX9q2 --from 1 --to 3`
    Then 退出码为 0
    And response.data.from_version = 1
    And response.data.to_version = 3
    And response.data.unified_diff 包含 diff 文本
    And response.data.stats.additions >= 0
    And response.data.stats.deletions >= 0
```

### Scenario: 回滚到旧版本

```gherkin
  Scenario: git revert 语义回滚
    Given 文档当前 active_version = 5
    And v3 是历史版本
    When 用户执行 `ai-kbs version rollback --space dev --doc aB3kX9q2 --to 3 --message "revert to v3"`
    Then 退出码为 0
    And response.data.new_version = 6   # 产生新版本
    And response.data.rolled_back_to = 3
    And response.data.rolled_back_from = 5
    And 中间版本（v4, v5）保留可查
```

## Feature: 双向同步

### Scenario: 拉取远端文档到本地

```gherkin
  Scenario: 首次拉取
    Given 绑定 docs → dev:/api/，本地为空
    And 远端有 10 个文档
    When 用户执行 `ai-kbs pull`
    Then 退出码为 0
    And response.data.totals.downloaded = 10
    And response.data.totals.skipped = 0
    And 本地 10 个文件被创建
    And .ai-kbs/sync-state.json 记录所有 10 个文件

  Scenario: 检测 rename（docCode 不变）
    Given 本地 sync-state 中 file "users.md" 的 docCode = "aB3kX9q2"
    And 远端 "users.md" 被重命名为 "members.md"，但 docCode 仍 = "aB3kX9q2"
    When 用户执行 `ai-kbs pull`
    Then 退出码为 0
    And 本地 "users.md" 被重命名为 "members.md"
    And sync-state 中路径更新为 "members.md"，docCode 仍 = "aB3kX9q2"
```

### Scenario: 推送本地文档到远端

```gherkin
  Scenario: 推送修改（无冲突）
    Given 本地 "api.md" 与远端版本一致（md5 相同）
    When 用户修改本地 "api.md"
    And 用户执行 `ai-kbs push`
    Then 退出码为 0
    And 远端 api.md 版本 +1
    And sync-state 更新

  Scenario: 推送时检测到冲突
    Given 本地 "api.md" 基于 v5 修改
    And 远端 api.md 已被他人更新到 v6
    When 用户执行 `ai-kbs push`
    Then 退出码为 22
    And 错误码 = "doc.version_conflict"
    And next_action.cmd = "ai-kbs pull"

  Scenario: --strategy auto 自动解决
    Given 本地基于 v5 修改，远端已更新到 v6，但本地 v5 内容 == 远端 v6 内容（实质无冲突）
    When 用户执行 `ai-kbs push --strategy auto`
    Then 退出码为 0
    And 推送成功（auto 检测 md5 三路分层为 keep-theirs）
```

### Scenario: 删除护栏

```gherkin
  Scenario: --prune 触发删除护栏（>10 文件）
    Given 远端 15 个文件已在本地被删除
    When 用户执行 `ai-kbs pull --prune`（不带 --yes）
    Then 退出码为 2
    And 错误信息提示 "需要 --yes 确认"

  Scenario: --prune + --yes
    When 用户执行 `ai-kbs pull --prune --yes`
    Then 退出码为 0
    And 本地 15 个文件被删除

  Scenario: Detached 文件不被删除
    Given 文件 "draft.md" 已同步，后被加入 .wikignore
    When 用户执行 `ai-kbs push --prune --yes`
    Then "draft.md" 不被远端删除（Detached 安全）
```

## Feature: AI Agent MCP 调用

### Scenario: Claude Code 通过 MCP 搜索文档

```gherkin
  Scenario: 搜索命中
    Given MCP server 已配置到 Claude Code
    And 服务端有文档包含 "退款规则"
    When AI Agent 调用 ai_kbs_search(query="退款上限", top_k=5)
    Then 返回 items 数组
    And items 长度 > 0
    And items[0].score > 0
    And items[0].doc_code 匹配 "^[a-zA-Z0-9]{8}$"

  Scenario: 搜索无结果
    When AI Agent 调用 ai_kbs_search(query="不存在的关键词xyz123")
    Then 返回 items = []
    And total = 0
    And 不返回错误
```

### Scenario: AI Agent 读取文档并编辑

```gherkin
  Scenario: 完整读 → 改 → 写循环
    Given 文档 docCode = "aB3kX9q2" version = 5
    When AI Agent 调用 ai_kbs_read_doc(doc_code="aB3kX9q2")
    Then 返回 content + version=5

    When AI Agent 调用 ai_kbs_update_doc(doc_code="aB3kX9q2", content="...", base_version=5)
    Then 返回 version=6
    And update 成功

  Scenario: CAS 冲突时 next_action 引导 Agent 重试
    Given 文档当前 version = 6
    When AI Agent 调用 ai_kbs_update_doc(doc_code, content, base_version=5)
    Then 返回错误 code = "doc.version_conflict"
    And next_action.tool = "ai_kbs_read_doc"
    And next_action.args.doc_code = "aB3kX9q2"
```

### Scenario: AI Agent 自然语言问答

```gherkin
  Scenario: 单跳问题走 ES
    Given 文档 "退款规则.md" 明确写了 "单笔退款上限 5000 元"
    When AI Agent 调用 ai_kbs_ask(question="单笔退款上限是多少？")
    Then 返回流式 answer
    And answer.citations 包含 "退款规则.md"
    And answer.citations[0].quote = "单笔退款上限 5000 元"

  Scenario: 多跳问题走 SAG
    Given 文档 A 提到 "X 公司"
    And 文档 B 提到 "X 公司创始人 Y"
    And 文档 C 提到 "Y 在 2010 年创立 Z 产品"
    When AI Agent 调用 ai_kbs_ask(question="Z 产品的创始人是谁？什么时候创立的？")
    Then engine = "sag" 或 "hybrid"
    And answer 正确返回 "Y" 和 "2010 年"

  Scenario: 流式响应
    When AI Agent 调用 ai_kbs_ask(question, stream=true)
    Then 第一个事件 type = "init"
    And 至少包含一个 type = "answer" 事件
    And 最后一个事件 type = "complete"，done = true
```

## Feature: 鉴权与凭证

### Scenario: 通过环境变量凭证自动登录

```gherkin
  Scenario: AIKBS_KEY_ID + AIKBS_KEY_SECRET 注入
    Given 环境变量 AIKBS_KEY_ID = "ak_xxx"
    And 环境变量 AIKBS_KEY_SECRET = "sk_xxx"
    And 环境变量 AIKBS_URL = "https://ai-kbs.example.com"
    When 用户执行 `ai-kbs spaces`
    Then 退出码为 0
    And 返回 space 列表（无需交互式 login）

  Scenario: 优先级覆盖
    Given 环境变量 AIKBS_KEY_ID + AIKBS_KEY_SECRET 存在
    And ~/.ai-kbs/credentials 也有 hmac 凭证
    When 用户执行任何需要认证的命令
    Then 使用 AIKBS_KEY_ID/SECRET（环境变量优先）
```

### Scenario: 凭证存储安全

```gherkin
  Scenario: key_secret 不出现在日志
    Given 凭证已写入 ~/.ai-kbs/credentials
    When 用户执行任何命令并开启 WIKI_DEBUG
    Then 日志中不包含 key_secret 原始值
    And 日志中不包含 AIKBS_KEY_SECRET 原始值

  Scenario: Token 不出现在 AI 对话
    When AI Agent 调用 ai_kbs_* 工具
    Then 工具返回的 JSON 中不包含 user 的 API Key / Token
    And 错误响应中也不包含敏感信息
```

## Feature: 检索路由

### Scenario: 双引擎路由

```gherkin
  Scenario: 单跳查询路由到 ES
    When 用户执行 `ai-kbs search "退款"`
    Then response.data.engine = "es"
    And response.data.took_ms < 500

  Scenario: 多跳查询路由到 SAG
    When 用户执行 `ai-kbs search "创始人 创立时间"`
    Then response.data.engine = "sag" 或 "hybrid"

  Scenario: 强制路由
    When 用户执行 `ai-kbs search "X" --engine sag`
    Then response.data.engine = "sag"

  Scenario: ES 不可用时降级
    Given Elasticsearch 服务宕机
    When 用户执行 `ai-kbs search "X"`
    Then 仍能返回结果（fallback 到 PostgreSQL 全文）
    And response.data.engine 包含 "fallback" 标注
```

## Feature: RBAC 与多租户

### Scenario: Contributor 角色无法删除空间

```gherkin
  Scenario: 权限不足
    Given 用户角色 = contributor
    When 用户尝试删除 space
    Then 退出码为 11
    And 错误码 = "rbac.permission_denied"
    And next_action.reason 包含 "需要 admin 或 owner 角色"
```

### Scenario: 跨 workspace 隔离

```gherkin
  Scenario: workspace A 用户无法访问 workspace B
    Given 用户属于 workspace A
    When 用户尝试访问 workspace B 的 doc
    Then 退出码为 11
    And 错误码 = "rbac.workspace_forbidden"
```

## Feature: 迁移（from-losu-wiki）

### Scenario: 迁移工作空间

```gherkin
  Scenario: 完整迁移
    Given 源路径 /old-project 含 .wiki/wiki.conf 和 sync-state.json
    And 远端 ai-kbs 服务可访问
    When 用户执行 `ai-kbs migrate from-losu-wiki /old-project`
    Then 退出码为 0
    And /old-project/.ai-kbs/workspace.yaml 被创建
    And .ai-kbs/sync-state.json 包含原 sync-state 中的所有文件
    And 报告 generated: {success: N, failed: M, skipped: K}
```

## Feature: 性能与可观测

### Scenario: 1000 文件批量 pull

```gherkin
  Scenario: 性能基线
    Given 远端 space 有 1000 个文档（每个 ~5KB）
    When 用户执行 `ai-kbs pull`
    Then 退出码为 0
    And 总耗时 < 60 秒
    And response.data.totals.downloaded = 1000

  Scenario: Langfuse 追踪完整
    When 用户执行 `ai-kbs pull`
    Then Langfuse 中产生对应 trace
    And trace 包含 spans: api_call → file_download × 1000 → sync_state_update
    And 每个 span 包含 request_id 与 timing
```

## Feature: 错误处理边界

### Scenario: 网络中断

```gherkin
  Scenario: 推送过程中网络中断
    Given 已推送 50 个文件
    When 第 51 个文件传输时网络断开
    Then 已推送的 50 个文件在远端保留
    And response.data.sync_results[0].errors 包含 51 个失败
    And 退出码 = 23 (partial_success)
    And sync-state 仅记录成功的 50 个文件
```

### Scenario: 远端超时

```gherkin
  Scenario: 单文件读取超时
    Given 远端某个文件 size = 100MB
    When 用户执行 `ai-kbs read --space dev --path /huge.md`（默认 30s 超时）
    Then 退出码 = 31
    And 错误码 = "internal.timeout"
    And next_action.suggestion = "增加 --timeout 或分页读取"
```

## 总结：场景覆盖矩阵

| 模块 | 场景数 | 优先级 |
|------|--------|--------|
| Workspace & Binding | 6 | P0 |
| 单文档 CRUD | 9 | P0 |
| 版本管理 | 4 | P0 |
| 双向同步 | 7 | P0 |
| MCP / Agent | 7 | P0 |
| 鉴权 | 4 | P0 |
| 检索路由 | 4 | P0 |
| RBAC | 2 | P0 |
| 迁移 | 1 | P2 |
| 性能与可观测 | 2 | P1 |
| 错误边界 | 2 | P0 |
| **合计** | **48** | — |
