package sign_in

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/dev1klas/1klas-identity/internal/domain"
	"github.com/dev1klas/1klas-identity/internal/domain/clock"
	"github.com/dev1klas/1klas-identity/internal/domain/outbox"
	"github.com/dev1klas/1klas-identity/internal/domain/session"
	"github.com/dev1klas/1klas-identity/internal/domain/user"
)

// dummyHash is a pre-computed argon2id hash of an arbitrary password. It is
// used to make sign-in attempts on unknown emails run the KDF, equalising the
// timing profile vs known-email-wrong-password.
//
// Generated once at process start by the constructor.
type dummyState struct {
	hash user.PasswordHash
}

// UseCase orchestrates session issuance after credential verification.
type UseCase struct {
	uow        domain.UnitOfWork
	users      user.Repository
	sessions   session.Repository
	outbox     outbox.Repository
	cache      session.Cache
	hasher     user.PasswordHasher
	tokens     session.TokenGenerator
	clock      clock.Clock
	sessionTTL time.Duration
	dummy      dummyState
	logger     *slog.Logger
}

// New constructs the use case. The dummy hash is generated here for timing
// safety on unknown-email paths.
//
// cache is the write-through session cache (Postgres remains source of truth).
// On a cache write failure the use case logs WARN and proceeds; the next
// SessionAuth miss will repopulate via Postgres.
//
// logger is used to surface unexpected dummy-KDF verify failures (which would
// signal hasher misconfiguration); it MUST be non-nil. Callers in tests can
// pass slog.New(slog.NewJSONHandler(io.Discard, nil)).
func New(
	ctx context.Context,
	uow domain.UnitOfWork,
	users user.Repository,
	sessions session.Repository,
	outboxRepo outbox.Repository,
	cache session.Cache,
	hasher user.PasswordHasher,
	tokens session.TokenGenerator,
	clk clock.Clock,
	sessionTTL time.Duration,
	logger *slog.Logger,
) (*UseCase, error) {
	dh, err := hasher.Hash(ctx, "dummy-password-for-timing-safety-only")
	if err != nil {
		return nil, ErrInternal
	}
	return &UseCase{
		uow:        uow,
		users:      users,
		sessions:   sessions,
		outbox:     outboxRepo,
		cache:      cache,
		hasher:     hasher,
		tokens:     tokens,
		clock:      clk,
		sessionTTL: sessionTTL,
		dummy:      dummyState{hash: dh},
		logger:     logger,
	}, nil
}

// Execute runs the use case.
func (uc *UseCase) Execute(ctx context.Context, in Input) (Output, error) {
	if in.TenantID.IsZero() {
		return Output{}, ErrInternal
	}

	email, err := user.NewEmail(in.Email)
	if err != nil {
		// Still burn an argon verify to keep timing roughly equal. The result
		// is intentionally discarded — only an error here indicates hasher
		// misconfiguration, which we surface at DEBUG for ops visibility.
		if _, verr := uc.hasher.Verify(ctx, uc.dummy.hash, in.Password); verr != nil {
			uc.logger.DebugContext(ctx, "dummy hasher verify failed", "err", verr.Error())
		}
		return Output{}, ErrInvalidCredentials
	}

	u, err := uc.users.FindByEmail(ctx, in.TenantID, email)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			if _, verr := uc.hasher.Verify(ctx, uc.dummy.hash, in.Password); verr != nil {
				uc.logger.DebugContext(ctx, "dummy hasher verify failed", "err", verr.Error())
			}
			return Output{}, ErrInvalidCredentials
		}
		return Output{}, ErrInternal
	}

	ok, err := uc.hasher.Verify(ctx, u.PasswordHash(), in.Password)
	if err != nil {
		return Output{}, ErrInternal
	}
	if !ok {
		return Output{}, ErrInvalidCredentials
	}

	tok, err := uc.tokens.NewToken()
	if err != nil {
		return Output{}, ErrInternal
	}

	now := uc.clock.Now()
	sessionID := uuid.New()
	expiresAt := now.Add(uc.sessionTTL)
	sess := session.New(sessionID, in.TenantID, u.ID(), tok.Hash(), now, expiresAt)

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

	if err := uc.sessions.SaveTx(ctx, tx, sess); err != nil {
		return Output{}, ErrInternal
	}

	payload, err := json.Marshal(struct {
		SessionID uuid.UUID `json:"session_id"`
		UserID    uuid.UUID `json:"user_id"`
		TenantID  string    `json:"tenant_id"`
		CreatedAt time.Time `json:"created_at"`
	}{
		SessionID: sessionID,
		UserID:    u.ID(),
		TenantID:  in.TenantID.String(),
		CreatedAt: now,
	})
	if err != nil {
		return Output{}, ErrInternal
	}

	if err := uc.outbox.WriteTx(ctx, tx,
		outbox.New(uuid.New(), in.TenantID, "session", sessionID, outbox.TopicSessionCreated, payload, now),
	); err != nil {
		return Output{}, ErrInternal
	}

	if err := uc.uow.Commit(ctx, tx); err != nil {
		return Output{}, ErrInternal
	}
	committed = true

	// Write-through into the session cache. Postgres is the source of truth;
	// a failure here is a logged warning, never a fatal — the next
	// SessionAuth miss will repopulate via Postgres.
	if uc.cache != nil {
		tokenHashHex := hex.EncodeToString(tok.Hash())
		payload := session.CachedSession{
			SessionID: sessionID,
			UserID:    u.ID(),
			TenantID:  in.TenantID,
			ExpiresAt: expiresAt,
		}
		// TTL uses time.Until(expiresAt) so the value is identical to the
		// SessionAuth re-populate path on a Postgres fallthrough.
		if err := uc.cache.Set(ctx, tokenHashHex, payload, time.Until(expiresAt)); err != nil {
			uc.logger.WarnContext(ctx, "session cache write failed after sign-in",
				slog.String("session_id", sessionID.String()),
				slog.String("err", err.Error()),
			)
		}
	}

	return Output{
		UserID:           u.ID(),
		SessionToken:     tok.String(),
		SessionExpiresAt: expiresAt,
	}, nil
}
