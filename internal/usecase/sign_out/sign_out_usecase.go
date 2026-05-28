package sign_out

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/dev1klas/1klas-identity/internal/domain"
	"github.com/dev1klas/1klas-identity/internal/domain/clock"
	"github.com/dev1klas/1klas-identity/internal/domain/outbox"
	"github.com/dev1klas/1klas-identity/internal/domain/session"
)

// UseCase revokes the current session and writes an outbox event.
type UseCase struct {
	uow      domain.UnitOfWork
	sessions session.Repository
	outbox   outbox.Repository
	cache    session.Cache
	clock    clock.Clock
	logger   *slog.Logger
}

// New constructs the use case.
//
// cache is the write-through session cache. After Postgres commit the cache
// entry is deleted (best-effort: a failure here is a logged warning, not
// fatal — the cache entry will expire naturally).
//
// logger MUST be non-nil; tests can pass slog.New(slog.NewJSONHandler(io.Discard, nil)).
func New(
	uow domain.UnitOfWork,
	sessions session.Repository,
	outboxRepo outbox.Repository,
	cache session.Cache,
	clk clock.Clock,
	logger *slog.Logger,
) *UseCase {
	return &UseCase{
		uow:      uow,
		sessions: sessions,
		outbox:   outboxRepo,
		cache:    cache,
		clock:    clk,
		logger:   logger,
	}
}

// Execute revokes the session and writes the outbox event. Idempotent.
func (uc *UseCase) Execute(ctx context.Context, in Input) (Output, error) {
	if in.TenantID.IsZero() || in.SessionID == uuid.Nil {
		return Output{}, ErrInternal
	}

	now := uc.clock.Now()

	tx, err := uc.uow.Begin(ctx)
	if err != nil {
		return Output{}, ErrInternal
	}
	committed := false
	defer func() {
		if !committed {
			_ = uc.uow.Rollback(ctx, tx)
		}
	}()

	if err := uc.sessions.RevokeTx(ctx, tx, in.TenantID, in.SessionID); err != nil {
		return Output{}, ErrInternal
	}

	payload, err := json.Marshal(struct {
		SessionID uuid.UUID `json:"session_id"`
		UserID    uuid.UUID `json:"user_id"`
		TenantID  string    `json:"tenant_id"`
		RevokedAt time.Time `json:"revoked_at"`
	}{
		SessionID: in.SessionID,
		UserID:    in.UserID,
		TenantID:  in.TenantID.String(),
		RevokedAt: now,
	})
	if err != nil {
		return Output{}, ErrInternal
	}

	if err := uc.outbox.WriteTx(ctx, tx,
		outbox.New(uuid.New(), in.TenantID, "session", in.SessionID, outbox.TopicSessionRevoked, payload, now),
	); err != nil {
		return Output{}, ErrInternal
	}

	if err := uc.uow.Commit(ctx, tx); err != nil {
		return Output{}, ErrInternal
	}
	committed = true

	// Invalidate the write-through session cache. Best-effort: cache misses
	// will fall through to Postgres which now reflects the revocation.
	if uc.cache != nil && in.TokenHashHex != "" {
		if err := uc.cache.Delete(ctx, in.TokenHashHex); err != nil {
			uc.logger.WarnContext(ctx, "session cache delete failed after sign-out",
				slog.String("session_id", in.SessionID.String()),
				slog.String("err", err.Error()),
			)
		}
	}

	return Output{}, nil
}
