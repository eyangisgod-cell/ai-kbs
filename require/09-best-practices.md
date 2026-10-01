# 09 · 最佳实践、安全与常见陷阱

## 1. 安全最佳实践

### 1.1 静态加密

| 数据 | 算法 | 密钥管理 |
|------|------|----------|
| API Key 哈希 | AES-256-GCM | KMS / 环境变量 |
| 对象存储凭据 | AES-256-GCM | KMS |
| 用户密码 | bcrypt (cost=12) | 不存储明文 |
| HMAC Key Secret | 客户端本地，仅签名不上传 | 文件权限 0600 |

**实现要点**：
- 加密密钥绝不出现在代码 / Git / 日志
- 凭据轮换：API Key 定期（如 90 天）强制更换；轮换期间新旧 key 并行生效
- 凭据存储：使用 OS 原生 keyring（macOS Keychain / Windows Credential Manager / Linux Secret Service）

### 1.2 传输加密

| 场景 | 要求 |
|------|------|
| 外部 API | TLS 1.2+，禁用弱 cipher |
| 内部 gRPC（app ↔ docreader） | mTLS + Token |
| 内部 Redis（可选） | TLS |
| 后端 ↔ SAG | 内网 HTTP（生产建议 TLS） |

### 1.3 SSRF 防护

**场景**：用户通过 URL 导入文档或配置数据源时，恶意 URL 可访问内网。

**防护措施**：
1. URL 白名单（仅允许公网域名或特定内网段）
2. DNS rebinding 防护：解析 IP 后再次校验
3. 重定向跟随限制：仅同源重定向
4. 内网 IP 黑名单：10.0.0.0/8、172.16.0.0/12、192.168.0.0/16、127.0.0.0/8
5. 端口黑名单：22、23、25、445、3389 等

**实现位置**：
- URL 导入 handler
- 数据源同步 client
- Webhook receiver
- 所有 outbound HTTP client 统一通过 SSRF-safe client

### 1.4 凭据脱敏

**永不输出**到 stdout / 日志 / AI 对话 / API 响应：
- API Key（key_id 可以，但 secret 不行）
- JWT / Session Token
- 对象存储 access key / secret key
- 数据库密码
- LLM provider API key

**实现**：
- 统一 logger middleware：检测字段名包含 `secret`/`token`/`password`/`key` 的字段自动脱敏
- API 响应序列化：DTO 中敏感字段加 `redact:"true"` tag，序列化器统一处理
- MCP tool 输出：tool 输出 schema 中标记 `sensitive` 的字段自动脱敏

### 1.5 沙箱执行

Agent Skills 执行环境：
- 独立容器/进程，无特权
- 资源限制：CPU 1 / 内存 512MB / 超时 30s
- 网络限制：仅允许访问 ai-kbs backend + LLM API
- 文件系统：只读挂载除 /tmp 外的所有路径

### 1.6 IDOR 防护

每个写操作校验：
- 资源所属 workspace
- 当前用户在该 workspace 的角色
- 角色与操作类型匹配（如：contributor 不能 delete）

**实现**：
- 中间件统一拦截（在 handler 之前）
- 不依赖业务代码自觉
- 测试覆盖：每个端点至少 1 个 IDOR 用例

### 1.7 输入验证

所有外部输入（HTTP body / query / path）必须验证：
- 长度限制（防 DoS）
- 字符集限制（防注入）
- 路径遍历：`tree_path` 必须以 `/` 开头、不含 `..`
- URL 导入：URL 格式 + SSRF 检查

### 1.8 SQL 注入防护

- 100% 使用参数化查询（pgx / sqlx / GORM 都支持）
- 禁止字符串拼接 SQL
- CI 检查：`gosec` / `bandit` 检测 SQL 拼接
- DBA 权限：应用账户仅 DML + 必要 DDL，无 SUPER/DROP

### 1.9 XSS / CSRF

- Web UI：严格 CSP、`httpOnly` cookie、所有用户输入 escape
- CSRF：双重提交 cookie / SameSite=Lax
- API：纯 token 认证，不需要 CSRF（但 Web 表单需要）

