package observe_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/zambone/pfm-go/internal/platform/ctxutil"
	"github.com/zambone/pfm-go/internal/platform/observe"
)

func TestNewLogger_InfoLog_ContainsBaseJsonFields(t *testing.T) {
	var buf bytes.Buffer
	logger := observe.NewLogger("info", &buf)

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
	logger := observe.NewLogger("info", &buf)

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
