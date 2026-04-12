//go:build integration

package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registerAndLogin bootstraps a user directly via SQL and logs in via HTTP,
// returning the auth token and user ID. Used by tests that need an authenticated
// user to set up preconditions — not for testing the registration endpoint itself.
func registerAndLogin(t *testing.T, env *e2eEnv, email, displayName, password string) (token string, userID string) {
	t.Helper()
	ctx := context.Background()
	token, userID, _ = env.bootstrapAdmin(t, ctx, email, displayName, password)
	return token, userID
}

// --- Household-scoped user creation E2E tests ---

// TestE2E_HouseholdUser_Create_Success verifies that a household admin can create
// a new user within their household. The new user is returned with ID, email, and
// display_name; they can subsequently log in.
func TestE2E_HouseholdUser_Create_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	adminToken, _, householdID := env.bootstrapAdmin(t, ctx, "admin-hcu@example.com", "Admin", "secret1234")

	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/users", map[string]string{
		"email": "newuser@example.com", "display_name": "New User", "password": "secret1234",
	}, adminToken)

	require.Equal(t, http.StatusCreated, w.Code, "create household user: %s", w.Body.String())
	resp := decodeJSON(t, w)
	assert.NotEmpty(t, resp["id"])
	assert.Equal(t, "newuser@example.com", resp["email"])
	assert.Equal(t, "New User", resp["display_name"])
	assert.NotEmpty(t, w.Header().Get("Location"))
	// PasswordHash must never appear in the response.
	assert.NotContains(t, w.Body.String(), "password")

	// The new user can log in.
	w = env.do(t, http.MethodPost, "/auth/login", map[string]string{
		"email": "newuser@example.com", "password": "secret1234",
	}, "")
	require.Equal(t, http.StatusOK, w.Code, "login new user: %s", w.Body.String())
	login := decodeJSON(t, w)
	assert.NotEmpty(t, login["token"])
}

// TestE2E_HouseholdUser_Create_NoToken_Returns401 verifies that an unauthenticated
// request to POST /api/v1/households/{id}/users returns 401.
func TestE2E_HouseholdUser_Create_NoToken_Returns401(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	adminToken, _, householdID := env.bootstrapAdmin(t, ctx, "admin-ntoken@example.com", "Admin", "secret1234")
	_ = adminToken

	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/users", map[string]string{
		"email": "x@example.com", "display_name": "X", "password": "secret1234",
	}, "")
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestE2E_HouseholdUser_Create_CallerIsMember_Returns403 verifies that a MEMBER
// (non-admin) of a household cannot create new users.
func TestE2E_HouseholdUser_Create_CallerIsMember_Returns403(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	adminToken, _, householdID := env.bootstrapAdmin(t, ctx, "admin-member@example.com", "Admin", "secret1234")

	// Admin creates a second user — that user becomes a MEMBER.
	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/users", map[string]string{
		"email": "member@example.com", "display_name": "Member", "password": "secret1234",
	}, adminToken)
	require.Equal(t, http.StatusCreated, w.Code, "create member: %s", w.Body.String())

	// Member logs in.
	w = env.do(t, http.MethodPost, "/auth/login", map[string]string{
		"email": "member@example.com", "password": "secret1234",
	}, "")
	require.Equal(t, http.StatusOK, w.Code)
	memberToken := decodeJSON(t, w)["token"].(string)

	// Member tries to create another user — 403.
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/users", map[string]string{
		"email": "another@example.com", "display_name": "Another", "password": "secret1234",
	}, memberToken)
	require.Equal(t, http.StatusForbidden, w.Code)
}

// TestE2E_HouseholdUser_Create_DifferentHousehold_Returns403 verifies that an admin
// of household A cannot create users in household B (cross-household injection is blocked).
func TestE2E_HouseholdUser_Create_DifferentHousehold_Returns403(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// Admin A and admin B each have their own household.
	tokenA, _, _ := env.bootstrapAdmin(t, ctx, "adminA-"+suffix+"@example.com", "Admin A", "secret1234")
	_, _, householdB := env.bootstrapAdmin(t, ctx, "adminB-"+suffix+"@example.com", "Admin B", "secret1234")

	// Admin A tries to POST to household B's user endpoint → 403.
	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdB+"/users", map[string]string{
		"email": "injected@example.com", "display_name": "Injected", "password": "secret1234",
	}, tokenA)
	require.Equal(t, http.StatusForbidden, w.Code)
}

