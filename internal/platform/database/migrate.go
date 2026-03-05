package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/pressly/goose/v3"

	"github.com/zambone/pfm-go/internal/message"
)

// Migrate applies all pending SQL migrations from the provided filesystem using goose.
// It must be called after Open and before the application begins serving requests.
// The filesystem must contain .sql files with goose Up/Down annotations at its root.
// Passing an empty filesystem is valid — goose will simply report zero pending migrations.
func Migrate(ctx context.Context, db *sql.DB, migrations fs.FS) error {
	slog.InfoContext(ctx, message.MsgMigrationsRunning)

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations,
		goose.WithLogger(goose.NopLogger()), // goose output is handled via our slog calls below
	)
	if err != nil {
		// ErrNoMigrations is not an error — the system is valid before any schema files exist.
		if errors.Is(err, goose.ErrNoMigrations) {
			slog.InfoContext(ctx, message.MsgMigrationsComplete, "applied", 0)
			return nil
		}
		return fmt.Errorf(message.ErrMigrateProvider, err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf(message.ErrMigrateUp, err)
	}

	for _, r := range results {
		slog.InfoContext(ctx, message.MsgMigrationApplied,
			"version", r.Source.Version,
			"filename", r.Source.Path,
			"duration_ms", r.Duration.Milliseconds(),
		)
	}

	slog.InfoContext(ctx, message.MsgMigrationsComplete, "applied", len(results))

	return nil
}
