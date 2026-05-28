package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valyala/fasthttp"

	"github.com/dev1klas/1klas-identity/internal/config"
	"github.com/dev1klas/1klas-identity/internal/domain/clock"
	"github.com/dev1klas/1klas-identity/internal/infrastructure/argon2id"
	"github.com/dev1klas/1klas-identity/internal/infrastructure/postgres"
	"github.com/dev1klas/1klas-identity/internal/infrastructure/tokens"
	"github.com/dev1klas/1klas-identity/internal/infrastructure/valkey"
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

	if cfg.RunMigrationsOnBoot {
		// Optional convenience for local docker-compose where running a
		// separate migrate binary is friction. Production applies
		// migrations via the cmd/migrate pre-deploy job and leaves this
		// flag false so multiple server instances do not race on boot.
		migCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		if err := postgres.Migrate(migCtx, pool); err != nil {
			logger.Error("migrations failed", "error", err.Error())
			os.Exit(1)
		}
	}

	// Valkey session cache. Fail-fast PING — a Valkey outage at boot is a
	// configuration error, not a runtime degradation.
	cache, err := valkey.New(valkey.Config{
		URL:         cfg.ValkeyURL,
		DialTimeout: cfg.ValkeyDialTimeout,
		OpTimeout:   cfg.ValkeyOpTimeout,
	})
	if err != nil {
		logger.Error("valkey init failed", "error", err.Error())
		os.Exit(1)
	}
	defer func() { _ = cache.Close() }()

	pingCtx, pingCancel := context.WithTimeout(ctx, 3*time.Second)
	if err := cache.Ping(pingCtx); err != nil {
		pingCancel()
		logger.Error("valkey ping failed", "error", err.Error())
		os.Exit(1)
	}
	pingCancel()

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

	signUpUC := sign_up.New(uow, userRepo, sessionRepo, outboxRepo, cache, hasher, tokGen, clk, cfg.SessionTTL, logger)
	signInUC, err := sign_in.New(ctx, uow, userRepo, sessionRepo, outboxRepo, cache, hasher, tokGen, clk, cfg.SessionTTL, logger)
	if err != nil {
		logger.Error("sign_in init failed", "error", err.Error())
		os.Exit(1)
	}
	signOutUC := sign_out.New(uow, sessionRepo, outboxRepo, cache, clk, logger)
	getMeUC := get_me.New(userRepo)

	cookieCfg := cookies.Config{Secure: cfg.CookieSecure}

	handler := transport.NewHandler(transport.Deps{
		SignUp:    signUpUC,
		SignIn:    signInUC,
		SignOut:   signOutUC,
		GetMe:     getMeUC,
		Sessions:  sessionRepo,
		Cache:     cache,
		Cookie:    cookieCfg,
		Recover:   middleware.Recover(logger),
		AccessLog: middleware.AccessLog(logger),
		Origin:    middleware.OriginCheck(logger, cfg.AllowedOrigins),
		OTel:      middleware.OTelTrace,
		SessionMW: middleware.Session(sessionRepo, cache, logger),
	})

	srv := &fasthttp.Server{
		Handler:            handler,
		Name:               "1klas-identity",
		ReadTimeout:        35 * time.Second,
		WriteTimeout:       35 * time.Second,
		IdleTimeout:        120 * time.Second,
		MaxRequestBodySize: 1 << 20, // 1 MiB ceiling; per-handler caps apply at the handler.
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("identity listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(cfg.Addr); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down")
	case err := <-errCh:
		if err != nil {
			logger.Error("server error", "error", err.Error())
			os.Exit(1)
		}
	}

	shutCtx, shutCancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer shutCancel()
	if err := srv.ShutdownWithContext(shutCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		logger.Error("graceful shutdown failed", "error", err.Error())
		os.Exit(1)
	}
}
