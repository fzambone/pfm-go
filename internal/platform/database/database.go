package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jackc/pgx/v5/pgxpool"

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
		return nil, fmt.Errorf(message.ErrDBVerifyConn, err)
	}

	slog.InfoContext(ctx, message.MsgDBReady)

	return db, nil
}

// NewPool creates a pgx-native connection pool for application queries.
// Unlike Open, which uses the database/sql adapter for goose migrations,
// NewPool returns a *pgxpool.Pool suitable for use with pgx-native queries
// and the PostgresTransactor.
func NewPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	slog.InfoContext(ctx, message.MsgDBConnecting)

	poolCfg, err := pgxpool.ParseConfig(buildDSN(cfg))
	if err != nil {
		return nil, fmt.Errorf(message.ErrDBParsePoolConfig, err)
	}

	poolCfg.MaxConns = int32(cfg.DBMaxOpenConns)
	poolCfg.MinConns = int32(cfg.DBMaxIdleConns)
	poolCfg.MaxConnLifetime = time.Duration(cfg.DBConnMaxLifetimeSec) * time.Second
	poolCfg.MaxConnIdleTime = time.Duration(cfg.DBConnMaxIdleTimeSec) * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf(message.ErrDBNewPool, err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf(message.ErrDBVerifyConn, err)
	}

	slog.InfoContext(ctx, message.MsgDBReady)

	return pool, nil
}
