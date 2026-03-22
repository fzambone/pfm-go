//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	pfmdb "github.com/zambone/pfm-go/db"
	"github.com/zambone/pfm-go/internal/adapter/postgres"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/config"
	"github.com/zambone/pfm-go/internal/platform/database"
)

// testPasswordHash is a valid-length placeholder hash used for insertion fixtures.
var integTestPasswordHash = strings.Repeat("x", 128)

// newTestPool spins up a Postgres container, runs all migrations, and returns
// a *pgxpool.Pool. Each call creates an isolated container — tests never share state.
func newTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	ctr, err := tcpostgres.Run(ctx,
		"postgres:18-alpine3.23",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpass"),
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

	// Run migrations via database/sql (goose requirement).
	sqlDB, err := database.Open(ctx, cfg)
	require.NoError(t, err)
	sub, err := fs.Sub(pfmdb.Migrations, "migrations")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(ctx, sqlDB, sub))
	require.NoError(t, sqlDB.Close())

	// Re-connect with the pgx-native pool for application queries.
	pool, err := database.NewPool(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	return pool
}

// insertUser is a test helper that inserts a user row directly via SQL.
func insertUser(t *testing.T, pool *pgxpool.Pool, email, passwordHash string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		"INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3)",
		email, passwordHash, "Test User",
	)
	require.NoError(t, err)
}

// TestUserRepo_FindByEmail_ReturnsUser verifies the happy path:
// an existing active user is found and all fields are populated.
func TestUserRepo_FindByEmail_ReturnsUser(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewUserRepo(pool)

	insertUser(t, pool, "alice@example.com", integTestPasswordHash)

	u, err := repo.FindByEmail(ctx, "alice@example.com")

	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", u.Email)
	assert.Equal(t, integTestPasswordHash, u.PasswordHash)
	assert.NotEmpty(t, u.ID)
}

// TestUserRepo_FindByEmail_CaseInsensitive verifies that email lookup ignores case:
// "ALICE@EXAMPLE.COM" matches the record stored as "alice@example.com".
func TestUserRepo_FindByEmail_CaseInsensitive(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewUserRepo(pool)

	insertUser(t, pool, "alice@example.com", integTestPasswordHash)

	u, err := repo.FindByEmail(ctx, "ALICE@EXAMPLE.COM")

	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", u.Email)
}

// TestUserRepo_FindByEmail_NotFound_ReturnsInvalidCredentials verifies that
// a missing email returns an error wrapping ErrLoginInvalidCredentials.
func TestUserRepo_FindByEmail_NotFound_ReturnsInvalidCredentials(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewUserRepo(pool)

	_, err := repo.FindByEmail(ctx, "nobody@example.com")

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrLoginInvalidCredentials),
		"expected ErrLoginInvalidCredentials, got: %v", err)
}

// TestUserRepo_FindByEmail_SoftDeleted_ReturnsInvalidCredentials verifies AC4:
// a soft-deleted user is invisible to FindByEmail (deleted_at IS NULL in SQL).
func TestUserRepo_FindByEmail_SoftDeleted_ReturnsInvalidCredentials(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewUserRepo(pool)

	insertUser(t, pool, "deleted@example.com", integTestPasswordHash)
	_, err := pool.Exec(ctx,
		"UPDATE users SET deleted_at = NOW() WHERE email = $1",
		"deleted@example.com",
	)
	require.NoError(t, err)

	_, err = repo.FindByEmail(ctx, "deleted@example.com")

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrLoginInvalidCredentials),
		"soft-deleted user must return ErrLoginInvalidCredentials, got: %v", err)
}
