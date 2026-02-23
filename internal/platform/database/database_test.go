package database

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zambone/pfm-go/internal/platform/config"
)

// fakePinger is a test double that returns errors from a preloaded list.
// then returns nil once exhausted.
type fakePinger struct {
	errs []error
}

func TestBuildDSN_ContainsAllConfigFields(t *testing.T) {
	cfg := &config.Config{
		DatabaseUser:        "alice",
		DatabasePassword:    "s3cr3t",
		DatabaseHost:        "db.internal",
		DatabasePort:        5433,
		DatabaseName:        "pfm_test",
		DatabaseSSLMode:     "require",
		DBConnectTimeoutSec: 10,
	}

	dsn := buildDSN(cfg)

	assert.Contains(t, dsn, "alice")
	assert.Contains(t, dsn, "s3cr3t")
	assert.Contains(t, dsn, "db.internal")
	assert.Contains(t, dsn, "5433")
	assert.Contains(t, dsn, "pfm_test")
	assert.Contains(t, dsn, "require")
	assert.Contains(t, dsn, "10")
}

func (f *fakePinger) PingContext(_ context.Context) error {
	if len(f.errs) == 0 {
		return nil
	}
	err := f.errs[0]
	f.errs = f.errs[1:]
	return err
}

func TestPingWithRetry_WhenPingSucceeds_ReturnsNil(t *testing.T) {
	cfg := &config.Config{DBStartupRetries: 3, DBStartupRetryDelaySec: 0}
	p := &fakePinger{} // no errors - succeeds on first attempt

	err := pingWithRetry(context.Background(), p, cfg)

	assert.NoError(t, err)
}

func TestPingWithRetry_WhenPingFailsThenSucceeds_ReturnsNil(t *testing.T) {
	cfg := &config.Config{DBStartupRetries: 3, DBStartupRetryDelaySec: 0}
	p := &fakePinger{errs: []error{
		errors.New("connection refused"),
		errors.New("connection refused"),
	}}

	err := pingWithRetry(context.Background(), p, cfg)

	assert.NoError(t, err)
}

func TestPingWithRetry_WhenContextCancelled_ReturnsContextError(t *testing.T) {
	cfg := &config.Config{DBStartupRetries: 5, DBStartupRetryDelaySec: 0}
	p := &fakePinger{errs: []error{errors.New("connection refused")}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := pingWithRetry(ctx, p, cfg)

	assert.ErrorIs(t, err, context.Canceled)
}
