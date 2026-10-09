package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/eyangisgod-cell/ai-kbs/internal/repository/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TableUsers is the SQL table backing UserRepository.
const TableUsers = "users"

// UserRepository persists users. Password hashes and credential fields
// are NEVER logged by the repository; the credential-bearing code path
// (T1.5 auth) handles redaction at its boundary.
type UserRepository interface {
	Create(ctx context.Context, u *domain.User) error
	Get(ctx context.Context, id string) (*domain.User, error)
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	UpdatePasswordHash(ctx context.Context, id, passwordHash string) error
	SoftDelete(ctx context.Context, id string) error
}

type userRepo struct {
	db DBTX
}

// NewUserRepository wires a UserRepository to a DBTX (pool or tx).
func NewUserRepository(db *pgxpool.Pool) UserRepository { return &userRepo{db: db} }

func (r *userRepo) Create(ctx context.Context, u *domain.User) error {
	if u == nil {
		return errors.New("repository: user is nil")
	}
	const q = `INSERT INTO users (id, username, email, display_name, password_hash,
	                              is_super_admin, created_at, updated_at, deleted_flag)
	           VALUES (COALESCE(NULLIF($1, ''), gen_random_uuid()), $2, $3, $4, $5,
	                   $6, NOW(), NOW(), FALSE)`
	_, err := r.db.Exec(ctx, q,
		u.ID, u.Username, nullable(u.Email), nullable(u.DisplayName), nullable(u.PasswordHash),
		u.IsSuperAdmin,
	)
	if err != nil {
		if mapped := mapPgError(err); errors.Is(mapped, ErrAlreadyExists) {
			return fmt.Errorf("user username=%q: %w", u.Username, mapped)
		}
		return fmt.Errorf("user.Create username=%q: %w", u.Username, err)
	}
	return nil
}

func (r *userRepo) Get(ctx context.Context, id string) (*domain.User, error) {
	const q = `SELECT id, username, email, display_name, password_hash,
	                  is_super_admin, created_at, updated_at, deleted_flag
	           FROM users WHERE id = $1 AND deleted_flag = FALSE`
	u, err := scanUser(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("user.Get id=%s: %w", id, err)
	}
	return u, nil
}

func (r *userRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	const q = `SELECT id, username, email, display_name, password_hash,
	                  is_super_admin, created_at, updated_at, deleted_flag
	           FROM users WHERE username = $1 AND deleted_flag = FALSE`
	u, err := scanUser(r.db.QueryRow(ctx, q, username))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("user.GetByUsername %q: %w", username, err)
	}
	return u, nil
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	const q = `SELECT id, username, email, display_name, password_hash,
	                  is_super_admin, created_at, updated_at, deleted_flag
	           FROM users WHERE email = $1 AND deleted_flag = FALSE`
	u, err := scanUser(r.db.QueryRow(ctx, q, email))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("user.GetByEmail %q: %w", email, err)
	}
	return u, nil
}

func (r *userRepo) UpdatePasswordHash(ctx context.Context, id, passwordHash string) error {
	if passwordHash == "" {
		return errors.New("repository: passwordHash is empty")
	}
	const q = `UPDATE users SET password_hash = $2, updated_at = NOW()
	           WHERE id = $1 AND deleted_flag = FALSE`
	tag, err := r.db.Exec(ctx, q, id, passwordHash)
	if err != nil {
		return fmt.Errorf("user.UpdatePasswordHash id=%s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *userRepo) SoftDelete(ctx context.Context, id string) error {
	const q = `UPDATE users SET deleted_flag = TRUE, updated_at = NOW()
	           WHERE id = $1 AND deleted_flag = FALSE`
	tag, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("user.SoftDelete id=%s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	var email, displayName, passwordHash *string
	err := row.Scan(
		&u.ID, &u.Username, &email, &displayName, &passwordHash,
		&u.IsSuperAdmin, &u.CreatedAt, &u.UpdatedAt, &u.DeletedFlag,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if email != nil {
		u.Email = *email
	}
	if displayName != nil {
		u.DisplayName = *displayName
	}
	if passwordHash != nil {
		u.PasswordHash = *passwordHash
	}
	return &u, nil
}