package postgres

import (
	"context"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dev1klas/1klas-identity/internal/domain/session"
	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
	"github.com/dev1klas/1klas-identity/internal/domain/user"
)

// SessionRepository implements session.Repository against Postgres.
type SessionRepository struct {
	pool *pgxpool.Pool
}

// NewSessionRepository constructs the repository.
func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool}
}

// SaveTx inserts a new session inside tx.
func (r *SessionRepository) SaveTx(ctx context.Context, t user.Tx, s session.Session) error {
	tx, err := txOf(t)
	if err != nil {
		return err
	}
	sql, args, err := sq.
		Insert("identity.sessions").
		Columns("id", "tenant_id", "user_id", "token_hash", "created_at", "expires_at").
		Values(s.ID(), s.TenantID().UUID(), s.UserID(), s.TokenHash(), s.CreatedAt(), s.ExpiresAt()).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, sql, args...)
	return err
}

// FindByTokenHash loads a session by SHA-256 token hash.
func (r *SessionRepository) FindByTokenHash(ctx context.Context, tokenHash []byte) (session.Session, error) {
	sql, args, err := sq.
		Select("id", "tenant_id", "user_id", "token_hash", "created_at", "expires_at", "revoked_at").
		From("identity.sessions").
		Where(sq.Eq{"token_hash": tokenHash}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return session.Session{}, err
	}

	var (
		id, tid, uid         uuid.UUID
		hash                 []byte
		createdAt, expiresAt time.Time
		revokedAt            *time.Time
	)
	err = r.pool.QueryRow(ctx, sql, args...).Scan(&id, &tid, &uid, &hash, &createdAt, &expiresAt, &revokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return session.Session{}, session.ErrSessionNotFound
		}
		return session.Session{}, err
	}
	return session.Hydrate(id, tenant.NewID(tid), uid, hash, createdAt, expiresAt, revokedAt), nil
}

// RevokeTx marks the session id as revoked. Idempotent.
func (r *SessionRepository) RevokeTx(ctx context.Context, t user.Tx, ten tenant.ID, id uuid.UUID) error {
	tx, err := txOf(t)
	if err != nil {
		return err
	}
	sql, args, err := sq.
		Update("identity.sessions").
		Set("revoked_at", sq.Expr("COALESCE(revoked_at, now())")).
		Where(sq.Eq{"tenant_id": ten.UUID(), "id": id}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return session.ErrSessionNotFound
	}
	return nil
}
