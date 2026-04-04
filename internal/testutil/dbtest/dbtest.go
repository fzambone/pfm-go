//go:build integration

// Package dbtest provides shared PostgreSQL infrastructure for integration tests
// in external test packages (package xxx_test). It manages a single Postgres
// connection per test binary — either a GitHub Actions service container (when
// TEST_DATABASE_URL is set) or a testcontainer for local development — and
// creates per-test isolated databases cloned from a migrated template.
//
// Import cycle note: internal/platform/database tests (package database, internal)
// cannot import this package without creating an import cycle. Those packages
// replicate the same pattern inline.
package dbtest

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	pfmdb "github.com/zambone/pfm-go/db"
	"github.com/zambone/pfm-go/internal/platform/config"
	"github.com/zambone/pfm-go/internal/platform/database"
)

const templateDB = "pfm_template"

// SharedDB holds the admin connection URL (pointing to the postgres system
// database) used to issue CREATE DATABASE / DROP DATABASE DDL commands.
// NewPool is safe to call concurrently from parallel tests.
type SharedDB struct {
	// adminURL connects to the "postgres" system database as a superuser,
	// used exclusively for DDL (CREATE DATABASE, DROP DATABASE).
	adminURL string
}

// Setup returns a SharedDB ready for per-test database creation and a cleanup
// function that terminates any testcontainer started for local development.
//
// If TEST_DATABASE_URL is set (e.g. by the GitHub Actions merge-quality-gate
// workflow), the value is used directly and no container is started.
// Otherwise, a postgres:18-alpine3.23 testcontainer is started.
//
// Call PrepareTemplate after Setup and before m.Run():
//
//	func TestMain(m *testing.M) {
//	    ctx := context.Background()
//	    db, cleanup := dbtest.Setup(ctx)
//	    defer cleanup()
//	    db.PrepareTemplate(ctx)
//	    os.Exit(m.Run())
//	}
func Setup(ctx context.Context) (*SharedDB, func()) {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return &SharedDB{adminURL: url}, func() {}
	}
	return startContainer(ctx)
}

// PrepareTemplate creates the pfm_template database and applies all goose
// migrations to it. After this call, pfm_template has no active connections
// and is ready to be cloned by NewPool.
//
// Must be called once in TestMain, after Setup and before m.Run().
func (s *SharedDB) PrepareTemplate(ctx context.Context) {
	conn := s.adminConn(ctx)

	// Drop and recreate so TestMain is idempotent when a persistent service
	// container is reused across workflow retries.
	mustExec(ctx, conn, fmt.Sprintf(`DROP DATABASE IF EXISTS %s`, templateDB))
	mustExec(ctx, conn, fmt.Sprintf(`CREATE DATABASE %s`, templateDB))
	conn.Close(ctx)

	// Apply all goose migrations via the database/sql adapter.
	templateURL := replaceDBName(s.adminURL, templateDB)
	sqlDB, err := database.Open(ctx, urlToConfig(templateURL))
	if err != nil {
		panic(fmt.Sprintf("dbtest: open template db: %v", err))
	}
	sub, err := fs.Sub(pfmdb.Migrations, "migrations")
	if err != nil {
		panic(fmt.Sprintf("dbtest: sub migrations fs: %v", err))
	}
	if err := database.Migrate(ctx, sqlDB, sub); err != nil {
		panic(fmt.Sprintf("dbtest: migrate template db: %v", err))
	}
	// Close ALL connections to pfm_template. Postgres requires no active
	// sessions on the template database when CREATE DATABASE ... TEMPLATE
	// is executed. This satisfies that requirement before m.Run().
	if err := sqlDB.Close(); err != nil {
		panic(fmt.Sprintf("dbtest: close template db: %v", err))
	}
}

