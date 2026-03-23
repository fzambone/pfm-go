// Package testutil provides reusable test helpers for the pfm-go project.
// Import this package only from _test.go files — it is not for production use.
package testutil

import (
	"context"

	"github.com/google/uuid"

	"github.com/zambone/pfm-go/internal/platform/ctxutil"
	"github.com/zambone/pfm-go/internal/types"
)

// AuthenticatedContext returns a context with the given user ID injected,
// simulating a request that has passed the authentication middleware.
func AuthenticatedContext(userID uuid.UUID) context.Context {
	return ctxutil.WithUserID(context.Background(), userID)
}

// AuthorizedContext returns a context with user ID, household ID, and role
// injected, simulating a request that has passed both the authentication
// and household guard middleware.
func AuthorizedContext(userID, householdID uuid.UUID, role types.Role) context.Context {
	ctx := ctxutil.WithUserID(context.Background(), userID)
	ctx = ctxutil.WithHouseholdID(ctx, householdID)
	ctx = ctxutil.WithRole(ctx, role)
	return ctx
}
