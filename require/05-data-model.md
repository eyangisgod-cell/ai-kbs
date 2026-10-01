# 05 · 数据模型

## 1. ER 图（核心实体）

```
┌──────────────┐         ┌──────────────────┐         ┌──────────────────┐
│  workspaces  │ 1     * │     spaces       │ 1     * │    documents     │
│  (跨平台)    │─────────│  (knowledge base)│─────────│  (单文档元数据)  │
└──────────────┘         └──────────────────┘         └─────────┬────────┘
                                                                 │ 1
                                                                 │
                                                                 ▼ *
                                                       ┌──────────────────┐
                                                       │ document_versions│
                                                       │  (版本链)        │
                                                       └──────────────────┘
                                                                 │
                                                                 │ 1
                                                                 │
                                                                 ▼ *
                                                       ┌──────────────────┐
                                                       │     chunks       │
                                                       │  (文本块+向量)   │
                                                       └──────────────────┘
                                                                 │
                                                                 │ 1
                                                                 │
                                                                 ▼ *
                                                       ┌──────────────────┐
                                                       │  sag_events      │
                                                       │  (事件)          │
                                                       └──────────────────┘
                                                                 │ *
                                                                 │
                                                                 ▼ *
                                                       ┌──────────────────┐
                                                       │  sag_entities    │
                                                       │  (实体)          │
                                                       └──────────────────┘

┌──────────────┐         ┌──────────────────┐
│    users     │ *     * │  workspace_users │
│  (用户)      │─────────│  (用户-空间映射) │
└──────────────┘         └──────────────────┘
       │                          │
       │                          │ RBAC: owner / admin /
       │                          │       contributor / viewer
       │                          ▼
       │                  ┌──────────────────┐
       └─────────────────▶│  audit_logs      │
                          │  (审计日志)      │
                          └──────────────────┘

┌──────────────┐         ┌──────────────────┐         ┌──────────────────┐
│ api_keys     │ *     1 │   users          │ 1     * │   sessions       │
│ (scoped)     │─────────│                  │─────────│  (对话会话)      │
└──────────────┘         └──────────────────┘         └──────────────────┘
```

## 2. 核心表结构

### 2.1 workspaces（工作空间元信息，仅在服务端存储必要字段）

> CLI 端是 `.ai-kbs/workspace.yaml` 文件，服务端不直接存储 workspace。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uuid | 主键 |
| name | text | 工作空间名 |
| slug | text | URL 友好标识 |
| owner_id | uuid | 创建者 |
| created_at | timestamptz | |
| updated_at | timestamptz | |
| deleted_at | timestamptz | 软删 |

### 2.2 spaces（文档空间 / 知识库）

```sql
CREATE TABLE spaces (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id),
    code         VARCHAR(64) NOT NULL,        -- 空间编码（唯一于 workspace 内）
    name         VARCHAR(255) NOT NULL,
    description  TEXT,
    owner_id     UUID NOT NULL REFERENCES users(id),
    -- 检索配置
    embedding_model  VARCHAR(64) NOT NULL,
    embedding_dim    INT NOT NULL DEFAULT 1024,
    chunk_strategy   VARCHAR(32) NOT NULL DEFAULT 'semantic',  -- semantic | parent_child | fixed
    chunk_size       INT NOT NULL DEFAULT 512,
    chunk_overlap    INT NOT NULL DEFAULT 64,
    -- 路由配置
    sag_enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    -- 元数据
    metadata         JSONB DEFAULT '{}'::jsonb,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ DEFAULT '1970-01-01 00:00:00+00',
    UNIQUE (workspace_id, code, deleted_at)
);

CREATE INDEX idx_spaces_workspace ON spaces(workspace_id) WHERE deleted_at < '2100-01-01';
```

### 2.3 documents（文档元数据）

