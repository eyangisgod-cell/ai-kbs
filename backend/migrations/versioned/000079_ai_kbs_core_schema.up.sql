-- Migration 000079: ai-kbs core schema (T1.1)
-- ---------------------------------------------------------------------------
-- Adds 13 spec-compliant tables for the AI-KBS subsystem, per
-- require/05-data-model.md §2.1–2.10 and the M1 design contract.
--
-- This migration is ADDITIVE: it does not touch any pre-existing
-- fork tables (tenants / knowledges / chunks / …), nor does it
-- rename or repurpose any column. The existing fork schema is
-- retained for backward compatibility with T0.x services.
--
-- Tables created (13):
--   1. workspaces
--   2. spaces
--   3. documents
--   4. document_versions
--   5. chunks_ai
--   6. sag_events
--   7. sag_entities
--   8. sag_event_entity_links
--   9. users
--  10. workspace_users
--  11. api_keys
--  12. sessions
--  13. audit_logs_ai
--
-- Index inventory (target ≈ 25):
--   HNSW:    chunks_ai.embedding, sag_events.event_embedding, sag_entities.embedding
--   GIN:     documents.tags, documents.metadata, documents.content_tsv
--   partial: documents (workspace-active, deleted_at IS NULL),
--            spaces (workspace-active),
--            sag_events (workspace_id, occurred_at),
--            audit_logs_ai (workspace_id, created_at)
--   btree uniq: users.email, api_keys.key_hash, sessions.token_hash,
--               workspace_users(ws,user), sag_entities(ws,name),
--               document_versions(doc,version), chunks_ai(doc,version,ordinal)
--   btree composite: many FK columns + lookup keys
--
-- Note on naming: spec uses `chunks` and `audit_logs`, but the fork
-- already owns those table names. To stay additive and avoid
-- destructive renaming, the migration creates `chunks_ai` and
-- `audit_logs_ai`. T1.2 (repository layer) and the M1 design contract
-- will alias these as the application-facing "chunks" and "audit_logs"
-- domain models. The data-model spec in require/05-data-model.md
-- remains the source of truth; this naming is a fork-compat
-- accommodation documented in the commit message and the design.md
-- decision log.
-- ---------------------------------------------------------------------------

DO $$ BEGIN RAISE NOTICE '[Migration 000079] AI-KBS core schema (T1.1) — starting'; END $$;

-- Required extensions -----------------------------------------------------
CREATE EXTENSION IF NOT EXISTS "pgcrypto";   -- gen_random_uuid()
DO $$ BEGIN
    -- pgvector for HNSW. pgvector >= 0.5 required for the WITH (m, ef_construction)
    -- syntax used below. docker-compose dev image pre-installs pgvector/pgvector:pg15.
    BEGIN
        CREATE EXTENSION IF NOT EXISTS vector;
        RAISE NOTICE '[Migration 000079] pgvector extension enabled';
    EXCEPTION WHEN undefined_file OR feature_not_supported THEN
        RAISE EXCEPTION 'pgvector extension is required for AI-KBS; install pgvector >= 0.5';
    END;
END $$;