// TestE2E_HouseholdUser_Create_NonExistentHousehold_Returns403 verifies the edge
// case where the household_id in the URL refers to a household that does not exist.
// The adminGuard calls FindRole for the caller against the given household; since no
// membership exists for a non-existent household, it returns 403 — the same as any
// other unauthorized access. The API intentionally does not reveal whether a household
// ID exists (prevents enumeration).
func TestE2E_HouseholdUser_Create_NonExistentHousehold_Returns403(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	adminToken, _, _ := env.bootstrapAdmin(t, ctx, "admin-nohouse@example.com", "Admin", "secret1234")
	nonExistentID := uuid.New().String()

	w := env.do(t, http.MethodPost, "/api/v1/households/"+nonExistentID+"/users", map[string]string{
		"email": "x@example.com", "display_name": "X", "password": "secret1234",
	}, adminToken)
	require.Equal(t, http.StatusForbidden, w.Code)
}

// TestE2E_HouseholdUser_Create_PasswordExactlyMinimum_Succeeds verifies the boundary
// value: a password of exactly 8 characters (the minimum) is accepted.
func TestE2E_HouseholdUser_Create_PasswordExactlyMinimum_Succeeds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	adminToken, _, householdID := env.bootstrapAdmin(t, ctx, "admin-pwmin@example.com", "Admin", "secret1234")

	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/users", map[string]string{
		"email": "minpass@example.com", "display_name": "Min Pass", "password": "8charPwd",
	}, adminToken)
	require.Equal(t, http.StatusCreated, w.Code, "8-char password must be accepted: %s", w.Body.String())
}

// TestE2E_HouseholdUser_Create_PasswordBelowMinimum_Returns400 verifies the boundary
// value: a password of 7 characters (one below the minimum of 8) is rejected with 400.
func TestE2E_HouseholdUser_Create_PasswordBelowMinimum_Returns400(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	adminToken, _, householdID := env.bootstrapAdmin(t, ctx, "admin-pwbelow@example.com", "Admin", "secret1234")

	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/users", map[string]string{
		"email": "shortpw@example.com", "display_name": "Short PW", "password": "7charPw",
	}, adminToken)
	require.Equal(t, http.StatusBadRequest, w.Code, "7-char password must be rejected: %s", w.Body.String())
	resp := decodeJSON(t, w)
	fields, ok := resp["fields"].(map[string]any)
	require.True(t, ok, "expected fields in response: %v", resp)
	assert.Contains(t, fields, "password")
}

// TestE2E_HouseholdUser_Create_DuplicateEmail_Returns409 verifies 409 when the
// email is already registered globally (even in another household).
func TestE2E_HouseholdUser_Create_DuplicateEmail_Returns409(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	adminToken, _, householdID := env.bootstrapAdmin(t, ctx, "admin-dup@example.com", "Admin", "secret1234")

	// Create the user once.
	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/users", map[string]string{
		"email": "dup@example.com", "display_name": "First", "password": "secret1234",
	}, adminToken)
	require.Equal(t, http.StatusCreated, w.Code)

	// Create again with the same email → 409.
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/users", map[string]string{
		"email": "dup@example.com", "display_name": "Second", "password": "secret1234",
	}, adminToken)
	require.Equal(t, http.StatusConflict, w.Code)
}

// TestE2E_HouseholdUser_Create_ValidationError_Returns400 verifies that missing
// required fields return 400 with a validation error body.
func TestE2E_HouseholdUser_Create_ValidationError_Returns400(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	adminToken, _, householdID := env.bootstrapAdmin(t, ctx, "admin-val@example.com", "Admin", "secret1234")

	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/users", map[string]string{
		"display_name": "No Email",
		// missing email and password
	}, adminToken)
	require.Equal(t, http.StatusBadRequest, w.Code)
	resp := decodeJSON(t, w)
	fields, ok := resp["fields"].(map[string]any)
	require.True(t, ok, "expected fields in response: %v", resp)
	assert.NotEmpty(t, fields)
}

// --- User profile E2E tests (use bootstrapAdmin for setup) ---

// TestE2E_User_GetProfile verifies AC4: authenticated user retrieves profile by ID.
func TestE2E_User_GetProfile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, userID := registerAndLogin(t, env, "bob-get@example.com", "Bob", "secret1234")

	w := env.do(t, http.MethodGet, "/api/v1/users/"+userID, nil, token)

	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON(t, w)
	assert.Equal(t, "bob-get@example.com", resp["email"])
	assert.Equal(t, "Bob", resp["display_name"])
	assert.Equal(t, float64(1), resp["version"])
}

