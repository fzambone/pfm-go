package observe_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zambone/pfm-go/internal/platform/ctxutil"
	"github.com/zambone/pfm-go/internal/platform/observe"
)

func TestNewLogger_InfoLog_ContainsBaseJsonFields(t *testing.T) {
	var buf bytes.Buffer
	logger := observe.NewLogger(slog.LevelInfo, &buf)

	logger.InfoContext(context.Background(), "hello world")

	var entry map[string]any
	err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry)

	assert.NoError(t, err)
	assert.Contains(t, entry, "time")
	assert.Equal(t, "INFO", entry["level"])
	assert.Equal(t, "hello world", entry["msg"])
	assert.Contains(t, entry, "source")
}

func TestNewLogger_InfoLog_IncludesRequestContextFields(t *testing.T) {
	var buf bytes.Buffer
	logger := observe.NewLogger(slog.LevelInfo, &buf)

	userId := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	householdId := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	ctx := context.Background()
	ctx = ctxutil.WithTraceID(ctx, "trace-123")
	ctx = ctxutil.WithUserID(ctx, userId)
	ctx = ctxutil.WithHouseholdID(ctx, householdId)

	logger.InfoContext(ctx, "request log")

	var entry map[string]any
	err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry)

	assert.NoError(t, err)
	assert.Equal(t, "trace-123", entry["trace_id"])
	assert.Equal(t, userId.String(), entry["user_id"])
	assert.Equal(t, householdId.String(), entry["household_id"])
	assert.Contains(t, entry, "span_id")
}

func TestNewLogger_WhenLevelInfo_DebugMessagesSuppressed(t *testing.T) {
	var buf bytes.Buffer
	logger := observe.NewLogger(slog.LevelInfo, &buf)

	logger.DebugContext(context.Background(), "should not appear")

	assert.Equal(t, 0, buf.Len())
}

func TestNewLogger_NilContext_DoesNotPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := observe.NewLogger(slog.LevelInfo, &buf)

	//nolint:staticcheck // intentionally testing nil context guard
	assert.NotPanics(t, func() {
		logger.InfoContext(nil, "nil context message")
	})
}

func TestNewLogger_WithAttrs_IncludesStaticAndContextAttrs(t *testing.T) {
	var buf bytes.Buffer
	logger := observe.NewLogger(slog.LevelInfo, &buf)
	logger = logger.With("component", "payments")

	userId := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ctx := ctxutil.WithUserID(context.Background(), userId)

	logger.InfoContext(ctx, "payment processed")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry))
	assert.Equal(t, "payments", entry["component"])    // static attr from With
	assert.Equal(t, userId.String(), entry["user_id"]) // dynamic attr from context
}
func TestNewLogger_WithGroup_NestsAttrsUnderGroup(t *testing.T) {
	var buf bytes.Buffer
	logger := observe.NewLogger(slog.LevelInfo, &buf)
	logger = logger.WithGroup("http").With("method", "GET", "path", "/healthz")

	logger.InfoContext(context.Background(), "request received")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry))

	httpGroup, ok := entry["http"].(map[string]any)
	require.True(t, ok, "expected 'http' group to be a JSON object")
	assert.Equal(t, "GET", httpGroup["method"])
	assert.Equal(t, "/healthz", httpGroup["path"])
}