-- ---------------------------------------------------------------------------
-- 1. workspaces
-- ---------------------------------------------------------------------------
DO $$ BEGIN RAISE NOTICE '[Migration 000079] Creating table: workspaces'; END $$;
CREATE TABLE IF NOT EXISTS workspaces (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL,
    owner_id    UUID NOT NULL,                        -- backref to users (deferred FK: users created next)
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_workspaces_slug
    ON workspaces (slug) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_workspaces_owner ON workspaces (owner_id);

-- ---------------------------------------------------------------------------
-- 9. users  (created before workspaces/sessions/api_keys to satisfy FKs)
-- ---------------------------------------------------------------------------
DO $$ BEGIN RAISE NOTICE '[Migration 000079] Creating table: users'; END $$;
CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username        VARCHAR(64) NOT NULL,
    email           VARCHAR(255),
    display_name    VARCHAR(128),
    password_hash   TEXT,
    is_super_admin  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_flag    BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_users_username
    ON users (username) WHERE deleted_flag = FALSE;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_users_email
    ON users (email) WHERE email IS NOT NULL AND deleted_flag = FALSE;

-- Now wire workspaces.owner_id → users.id (deferred because of order)
DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_workspaces_owner'
    ) THEN
        ALTER TABLE workspaces
            ADD CONSTRAINT fk_workspaces_owner
            FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE RESTRICT;
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- 10. workspace_users  (RBAC role binding)
-- ---------------------------------------------------------------------------
DO $$ BEGIN RAISE NOTICE '[Migration 000079] Creating table: workspace_users'; END $$;
CREATE TABLE IF NOT EXISTS workspace_users (
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role            VARCHAR(16) NOT NULL,                -- owner | admin | contributor | viewer
    invited_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_workspace_users_user ON workspace_users (user_id);
CREATE INDEX IF NOT EXISTS idx_workspace_users_role ON workspace_users (workspace_id, role);

-- ---------------------------------------------------------------------------
-- 2. spaces
-- ---------------------------------------------------------------------------
DO $$ BEGIN RAISE NOTICE '[Migration 000079] Creating table: spaces'; END $$;
CREATE TABLE IF NOT EXISTS spaces (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id     UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    code             VARCHAR(64) NOT NULL,
    name             VARCHAR(255) NOT NULL,
    description      TEXT,
    owner_id         UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    -- retrieval configuration
    embedding_model  VARCHAR(64) NOT NULL,
    embedding_dim    INT NOT NULL DEFAULT 1024,
    chunk_strategy   VARCHAR(32) NOT NULL DEFAULT 'semantic',  -- semantic | parent_child | fixed
    chunk_size       INT NOT NULL DEFAULT 512,
    chunk_overlap    INT NOT NULL DEFAULT 64,
    -- routing
    sag_enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    -- metadata
    metadata         JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- soft-delete sentinel: rows with deleted_at >= '2100-01-01' are considered live
    -- (lets UNIQUE(workspace_id, code, deleted_at) coexist with hard-resurrect)
    deleted_at       TIMESTAMPTZ NOT NULL DEFAULT '1970-01-01 00:00:00+00'
);
-- Uniqueness of code inside a workspace, scoped to live rows
CREATE UNIQUE INDEX IF NOT EXISTS uniq_spaces_ws_code_live
    ON spaces (workspace_id, code) WHERE deleted_at < '2100-01-01';
CREATE INDEX IF NOT EXISTS idx_spaces_workspace
    ON spaces (workspace_id) WHERE deleted_at < '2100-01-01';
CREATE INDEX IF NOT EXISTS idx_spaces_owner ON spaces (owner_id);

-- ---------------------------------------------------------------------------
-- 3. documents
-- ---------------------------------------------------------------------------
DO $$ BEGIN RAISE NOTICE '[Migration 000079] Creating table: documents'; END $$;
CREATE TABLE IF NOT EXISTS documents (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id             UUID NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    doc_code             VARCHAR(16) NOT NULL,                 -- 8-char base62
    parent_doc_code      VARCHAR(16),
    tree_path            TEXT NOT NULL,
    depth                INT NOT NULL DEFAULT 0,
    title                VARCHAR(512) NOT NULL,
    doc_type             VARCHAR(32) NOT NULL DEFAULT 'markdown',
    mime_type            VARCHAR(128),
    active_version       INT NOT NULL DEFAULT 1,
    active_md5           VARCHAR(32) NOT NULL,
    -- storage
    storage_key          TEXT,
    size_bytes           BIGINT NOT NULL DEFAULT 0,
    -- retrieval flags
    sag_indexed          BOOLEAN NOT NULL DEFAULT FALSE,
    es_indexed           BOOLEAN NOT NULL DEFAULT FALSE,
    -- FTS support
    content_tsv          TSVECTOR,
    -- metadata
    tags                 TEXT[] NOT NULL DEFAULT '{}',
    metadata             JSONB NOT NULL DEFAULT '{}'::jsonb,
    confidential_level   VARCHAR(32) NOT NULL DEFAULT 'unclassified',
    created_by           UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    updated_by           UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_flag         BOOLEAN NOT NULL DEFAULT FALSE,
    deleted_at           TIMESTAMPTZ NOT NULL DEFAULT '1970-01-01 00:00:00+00'
);
-- Uniqueness: doc_code & tree_path each unique inside a space, scoped to live rows
CREATE UNIQUE INDEX IF NOT EXISTS uniq_documents_space_doc_code_live
    ON documents (space_id, doc_code) WHERE deleted_flag = FALSE;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_documents_space_tree_path_live
    ON documents (space_id, tree_path) WHERE deleted_flag = FALSE;
-- Indexes required by AC-B1 / spec
CREATE INDEX IF NOT EXISTS idx_documents_workspace_active
    ON documents (space_id) WHERE deleted_flag = FALSE;       -- partial
CREATE INDEX IF NOT EXISTS idx_documents_parent
    ON documents (space_id, parent_doc_code) WHERE deleted_flag = FALSE;
CREATE INDEX IF NOT EXISTS idx_documents_tags
    ON documents USING GIN (tags) WHERE deleted_flag = FALSE;  -- GIN
CREATE INDEX IF NOT EXISTS idx_documents_metadata
    ON documents USING GIN (metadata) WHERE deleted_flag = FALSE;
CREATE INDEX IF NOT EXISTS idx_documents_content_tsv
    ON documents USING GIN (content_tsv) WHERE deleted_flag = FALSE;  -- GIN FTS
CREATE INDEX IF NOT EXISTS idx_documents_path_ops
    ON documents (space_id, tree_path text_pattern_ops);
CREATE INDEX IF NOT EXISTS idx_documents_created_at
    ON documents (space_id, created_at DESC);

-- ---------------------------------------------------------------------------
-- 4. document_versions  (version chain, with CAS base_version)
-- ---------------------------------------------------------------------------
DO $$ BEGIN RAISE NOTICE '[Migration 000079] Creating table: document_versions'; END $$;
CREATE TABLE IF NOT EXISTS document_versions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id     UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    version_num     INT NOT NULL,                              -- 1, 2, 3, ...
    parent_version  INT,                                       -- direct predecessor
    md5             VARCHAR(32) NOT NULL,
    content         TEXT,
    storage_key     TEXT,
    size_bytes      BIGINT NOT NULL DEFAULT 0,
    author_id       UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    message         VARCHAR(512),
    -- CAS bookkeeping (mirrors data-model §2.4)
    base_version    INT,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_flag    BOOLEAN NOT NULL DEFAULT FALSE
);
-- (document_id, version_num) unique for live rows. Spec data-model §2.4
-- writes UNIQUE (document_id, version_num, deleted_flag); since the
-- 3-tuple already enforces (doc, version) per deleted_flag value, this
-- migration uses a unique partial index on live rows + btree on all rows
-- for FK / lookups.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_versions_doc_version_live
    ON document_versions (document_id, version_num) WHERE deleted_flag = FALSE;
