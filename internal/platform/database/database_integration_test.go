//go:build integration

package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/zambone/pfm-go/internal/platform/config"
)

func TestOpen_WhenDatabaseAvailable_ReturnsConfiguredPool(t *testing.T) {
	ctx := context.Background()

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
	t.Cleanup(func() { _ = ctr.Terminate(ctx) }) // best-effort

	host, err := ctr.Host(ctx)
	require.NoError(t, err)
	mappedPort, err := ctr.MappedPort(ctx, "5432")
	require.NoError(t, err)

	cfg := &config.Config{
		DatabaseHost:           host,
		DatabasePort:           mappedPort.Int(),
		DatabaseUser:           "testuser",
		DatabasePassword:       "testpass",
		DatabaseName:           "testdb",
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
	assert.NotNil(t, db)
	t.Cleanup(func() { _ = db.Close() }) // best-effort
}
