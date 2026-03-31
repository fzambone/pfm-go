//go:build integration

package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registerAndLogin is a test helper that registers a user and logs in,
// returning the auth token and user ID. Fails the test if either step fails.
func registerAndLogin(t *testing.T, env *e2eEnv, email, displayName, password string) (token string, userID string) {
	t.Helper()

	w := env.do(t, http.MethodPost, "/api/v1/users", map[string]string{
		"email": email, "display_name": displayName, "password": password,
	}, "")
	require.Equal(t, http.StatusCreated, w.Code, "register failed: %s", w.Body.String())
	user := decodeJSON(t, w)
	userID = user["id"].(string)

	w = env.do(t, http.MethodPost, "/auth/login", map[string]string{
		"email": email, "password": password,
	}, "")
	require.Equal(t, http.StatusOK, w.Code, "login failed: %s", w.Body.String())
	login := decodeJSON(t, w)
	token = login["token"].(string)

	return token, userID
}

// --- User Domain E2E Tests ---

// TestE2E_User_Register_Success verifies AC1: registration returns the new user
// with a server-assigned ID and the user can subsequently log in.
func TestE2E_User_Register_Success(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	w := env.do(t, http.MethodPost, "/api/v1/users", map[string]string{
		"email": "alice@example.com", "display_name": "Alice", "password": "secret1234",
	}, "")

	require.Equal(t, http.StatusCreated, w.Code)
	resp := decodeJSON(t, w)
	assert.NotEmpty(t, resp["id"])
	assert.Equal(t, "alice@example.com", resp["email"])
	assert.Equal(t, "Alice", resp["display_name"])
	assert.NotEmpty(t, w.Header().Get("Location"))

	// Verify can log in with the registered credentials.
	w = env.do(t, http.MethodPost, "/auth/login", map[string]string{
		"email": "alice@example.com", "password": "secret1234",
	}, "")
	require.Equal(t, http.StatusOK, w.Code)
	login := decodeJSON(t, w)
	assert.NotEmpty(t, login["token"])
}

// TestE2E_User_Register_DuplicateEmail verifies AC2: duplicate email returns 409.
func TestE2E_User_Register_DuplicateEmail(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	body := map[string]string{
		"email": "dup@example.com", "display_name": "First", "password": "secret1234",
	}
	w := env.do(t, http.MethodPost, "/api/v1/users", body, "")
	require.Equal(t, http.StatusCreated, w.Code)

	// Second registration with same email.
	w = env.do(t, http.MethodPost, "/api/v1/users", body, "")
	require.Equal(t, http.StatusConflict, w.Code)
}

// TestE2E_User_Register_ValidationErrors verifies AC3: invalid fields return 400
// with per-field violations.
func TestE2E_User_Register_ValidationErrors(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	w := env.do(t, http.MethodPost, "/api/v1/users", map[string]string{
		"email": "", "display_name": "A", "password": "short",
	}, "")

	require.Equal(t, http.StatusBadRequest, w.Code)
	resp := decodeJSON(t, w)
	fields, ok := resp["fields"].(map[string]any)
	require.True(t, ok, "expected fields in response: %v", resp)
	assert.NotEmpty(t, fields)
}

// TestE2E_User_Register_EmptyBody verifies edge case: empty body returns 400.
func TestE2E_User_Register_EmptyBody(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	w := env.do(t, http.MethodPost, "/api/v1/users", nil, "")
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestE2E_User_GetProfile verifies AC4: authenticated user retrieves profile by ID.
func TestE2E_User_GetProfile(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, userID := registerAndLogin(t, env, "bob@example.com", "Bob", "secret1234")

	w := env.do(t, http.MethodGet, "/api/v1/users/"+userID, nil, token)

	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON(t, w)
	assert.Equal(t, "bob@example.com", resp["email"])
	assert.Equal(t, "Bob", resp["display_name"])
	assert.Equal(t, float64(1), resp["version"])
}

// TestE2E_User_GetProfile_NotFound verifies AC5: non-existent user returns 404.
func TestE2E_User_GetProfile_NotFound(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _ := registerAndLogin(t, env, "x@example.com", "XX", "secret1234")

	w := env.do(t, http.MethodGet, "/api/v1/users/"+uuid.New().String(), nil, token)
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestE2E_User_GetProfile_InvalidUUID verifies edge case: bad UUID returns 400.
func TestE2E_User_GetProfile_InvalidUUID(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _ := registerAndLogin(t, env, "x@example.com", "XX", "secret1234")

	w := env.do(t, http.MethodGet, "/api/v1/users/not-a-uuid", nil, token)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestE2E_User_UpdateProfile verifies AC6: update profile with correct version
// returns updated user with incremented version.
func TestE2E_User_UpdateProfile(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, userID := registerAndLogin(t, env, "carol@example.com", "Carol", "secret1234")

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
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, userID := registerAndLogin(t, env, "dave@example.com", "Dave", "secret1234")

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
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, userID := registerAndLogin(t, env, "eve@example.com", "Eve", "oldpass1234")

	w := env.do(t, http.MethodPut, "/api/v1/users/"+userID+"/password", map[string]any{
		"old_password": "oldpass1234", "new_password": "newpass1234", "version": 1,
	}, token)
	require.Equal(t, http.StatusOK, w.Code)

	// Login with new password succeeds.
	w = env.do(t, http.MethodPost, "/auth/login", map[string]string{
		"email": "eve@example.com", "password": "newpass1234",
	}, "")
	require.Equal(t, http.StatusOK, w.Code)

	// Login with old password fails.
	w = env.do(t, http.MethodPost, "/auth/login", map[string]string{
		"email": "eve@example.com", "password": "oldpass1234",
	}, "")
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestE2E_User_ChangePassword_WrongOldPassword verifies AC9: wrong old password returns 401.
func TestE2E_User_ChangePassword_WrongOldPassword(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, userID := registerAndLogin(t, env, "frank@example.com", "Frank", "secret1234")

	w := env.do(t, http.MethodPut, "/api/v1/users/"+userID+"/password", map[string]any{
		"old_password": "wrongpass", "new_password": "newpass1234", "version": 1,
	}, token)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestE2E_User_Deactivate verifies AC10: deactivation returns 204 and subsequent
// requests for that user return 404.
func TestE2E_User_Deactivate(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, userID := registerAndLogin(t, env, "gone@example.com", "Gone", "secret1234")

	w := env.do(t, http.MethodDelete, "/api/v1/users/"+userID, nil, token)
	require.Equal(t, http.StatusNoContent, w.Code)

	// Subsequent get returns 404.
	w = env.do(t, http.MethodGet, "/api/v1/users/"+userID, nil, token)
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestE2E_User_Deactivate_CannotLogin verifies edge case: deactivated user cannot log in.
func TestE2E_User_Deactivate_CannotLogin(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, userID := registerAndLogin(t, env, "byebye@example.com", "ByeBye", "secret1234")

	w := env.do(t, http.MethodDelete, "/api/v1/users/"+userID, nil, token)
	require.Equal(t, http.StatusNoContent, w.Code)

	// Login attempt fails.
	w = env.do(t, http.MethodPost, "/auth/login", map[string]string{
		"email": "byebye@example.com", "password": "secret1234",
	}, "")
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestE2E_User_GetProfile_Unauthenticated verifies that accessing a user
// profile without a token returns 401.
func TestE2E_User_GetProfile_Unauthenticated(t *testing.T) {
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