## 2. 性能最佳实践

### 2.1 数据库

| 优化点 | 措施 |
|--------|------|
| 元数据查询 | 索引覆盖（避免回表） |
| 向量检索 | HNSW 索引（m=16, ef_construction=64） |
| 大表分页 | keyset pagination（避免 OFFSET） |
| 慢查询 | `pg_stat_statements` 监控 + EXPLAIN 巡检 |
| 连接池 | pgx pool（max=20, min=5） |
| 事务粒度 | 短事务，避免长事务 |
| 软删除 | `deleted_flag + deleted_at` 双字段，索引条件包含 |

### 2.2 Elasticsearch

| 优化点 | 措施 |
|--------|------|
| 索引设计 | 按 space 分索引或共享索引按 routing |
| 分片数 | 按文档量估算（每分片 10-50GB） |
| 副本数 | 生产 2 副本，开发 0 副本 |
| refresh_interval | 默认 1s，批量索引时临时 -1 |
| 查询优化 | filter context、source 限制、size 限制 |
| 向量检索 | dims 匹配 embedding 模型，HNSW ef_search 调优 |

### 2.3 SAG

| 优化点 | 措施 |
|--------|------|
| 索引存储 | LanceDB / SQLite（v1 单机），pgvector（v2 分布式） |
| 事件提取 | 批量异步（embedding 批量调用） |
| 查询 | entity 先 recall → event 再 recall → 排序 |
| 缓存 | 高频 query embedding 缓存（Redis） |

### 2.4 双引擎路由性能

```
目标：P95 < 200ms (ES) / < 2s (SAG)

策略：
1. ES path：直接走 ES，P95 通常 < 100ms
2. SAG path：
   - query embedding：< 100ms（缓存命中 < 5ms）
   - entity recall：< 50ms
   - event recall + SQL join：< 200ms
   - LLM 生成：1-3s（独立计费）
3. Hybrid path：并行 + RRF 重排
```

### 2.5 CLI 性能

- 启动时间 < 200ms：用 Go 而非 Node（WeKnora/SAG CLI 是 Node，启动慢）
- 8 路并发同步：`errgroup` + semaphore
- HTTP 客户端：连接池复用（keep-alive）
- 大文件下载：流式写入，不全读到内存

### 2.6 缓存策略

| 数据 | 缓存位置 | TTL |
|------|----------|-----|
| LLM API 响应（按 prompt hash） | Redis | 1h |
| Embedding（按 text hash） | Redis | 24h |
| ES 查询结果 | 不缓存（实时性要求高） | - |
| Space 元数据 | 进程内 LRU + Redis | 5min |
| 用户权限 | 进程内 LRU | 5min |

## 3. 代码质量

### 3.1 Go 代码规范

- **Lint**：`golangci-lint`（启用 errcheck、govet、staticcheck、revive、gocritic）
- **Format**：`gofmt` + `goimports`
- **测试覆盖率**：核心模块 ≥ 85%，整体 ≥ 70%
- **Error 处理**：errors.Is/As、wrap with %w、不忽略 error
- **Context 传播**：所有 IO 函数第一个参数为 `context.Context`
- **Logger**：slog（Go 1.22+），结构化字段
- **依赖注入**：wire 或 container pattern（沿用 WeKnora）

### 3.2 Python 代码规范

- **Lint**：`ruff` + `mypy`
- **Format**：`black`
- **测试**：`pytest` + `pytest-bdd` + `pytest-cov`
- **类型**：全量 type hints
- **Async**：httpx + asyncio，避免阻塞 IO

### 3.3 TypeScript 代码规范（前端）

- **Lint**：`eslint` + `@typescript-eslint`
- **Format**：`prettier`
- **测试**：`vitest` + `@vue/test-utils`
- **E2E**：`playwright`

### 3.4 Git 提交规范（Conventional Commits）

