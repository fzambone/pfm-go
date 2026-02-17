package ctxutil_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/zambone/pfm-go/internal/platform/ctxutil"
)

func TestUserID_RoundTrip(t *testing.T) {
	id := uuid.New()
	ctx := ctxutil.WithUserID(context.Background(), id)

	got, ok := ctxutil.UserID(ctx)

	assert.True(t, ok)
	assert.Equal(t, id, got)
}

func TestUserID_MissingFromContext_ReturnsFalse(t *testing.T) {
	ctx := context.Background()

	_, ok := ctxutil.UserID(ctx)

	assert.False(t, ok)
}

func TestHouseholdID_RoundTrip(t *testing.T) {
	id := uuid.New()
	ctx := ctxutil.WithHouseholdID(context.Background(), id)

	got, ok := ctxutil.HouseholdID(ctx)

	assert.True(t, ok)
	assert.Equal(t, id, got)
}

func TestTraceID_RoundTrip(t *testing.T) {
	ctx := ctxutil.WithTraceID(context.Background(), "abc123")

	got, ok := ctxutil.TraceID(ctx)

	assert.True(t, ok)
	assert.Equal(t, "abc123", got)
}

func TestMultipleValues_Coexist(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()
	traceID := "trace-123"

	ctx := context.Background()
	ctx = ctxutil.WithUserID(ctx, userID)
	ctx = ctxutil.WithHouseholdID(ctx, householdID)
	ctx = ctxutil.WithTraceID(ctx, traceID)

	gotUser, ok := ctxutil.UserID(ctx)
	assert.True(t, ok)
	assert.Equal(t, userID, gotUser)

	gotHousehold, ok := ctxutil.HouseholdID(ctx)
	assert.True(t, ok)
	assert.Equal(t, householdID, gotHousehold)

	gotTrace, ok := ctxutil.TraceID(ctx)
	assert.True(t, ok)
	assert.Equal(t, traceID, gotTrace)
}