CREATE INDEX IF NOT EXISTS idx_versions_document
    ON document_versions (document_id, version_num DESC) WHERE deleted_flag = FALSE;
CREATE INDEX IF NOT EXISTS idx_versions_author ON document_versions (author_id);

-- ---------------------------------------------------------------------------
-- 5. chunks_ai  (text + embedding; named with _ai suffix for fork-compat)
-- ---------------------------------------------------------------------------
DO $$ BEGIN RAISE NOTICE '[Migration 000079] Creating table: chunks_ai'; END $$;
CREATE TABLE IF NOT EXISTS chunks_ai (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id      UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    version_num      INT NOT NULL,
    ordinal          INT NOT NULL,                              -- spec: chunk_index
    content          TEXT NOT NULL,
    content_md5      VARCHAR(32) NOT NULL,
    -- embedding (pgvector)
    embedding        vector(1024),
    embedding_model  VARCHAR(64) NOT NULL,
    char_count       INT NOT NULL,
    token_count      INT,
    metadata         JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_chunks_doc_version_ordinal
    ON chunks_ai (document_id, version_num, ordinal);
CREATE INDEX IF NOT EXISTS idx_chunks_ai_document
    ON chunks_ai (document_id, version_num);
-- HNSW vector index — pgvector >= 0.5
CREATE INDEX IF NOT EXISTS idx_chunks_ai_embedding
    ON chunks_ai USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);