```
feat: 新增 ai-kbs_search 工具
fix: 修复 read --paths 空文件报错
docs: 更新 SKILL.md 中 hard rules
refactor: 拆分 doc_service.go 为多个文件
test: 添加 pull 重命名场景测试
chore: 升级 weknora 到 v0.7.1
```

### 3.5 PR 检查清单

- [ ] 单元测试通过，覆盖率 ≥ 70%
- [ ] Lint 无 error
- [ ] `next_action` 在错误响应中完整
- [ ] 无敏感信息（API Key / Token）硬编码
- [ ] 文档更新（README / SKILL / API doc）
- [ ] BDD 场景覆盖（如适用）
- [ ] 至少 1 个 Code Reviewer approve

## 4. 常见陷阱与对策

### 4.1 文档并发冲突

**陷阱**：A 和 B 同时编辑同一文档，A 先 push，B 后 push 失败。

**对策**：
- 强 CAS（baseVersion 必填）
- 失败时 next_action 提示 pull + retry
- push --strategy auto 自动分层解决（md5 三路）
- 单文件冲突可 keep-mine / keep-theirs / both

### 4.2 移动/重命名导致链接失效

**陷阱**：losu-wiki 早期版本用 path 寻址，重命名后旧链接全失效。

**对策**：
- docCode 稳定标识（8 位 base62，永不变）
- 所有引用走 docCode 而非 path
- AI Agent 调用工具时优先用 doc_code 字段而非 path

### 4.3 大文档同步性能

**陷阱**：100MB PDF 同步时单文件阻塞。

**对策**：
- 单文件 ≤ 10MB 入版本链（CAS 友好）
- 10-100MB 走对象存储 + md5 校验
- >100MB 拒绝（或走分片上传）
- 8 路并发时，semaphore 限制单文件 IO（避免大文件占用所有带宽）

### 4.4 删除误操作

**陷阱**：`pull --prune` 误删本地大量文件。

**对策**：
- 删除护栏三层：未传 `--prune` 完全不删；>10 文件需 `--yes`；>5MB 单文件需 `--yes`
- Detached 安全：sync-state 中记录但已 ignore 的文件不被 prune 删除
- 文档先软删除（远端可 undelete；本地 pull --prune 不可恢复）

### 4.5 AI Agent 误操作

**陷阱**：Agent 错误地写入大量垃圾文档。

**对策**：
- write --dir 默认 create-only，需 `--overwrite` 才能覆盖
- delete / rollback 默认 dry-run 预览
- 限流：单 API key 60 req/min
- 审计日志：所有写操作记录 actor / action / target
- Token 不进对话：避免 Token 泄露导致滥用

### 4.6 LLM 成本失控

**陷阱**：Agent 反复调用 LLM，月度账单爆表。

**对策**：
- Token 限速（per API key）：100k tokens/hour 默认
- Embedding 缓存（按 text hash）
- 提示词缓存（Anthropic prompt cache / OpenAI caching）
- 复杂查询走 SAG（避免对每个 chunk 调 LLM）
- LLM 选择分层：简单任务用小模型，复杂任务用大模型

### 4.7 双引擎一致性问题

**陷阱**：文档已写入 PostgreSQL，但 ES 索引失败，导致搜索缺失。

**对策**：
- 索引同步走 MQ 异步 + 重试（指数退避）
- DLQ（dead letter queue）记录永久失败
- 周期性全量对账任务（每晚）
- 提供管理员手动 reindex 工具

### 4.8 多语言分词

**陷阱**：ES 默认分词器对中文不友好，搜索"退款规则"找不到。

**对策**：
- 必装 IK 分词器（中文）
- 索引 analyzer 用 ik_smart
- 搜索 analyzer 用 ik_max_word
- 英文 fallback 到 standard

### 4.9 embedding 维度不一致

**陷阱**：换 embedding 模型后向量维度变化，旧索引全失效。

**对策**：
- 索引名包含模型名和维度（ai-kbs-chunks-bge-large-1024）
- 切换模型时强制全量重建索引
- metadata 记录 chunk 的 embedding_model 与维度
- 检索前校验维度匹配

### 4.10 软删除与硬删除混淆

