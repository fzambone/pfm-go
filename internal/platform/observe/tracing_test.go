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

	tp, shutdown, err := observe.NewTracerProvider(context.Background(), cfg, "test-service", "v-0.0.0-test")

	require.NoError(t, err)
	assert.NotNil(t, tp)
	assert.NotNil(t, shutdown)
}

func TestNewTracerProvider_WhenNoEndpoint_ShutdownCleanly(t *testing.T) {
	cfg := &config.Config{OTELEndpoint: ""}

	_, shutdown, err := observe.NewTracerProvider(context.Background(), cfg, "test-service", "v-0.0.0-test")
	require.NoError(t, err)

	err = shutdown(context.Background())
	assert.NoError(t, err)
}

func TestNewTracerProvider_ShutdownWithCancelledContext_ReturnsNoError(t *testing.T) {
	cfg := &config.Config{OTELEndpoint: ""}

	_, shutdown, err := observe.NewTracerProvider(context.Background(), cfg, "test-service", "v-0.0.0-test")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately before shutdown

	err = shutdown(ctx)
	assert.NoError(t, err)
}