// TestE2E_User_GetProfile_NotFound verifies AC5: non-existent user returns 404.
func TestE2E_User_GetProfile_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _ := registerAndLogin(t, env, "x-notfound@example.com", "X", "secret1234")

	w := env.do(t, http.MethodGet, "/api/v1/users/"+uuid.New().String(), nil, token)
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestE2E_User_GetProfile_InvalidUUID verifies edge case: bad UUID returns 400.
func TestE2E_User_GetProfile_InvalidUUID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _ := registerAndLogin(t, env, "x-uuid@example.com", "X", "secret1234")

	w := env.do(t, http.MethodGet, "/api/v1/users/not-a-uuid", nil, token)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestE2E_User_UpdateProfile verifies AC6: update profile with correct version
// returns updated user with incremented version.
func TestE2E_User_UpdateProfile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, userID := registerAndLogin(t, env, "carol-upd@example.com", "Carol", "secret1234")

	w := env.do(t, http.MethodPut, "/api/v1/users/"+userID, map[string]any{
		"display_name": "Carol Updated", "version": 1,
	}, token)

	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON(t, w)
	assert.Equal(t, "Carol Updated", resp["display_name"])
	assert.Equal(t, float64(2), resp["version"])
}

// TestE2E_User_UpdateProfile_StaleVersion verifies AC7: stale version returns 409.
func TestE2E_User_UpdateProfile_StaleVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, userID := registerAndLogin(t, env, "dave-stale@example.com", "Dave", "secret1234")

	// First update succeeds (version 1 → 2).
	w := env.do(t, http.MethodPut, "/api/v1/users/"+userID, map[string]any{
		"display_name": "Dave v2", "version": 1,
	}, token)
	require.Equal(t, http.StatusOK, w.Code)

	// Second update with stale version 1 → 409.
	w = env.do(t, http.MethodPut, "/api/v1/users/"+userID, map[string]any{
		"display_name": "Dave v3", "version": 1,
	}, token)
	require.Equal(t, http.StatusConflict, w.Code)
}

// TestE2E_User_ChangePassword verifies AC8: password change with correct old
// password succeeds, and subsequent login uses new password.
func TestE2E_User_ChangePassword(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, userID := registerAndLogin(t, env, "eve-pwd@example.com", "Eve", "oldpass1234")

	w := env.do(t, http.MethodPut, "/api/v1/users/"+userID+"/password", map[string]any{
		"old_password": "oldpass1234", "new_password": "newpass1234", "version": 1,
	}, token)
	require.Equal(t, http.StatusOK, w.Code)

	// Login with new password succeeds.
	w = env.do(t, http.MethodPost, "/auth/login", map[string]string{
		"email": "eve-pwd@example.com", "password": "newpass1234",
	}, "")
	require.Equal(t, http.StatusOK, w.Code)

	// Login with old password fails.
	w = env.do(t, http.MethodPost, "/auth/login", map[string]string{
		"email": "eve-pwd@example.com", "password": "oldpass1234",
	}, "")
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestE2E_User_ChangePassword_WrongOldPassword verifies AC9: wrong old password returns 401.
func TestE2E_User_ChangePassword_WrongOldPassword(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, userID := registerAndLogin(t, env, "frank-pwd@example.com", "Frank", "secret1234")

	w := env.do(t, http.MethodPut, "/api/v1/users/"+userID+"/password", map[string]any{
		"old_password": "wrongpass", "new_password": "newpass1234", "version": 1,
	}, token)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestE2E_User_Deactivate verifies AC10: deactivation returns 204 and subsequent
// requests for that user return 404.
func TestE2E_User_Deactivate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, userID := registerAndLogin(t, env, "gone-del@example.com", "Gone", "secret1234")

	w := env.do(t, http.MethodDelete, "/api/v1/users/"+userID, nil, token)
	require.Equal(t, http.StatusNoContent, w.Code)

	// Subsequent get returns 404.
	w = env.do(t, http.MethodGet, "/api/v1/users/"+userID, nil, token)
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestE2E_User_Deactivate_CannotLogin verifies edge case: deactivated user cannot log in.
func TestE2E_User_Deactivate_CannotLogin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, userID := registerAndLogin(t, env, "bye-del@example.com", "ByeBye", "secret1234")

	w := env.do(t, http.MethodDelete, "/api/v1/users/"+userID, nil, token)
	require.Equal(t, http.StatusNoContent, w.Code)

	// Login attempt fails.
	w = env.do(t, http.MethodPost, "/auth/login", map[string]string{
		"email": "bye-del@example.com", "password": "secret1234",
	}, "")
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestE2E_User_GetProfile_Unauthenticated verifies that accessing a user
// profile without a token returns 401.
func TestE2E_User_GetProfile_Unauthenticated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	w := env.do(t, http.MethodGet, "/api/v1/users/"+uuid.New().String(), nil, "")
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// decodeJSONArray decodes a JSON array response.
func decodeJSONArray(t *testing.T, w *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var arr []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&arr))
	return arr
}
