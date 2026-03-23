package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/ctxutil"
	"github.com/zambone/pfm-go/internal/types"
)

// membershipFinder is the subset of household.Repository required by the guard.
// Accepts a narrow interface (1 method) rather than the full Repository per interface
// segregation. Returns only the role — the middleware doesn't need the full Membership.
type membershipFinder interface {
	FindRole(ctx context.Context, householdID, userID uuid.UUID) (types.Role, error)
}

// HouseholdGuard returns middleware that verifies the caller is an active member of the
// household identified by the URL path parameter named pathParam. On success it injects
// the household ID and role into the request context. Panics if mf is nil.
//
// Errors:
//   - 400 Bad Request: malformed household UUID in URL
//   - 401 Unauthorized: no user ID in context (authn middleware missing or failed)
//   - 403 Forbidden: caller is not a member of the household
//   - 404 Not Found: household does not exist
func HouseholdGuard(mf membershipFinder, pathParam string) func(http.Handler) http.Handler {
	if mf == nil {
		panic("middleware: HouseholdGuard requires non-nil membershipFinder")
	}
	return householdGuard(mf, pathParam, false)
}

// HouseholdAdminGuard returns middleware identical to HouseholdGuard but additionally
// requires the caller to have the ADMIN role. Returns 403 if the caller is a MEMBER
// but not an ADMIN. Panics if mf is nil.
func HouseholdAdminGuard(mf membershipFinder, pathParam string) func(http.Handler) http.Handler {
	if mf == nil {
		panic("middleware: HouseholdAdminGuard requires non-nil membershipFinder")
	}
	return householdGuard(mf, pathParam, true)
}

// householdGuard is the shared implementation for both guard variants.
func householdGuard(mf membershipFinder, pathParam string, requireAdmin bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Ensure the caller is authenticated (user ID must be in context).
			userID, ok := ctxutil.UserID(r.Context())
			if !ok {
				writeJSON(w, http.StatusUnauthorized, message.MsgAuthzUnauthenticated)
				return
			}

			// 2. Extract and parse the household ID from the URL path.
			rawID := r.PathValue(pathParam)
			householdID, err := uuid.Parse(rawID)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, message.MsgAuthzBadRequest)
				return
			}

			// 3. Verify membership and retrieve role.
			role, err := mf.FindRole(r.Context(), householdID, userID)
			if err != nil {
				if errors.Is(err, message.ErrHouseholdMemberNotFound) {
					writeJSON(w, http.StatusForbidden, message.MsgAuthzForbidden)
					return
				}
				if errors.Is(err, message.ErrHouseholdNotFound) {
					writeJSON(w, http.StatusNotFound, message.MsgAuthzNotFound)
					return
				}
				writeJSON(w, http.StatusForbidden, message.MsgAuthzForbidden)
				return
			}

			// 4. If admin is required, check the role.
			if requireAdmin && role != types.RoleAdmin {
				writeJSON(w, http.StatusForbidden, message.MsgAuthzAdminRequired)
				return
			}

			// 5. Inject household ID and role into context, call next handler.
			ctx := ctxutil.WithHouseholdID(r.Context(), householdID)
			ctx = ctxutil.WithRole(ctx, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
