-- =============================================================================
-- AI-KBS PostgreSQL bootstrap — pgvector extension + dev DB
--
-- Runs on first container start (when the data dir is empty), via the
-- official postgres image's /docker-entrypoint-initdb.d hook. Subsequent
-- `docker compose up` invocations skip this because the data dir is
-- already initialised.
--
-- Why here, not in a T1.x migration:
--   pgvector must exist at the database-level BEFORE the backend starts
--   AUTO_MIGRATE; a pure SQL migration runs inside a transaction and
--   cannot CREATE EXTENSION outside the bootstrap phase. Putting it here
--   guarantees the extension is installed idempotently.
--
-- Real application schema (workspaces / spaces / documents / versions /
-- chunks / sag_events / sag_entities / sag_event_entity_links / users /
-- workspace_users / api_keys / sessions / audit_logs) ships in T1.1 via
-- migrations/001_init.up.sql.
-- =============================================================================

-- pgvector ships in the pgvector/pgvector:pg16-alpine image (the extension
-- is baked into the image rather than installed via apt). IF NOT EXISTS
-- keeps the script idempotent so a non-pgvector image (e.g. for tests) can
-- still bootstrap successfully.
CREATE EXTENSION IF NOT EXISTS vector;

-- uuid generation for primary keys in T1.x tables.
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- trigram matching for ILIKE / fuzzy search fallback (NFR-AVAIL-03).
CREATE EXTENSION IF NOT EXISTS pg_trgm;