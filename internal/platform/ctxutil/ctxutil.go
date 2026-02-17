package ctxutil

import (
	"context"

	"github.com/google/uuid"
)

type contextKey int

const (
	userIdKey contextKey = iota
	householdIdKey
	traceIdKey
)

// WithUserID returns a new context with a given user id.
func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userIdKey, id)
}

// UserID extracts the user ID from the context.
func UserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIdKey).(uuid.UUID)
	return id, ok
}

// WithHouseholdID returns a new context with a given household id.
func WithHouseholdID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, householdIdKey, id)
}

// HouseholdID extracts the household ID from the context.
func HouseholdID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(householdIdKey).(uuid.UUID)
	return id, ok
}

// WithTraceID returns a new context with the given trace ID.
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceIdKey, id)
}

// TraceID extracts the trace id from the context.
func TraceID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(traceIdKey).(string)
	return id, ok
}
