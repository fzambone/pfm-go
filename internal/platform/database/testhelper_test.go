//go:build integration

package database

import (
	"context"
	"database/sql"
	"io/fs"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	pfmdb "github.com/zambone/pfm-go/db"
	"github.com/zambone/pfm-go/internal/platform/config"
)

// newTestDb spins up a real Postgres container and returns an open *sql.DB.
// Each call creates an isolated container - tests never share state.
func newTestDB(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()

	ctr, err := postgres.Run(ctx,
		"postgres:18-alpine3.23",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	host, err := ctr.Host(ctx)
	require.NoError(t, err)
	mappedPort, err := ctr.MappedPort(ctx, "5432")
	require.NoError(t, err)

	cfg := &config.Config{
		DatabaseHost:           host,
		DatabasePort:           mappedPort.Int(),
		DatabaseName:           "testdb",
		DatabaseUser:           "testuser",
		DatabasePassword:       "testpass",
		DatabaseSSLMode:        "disable",
		DBConnectTimeoutSec:    5,
		DBStartupRetries:       3,
		DBStartupRetryDelaySec: 1,
		DBMaxOpenConns:         5,
		DBMaxIdleConns:         2,
		DBConnMaxLifetimeSec:   60,
		DBConnMaxIdleTimeSec:   30,
	}

	db, err := Open(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	return db
}

// newTestPool spins up a real Postgres container, runs all migrations, and returns
// a *pgxpool.Pool connected to it. Used by transactor integration tests.
// Each call creates an isolated container — tests never share state.
func newTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	ctr, err := postgres.Run(ctx,
		"postgres:18-alpine3.23",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	host, err := ctr.Host(ctx)
	require.NoError(t, err)
	mappedPort, err := ctr.MappedPort(ctx, "5432")
	require.NoError(t, err)

	cfg := &config.Config{
		DatabaseHost:           host,
		DatabasePort:           mappedPort.Int(),
		DatabaseName:           "testdb",
		DatabaseUser:           "testuser",
		DatabasePassword:       "testpass",
		DatabaseSSLMode:        "disable",
		DBConnectTimeoutSec:    5,
		DBStartupRetries:       3,
		DBStartupRetryDelaySec: 1,
		DBMaxOpenConns:         5,
		DBMaxIdleConns:         2,
		DBConnMaxLifetimeSec:   60,
		DBConnMaxIdleTimeSec:   30,
	}

	// Run migrations using the database/sql adapter (required by goose).
	sqlDB, err := Open(ctx, cfg)
	require.NoError(t, err)
	sub, err := fs.Sub(pfmdb.Migrations, "migrations")
	require.NoError(t, err)
	require.NoError(t, Migrate(ctx, sqlDB, sub))
	require.NoError(t, sqlDB.Close())

	// Open the pgx-native pool for application use.
	pool, err := NewPool(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	return pool
}
