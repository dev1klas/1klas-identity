package sign_up

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/dev1klas/1klas-identity/internal/domain"
	"github.com/dev1klas/1klas-identity/internal/domain/clock"
	"github.com/dev1klas/1klas-identity/internal/domain/outbox"
	"github.com/dev1klas/1klas-identity/internal/domain/session"
	"github.com/dev1klas/1klas-identity/internal/domain/user"
)

// UseCase orchestrates user creation and initial session issuance inside a
// single database transaction.
type UseCase struct {
	uow          domain.UnitOfWork
	users        user.Repository
	sessions     session.Repository
	outbox       outbox.Repository
	hasher       user.PasswordHasher
	tokens       session.TokenGenerator
	clock        clock.Clock
	sessionTTL   time.Duration
}

// New constructs the use case.
func New(
	uow domain.UnitOfWork,
	users user.Repository,
	sessions session.Repository,
	outboxRepo outbox.Repository,
	hasher user.PasswordHasher,
	tokens session.TokenGenerator,
	clk clock.Clock,
	sessionTTL time.Duration,
) *UseCase {
	return &UseCase{
		uow:        uow,
		users:      users,
		sessions:   sessions,
		outbox:     outboxRepo,
		hasher:     hasher,
		tokens:     tokens,
		clock:      clk,
		sessionTTL: sessionTTL,
	}
}

// Execute runs the use case.
func (uc *UseCase) Execute(ctx context.Context, in Input) (Output, error) {
	if in.TenantID.IsZero() {
		return Output{}, ErrInternal
	}

	email, err := user.NewEmail(in.Email)
	if err != nil {
		return Output{}, ErrInvalidEmail
	}
	if err := user.ValidatePasswordPolicy(in.Password); err != nil {
		return Output{}, ErrWeakPassword
	}

	hash, err := uc.hasher.Hash(ctx, in.Password)
	if err != nil {
		return Output{}, ErrInternal
	}

	tok, err := uc.tokens.NewToken()
	if err != nil {
		return Output{}, ErrInternal
	}

	now := uc.clock.Now()
	userID := uuid.New()
	u := user.New(userID, in.TenantID, email, hash, now)

	sessionID := uuid.New()
	expiresAt := now.Add(uc.sessionTTL)
	sess := session.New(sessionID, in.TenantID, userID, tok.Hash(), now, expiresAt)

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

	if err := uc.users.SaveTx(ctx, tx, u); err != nil {
		if errors.Is(err, user.ErrEmailTaken) {
			return Output{}, ErrEmailTaken
		}
		return Output{}, ErrInternal
	}

	if err := uc.sessions.SaveTx(ctx, tx, sess); err != nil {
		return Output{}, ErrInternal
	}

	userPayload, err := json.Marshal(struct {
		UserID    uuid.UUID `json:"user_id"`
		TenantID  string    `json:"tenant_id"`
		Email     string    `json:"email"`
		CreatedAt time.Time `json:"created_at"`
	}{
		UserID:    userID,
		TenantID:  in.TenantID.String(),
		Email:     email.String(),
		CreatedAt: now,
	})
	if err != nil {
		return Output{}, ErrInternal
	}

	if err := uc.outbox.WriteTx(ctx, tx,
		outbox.New(uuid.New(), in.TenantID, "user", userID, outbox.TopicUserCreated, userPayload, now),
	); err != nil {
		return Output{}, ErrInternal
	}

	sessionPayload, err := json.Marshal(struct {
		SessionID uuid.UUID `json:"session_id"`
		UserID    uuid.UUID `json:"user_id"`
		TenantID  string    `json:"tenant_id"`
		CreatedAt time.Time `json:"created_at"`
	}{
		SessionID: sessionID,
		UserID:    userID,
		TenantID:  in.TenantID.String(),
		CreatedAt: now,
	})
	if err != nil {
		return Output{}, ErrInternal
	}

	if err := uc.outbox.WriteTx(ctx, tx,
		outbox.New(uuid.New(), in.TenantID, "session", sessionID, outbox.TopicSessionCreated, sessionPayload, now),
	); err != nil {
		return Output{}, ErrInternal
	}

	if err := uc.uow.Commit(ctx, tx); err != nil {
		return Output{}, ErrInternal
	}
	committed = true

	return Output{
		UserID:           userID,
		SessionToken:     tok.String(),
		SessionExpiresAt: expiresAt,
	}, nil
}