```sql
CREATE TABLE documents (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id            UUID NOT NULL REFERENCES spaces(id),
    doc_code            VARCHAR(16) NOT NULL,           -- 8 位 base62 稳定标识
    parent_doc_code     VARCHAR(16),                    -- 父文档（可空）
    tree_path           TEXT NOT NULL,                  -- 如 /API/users/list.md
    depth               INT NOT NULL DEFAULT 0,
    title               VARCHAR(512) NOT NULL,
    doc_type            VARCHAR(32) NOT NULL DEFAULT 'markdown',  -- markdown | pdf | word | ...
    mime_type           VARCHAR(128),
    active_version      INT NOT NULL DEFAULT 1,
    active_md5          VARCHAR(32) NOT NULL,           -- 当前激活版本 md5
    -- 存储
    storage_key         TEXT,                          -- 对象存储 key
    size_bytes          BIGINT NOT NULL DEFAULT 0,
    -- 检索
    sag_indexed         BOOLEAN NOT NULL DEFAULT FALSE,
    es_indexed          BOOLEAN NOT NULL DEFAULT FALSE,
    -- 元数据
    tags                TEXT[] DEFAULT '{}',
    metadata            JSONB DEFAULT '{}'::jsonb,
    confidential_level  VARCHAR(32) NOT NULL DEFAULT 'unclassified',
    created_by          UUID NOT NULL REFERENCES users(id),
    updated_by          UUID NOT NULL REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_flag        BOOLEAN NOT NULL DEFAULT FALSE,  -- 软删标记
    deleted_at          TIMESTAMPTZ NOT NULL DEFAULT '1970-01-01 00:00:00+00',
    UNIQUE (space_id, doc_code, deleted_at),
    UNIQUE (space_id, tree_path, deleted_at)
);

CREATE INDEX idx_documents_space ON documents(space_id) WHERE deleted_flag = FALSE;
CREATE INDEX idx_documents_parent ON documents(space_id, parent_doc_code) WHERE deleted_flag = FALSE;
CREATE INDEX idx_documents_tags ON documents USING GIN(tags) WHERE deleted_flag = FALSE;
CREATE INDEX idx_documents_path ON documents(space_id, tree_path text_pattern_ops);
```

### 2.4 document_versions（版本链）

```sql
CREATE TABLE document_versions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id     UUID NOT NULL REFERENCES documents(id),
    version_num     INT NOT NULL,                   -- 从 1 递增
    parent_version  INT,                            -- 直接父版本（用于 diff 加速）
    md5             VARCHAR(32) NOT NULL,
    content         TEXT,                           -- 文本内容（markdown/text）
    storage_key     TEXT,                           -- 二进制文件存储 key
    size_bytes      BIGINT NOT NULL DEFAULT 0,
    author_id       UUID NOT NULL REFERENCES users(id),
    message         VARCHAR(512),                   -- 提交备注
    -- CAS
    base_version    INT,                            -- 写入时基于的版本号（用于审计）
    -- 元数据
    metadata        JSONB DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_flag    BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE (document_id, version_num, deleted_flag)
);

CREATE INDEX idx_versions_document ON document_versions(document_id, version_num DESC) WHERE deleted_flag = FALSE;
```

### 2.5 chunks（文本块 + 向量）

```sql
CREATE TABLE chunks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id     UUID NOT NULL REFERENCES documents(id),
    version_num     INT NOT NULL,                   -- 属于哪个版本
    chunk_index     INT NOT NULL,
    content         TEXT NOT NULL,
    content_md5     VARCHAR(32) NOT NULL,
    -- 向量（pgvector）
    embedding       vector(1024),                   -- 维度可配
    embedding_model VARCHAR(64) NOT NULL,
    -- 元数据
    char_count      INT NOT NULL,
    token_count     INT,
    metadata        JSONB DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (document_id, version_num, chunk_index)
);

CREATE INDEX idx_chunks_document ON chunks(document_id, version_num);
-- 向量索引（HNSW）
CREATE INDEX idx_chunks_embedding ON chunks USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);
```

### 2.6 sag_events（SAG 事件）

```sql
CREATE TABLE sag_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chunk_id        UUID NOT NULL REFERENCES chunks(id),
    document_id     UUID NOT NULL REFERENCES documents(id),
    event_text      TEXT NOT NULL,                  -- 事件完整文本
    event_embedding vector(1024),
    event_metadata  JSONB DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sag_events_chunk ON sag_events(chunk_id);
CREATE INDEX idx_sag_events_doc ON sag_events(document_id);
CREATE INDEX idx_sag_events_embedding ON sag_events USING hnsw (event_embedding vector_cosine_ops);
```

### 2.7 sag_entities（SAG 实体）

```sql
CREATE TABLE sag_entities (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id        UUID NOT NULL REFERENCES spaces(id),
    entity_name     VARCHAR(256) NOT NULL,
    entity_type     VARCHAR(64),                    -- person | org | concept | ...
    description     TEXT,
    -- 合并与别名
    canonical_id    UUID REFERENCES sag_entities(id),  -- 主实体（被合并到此）
    aliases         TEXT[] DEFAULT '{}',
    embedding       vector(1024),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (space_id, entity_name)
);

CREATE INDEX idx_sag_entities_space ON sag_entities(space_id);
CREATE INDEX idx_sag_entities_embedding ON sag_entities USING hnsw (embedding vector_cosine_ops);
```

