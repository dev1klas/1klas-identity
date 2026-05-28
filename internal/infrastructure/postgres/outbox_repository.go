package postgres

import (
	"context"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dev1klas/1klas-identity/internal/domain/outbox"
	"github.com/dev1klas/1klas-identity/internal/domain/user"
)

// OutboxRepository implements outbox.Repository against Postgres.
type OutboxRepository struct {
	pool *pgxpool.Pool
}

// NewOutboxRepository constructs the repository. The pool is currently unused
// (all writes go via tx) but kept for future query/drain methods.
func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool}
}

// WriteTx inserts an outbox event inside tx.
func (r *OutboxRepository) WriteTx(ctx context.Context, t user.Tx, ev outbox.Event) error {
	tx, err := txOf(t)
	if err != nil {
		return err
	}
	sql, args, err := sq.
		Insert("identity.outbox_events").
		Columns("id", "tenant_id", "aggregate_type", "aggregate_id", "event_type", "payload", "created_at").
		Values(ev.ID(), ev.TenantID().UUID(), ev.AggregateType(), ev.AggregateID(), ev.EventType(), ev.Payload(), ev.CreatedAt()).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, sql, args...)
	return err
}
