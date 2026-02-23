package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/config"
)

// pinger is satisfied by *sql.DB and allows retry logic to be tested without real database.
type pinger interface {
	PingContext(ctx context.Context) error
}

func buildDSN(cfg *config.Config) string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s&connect_timeout=%d",
		cfg.DatabaseUser,
		cfg.DatabasePassword,
		cfg.DatabaseHost,
		cfg.DatabasePort,
		cfg.DatabaseName,
		cfg.DatabaseSSLMode,
		cfg.DBConnectTimeoutSec,
	)
}

// pingWithRetry pings the database with retry attempts, using the provided context and returns nil if the database is reachable.
func pingWithRetry(ctx context.Context, p pinger, cfg *config.Config) error {
	var lastErr error
	for i := 0; i < cfg.DBStartupRetries; i++ {
		if ctx.Err() != nil {
			return fmt.Errorf(message.ErrDBContextDone, ctx.Err())
		}
		lastErr = p.PingContext(ctx)
		if lastErr == nil {
			return nil
		}
		slog.WarnContext(ctx, message.MsgDBPingRetry, "attempt", i+1, "error", lastErr)
		time.Sleep(time.Duration(cfg.DBStartupRetryDelaySec) * time.Second)
	}
	return fmt.Errorf(message.ErrDBPing, cfg.DBStartupRetries, lastErr)
}

// Open creates, configures, and validates a PostgreSQL connection pool.
// It retries the initial ping according to cfg.DBStartupRetries before returning.
func Open(ctx context.Context, cfg *config.Config) (*sql.DB, error) {
	slog.InfoContext(ctx, message.MsgDBConnecting)

	db, err := sql.Open("pgx", buildDSN(cfg))
	if err != nil {
		return nil, fmt.Errorf(message.ErrDBOpen, err)
	}

	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.DBConnMaxLifetimeSec) * time.Second)
	db.SetConnMaxIdleTime(time.Duration(cfg.DBConnMaxIdleTimeSec) * time.Second)

	if err := pingWithRetry(ctx, db, cfg); err != nil {
		return nil, fmt.Errorf("verify connection: %w", err)
	}

	slog.InfoContext(ctx, message.MsgDBReady)

	return db, nil
}
