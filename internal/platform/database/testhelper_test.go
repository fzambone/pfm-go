//go:build integration

package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// newTestDB creates an isolated per-test database (no migrations) cloned from
// the admin connection. The caller is expected to run migrations inside the test.
// The database is dropped in t.Cleanup.
func newTestDB(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()

	dbName := "test_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	conn := dbAdminConn(ctx)
	dbAdminExec(ctx, conn, fmt.Sprintf(`CREATE DATABASE %s`, dbName))
	conn.Close(ctx)

	t.Cleanup(func() {
		dropCtx := context.Background()
		dropConn := dbAdminConn(dropCtx)
		defer dropConn.Close(dropCtx)
		_, _ = dropConn.Exec(dropCtx, // best-effort
			`SELECT pg_terminate_backend(pid) FROM pg_stat_activity `+
				`WHERE datname = $1 AND pid <> pg_backend_pid()`,
			dbName,
		)
		_, _ = dropConn.Exec(dropCtx, fmt.Sprintf(`DROP DATABASE IF EXISTS %s`, dbName)) // best-effort
	})

	db, err := Open(ctx, dbURLToConfig(dbReplaceDBName(dbAdminURL, dbName)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() }) // best-effort

	return db
}

// newTestPool creates an isolated per-test database cloned from pfm_template
// (which has all migrations applied) and returns a *pgxpool.Pool connected to it.
// The database is dropped in t.Cleanup.
func newTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	dbName := "test_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	conn := dbAdminConn(ctx)
	dbAdminExec(ctx, conn, fmt.Sprintf(`CREATE DATABASE %s TEMPLATE %s`, dbName, dbTemplateDB))
	conn.Close(ctx)

	t.Cleanup(func() {
		dropCtx := context.Background()
		dropConn := dbAdminConn(dropCtx)
		defer dropConn.Close(dropCtx)
		_, _ = dropConn.Exec(dropCtx, // best-effort
			`SELECT pg_terminate_backend(pid) FROM pg_stat_activity `+
				`WHERE datname = $1 AND pid <> pg_backend_pid()`,
			dbName,
		)
		_, _ = dropConn.Exec(dropCtx, fmt.Sprintf(`DROP DATABASE IF EXISTS %s`, dbName)) // best-effort
	})

	pool, err := NewPool(ctx, dbURLToConfig(dbReplaceDBName(dbAdminURL, dbName)))
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return pool
}
