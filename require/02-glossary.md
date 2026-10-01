# 02 · 词汇表（Glossary）

> 三源参考项目（losu-wiki / WeKnora / SAG）存在术语重叠与歧义，本文档统一规范，作为后续文档与代码的唯一事实源。

## 1. 核心实体

| 术语 | 规范定义 | 出处 / 对应 | 备注 |
|------|----------|-------------|------|
| **Workspace（工作空间）** | 一个本地目录，通过 `ai-kbs init` 初始化为工作空间，包含绑定与 sync-state | losu: workspace；WeKnora: workspace | 与 WeKnora 一致 |
| **Space（文档空间）** | 远端的命名空间，文档按 space 隔离 | losu: space；WeKnora: knowledge base（KB） | **统一用 Space**（KB 等价于 Space） |
| **Document（文档）** | 一篇 markdown / PDF / 其他格式的知识条目 | losu: document；WeKnora: document / knowledge entry | 统一用 Document |
| **docCode** | 文档的稳定标识符，8 位 base62 字符串，move/rename 不变 | losu: docCode；WeKnora: knowledge_id / doc_id | **沿用 docCode**，跨来源通用 |
| **Version（版本）** | 文档的某一次完整内容快照 | losu: document_version；WeKnora: knowledge version | 统一用 Version |
| **Chunk（文本块）** | 文档解析后的一段语义完整文本 | losu: chunk；WeKnora: chunk | 统一用 Chunk |
| **Event（事件）** | SAG 的核心数据结构，每个 chunk 对应 1 个 event，承载完整语义 | SAG: event | **新增**（SAG 专用） |
| **Entity（实体）** | SAG 的索引节点，每个 chunk 对应多个 entity | SAG: entity | **新增**（SAG 专用） |
| **Hyperedge（超边）** | SAG 查询时由共享 entity 的事件动态联接生成 | SAG: hyperedge | **新增**（SAG 专用） |
| **Binding（绑定）** | 本地目录 ↔ 远端 space:path 的关联 | losu: binding；WeKnora: link | 统一用 Binding |
| **Sync-state（同步状态）** | 本地工作空间记录每个文件与远端的版本、md5、docCode 映射 | losu: sync-state.json | 沿用 |
| **RBAC（角色权限）** | 4 级角色：Owner / Admin / Contributor / Viewer | WeKnora: RBAC；losu: 简化 | 沿用 WeKnora |
| **API Key** | scoped API key（含 capability 限制 + per-KB 限制） | WeKnora: API Key | 沿用 |
| **Knowledge Graph（知识图谱）** | 文档中提取的实体-关系图，用于多跳推理 | WeKnora: knowledge graph；SAG: event-entity graph | **统一用 KG** |
| **Skill** | AI Agent 的操作手册（SKILL.md），含硬性纪律 | WeKnora: skill；losu: SKILL | 沿用 |
| **MCP** | Model Context Protocol，AI Agent 接入工具的标准协议 | WeKnora: MCP server；SAG: MCP | 沿用 |

## 2. 操作动词

| 动词 | 定义 | 出处 | ai-kbs 对应命令 |
|------|------|------|----------------|
| **Init** | 初始化本地工作空间 | losu: `wiki init` | `ai-kbs init` |
| **Bind** | 绑定本地目录到远端 space:path | losu: `wiki bind` | `ai-kbs bind` |
| **Unbind** | 解绑 | losu: `wiki unbind` | `ai-kbs unbind` |
| **Pull** | 远端 → 本地 同步 | losu: `wiki pull` | `ai-kbs pull` |
| **Push** | 本地 → 远端 同步 | losu: `wiki push` | `ai-kbs push` |
| **Read** | 读取单文档内容 | losu: `wiki read` | `ai-kbs read` |
| **Write** | 创建/覆盖文档 | losu: `wiki write` | `ai-kbs write` |
| **Edit** | 局部替换 | losu: `wiki edit` | `ai-kbs edit` |
| **Search** | 关键词搜索 | losu: `wiki search` | `ai-kbs search` |
| **Grep** | 正则搜索 | losu: `wiki grep` | `ai-kbs grep` |
| **Tree** | 目录树浏览 | losu: `wiki tree` | `ai-kbs tree` |
| **Spaces** | 列出可用空间 | losu: `wiki spaces` | `ai-kbs spaces` |
| **Version** | 版本管理（list/diff/rollback） | losu: `wiki version` | `ai-kbs version` |
| **Move** | 移动/重命名 | losu: `wiki move` | `ai-kbs move` |
| **Undelete** | 恢复软删除 | losu: `wiki undelete` | `ai-kbs undelete` |
| **Resolve** | 解决版本冲突 | losu: `wiki resolve` | `ai-kbs resolve` |
| **Login** | 登录获取凭证 | losu: `wiki login` | `ai-kbs login` |
| **Logout** | 登出 | losu: `wiki logout` | `ai-kbs logout` |
| **Ask** | 自然语言提问（走 Agent） | WeKnora: `chat` | `ai-kbs ask` |
| **Index** | 触发文档解析与索引 | WeKnora: `upload` | `ai-kbs index` |
| **Migrate** | 从 losu-wiki 迁移 | 新增 | `ai-kbs migrate from-losu-wiki` |

## 3. 寻址模式

