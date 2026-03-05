package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	pfmhttp "github.com/zambone/pfm-go/internal/adapter/http"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/config"
	"github.com/zambone/pfm-go/internal/platform/database"
	"github.com/zambone/pfm-go/internal/platform/observe"
	pfmdb "github.com/zambone/pfm-go/db"
)

// Version information injected at build via ldflags.
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	var logLevel slog.Level
	if err := logLevel.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		return fmt.Errorf("parse log level %q: %w", cfg.LogLevel, err)
	}
	logger := observe.NewLogger(logLevel, nil)

	slog.SetDefault(logger)
	slog.InfoContext(ctx, message.MsgLoggerReady)
	slog.InfoContext(ctx, message.MsgStartupInfo,
		"version", Version,
		"build_time", BuildTime,
		"git_commit", GitCommit,
		"config", cfg.String(),
		"log_level", logLevel.String(),
	)

	tp, tracerShutdown, err := observe.NewTracerProvider(ctx, cfg, "pfm-go", Version)
	if err != nil {
		return fmt.Errorf("init tracer: %w", err)
	}
	_ = tp
	slog.InfoContext(ctx, message.MsgTracerReady)

	db, err := database.Open(ctx, cfg)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	migrationsFS, err := fs.Sub(pfmdb.Migrations, "migrations")
	if err != nil {
		return fmt.Errorf(message.ErrMigrateSubFS, err)
	}

	if err := database.Migrate(ctx, db, migrationsFS); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	var shuttingDown atomic.Bool

	mux := http.NewServeMux()
	mux.Handle("GET /healthz", pfmhttp.HealthHandler(Version))
	mux.Handle("GET /health/live", pfmhttp.LiveHandler())
	mux.Handle("GET /health/ready", pfmhttp.ReadyHandler(&shuttingDown))

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.ErrorContext(ctx, message.MsgServerError, "error", err)
		}
	}()
	slog.InfoContext(ctx, message.MsgServerStarting, "port", cfg.HTTPPort)

	<-ctx.Done()
	shuttingDown.Store(true)
	slog.InfoContext(context.Background(), message.MsgShuttingDown)

	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		time.Duration(cfg.ShutdownTimeoutSec)*time.Second,
	)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.ErrorContext(shutdownCtx, message.MsgServerShutdownError, "error", err)
	}

	if err := db.Close(); err != nil {
		slog.ErrorContext(shutdownCtx, message.MsgDBCloseError, "error", err)
	}
	slog.InfoContext(context.Background(), message.MsgDBClosed)

	slog.InfoContext(context.Background(), message.MsgServerStopped)

	if err := tracerShutdown(shutdownCtx); err != nil {
		slog.ErrorContext(shutdownCtx, message.MsgTracerShutdownError, "error", err)
	}

	return nil
}
