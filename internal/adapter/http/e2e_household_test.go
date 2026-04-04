//go:build integration

package http_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createHouseholdHelper registers a user, logs in, creates a household, and returns
// the token, user ID, and household ID.
func createHouseholdHelper(t *testing.T, env *e2eEnv, email, householdName string) (token, userID, householdID string) {
	t.Helper()
	token, userID = registerAndLogin(t, env, email, "User", "secret1234")
	w := env.do(t, http.MethodPost, "/api/v1/households", map[string]string{
		"name": householdName,
	}, token)
	require.Equal(t, http.StatusCreated, w.Code, "create household failed: %s", w.Body.String())
	h := decodeJSON(t, w)
	householdID = h["id"].(string)
	return token, userID, householdID
}

// --- Household Domain E2E Tests ---

// TestE2E_Household_Create_Success verifies AC1: creating a household returns 201
// and the creator can access it (proving they're an ADMIN member).
func TestE2E_Household_Create_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _, householdID := createHouseholdHelper(t, env, "h1@example.com", "My House")

	// Creator can access the household (proves ADMIN membership).
	w := env.do(t, http.MethodGet, "/api/v1/households/"+householdID, nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON(t, w)
	assert.Equal(t, "My House", resp["name"])
}

// TestE2E_Household_Create_ValidationError verifies AC2: empty name returns 400.
func TestE2E_Household_Create_ValidationError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _ := registerAndLogin(t, env, "h2@example.com", "User", "secret1234")

	w := env.do(t, http.MethodPost, "/api/v1/households", map[string]string{
		"name": "",
	}, token)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestE2E_Household_GetByID verifies AC3: member retrieves household by ID.
func TestE2E_Household_GetByID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _, householdID := createHouseholdHelper(t, env, "h3@example.com", "Get House")

	w := env.do(t, http.MethodGet, "/api/v1/households/"+householdID, nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON(t, w)
	assert.Equal(t, "Get House", resp["name"])
	assert.Equal(t, householdID, resp["id"])
}

// TestE2E_Household_List verifies AC4: list returns all households the user belongs to.
func TestE2E_Household_List(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _ := registerAndLogin(t, env, "h4@example.com", "Lister", "secret1234")

	// Create two households.
	env.do(t, http.MethodPost, "/api/v1/households", map[string]string{"name": "House A"}, token)
	env.do(t, http.MethodPost, "/api/v1/households", map[string]string{"name": "House B"}, token)

	w := env.do(t, http.MethodGet, "/api/v1/households", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	list := decodeJSONArray(t, w)
	assert.Len(t, list, 2)
}

// TestE2E_Household_List_Empty verifies AC5: user with no households gets [].
func TestE2E_Household_List_Empty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _ := registerAndLogin(t, env, "h5@example.com", "NoHouse", "secret1234")

	w := env.do(t, http.MethodGet, "/api/v1/households", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	list := decodeJSONArray(t, w)
	assert.Empty(t, list)
}

// TestE2E_Household_UpdateName verifies AC6: admin renames with correct version.
func TestE2E_Household_UpdateName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _, householdID := createHouseholdHelper(t, env, "h6@example.com", "Old Name")

	w := env.do(t, http.MethodPut, "/api/v1/households/"+householdID, map[string]any{
		"name": "New Name", "version": 1,
	}, token)
	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON(t, w)
	assert.Equal(t, "New Name", resp["name"])
	assert.Equal(t, float64(2), resp["version"])
}

// TestE2E_Household_UpdateName_StaleVersion verifies AC7: stale version returns 409.
func TestE2E_Household_UpdateName_StaleVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _, householdID := createHouseholdHelper(t, env, "h7@example.com", "Stale")

	// First update succeeds.
	env.do(t, http.MethodPut, "/api/v1/households/"+householdID, map[string]any{
		"name": "V2", "version": 1,
	}, token)

	// Second with stale version 1 → 409.
	w := env.do(t, http.MethodPut, "/api/v1/households/"+householdID, map[string]any{
		"name": "V3", "version": 1,
	}, token)
	require.Equal(t, http.StatusConflict, w.Code)
}

// TestE2E_Household_AddMember verifies AC8: admin adds a member who can then
// access the household.
func TestE2E_Household_AddMember(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	tokenA, _, householdID := createHouseholdHelper(t, env, "admin@example.com", "Shared House")

	// Register user B.
	tokenB, userBID := registerAndLogin(t, env, "memberb@example.com", "Bob", "secret1234")

	// Admin adds B as MEMBER.
	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/members", map[string]string{
		"user_id": userBID, "role": "MEMBER",
	}, tokenA)
	require.Equal(t, http.StatusCreated, w.Code)

	// B can access the household.
	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID, nil, tokenB)
	require.Equal(t, http.StatusOK, w.Code)
}