| 模式 | 语法 | 优先级 | 示例 |
|------|------|--------|------|
| **完整 URL** | `https://host/kb/space/{space}/page/{docCode}` | 1（最高） | `https://kb.example.com/kb/space/dev/page/aB3kX9q2` |
| **docCode 寻址** | `--space {space} --doc {docCode}` | 2 | `--space dev --doc aB3kX9q2` |
| **路径寻址** | `--space {space} --path {path}` | 3 | `--space dev --path /api/users.md` |

## 4. 冲突策略

| 策略 | 行为 | 来源 |
|------|------|------|
| **manual** | 写 `.THEIRS.md` 并排 + exit 22 | losu |
| **keep-mine** | 强写本地内容到远端（覆盖，产生新版本） | losu |
| **keep-theirs** | 接受远端，放弃本地 | losu |
| **both / concat** | 远端内容追加到本地内容后写入 | losu |
| **auto** | 客户端 md5 三路分层自动解决 | losu |

## 5. 退出码

| 码 | 符号 | 含义 |
|----|------|------|
| 0 | OK | 成功 |
| 1 | General | 通用错误 |
| 2 | Usage | 参数错误 |
| 10 | NotAuthenticated | 未认证 |
| 11 | PermissionDenied | 无权限 |
| 20 | NotFound | 文档/空间不存在 |
| 21 | AlreadyExists | 文档已存在 |
| 22 | VersionConflict | 版本冲突 |
| 23 | PartialSuccess | 部分成功 |
| 24 | ConflictsRemain | 仍有冲突残留 |
| 25 | ManifestStale | sync-state 过期 |
| 30 | NetworkError | 网络错误 |
| 31 | Timeout | 超时 |
| 32 | RateLimited | 限流 |
| 40 | PatternNotFound | edit/grep 未匹配 |
| 41 | NoChanges | 无变更 |

## 6. 数据存储术语

| 术语 | 定义 |
|------|------|
| **PostgreSQL** | 主数据库，存元数据（spaces / documents / versions / bindings / users / api_keys） |
| **pgvector** | PostgreSQL 的向量扩展，存 chunk embedding（与文档同库，便于事务） |
| **Elasticsearch** | 检索副本，存 chunk 全文 + 向量 + KG 节点（用于 BM25 / hybrid search） |
| **SAG (LanceDB / SQLite)** | SAG 本地副本，存 event / entity / event-entity 关联 |
| **Object Storage** | 文档二进制（PDF/Word/Image）的存储后端；多种适配（Local/MinIO/S3/OSS/...） |

## 7. AI Agent 集成术语

| 术语 | 定义 |
|------|------|
| **Tool** | MCP 协议中的一个工具（如 `ai_kbs_read_doc`） |
| **Skill** | Markdown 格式的操作手册（SKILL.md），告诉 AI 如何使用一组 Tool |
| **HARD RULES** | Skill 中的硬性纪律（如"edit 必先 read"），AI 必须遵守 |
| **Wire Contract** | CLI/MCP 输出的 JSON schema（frozen schema），AI 据此解析 |
| **next_action** | 错误响应中的可执行修复建议字段，AI 可直接重试 |

## 8. 易混淆对照表

| 容易混淆 | 区分 |
|----------|------|
| **Space vs Workspace** | Space 是远端的逻辑容器；Workspace 是本地的物理目录 |
| **Knowledge Base (KB) vs Space** | WeKnora 用 KB，losu 用 space；本项目统一为 Space |
| **Knowledge Graph vs SAG Event-Entity** | KG 是通用术语；SAG 的 event-entity 是 KG 的一种实现 |
| **RAG vs SAG** | RAG 是通用术语；SAG 是 SAG 论文提出的具体架构 |
| **Doc ID vs docCode** | 数据库自增 ID 是物理标识；docCode 是稳定逻辑标识 |
| **Version vs Revision** | losu 用 Version（每次提交递增）；Git 用 Revision/Commit |
| **Commit vs Sync** | Git 用 commit；本项目用 sync（pull/push）描述文档同步 |
| **rollback vs revert** | 两者均不删除中间版本，本项目沿用 losu 的 `rollback`（git revert 语义） |
| **profile vs environment** | Spring Boot 的 profile 对应本项目的 environment（dev/uat/prod） |

## 9. 命名约定

| 类别 | 规则 | 示例 |
|------|------|------|
| 包/模块 | 小写，连字符分隔 | `ai-kbs/cli`、`ai-kbs/internal/agent` |
| Go 标识符 | camelCase / PascalCase | `DocCode`、`SyncState` |
| 数据库表 | snake_case | `ai_kbs_documents`、`ai_kbs_versions` |
| JSON 字段 | camelCase | `docCode`、`syncState`、`nextAction` |
| CLI flag | kebab-case | `--doc-code`、`--base-version` |
| 环境变量 | UPPER_SNAKE，前缀 AIKBS_ | `AIKBS_URL`、`AIKBS_KEY_ID` |
| API 路径 | 复数资源，kebab-case | `/api/ai-kbs/documents`、`/api/ai-kbs/spaces/{code}/versions` |
| HTTP method | GET（读）/ POST（写） | 与 losu-wiki 一致（简化 REST） |
| 日志 key | 小写，下划线分隔 | `request_id`、`doc_code` |
| 错误码 | 点号分隔的命名空间 | `auth.unauthenticated`、`doc.version_conflict` |
