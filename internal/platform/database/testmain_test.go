//go:build integration

package database

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	pfmdb "github.com/zambone/pfm-go/db"
	"github.com/zambone/pfm-go/internal/platform/config"
)

// dbAdminURL points to the "postgres" admin database used for DDL (CREATE/DROP DATABASE).
var dbAdminURL string

const dbTemplateDB = "pfm_template"

func TestMain(m *testing.M) {
	ctx := context.Background()
	cleanup := setupAdminURL(ctx)
	defer cleanup()
	prepareTemplate(ctx)
	os.Exit(m.Run())
}

// setupAdminURL sets dbAdminURL from TEST_DATABASE_URL or starts a testcontainer.
func setupAdminURL(ctx context.Context) func() {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		dbAdminURL = url
		return func() {}
	}
	return startLocalContainer(ctx)
}

// prepareTemplate creates pfm_template and applies all goose migrations to it.
// Safe to call concurrently: uses a Postgres advisory lock so only the first
// caller creates and migrates the template; others wait, see it exists, and return.
func prepareTemplate(ctx context.Context) {
	conn := dbAdminConn(ctx)
	defer conn.Close(ctx)

	// Serialize concurrent prepareTemplate calls across packages sharing the
	// same Postgres service container in CI.
	dbAdminExec(ctx, conn, `SELECT pg_advisory_lock(7382910)`)

	var exists bool
	if err := conn.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`, dbTemplateDB,
	).Scan(&exists); err != nil {
		panic(fmt.Sprintf("testmain: check template exists: %v", err))
	}
	if exists {
		// Already created and migrated by another package's TestMain.
		return
	}

	dbAdminExec(ctx, conn, fmt.Sprintf(`CREATE DATABASE %s`, dbTemplateDB))

	templateURL := dbReplaceDBName(dbAdminURL, dbTemplateDB)
	sqlDB, err := Open(ctx, dbURLToConfig(templateURL))
	if err != nil {
		panic(fmt.Sprintf("testmain: open template: %v", err))
	}
	sub, err := fs.Sub(pfmdb.Migrations, "migrations")
	if err != nil {
		panic(fmt.Sprintf("testmain: sub migrations: %v", err))
	}
	if err := Migrate(ctx, sqlDB, sub); err != nil {
		panic(fmt.Sprintf("testmain: migrate template: %v", err))
	}
	if err := sqlDB.Close(); err != nil {
		panic(fmt.Sprintf("testmain: close template: %v", err))
	}
	// defer conn.Close(ctx) releases the advisory lock.
}

func startLocalContainer(ctx context.Context) func() {
	ctr, err := tcpostgres.Run(ctx,
		"postgres:18-alpine3.23",
		tcpostgres.WithDatabase("postgres"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	if err != nil {
		panic(fmt.Sprintf("testmain: start container: %v", err))
	}
	host, err := ctr.Host(ctx)
	if err != nil {
		panic(fmt.Sprintf("testmain: container host: %v", err))
	}
	port, err := ctr.MappedPort(ctx, "5432")
	if err != nil {
		panic(fmt.Sprintf("testmain: container port: %v", err))
	}
	dbAdminURL = fmt.Sprintf(
		"postgres://testuser:testpass@%s:%s/postgres?sslmode=disable",
		host, port.Port(),
	)
	return func() { _ = ctr.Terminate(ctx) } // best-effort
}

func dbAdminConn(ctx context.Context) *pgx.Conn {
	conn, err := pgx.Connect(ctx, dbAdminURL)
	if err != nil {
		panic(fmt.Sprintf("testmain: admin connect: %v", err))
	}
	return conn
}

func dbAdminExec(ctx context.Context, conn *pgx.Conn, sql string) {
	if _, err := conn.Exec(ctx, sql); err != nil {
		panic(fmt.Sprintf("testmain: exec %q: %v", sql, err))
	}
}

func dbReplaceDBName(adminURL, newDB string) string {
	idx := strings.LastIndex(adminURL, "/")
	if idx < 0 {
		panic(fmt.Sprintf("testmain: malformed URL: %s", adminURL))
	}
	base := adminURL[:idx+1]
	rest := adminURL[idx+1:]
	if q := strings.Index(rest, "?"); q >= 0 {
		return base + newDB + rest[q:]
	}
	return base + newDB
}

func dbURLToConfig(url string) *config.Config {
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
