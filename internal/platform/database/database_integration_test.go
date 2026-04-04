//go:build integration

package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen_WhenDatabaseAvailable_ReturnsConfiguredPool(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// newTestDB creates an isolated per-test database and opens it via Open().
	// A successful open proves the database is available and Open is configured correctly.
	db := newTestDB(t, ctx)

	require.NotNil(t, db)
	assert.NoError(t, db.Ping())
}
