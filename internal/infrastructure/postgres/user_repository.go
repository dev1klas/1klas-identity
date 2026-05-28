package postgres

import (
	"context"
	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
	"github.com/dev1klas/1klas-identity/internal/domain/user"
)

// UserRepository implements user.Repository against Postgres.
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository constructs the repository.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

const uniqueViolationCode = "23505"

// SaveTx inserts a fresh user inside tx. Maps unique-violation to ErrEmailTaken.
func (r *UserRepository) SaveTx(ctx context.Context, t user.Tx, u user.User) error {
	tx, err := txOf(t)
	if err != nil {
		return err
	}
	sql, args, err := sq.
		Insert("identity.users").
		Columns("id", "tenant_id", "email", "password_hash", "status", "created_at", "updated_at").
		Values(u.ID(), u.TenantID().UUID(), u.Email().String(), u.PasswordHash().String(), string(u.Status()), u.CreatedAt(), u.UpdatedAt()).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, sql, args...); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
			return user.ErrEmailTaken
		}
		return err
	}
	return nil
}

// FindByEmail looks up by (tenant_id, email).
func (r *UserRepository) FindByEmail(ctx context.Context, t tenant.ID, email user.Email) (user.User, error) {
	sql, args, err := sq.
		Select("id", "tenant_id", "email", "password_hash", "status", "created_at", "updated_at").
		From("identity.users").
		Where(sq.Eq{"tenant_id": t.UUID(), "email": email.String()}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return user.User{}, err
	}
	return r.scanOne(ctx, sql, args)
}

// FindByID looks up by (tenant_id, id).
func (r *UserRepository) FindByID(ctx context.Context, t tenant.ID, id uuid.UUID) (user.User, error) {
	sql, args, err := sq.
		Select("id", "tenant_id", "email", "password_hash", "status", "created_at", "updated_at").
		From("identity.users").
		Where(sq.Eq{"tenant_id": t.UUID(), "id": id}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return user.User{}, err
	}
	return r.scanOne(ctx, sql, args)
}

func (r *UserRepository) scanOne(ctx context.Context, sql string, args []interface{}) (user.User, error) {
	var (
		id, tid                            uuid.UUID
		emailStr, hashStr, statusStr       string
		createdAt, updatedAt               = newTimes()
	)
	err := r.pool.QueryRow(ctx, sql, args...).Scan(&id, &tid, &emailStr, &hashStr, &statusStr, createdAt, updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return user.User{}, user.ErrUserNotFound
		}
		return user.User{}, err
	}
	em, err := user.NewEmail(emailStr)
	if err != nil {
		return user.User{}, err
	}
	return user.Hydrate(
		id,
		tenant.NewID(tid),
		em,
		user.NewPasswordHash(hashStr),
		user.Status(statusStr),
		*createdAt,
		*updatedAt,
	), nil
}