// TestE2E_Household_AddMember_Duplicate verifies AC9: adding existing member → 409.
func TestE2E_Household_AddMember_Duplicate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	tokenA, _, householdID := createHouseholdHelper(t, env, "dup-admin@example.com", "DupHouse")
	_, userBID := registerAndLogin(t, env, "dup-member@example.com", "Bob", "secret1234")

	// First add succeeds.
	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/members", map[string]string{
		"user_id": userBID, "role": "MEMBER",
	}, tokenA)
	require.Equal(t, http.StatusCreated, w.Code)

	// Second add → 409.
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/members", map[string]string{
		"user_id": userBID, "role": "MEMBER",
	}, tokenA)
	require.Equal(t, http.StatusConflict, w.Code)
}

// TestE2E_Household_RemoveMember verifies AC10: admin removes a member who then
// cannot access the household.
func TestE2E_Household_RemoveMember(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	tokenA, _, householdID := createHouseholdHelper(t, env, "rm-admin@example.com", "RmHouse")
	tokenB, userBID := registerAndLogin(t, env, "rm-member@example.com", "Bob", "secret1234")

	// Add then remove B.
	env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/members", map[string]string{
		"user_id": userBID, "role": "MEMBER",
	}, tokenA)
	w := env.do(t, http.MethodDelete, "/api/v1/households/"+householdID+"/members/"+userBID, nil, tokenA)
	require.Equal(t, http.StatusNoContent, w.Code)

	// B can no longer access.
	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID, nil, tokenB)
	require.Equal(t, http.StatusForbidden, w.Code)
}

// TestE2E_Household_RemoveMember_LastAdmin verifies AC11: last admin removal rejected.
func TestE2E_Household_RemoveMember_LastAdmin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, userID, householdID := createHouseholdHelper(t, env, "last-admin@example.com", "LastAdmin")

	w := env.do(t, http.MethodDelete, "/api/v1/households/"+householdID+"/members/"+userID, nil, token)
	require.Equal(t, http.StatusConflict, w.Code)
}

// TestE2E_Household_Deactivate verifies AC12: deactivation returns 204 and
// subsequent access returns 404.
func TestE2E_Household_Deactivate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _, householdID := createHouseholdHelper(t, env, "deact@example.com", "DeactHouse")

	w := env.do(t, http.MethodDelete, "/api/v1/households/"+householdID, nil, token)
	require.Equal(t, http.StatusNoContent, w.Code)

	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID, nil, token)
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestE2E_Household_NonAdminRejected verifies AC13: MEMBER cannot perform admin
// operations (rename, deactivate, add/remove member).
func TestE2E_Household_NonAdminRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	tokenA, _, householdID := createHouseholdHelper(t, env, "admin-guard@example.com", "GuardHouse")
	tokenB, userBID := registerAndLogin(t, env, "member-guard@example.com", "Bob", "secret1234")

	// Add B as MEMBER (not admin).
	env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/members", map[string]string{
		"user_id": userBID, "role": "MEMBER",
	}, tokenA)

	// B can read the household (guard passes for members).
	w := env.do(t, http.MethodGet, "/api/v1/households/"+householdID, nil, tokenB)
	require.Equal(t, http.StatusOK, w.Code)

	// B cannot rename (admin-only).
	w = env.do(t, http.MethodPut, "/api/v1/households/"+householdID, map[string]any{
		"name": "Hacked", "version": 1,
	}, tokenB)
	require.Equal(t, http.StatusForbidden, w.Code)

	// B cannot deactivate (admin-only).
	w = env.do(t, http.MethodDelete, "/api/v1/households/"+householdID, nil, tokenB)
	require.Equal(t, http.StatusForbidden, w.Code)

	// B cannot add members (admin-only).
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/members", map[string]string{
		"user_id": uuid.New().String(), "role": "MEMBER",
	}, tokenB)
	require.Equal(t, http.StatusForbidden, w.Code)

	// B cannot remove members (admin-only).
	w = env.do(t, http.MethodDelete, "/api/v1/households/"+householdID+"/members/"+uuid.New().String(), nil, tokenB)
	require.Equal(t, http.StatusForbidden, w.Code)
}

// TestE2E_Household_AddSecondAdmin verifies edge case: admin adds another admin,
// both can perform admin operations.
func TestE2E_Household_AddSecondAdmin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	tokenA, _, householdID := createHouseholdHelper(t, env, "admin1@example.com", "DualAdmin")
	tokenB, userBID := registerAndLogin(t, env, "admin2@example.com", "Admin2", "secret1234")

	// A adds B as ADMIN.
	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/members", map[string]string{
		"user_id": userBID, "role": "ADMIN",
	}, tokenA)
	require.Equal(t, http.StatusCreated, w.Code)

	// B can rename (admin operation).
	w = env.do(t, http.MethodPut, "/api/v1/households/"+householdID, map[string]any{
		"name": "Renamed by B", "version": 1,
	}, tokenB)
	require.Equal(t, http.StatusOK, w.Code)
}

// TestE2E_Household_RemoveMember_NonExistent verifies edge case: removing a user
// who is not a member is idempotent — returns 204 (domain treats it as a no-op).
func TestE2E_Household_RemoveMember_NonExistent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _, householdID := createHouseholdHelper(t, env, "rm-noop@example.com", "RmNoop")

	w := env.do(t, http.MethodDelete, "/api/v1/households/"+householdID+"/members/"+uuid.New().String(), nil, token)
	require.Equal(t, http.StatusNoContent, w.Code)
}
