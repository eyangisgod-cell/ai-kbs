-- Migration 000079 (down): drop the AI-KBS core schema.
-- Reverse order: child tables first, parents last, triggers / functions last.
-- All drops are idempotent (IF EXISTS) so re-running on a clean DB is a no-op.

DO $$ BEGIN RAISE NOTICE '[Migration 000079 down] Dropping AI-KBS core schema'; END $$;

-- Triggers
DROP TRIGGER IF EXISTS trg_sessions_updated_at ON sessions;
DROP TRIGGER IF EXISTS trg_users_updated_at    ON users;
DROP TRIGGER IF EXISTS trg_spaces_updated_at   ON spaces;
DROP TRIGGER IF EXISTS trg_documents_updated_at ON documents;

-- Trigger function (shared, only drop after triggers)
DROP FUNCTION IF EXISTS ai_kbs_set_updated_at();

-- Tables in reverse FK-dependency order
DROP TABLE IF EXISTS audit_logs_ai        CASCADE;
DROP TABLE IF EXISTS sag_event_entity_links CASCADE;
DROP TABLE IF EXISTS sag_entities         CASCADE;
DROP TABLE IF EXISTS sag_events           CASCADE;
DROP TABLE IF EXISTS chunks_ai            CASCADE;
DROP TABLE IF EXISTS document_versions    CASCADE;
DROP TABLE IF EXISTS documents            CASCADE;
DROP TABLE IF EXISTS spaces               CASCADE;
DROP TABLE IF EXISTS api_keys             CASCADE;
DROP TABLE IF EXISTS sessions             CASCADE;
DROP TABLE IF EXISTS workspace_users      CASCADE;
DROP TABLE IF EXISTS workspaces           CASCADE;
DROP TABLE IF EXISTS users                CASCADE;

-- Note: we do NOT drop the `vector` or `pgcrypto` extensions here; they may
-- be in use by other AI-KBS migrations (T2.x) and by the fork's tenants
-- / embeddings tables. Extensions are not migration-scoped state.

DO $$ BEGIN RAISE NOTICE '[Migration 000079 down] AI-KBS core schema dropped'; END $$;