// NewPool creates an isolated per-test database cloned from pfm_template and
// returns a *pgxpool.Pool connected to it. The database is dropped in
// t.Cleanup so each test starts and ends with clean state.
//
// Call t.Parallel() in the test before or after NewPool — both work correctly.
func (s *SharedDB) NewPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	dbName := "test_" + strings.ReplaceAll(uuid.New().String(), "-", "")

	// CREATE DATABASE runs against the admin (postgres) database, never against
	// pfm_template itself, so no session is connected to the template during cloning.
	conn := s.adminConn(ctx)
	mustExec(ctx, conn,
		fmt.Sprintf(`CREATE DATABASE %s TEMPLATE %s`, dbName, templateDB),
	)
	conn.Close(ctx)

	t.Cleanup(func() {
		dropCtx := context.Background()
		dropConn := s.adminConn(dropCtx)
		defer dropConn.Close(dropCtx)
		// Terminate lingering connections before dropping — prevents
		// "database is being accessed by other users" errors.
		_, _ = dropConn.Exec(dropCtx, // best-effort
			`SELECT pg_terminate_backend(pid) FROM pg_stat_activity `+
				`WHERE datname = $1 AND pid <> pg_backend_pid()`,
			dbName,
		)
		_, _ = dropConn.Exec(dropCtx, // best-effort
			fmt.Sprintf(`DROP DATABASE IF EXISTS %s`, dbName),
		)
	})

	pool, err := database.NewPool(ctx, urlToConfig(replaceDBName(s.adminURL, dbName)))
	if err != nil {
		t.Fatalf("dbtest: open pool for %s: %v", dbName, err)
	}
	t.Cleanup(pool.Close)

	return pool
}

// ── internal helpers ──────────────────────────────────────────────────────────

func startContainer(ctx context.Context) (*SharedDB, func()) {
	ctr, err := tcpostgres.Run(ctx,
		"postgres:18-alpine3.23",
		// Use "postgres" as the default database so the admin URL is predictable.
		tcpostgres.WithDatabase("postgres"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	if err != nil {
		panic(fmt.Sprintf("dbtest: start container: %v", err))
	}

	host, err := ctr.Host(ctx)
	if err != nil {
		panic(fmt.Sprintf("dbtest: container host: %v", err))
	}
	port, err := ctr.MappedPort(ctx, "5432")
	if err != nil {
		panic(fmt.Sprintf("dbtest: container port: %v", err))
	}

	adminURL := fmt.Sprintf(
		"postgres://testuser:testpass@%s:%s/postgres?sslmode=disable",
		host, port.Port(),
	)
	return &SharedDB{adminURL: adminURL}, func() {
		_ = ctr.Terminate(ctx) // best-effort
	}
}

// adminConn returns a single *pgx.Conn to the admin (postgres) database.
// The caller is responsible for closing it.
func (s *SharedDB) adminConn(ctx context.Context) *pgx.Conn {
	conn, err := pgx.Connect(ctx, s.adminURL)
	if err != nil {
		panic(fmt.Sprintf("dbtest: admin connect %s: %v", s.adminURL, err))
	}
	return conn
}

// mustExec executes a SQL statement on conn and panics on error.
func mustExec(ctx context.Context, conn *pgx.Conn, sql string) {
	if _, err := conn.Exec(ctx, sql); err != nil {
		panic(fmt.Sprintf("dbtest: exec %q: %v", sql, err))
	}
}

// replaceDBName replaces the database name segment in a postgres URL.
//
//	postgres://u:p@h:5432/postgres?sslmode=disable
//	→ postgres://u:p@h:5432/new_name?sslmode=disable
func replaceDBName(adminURL, newDB string) string {
	idx := strings.LastIndex(adminURL, "/")
	if idx < 0 {
		panic(fmt.Sprintf("dbtest: malformed URL (no slash): %s", adminURL))
	}
	base := adminURL[:idx+1]
	rest := adminURL[idx+1:] // everything after the last slash (db + optional query)
	if q := strings.Index(rest, "?"); q >= 0 {
		return base + newDB + rest[q:] // preserve ?sslmode=disable etc.
	}
	return base + newDB
}

// urlToConfig builds a *config.Config from a full postgres URL.
// database.Open and database.NewPool both honour cfg.DatabaseURL when set.
func urlToConfig(url string) *config.Config {
	return &config.Config{
		DatabaseURL:            url,
		DBConnectTimeoutSec:    5,
		DBStartupRetries:       3,
		DBStartupRetryDelaySec: 1,
		DBMaxOpenConns:         10,
		DBMaxIdleConns:         5,
		DBConnMaxLifetimeSec:   60,
		DBConnMaxIdleTimeSec:   30,
	}
}
