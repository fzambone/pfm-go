package ctxutil_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/zambone/pfm-go/internal/platform/ctxutil"
	"github.com/zambone/pfm-go/internal/types"
)

func TestUserID_RoundTrip(t *testing.T) {
	id := uuid.New()
	ctx := ctxutil.WithUserID(context.Background(), id)

	got, ok := ctxutil.UserID(ctx)

	assert.True(t, ok)
	assert.Equal(t, id, got)
}

func TestMissingFromContext_ReturnsFalse(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func(ctx2 context.Context) bool
	}{
		{"user ID", func(ctx context.Context) bool { _, ok := ctxutil.UserID(ctx); return ok }},
		{"household ID", func(ctx context.Context) bool { _, ok := ctxutil.HouseholdID(ctx); return ok }},
		{"trace ID", func(ctx context.Context) bool { _, ok := ctxutil.TraceID(ctx); return ok }},
		{"role", func(ctx context.Context) bool { _, ok := ctxutil.Role(ctx); return ok }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.False(t, tt.fn(ctx))
		})
	}
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

func TestRole_RoundTrip(t *testing.T) {
	ctx := ctxutil.WithRole(context.Background(), types.RoleAdmin)

	got, ok := ctxutil.Role(ctx)

	assert.True(t, ok)
	assert.Equal(t, types.RoleAdmin, got)
}

func TestMultipleValues_Coexist(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()
	traceID := "trace-123"
	role := types.RoleMember

	ctx := context.Background()
	ctx = ctxutil.WithUserID(ctx, userID)
	ctx = ctxutil.WithHouseholdID(ctx, householdID)
	ctx = ctxutil.WithTraceID(ctx, traceID)
	ctx = ctxutil.WithRole(ctx, role)

	gotUser, ok := ctxutil.UserID(ctx)
	assert.True(t, ok)
	assert.Equal(t, userID, gotUser)

	gotHousehold, ok := ctxutil.HouseholdID(ctx)
	assert.True(t, ok)
	assert.Equal(t, householdID, gotHousehold)

	gotTrace, ok := ctxutil.TraceID(ctx)
	assert.True(t, ok)
	assert.Equal(t, traceID, gotTrace)

	gotRole, ok := ctxutil.Role(ctx)
	assert.True(t, ok)
	assert.Equal(t, role, gotRole)
}

func TestContextValues_AccessibleAcrossGoroutines(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()
	traceID := "trace-goroutine"
	role := types.RoleAdmin

	ctx := context.Background()
	ctx = ctxutil.WithUserID(ctx, userID)
	ctx = ctxutil.WithHouseholdID(ctx, householdID)
	ctx = ctxutil.WithTraceID(ctx, traceID)
	ctx = ctxutil.WithRole(ctx, role)

	done := make(chan struct{})
	go func() {
		defer close(done)

		gotUser, ok := ctxutil.UserID(ctx)
		assert.True(t, ok)
		assert.Equal(t, userID, gotUser)

		gotHousehold, ok := ctxutil.HouseholdID(ctx)
		assert.True(t, ok)
		assert.Equal(t, householdID, gotHousehold)

		gotTrace, ok := ctxutil.TraceID(ctx)
		assert.True(t, ok)
		assert.Equal(t, traceID, gotTrace)

		gotRole, ok := ctxutil.Role(ctx)
		assert.True(t, ok)
		assert.Equal(t, role, gotRole)
	}()

	<-done
}
