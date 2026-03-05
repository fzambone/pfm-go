//go:build integration

package database

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validMigrationFS returns an in-memory FS with one well-formed goose migration.
// Using fstest.MapFS keeps tests hermetic — no dependency on files in db/migrations/.
func validMigrationFS() fstest.MapFS {
	return fstest.MapFS{
		"20240101000000_create_test_table.sql": &fstest.MapFile{
			Data: []byte("-- +goose Up\nCREATE TABLE migrate_test (id BIGINT PRIMARY KEY);\n-- +goose Down\nDROP TABLE migrate_test;\n"),
		},
	}
}

func TestMigrate_WhenNoMigrations_Succeeds(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t, ctx)

	err := Migrate(ctx, db, fstest.MapFS{})

	require.NoError(t, err)
}

func TestMigrate_WhenMigrationsApplied_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t, ctx)
	fs := validMigrationFS()

	require.NoError(t, Migrate(ctx, db, fs))
	err := Migrate(ctx, db, fs)

	assert.NoError(t, err)
}

func TestMigrate_WhenSingleMigrationApplied_TableExists(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t, ctx)

	require.NoError(t, Migrate(ctx, db, validMigrationFS()))

	var exists bool
	err := db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'migrate_test')",
	).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestMigrate_WhenMigrationFails_ReturnsError(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t, ctx)
	badFS := fstest.MapFS{
		"20240101000000_bad.sql": &fstest.MapFile{
			Data: []byte("-- +goose Up\nTHIS IS NOT VALID SQL;\n-- +goose Down\nSELECT 1;\n"),
		},
	}

	err := Migrate(ctx, db, badFS)

	assert.Error(t, err)
}
