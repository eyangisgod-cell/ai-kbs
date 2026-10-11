package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/eyangisgod-cell/ai-kbs/internal/repository/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TableAPIKeys is the SQL table backing APIKeyRepository.
const TableAPIKeys = "api_keys"

// APIKeyRepository persists API keys. Key hashes are NEVER written to
// logs; the repository itself does no logging, and the auth layer
// (T1.5) is responsible for redacting credentials in any error path.
type APIKeyRepository interface {
	Create(ctx context.Context, k *domain.APIKey) error
	Get(ctx context.Context, id string) (*domain.APIKey, error)
	GetByKeyID(ctx context.Context, keyID string) (*domain.APIKey, error)
	ListByUser(ctx context.Context, userID string) ([]*domain.APIKey, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]*domain.APIKey, error)
	Revoke(ctx context.Context, id string) error
	TouchLastUsed(ctx context.Context, id string) error
}

type apiKeyRepo struct {
	db DBTX
}

// NewAPIKeyRepository wires an APIKeyRepository to a DBTX (pool or tx).
func NewAPIKeyRepository(db *pgxpool.Pool) APIKeyRepository { return &apiKeyRepo{db: db} }

func (r *apiKeyRepo) Create(ctx context.Context, k *domain.APIKey) error {
	if k == nil {
		return errors.New("repository: api_key is nil")
	}
	if k.KeyID == "" || k.KeyHash == "" {
		return errors.New("repository: api_key requires key_id and key_hash")
	}
	caps := k.Capabilities
	if caps == nil {
		caps = []string{}
	}
	spaceIDs := k.SpaceIDs
	if spaceIDs == nil {
		spaceIDs = []string{}
	}
	const q = `INSERT INTO api_keys (id, user_id, workspace_id, key_id, key_hash,
	                                 name, capabilities, space_ids,
	                                 expires_at, last_used_at, created_at, revoked_at)
	           VALUES (COALESCE(NULLIF($1, ''), gen_random_uuid()), $2, $3, $4, $5,
	                   $6, $7, $8,
	                   $9, $10, NOW(), NULL)`
	_, err := r.db.Exec(ctx, q,
		k.ID, k.UserID, k.WorkspaceID, k.KeyID, k.KeyHash,
		k.Name, caps, spaceIDs,
		k.ExpiresAt, k.LastUsedAt,
	)
	if err != nil {
		if mapped := mapPgError(err); errors.Is(mapped, ErrAlreadyExists) {
			return fmt.Errorf("api_key key_id=%q: %w", k.KeyID, mapped)
		}
		return fmt.Errorf("api_key.Create key_id=%q: %w", k.KeyID, err)
	}
	return nil
}

func (r *apiKeyRepo) Get(ctx context.Context, id string) (*domain.APIKey, error) {
	const q = `SELECT id, user_id, workspace_id, key_id, key_hash, name,
	                  capabilities, space_ids, expires_at, last_used_at,
	                  created_at, revoked_at
	           FROM api_keys WHERE id = $1 AND revoked_at IS NULL`
	k, err := scanAPIKey(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("api_key.Get id=%s: %w", id, err)
	}
	return k, nil
}

func (r *apiKeyRepo) GetByKeyID(ctx context.Context, keyID string) (*domain.APIKey, error) {
	// NOTE: this returns the row even if revoked; the auth layer
	// decides whether a revoked key may still authenticate. The
	// repository just surfaces the row.
	const q = `SELECT id, user_id, workspace_id, key_id, key_hash, name,
	                  capabilities, space_ids, expires_at, last_used_at,
	                  created_at, revoked_at
	           FROM api_keys WHERE key_id = $1`
	k, err := scanAPIKey(r.db.QueryRow(ctx, q, keyID))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("api_key.GetByKeyID %q: %w", keyID, err)
	}
	return k, nil
}

func (r *apiKeyRepo) ListByUser(ctx context.Context, userID string) ([]*domain.APIKey, error) {
	const q = `SELECT id, user_id, workspace_id, key_id, key_hash, name,
	                  capabilities, space_ids, expires_at, last_used_at,
	                  created_at, revoked_at
	           FROM api_keys WHERE user_id = $1 AND revoked_at IS NULL
	           ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("api_key.ListByUser user=%s: %w", userID, err)
	}
	defer rows.Close()
	var out []*domain.APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("api_key.ListByUser rows.Err: %w", err)
	}
	return out, nil
}

func (r *apiKeyRepo) ListByWorkspace(ctx context.Context, workspaceID string) ([]*domain.APIKey, error) {
	const q = `SELECT id, user_id, workspace_id, key_id, key_hash, name,
	                  capabilities, space_ids, expires_at, last_used_at,
	                  created_at, revoked_at
	           FROM api_keys WHERE workspace_id = $1 AND revoked_at IS NULL
	           ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, q, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("api_key.ListByWorkspace ws=%s: %w", workspaceID, err)
	}
	defer rows.Close()
	var out []*domain.APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("api_key.ListByWorkspace rows.Err: %w", err)
	}
	return out, nil
}

func (r *apiKeyRepo) Revoke(ctx context.Context, id string) error {
	const q = `UPDATE api_keys SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`
	tag, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("api_key.Revoke id=%s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *apiKeyRepo) TouchLastUsed(ctx context.Context, id string) error {
	const q = `UPDATE api_keys SET last_used_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("api_key.TouchLastUsed id=%s: %w", id, err)
	}
	return nil
}

func scanAPIKey(row pgx.Row) (*domain.APIKey, error) {
	var k domain.APIKey
	err := row.Scan(
		&k.ID, &k.UserID, &k.WorkspaceID, &k.KeyID, &k.KeyHash, &k.Name,
		&k.Capabilities, &k.SpaceIDs, &k.ExpiresAt, &k.LastUsedAt,
		&k.CreatedAt, &k.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &k, nil
}