package sign_out

import (
	"context"
	"encoding/json"
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
	clock    clock.Clock
}

// New constructs the use case.
func New(
	uow domain.UnitOfWork,
	sessions session.Repository,
	outboxRepo outbox.Repository,
	clk clock.Clock,
) *UseCase {
	return &UseCase{
		uow:      uow,
		sessions: sessions,
		outbox:   outboxRepo,
		clock:    clk,
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

	return Output{}, nil
}