**陷阱**：代码到处写 `WHERE deleted_flag = FALSE` 漏一处导致已删文档暴露。

**对策**：
- 实体层封装：所有查询走 `BaseRepository`，自动加 `deleted_flag = FALSE`
- 软删除用 `lambdaUpdate().set(deleted_flag, true)`
- 数据库视图：在 PG 中创建视图自动过滤（备用防线）

### 4.11 webhook 与回调地址

**陷阱**：用户配置的 webhook URL 指向内网（SSRF）。

**对策**：
- 见 1.3 SSRF 防护
- webhook 发送前必须 DNS 解析 + IP 校验
- 支持 IP 白名单

### 4.12 数据导入的编码问题

**陷阱**：用户上传 GBK 编码的文档，UTF-8 系统读出乱码。

**对策**：
- docreader 探测编码（chardet），统一转 UTF-8
- 保存原始编码到 metadata
- 显示与搜索统一用 UTF-8

## 5. 可维护性

### 5.1 配置管理

- 12-factor：所有配置走环境变量
- 配置分层：defaults < config file < env vars < flags
- 敏感配置（密码 / Token）走 KMS / Vault，不入 config file
- 配置变更通过审计日志记录

### 5.2 依赖管理

- Go：`go.mod` + `go.sum`（vendored for offline build）
- Python：`pyproject.toml` + `uv.lock`
- TypeScript：`package.json` + `pnpm-lock.yaml`
- 依赖更新：Dependabot / Renovate 自动 PR
- 重大依赖升级需通过 RFC

### 5.3 监控与告警

| 指标 | 阈值 | 告警 |
|------|------|------|
| API QPS | < 1000（自部署场景） | 持续超阈 → Slack |
| API P95 延迟 | > 1s | 立即告警 |
| 错误率 | > 1% | 立即告警 |
| 队列深度 | > 1000 | 5min 持续 → 告警 |
| 磁盘使用 | > 80% | 告警 |
| DB 连接池 | > 80% | 告警 |
| LLM Token 用量 | 超出预算 80% | 告警 |

### 5.4 灾难恢复

- PostgreSQL：每日全量备份 + 持续 WAL 归档；RPO 5min，RTO 1h
- ES：snapshot 每日 + 跨可用区副本
- 对象存储：跨区域复制（开启版本控制）
- 配置 / secrets：备份到 Vault / KMS
- 定期演练：每季度一次故障恢复演练

### 5.5 文档维护

- README / SKILL / API doc 与代码同仓
- 重要变更必须更新 SKILL.md（AI Agent 依赖）
- CHANGELOG.md 记录每个版本的用户可见变更
- 架构决策记录（ADR）放在 `docs/adr/`

## 6. 可扩展性预留

### 6.1 插件化检索后端

```go
// internal/repository/search_backend.go
type SearchBackend interface {
    Search(ctx context.Context, req *SearchRequest) (*SearchResult, error)
    Index(ctx context.Context, chunks []*Chunk) error
    Delete(ctx context.Context, docCodes []string) error
    Name() string
}

// 注册多种实现
type Registry struct {
    backends map[string]SearchBackend
}
```

未来加 ColBERT / HyDE / GraphRAG 仅需实现接口。

### 6.2 插件化 LLM Provider

沿用 WeKnora 的实现：每 provider 一个文件，统一接口。

### 6.3 插件化存储

```go
type ObjectStorage interface {
    Put(ctx context.Context, key string, reader io.Reader) error
    Get(ctx context.Context, key string) (io.ReadCloser, error)
    Delete(ctx context.Context, key string) error
    Sign(ctx context.Context, key string, ttl time.Duration) (string, error)
}
```

实现：Local / MinIO / S3 / OSS / TOS / KS3 / OBS。

### 6.4 自定义 MCP Tools

插件机制：用户可上传自己的 MCP tool 包（Python wheel），ai-kbs MCP server 动态加载。

安全约束：
- tool 包需签名（可选）
- 资源限制：CPU / 内存 / 超时
- 网络限制：仅允许调用白名单 API
