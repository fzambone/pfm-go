package observe_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zambone/pfm-go/internal/platform/config"
	"github.com/zambone/pfm-go/internal/platform/observe"
)

func TestNewTracerProvider_WhenNoEndpoint_ReturnsProvider(t *testing.T) {
	cfg := &config.Config{OTELEndpoint: ""}

	tp, shutdown, err := observe.NewTracerProvider(context.Background(), cfg, "test-service")

	require.NoError(t, err)
	assert.NotNil(t, tp)
	assert.NotNil(t, shutdown)
}

func TestNewTracerProvider_WhenNoEndpoint_ShutdownCleanly(t *testing.T) {
	cfg := &config.Config{OTELEndpoint: ""}

	_, shutdown, err := observe.NewTracerProvider(context.Background(), cfg, "test-service")
	require.NoError(t, err)

	err = shutdown(context.Background())
	assert.NoError(t, err)
}