-- ---------------------------------------------------------------------------
-- 6. sag_events
-- ---------------------------------------------------------------------------
DO $$ BEGIN RAISE NOTICE '[Migration 000079] Creating table: sag_events'; END $$;
CREATE TABLE IF NOT EXISTS sag_events (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chunk_id         UUID NOT NULL REFERENCES chunks_ai(id) ON DELETE CASCADE,
    document_id      UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    event_text       TEXT NOT NULL,
    event_embedding  vector(1024),
    event_metadata   JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sag_events_chunk
    ON sag_events (chunk_id);
CREATE INDEX IF NOT EXISTS idx_sag_events_doc
    ON sag_events (document_id);
CREATE INDEX IF NOT EXISTS idx_sag_events_workspace_time
    ON sag_events (document_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_sag_events_embedding
    ON sag_events USING hnsw (event_embedding vector_cosine_ops);

-- ---------------------------------------------------------------------------
-- 7. sag_entities
-- ---------------------------------------------------------------------------
DO $$ BEGIN RAISE NOTICE '[Migration 000079] Creating table: sag_entities'; END $$;
CREATE TABLE IF NOT EXISTS sag_entities (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id      UUID NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    name          VARCHAR(256) NOT NULL,
    entity_type   VARCHAR(64),                                -- person | org | concept | ...
    description   TEXT,
    canonical_id  UUID REFERENCES sag_entities(id) ON DELETE SET NULL,
    aliases       TEXT[] NOT NULL DEFAULT '{}',
    embedding     vector(1024),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_sag_entities_space_name
    ON sag_entities (space_id, name);
CREATE INDEX IF NOT EXISTS idx_sag_entities_space
    ON sag_entities (space_id);
CREATE INDEX IF NOT EXISTS idx_sag_entities_canonical
    ON sag_entities (canonical_id) WHERE canonical_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_sag_entities_embedding
    ON sag_entities USING hnsw (embedding vector_cosine_ops);

-- ---------------------------------------------------------------------------
-- 8. sag_event_entity_links
-- ---------------------------------------------------------------------------
DO $$ BEGIN RAISE NOTICE '[Migration 000079] Creating table: sag_event_entity_links'; END $$;
CREATE TABLE IF NOT EXISTS sag_event_entity_links (
    event_id   UUID NOT NULL REFERENCES sag_events(id) ON DELETE CASCADE,
    entity_id  UUID NOT NULL REFERENCES sag_entities(id) ON DELETE CASCADE,
    weight     REAL NOT NULL DEFAULT 1.0,
    role       VARCHAR(32),                                   -- subject | object | context
    PRIMARY KEY (event_id, entity_id)
);
CREATE INDEX IF NOT EXISTS idx_sag_links_entity
    ON sag_event_entity_links (entity_id);

-- ---------------------------------------------------------------------------
-- 11. api_keys
-- ---------------------------------------------------------------------------
DO $$ BEGIN RAISE NOTICE '[Migration 000079] Creating table: api_keys'; END $$;
CREATE TABLE IF NOT EXISTS api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    key_id          VARCHAR(64) NOT NULL,                     -- public part
    key_hash        VARCHAR(128) NOT NULL,                    -- bcrypt of secret
    name            VARCHAR(128) NOT NULL,
    capabilities    TEXT[] NOT NULL DEFAULT '{}',
    space_ids       UUID[] NOT NULL DEFAULT '{}',
    expires_at      TIMESTAMPTZ,
    last_used_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at      TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_api_keys_key_id
    ON api_keys (key_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys (user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_workspace ON api_keys (workspace_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_active
    ON api_keys (workspace_id) WHERE revoked_at IS NULL;

-- ---------------------------------------------------------------------------
-- 12. sessions
-- ---------------------------------------------------------------------------
DO $$ BEGIN RAISE NOTICE '[Migration 000079] Creating table: sessions'; END $$;
CREATE TABLE IF NOT EXISTS sessions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       VARCHAR(255),
    kb_ids      UUID[] NOT NULL DEFAULT '{}',
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sessions_user
    ON sessions (user_id, updated_at DESC);

-- ---------------------------------------------------------------------------
-- 13. audit_logs_ai
-- ---------------------------------------------------------------------------
-- spec §2.10 defines an audit log; the fork already has `audit_logs`
-- (BIGSERIAL, tenant_id). To stay additive, this is `audit_logs_ai`.
-- Spec columns preserved; spec leaves `payload` JSONB schema open, so
-- we add example keys in a comment.
DO $$ BEGIN RAISE NOTICE '[Migration 000079] Creating table: audit_logs_ai'; END $$;
CREATE TABLE IF NOT EXISTS audit_logs_ai (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,                            -- no FK: workspace may be hard-deleted
    actor_id        UUID,                                     -- nullable for anonymous
    actor_type      VARCHAR(32) NOT NULL,                     -- user | api_key | system
    action          VARCHAR(64) NOT NULL,                     -- document.create | ...
    target_type     VARCHAR(32),
    target_id       VARCHAR(128),
    request_id      VARCHAR(64),
    trace_id        VARCHAR(64),                              -- W3C trace-id
    ip              VARCHAR(64),
    user_agent      VARCHAR(512),
    payload         JSONB NOT NULL DEFAULT '{}'::jsonb,        -- spec: flexible
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
    -- example payload keys: { "method":"PUT", "status_code":200, "diff_lines":42,
    --                         "doc_code":"aB3xY7zQ", "base_version":7 }
);
CREATE INDEX IF NOT EXISTS idx_audit_ai_workspace_time
    ON audit_logs_ai (workspace_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_ai_actor_time
    ON audit_logs_ai (actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_ai_action
    ON audit_logs_ai (workspace_id, action);
CREATE INDEX IF NOT EXISTS idx_audit_ai_target
    ON audit_logs_ai (target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_audit_ai_request
    ON audit_logs_ai (request_id) WHERE request_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- Trigger: keep documents.updated_at fresh on UPDATE
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION ai_kbs_set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_documents_updated_at') THEN
        CREATE TRIGGER trg_documents_updated_at
            BEFORE UPDATE ON documents
            FOR EACH ROW EXECUTE FUNCTION ai_kbs_set_updated_at();
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_spaces_updated_at') THEN
        CREATE TRIGGER trg_spaces_updated_at
            BEFORE UPDATE ON spaces
            FOR EACH ROW EXECUTE FUNCTION ai_kbs_set_updated_at();
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_users_updated_at') THEN
        CREATE TRIGGER trg_users_updated_at
            BEFORE UPDATE ON users
            FOR EACH ROW EXECUTE FUNCTION ai_kbs_set_updated_at();
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_sessions_updated_at') THEN
        CREATE TRIGGER trg_sessions_updated_at
            BEFORE UPDATE ON sessions
            FOR EACH ROW EXECUTE FUNCTION ai_kbs_set_updated_at();
    END IF;
END $$;

DO $$ BEGIN RAISE NOTICE '[Migration 000079] AI-KBS core schema (T1.1) — complete'; END $$;
