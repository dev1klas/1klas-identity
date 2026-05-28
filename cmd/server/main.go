package main

import (
	"context"
	"errors"
	"log/slog"
	stdhttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dev1klas/1klas-identity/internal/config"
	"github.com/dev1klas/1klas-identity/internal/domain/clock"
	"github.com/dev1klas/1klas-identity/internal/infrastructure/argon2id"
	"github.com/dev1klas/1klas-identity/internal/infrastructure/postgres"
	"github.com/dev1klas/1klas-identity/internal/infrastructure/tokens"
	"github.com/dev1klas/1klas-identity/internal/observability"
	transport "github.com/dev1klas/1klas-identity/internal/transport/http"
	"github.com/dev1klas/1klas-identity/internal/transport/http/cookies"
	"github.com/dev1klas/1klas-identity/internal/transport/http/middleware"
	"github.com/dev1klas/1klas-identity/internal/usecase/get_me"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_in"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_out"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_up"
)

const shutdownGrace = 10 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "identity")

	observability.InitTracing()

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config load failed", "error", err.Error())
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.PostgresURL)
	if err != nil {
		logger.Error("pgx pool init failed", "error", err.Error())
		os.Exit(1)
	}
	defer pool.Close()

	migCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := postgres.Migrate(migCtx, pool); err != nil {
		logger.Error("migrations failed", "error", err.Error())
		os.Exit(1)
	}

	// Wiring.
	uow := postgres.NewUnitOfWork(pool)
	userRepo := postgres.NewUserRepository(pool)
	sessionRepo := postgres.NewSessionRepository(pool)
	outboxRepo := postgres.NewOutboxRepository(pool)

	hasher := argon2id.New(argon2id.Params{
		MemoryKiB:   cfg.Argon2MemoryKiB,
		Time:        cfg.Argon2Time,
		Parallelism: cfg.Argon2Parallel,
		SaltLen:     16,
		KeyLen:      32,
	})
	tokGen := tokens.New()
	clk := clock.Real{}

	signUpUC := sign_up.New(uow, userRepo, sessionRepo, outboxRepo, hasher, tokGen, clk, cfg.SessionTTL)
	signInUC, err := sign_in.New(ctx, uow, userRepo, sessionRepo, outboxRepo, hasher, tokGen, clk, cfg.SessionTTL, logger)
	if err != nil {
		logger.Error("sign_in init failed", "error", err.Error())
		os.Exit(1)
	}
	signOutUC := sign_out.New(uow, sessionRepo, outboxRepo, clk)
	getMeUC := get_me.New(userRepo)

	cookieCfg := cookies.Config{Secure: cfg.CookieSecure}

	mux := transport.NewMux(transport.Deps{
		SignUp:    signUpUC,
		SignIn:    signInUC,
		SignOut:   signOutUC,
		GetMe:     getMeUC,
		Sessions:  sessionRepo,
		Cookie:    cookieCfg,
		AccessLog: middleware.AccessLog(logger),
	})

	srv := &stdhttp.Server{
		Addr:         cfg.Addr,
		Handler:      mux,
		ReadTimeout:  35 * time.Second,
		WriteTimeout: 35 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("identity listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
			logger.Error("server error", "error", err.Error())
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err.Error())
		os.Exit(1)
	}
}