### 2.8 sag_event_entity_links（事件-实体关联）

```sql
CREATE TABLE sag_event_entity_links (
    event_id        UUID NOT NULL REFERENCES sag_events(id),
    entity_id       UUID NOT NULL REFERENCES sag_entities(id),
    weight          REAL NOT NULL DEFAULT 1.0,      -- 关联强度
    role            VARCHAR(32),                    -- subject | object | context
    PRIMARY KEY (event_id, entity_id)
);

CREATE INDEX idx_links_entity ON sag_event_entity_links(entity_id);
```

### 2.9 users / workspace_users / api_keys / sessions

```sql
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username        VARCHAR(64) NOT NULL UNIQUE,
    email           VARCHAR(255) UNIQUE,
    display_name    VARCHAR(128),
    password_hash   TEXT,                           -- 可选（OAuth 登录可空）
    is_super_admin  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_flag    BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE workspace_users (
    workspace_id    UUID NOT NULL REFERENCES workspaces(id),
    user_id         UUID NOT NULL REFERENCES users(id),
    role            VARCHAR(16) NOT NULL,           -- owner | admin | contributor | viewer
    invited_by      UUID REFERENCES users(id),
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, user_id)
);

CREATE TABLE api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id),
    key_id          VARCHAR(64) NOT NULL UNIQUE,    -- 公钥部分
    key_secret_hash VARCHAR(128) NOT NULL,          -- secret 的哈希
    name            VARCHAR(128) NOT NULL,
    capabilities    TEXT[] NOT NULL,                -- 能力列表
    space_ids       UUID[] DEFAULT '{}',            -- 限制到特定空间
    expires_at      TIMESTAMPTZ,
    last_used_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at      TIMESTAMPTZ
);

CREATE TABLE sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id),
    title           VARCHAR(255),
    kb_ids          UUID[] DEFAULT '{}',            -- 引用的空间
    metadata        JSONB DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 2.10 audit_logs（审计日志）

```sql
CREATE TABLE audit_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    actor_id        UUID,                           -- 操作用户（可空，匿名操作）
    actor_type      VARCHAR(32) NOT NULL,           -- user | api_key | system
    action          VARCHAR(64) NOT NULL,           -- document.create | document.update | ...
    target_type     VARCHAR(32),                    -- document | space | binding | ...
    target_id       VARCHAR(128),
    request_id      VARCHAR(64),
    metadata        JSONB DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_workspace ON audit_logs(workspace_id, created_at DESC);
CREATE INDEX idx_audit_actor ON audit_logs(actor_id, created_at DESC);
```

## 3. 客户端文件格式

### 3.1 `.ai-kbs/workspace.yaml`

```yaml
# ai-kbs workspace configuration
# Schema version: 1.0

schema_version: "1.0"
wiki_url: "https://kb.example.com"     # ai-kbs 服务地址
default_space: "dev"                    # 默认 space code
profile: "default"                      # 本地 profile 名

# 本工作空间的绑定
bindings:
  - local: "docs"                       # 相对工作空间根
    remote: "dev:/api/"                 # space:path 格式
    created_at: "2026-09-14T10:00:00Z"
  - local: "docs/requirements"
    remote: "dev:/requirements/"

# 工作空间级选项
options:
  exclude: ["*.bak", "drafts/"]
  all_files: false
  binary_mode: "auto"                   # auto | skip | include

# 可选：每绑定独立配置（覆盖默认）
binding_options:
  "docs":
    binary_mode: "skip"
```

### 3.2 `.ai-kbs/sync-state.json`

```json
{
  "schema_version": "1.0",
  "workspace_id": "ws-uuid",
  "bindings": {
    "docs": {
      "remote": "dev:/api/",
      "files": {
        "users.md": {
          "docCode": "aB3kX9q2",
          "md5": "098f6bcd4621d373cade4e832627b4f6",
          "version": 7,
          "remotePath": "/api/users.md",
          "lastSyncAt": "2026-09-14T10:30:00Z"
        }
      }
    }
  }
}
```

### 3.3 `~/.ai-kbs/credentials`

```toml
# 凭证文件（TOML 格式）
url = "https://kb.example.com"

