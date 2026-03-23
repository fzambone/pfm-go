package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/middleware"
	"github.com/zambone/pfm-go/internal/platform/ctxutil"
	"github.com/zambone/pfm-go/internal/types"
)

// fakeMembershipFinder is a test double for the membershipFinder interface.
type fakeMembershipFinder struct {
	role types.Role
	err  error
}

func (f *fakeMembershipFinder) FindRole(_ context.Context, _, _ uuid.UUID) (types.Role, error) {
	return f.role, f.err
}

// passHandler is a simple handler that writes 200 OK — used to verify the guard
// called next. Separate from authn_test.go's okHandler which has a different signature.
var passHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// newAuthzRequest creates a GET request with the given path, routed through a ServeMux
// so that r.PathValue works. The caller's user ID is injected into context.
func newAuthzRequest(t *testing.T, path, pattern string, userID uuid.UUID, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(pattern, handler)

	req := httptest.NewRequest(http.MethodGet, path, nil)
	if userID != uuid.Nil {
		req = req.WithContext(ctxutil.WithUserID(req.Context(), userID))
	}

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

// parseErrorBody extracts the "error" field from a JSON response body.
func parseErrorBody(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	return body["error"]
}

// ---------------------------------------------------------------------------
// HouseholdGuard
// ---------------------------------------------------------------------------

func TestHouseholdGuard_MemberAllowed(t *testing.T) {
	userID := uuid.New()
	finder := &fakeMembershipFinder{role: types.RoleMember}

	var capturedHouseholdID uuid.UUID
	var capturedRole types.Role
	captureHandler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		capturedHouseholdID, _ = ctxutil.HouseholdID(r.Context())
		capturedRole, _ = ctxutil.Role(r.Context())
	})

	guarded := middleware.HouseholdGuard(finder, "household_id")(captureHandler)
	householdID := uuid.New()
	rr := newAuthzRequest(t, "/households/"+householdID.String(), "/households/{household_id}", userID, guarded)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, householdID, capturedHouseholdID)
	assert.Equal(t, types.RoleMember, capturedRole)
}

func TestHouseholdGuard_AdminAllowed(t *testing.T) {
	userID := uuid.New()
	finder := &fakeMembershipFinder{role: types.RoleAdmin}

	guarded := middleware.HouseholdGuard(finder, "household_id")(passHandler)
	rr := newAuthzRequest(t, "/households/"+uuid.New().String(), "/households/{household_id}", userID, guarded)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHouseholdGuard_NotMember_Returns403(t *testing.T) {
	userID := uuid.New()
	finder := &fakeMembershipFinder{err: message.ErrHouseholdMemberNotFound}

	guarded := middleware.HouseholdGuard(finder, "household_id")(passHandler)
	rr := newAuthzRequest(t, "/households/"+uuid.New().String(), "/households/{household_id}", userID, guarded)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Equal(t, message.MsgAuthzForbidden, parseErrorBody(t, rr))
}

func TestHouseholdGuard_HouseholdNotFound_Returns404(t *testing.T) {
	userID := uuid.New()
	finder := &fakeMembershipFinder{err: message.ErrHouseholdNotFound}

	guarded := middleware.HouseholdGuard(finder, "household_id")(passHandler)
	rr := newAuthzRequest(t, "/households/"+uuid.New().String(), "/households/{household_id}", userID, guarded)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Equal(t, message.MsgAuthzNotFound, parseErrorBody(t, rr))
}

func TestHouseholdGuard_NoUserID_Returns401(t *testing.T) {
	finder := &fakeMembershipFinder{role: types.RoleMember}

	guarded := middleware.HouseholdGuard(finder, "household_id")(passHandler)
	// Pass uuid.Nil to signal no user ID in context.
	rr := newAuthzRequest(t, "/households/"+uuid.New().String(), "/households/{household_id}", uuid.Nil, guarded)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Equal(t, message.MsgAuthzUnauthenticated, parseErrorBody(t, rr))
}

func TestHouseholdGuard_MalformedUUID_Returns400(t *testing.T) {
	userID := uuid.New()
	finder := &fakeMembershipFinder{role: types.RoleMember}

	guarded := middleware.HouseholdGuard(finder, "household_id")(passHandler)
	rr := newAuthzRequest(t, "/households/not-a-uuid", "/households/{household_id}", userID, guarded)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, message.MsgAuthzBadRequest, parseErrorBody(t, rr))
}

// ---------------------------------------------------------------------------
// HouseholdAdminGuard
// ---------------------------------------------------------------------------

func TestHouseholdAdminGuard_AdminAllowed(t *testing.T) {
	userID := uuid.New()
	finder := &fakeMembershipFinder{role: types.RoleAdmin}

	guarded := middleware.HouseholdAdminGuard(finder, "household_id")(passHandler)
	rr := newAuthzRequest(t, "/households/"+uuid.New().String(), "/households/{household_id}", userID, guarded)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHouseholdAdminGuard_MemberDenied_Returns403(t *testing.T) {
	userID := uuid.New()
	finder := &fakeMembershipFinder{role: types.RoleMember}

	guarded := middleware.HouseholdAdminGuard(finder, "household_id")(passHandler)
	rr := newAuthzRequest(t, "/households/"+uuid.New().String(), "/households/{household_id}", userID, guarded)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Equal(t, message.MsgAuthzAdminRequired, parseErrorBody(t, rr))
}

// ---------------------------------------------------------------------------
// Panic on nil
// ---------------------------------------------------------------------------

func TestHouseholdGuard_NilFinder_Panics(t *testing.T) {
	assert.Panics(t, func() {
		middleware.HouseholdGuard(nil, "household_id")
	})
}

func TestHouseholdAdminGuard_NilFinder_Panics(t *testing.T) {
	assert.Panics(t, func() {
		middleware.HouseholdAdminGuard(nil, "household_id")
	})
}