[hmac]
key_id = "ak_xxx"
key_secret = "sk_xxx"   # 仅本地签名，不传输

[token]
token = "eyJhbGc..."     # 直连后端 JWT（不推荐）

[profile]
name = "zhangsan"
display_name = "张三"
expires_at = "2026-12-01T00:00:00Z"
```

## 4. 状态机

### 4.1 文档版本状态机

```
[创建] ─── write --create ───▶ version 1 (active)
                                  │
                                  ▼
[更新] ─── write --base-version ─▶ version N+1 (active)
                                  │
                  conflict ──────┴─────── 22 VersionConflict
                                  │
                                  ▼
[编辑] ─── edit --base-version ─▶ version N+1 (active)
                                  │
                                  ▼
[回滚] ─── version rollback ───▶ version M+1 (active)
                                  │
              (中间版本全部保留) ◀┘
```

### 4.2 文档删除与恢复

```
[active] ─── push --prune ───▶ [soft-deleted]
                                   │
                            undelete
                                   │
                                   ▼
                                [active]    (不产生新版本)
```

### 4.3 同步状态机

```
[synced] ─── local edit ───▶ [local-modified]
                                 │
                  push ─────────┴──────────▶ [synced] (version+1)
                                 │
                  conflict ────┘
                                 │
                                 ▼
                          [conflict] (待 resolve)
                                 │
                  resolve ─────┴──────────▶ [synced]
```

## 5. ES 索引设计

### 5.1 chunks 索引

```json
PUT /ai-kbs-chunks
{
  "settings": {
    "number_of_shards": 3,
    "number_of_replicas": 1,
    "analysis": {
      "analyzer": {
        "ik_smart": {
          "type": "ik_smart"      // 中文 IK 分词
        },
        "edge_ngram": {
          "type": "custom",
          "tokenizer": "edge_ngram_tokenizer"
        }
      }
    }
  },
  "mappings": {
    "properties": {
      "chunk_id":     { "type": "keyword" },
      "doc_code":     { "type": "keyword" },
      "space_id":     { "type": "keyword" },
      "space_code":   { "type": "keyword" },
      "title":        { "type": "text", "analyzer": "ik_smart" },
      "content":      { "type": "text", "analyzer": "ik_smart" },
      "embedding":    { "type": "dense_vector", "dims": 1024, "index": true, "similarity": "cosine" },
      "tags":         { "type": "keyword" },
      "version_num":  { "type": "integer" },
      "doc_type":     { "type": "keyword" },
      "updated_at":   { "type": "date" }
    }
  }
}
```

### 5.2 documents 元数据索引（用于搜索结果展示）

```json
PUT /ai-kbs-docs
{
  "mappings": {
    "properties": {
      "doc_code":     { "type": "keyword" },
      "space_code":   { "type": "keyword" },
      "title":        { "type": "text", "analyzer": "ik_smart" },
      "tree_path":    { "type": "keyword" },
      "tags":         { "type": "keyword" },
      "doc_type":     { "type": "keyword" },
      "updated_at":   { "type": "date" },
      "size_bytes":   { "type": "long" }
    }
  }
}
```

## 6. SAG 本地副本（参考 SAG 项目）

### 6.1 LanceDB 表（按 SAG 论文）

```
sag-events:    id | chunk_id | event_text | event_embedding | space_id
sag-entities:  id | name | type | embedding | space_id | canonical_id
sag-links:     event_id | entity_id | weight | role
```

### 6.2 与 PostgreSQL 同步

- 写入：先写 PostgreSQL，MQ 异步任务同步到 SAG 本地副本（事件/实体/关联）
- 读取：SAG 检索走本地副本，返回 chunk_id 后回查 PostgreSQL 拿原文

## 7. 数据生命周期

| 对象 | 保留期 | 清理策略 |
|------|--------|----------|
| 文档 active 版本 | 永久 | 不清理 |
| 文档历史版本 | 永久 | 仅用户主动删除（不自动清理） |
| 软删除文档 | 30 天后 | 自动物理删除（可配） |
| 审计日志 | 1 年 | 自动归档到冷存储 |
| 会话 | 永久（用户控制） | 用户手动删除 |
| 临时文件（上传中） | 24 小时 | 自动清理 |
| SAG 事件 | 跟随对应 chunk 生命周期 | chunk 删除时同步删除 |
| ES 索引副本 | 实时同步 | 与 PostgreSQL 一致 |
