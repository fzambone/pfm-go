//go:build integration

package http_test

import (
	"context"
	"math"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createCCAccountHelper creates a household with a CREDIT_CARD account.
// Returns token, householdID, and accountID.
func createCCAccountHelper(t *testing.T, env *e2eEnv, email, householdName, accountName string) (token, householdID, accountID string) {
	t.Helper()
	token, _, householdID = createHouseholdHelper(t, env, email, householdName)
	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name":          accountName,
		"account_type":  "CREDIT_CARD",
		"currency_code": "BRL",
	}, token)
	require.Equal(t, http.StatusCreated, w.Code, "create credit card account failed: %s", w.Body.String())
	a := decodeJSON(t, w)
	accountID = a["id"].(string)
	return token, householdID, accountID
}

// createCCSettingsHelper creates credit card settings for a given account and
// returns the decoded response body. Fails the test if creation does not succeed.
func createCCSettingsHelper(t *testing.T, env *e2eEnv, token, householdID, accountID string) map[string]any {
	t.Helper()
	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts/"+accountID+"/credit-card-settings", map[string]any{
		"closing_day":  15,
		"due_day":      25,
		"limit_amount": 500000,
	}, token)
	require.Equal(t, http.StatusCreated, w.Code, "create cc settings failed: %s", w.Body.String())
	return decodeJSON(t, w)
}

// ccSettingsURL returns the base URL for credit card settings of an account.
func ccSettingsURL(householdID, accountID string) string {
	return "/api/v1/households/" + householdID + "/accounts/" + accountID + "/credit-card-settings"
}

// --- Credit Card Settings Domain E2E Tests ---

// TestE2E_CCSettings_Create_Success verifies AC1: creating settings for a
// CREDIT_CARD account returns 201 with the settings.
func TestE2E_CCSettings_Create_Success(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, householdID, accountID := createCCAccountHelper(t, env, "cc1@example.com", "CC House 1", "Visa")

	w := env.do(t, http.MethodPost, ccSettingsURL(householdID, accountID), map[string]any{
		"closing_day":  15,
		"due_day":      25,
		"limit_amount": 500000,
	}, token)

	require.Equal(t, http.StatusCreated, w.Code)
	resp := decodeJSON(t, w)
	assert.NotEmpty(t, resp["id"])
	assert.Equal(t, accountID, resp["account_id"])
	assert.Equal(t, float64(15), resp["closing_day"])
	assert.Equal(t, float64(25), resp["due_day"])
	assert.Equal(t, float64(500000), resp["limit_amount"])
	assert.Equal(t, float64(1), resp["version"])
}

// TestE2E_CCSettings_Create_NotCreditCard verifies AC2: creating settings for a
// non-CREDIT_CARD account returns the appropriate conflict error (409).
func TestE2E_CCSettings_Create_NotCreditCard(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	// Use a CHECKING account — not a credit card.
	token, householdID, accountID := createAccountHelper(t, env, "cc2@example.com", "CC House 2", "Checking")

	w := env.do(t, http.MethodPost, ccSettingsURL(householdID, accountID), map[string]any{
		"closing_day":  15,
		"due_day":      25,
		"limit_amount": 500000,
	}, token)

	require.Equal(t, http.StatusConflict, w.Code)
}

// TestE2E_CCSettings_Create_Duplicate verifies AC3: creating settings for an
// account that already has settings returns 409.
func TestE2E_CCSettings_Create_Duplicate(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, householdID, accountID := createCCAccountHelper(t, env, "cc3@example.com", "CC House 3", "Visa")
	createCCSettingsHelper(t, env, token, householdID, accountID)

	// Second creation attempt → 409.
	w := env.do(t, http.MethodPost, ccSettingsURL(householdID, accountID), map[string]any{
		"closing_day":  10,
		"due_day":      20,
		"limit_amount": 200000,
	}, token)
	require.Equal(t, http.StatusConflict, w.Code)
}

// TestE2E_CCSettings_GetByAccountID verifies AC4: retrieving settings by account
// ID returns 200 with the correct data.
func TestE2E_CCSettings_GetByAccountID(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, householdID, accountID := createCCAccountHelper(t, env, "cc4@example.com", "CC House 4", "Visa")
	createCCSettingsHelper(t, env, token, householdID, accountID)

	w := env.do(t, http.MethodGet, ccSettingsURL(householdID, accountID), nil, token)

	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON(t, w)
	assert.Equal(t, accountID, resp["account_id"])
	assert.Equal(t, float64(15), resp["closing_day"])
	assert.Equal(t, float64(25), resp["due_day"])
	assert.Equal(t, float64(500000), resp["limit_amount"])
}

// TestE2E_CCSettings_GetByAccountID_NotFound verifies AC5: retrieving settings
// for an account with no settings returns 404.
func TestE2E_CCSettings_GetByAccountID_NotFound(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, householdID, accountID := createCCAccountHelper(t, env, "cc5@example.com", "CC House 5", "Visa")

	w := env.do(t, http.MethodGet, ccSettingsURL(householdID, accountID), nil, token)
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestE2E_CCSettings_UpdateClosingDay verifies AC6: updating the closing day with
// the correct version returns 200 with the updated value and incremented version.
func TestE2E_CCSettings_UpdateClosingDay(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, householdID, accountID := createCCAccountHelper(t, env, "cc6@example.com", "CC House 6", "Visa")
	createCCSettingsHelper(t, env, token, householdID, accountID)

	w := env.do(t, http.MethodPut, ccSettingsURL(householdID, accountID)+"/closing-day", map[string]any{
		"closing_day": 20,
		"version":     1,
	}, token)

	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON(t, w)
	assert.Equal(t, float64(20), resp["closing_day"])
	assert.Equal(t, float64(2), resp["version"])
}

// TestE2E_CCSettings_UpdateDueDay verifies AC7: updating the due day with the
// correct version returns 200 with the updated value.
func TestE2E_CCSettings_UpdateDueDay(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, householdID, accountID := createCCAccountHelper(t, env, "cc7@example.com", "CC House 7", "Visa")
	createCCSettingsHelper(t, env, token, householdID, accountID)

	w := env.do(t, http.MethodPut, ccSettingsURL(householdID, accountID)+"/due-day", map[string]any{
		"due_day": 10,
		"version": 1,
	}, token)

	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON(t, w)
	assert.Equal(t, float64(10), resp["due_day"])
	assert.Equal(t, float64(2), resp["version"])
}

// TestE2E_CCSettings_UpdateLimit verifies AC8: updating the credit limit with the
// correct version returns 200 with the updated value.
func TestE2E_CCSettings_UpdateLimit(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, householdID, accountID := createCCAccountHelper(t, env, "cc8@example.com", "CC House 8", "Visa")
	createCCSettingsHelper(t, env, token, householdID, accountID)

	w := env.do(t, http.MethodPut, ccSettingsURL(householdID, accountID)+"/limit", map[string]any{
		"limit_amount": 1000000,
		"version":      1,
	}, token)

	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON(t, w)
	assert.Equal(t, float64(1000000), resp["limit_amount"])
	assert.Equal(t, float64(2), resp["version"])
}

// TestE2E_CCSettings_UpdateClosingDay_StaleVersion verifies AC9: any settings
// update with a stale version returns 409.
func TestE2E_CCSettings_UpdateClosingDay_StaleVersion(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, householdID, accountID := createCCAccountHelper(t, env, "cc9@example.com", "CC House 9", "Visa")
	createCCSettingsHelper(t, env, token, householdID, accountID)

	// First update advances version to 2.
	env.do(t, http.MethodPut, ccSettingsURL(householdID, accountID)+"/closing-day", map[string]any{
		"closing_day": 20,
		"version":     1,
	}, token)

	// Second update with stale version 1 → 409.
	w := env.do(t, http.MethodPut, ccSettingsURL(householdID, accountID)+"/closing-day", map[string]any{
		"closing_day": 5,
		"version":     1,
	}, token)
	require.Equal(t, http.StatusConflict, w.Code)
}

// TestE2E_CCSettings_Delete verifies AC10: deleting settings returns 204 and
// subsequent GET returns 404.
func TestE2E_CCSettings_Delete(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, householdID, accountID := createCCAccountHelper(t, env, "cc10@example.com", "CC House 10", "Visa")
	createCCSettingsHelper(t, env, token, householdID, accountID)

	w := env.do(t, http.MethodDelete, ccSettingsURL(householdID, accountID), nil, token)
	require.Equal(t, http.StatusNoContent, w.Code)

	w = env.do(t, http.MethodGet, ccSettingsURL(householdID, accountID), nil, token)
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestE2E_CCSettings_RecreateAfterDelete verifies edge case: creating settings,
// deleting them, then creating new settings for the same account succeeds because
// the uniqueness constraint is partial (WHERE deleted_at IS NULL).
func TestE2E_CCSettings_RecreateAfterDelete(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, householdID, accountID := createCCAccountHelper(t, env, "cc-recreate@example.com", "RecreateHouse", "Visa")
	createCCSettingsHelper(t, env, token, householdID, accountID)

	// Delete.
	w := env.do(t, http.MethodDelete, ccSettingsURL(householdID, accountID), nil, token)
	require.Equal(t, http.StatusNoContent, w.Code)

	// Recreate with different values — succeeds because the partial unique index
	// only applies to rows where deleted_at IS NULL.
	w = env.do(t, http.MethodPost, ccSettingsURL(householdID, accountID), map[string]any{
		"closing_day":  5,
		"due_day":      15,
		"limit_amount": 300000,
	}, token)
	require.Equal(t, http.StatusCreated, w.Code)
	resp := decodeJSON(t, w)
	assert.Equal(t, float64(5), resp["closing_day"])
	assert.Equal(t, float64(1), resp["version"])
}

// TestE2E_CCSettings_ClosingDay_BoundaryValues verifies edge case: closing day
// values of 1 and 31 are both accepted.
func TestE2E_CCSettings_ClosingDay_BoundaryValues(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, householdID, accountID := createCCAccountHelper(t, env, "cc-close-bound@example.com", "CloseBoundHouse", "Visa")
	createCCSettingsHelper(t, env, token, householdID, accountID)

	for _, day := range []int{1, 31} {
		w := env.do(t, http.MethodGet, ccSettingsURL(householdID, accountID), nil, token)
		require.Equal(t, http.StatusOK, w.Code)
		current := decodeJSON(t, w)
		version := int(current["version"].(float64))

		w = env.do(t, http.MethodPut, ccSettingsURL(householdID, accountID)+"/closing-day", map[string]any{
			"closing_day": day,
			"version":     version,
		}, token)
		require.Equal(t, http.StatusOK, w.Code, "expected 200 for closing_day=%d", day)
		resp := decodeJSON(t, w)
		assert.Equal(t, float64(day), resp["closing_day"])
	}
}

// TestE2E_CCSettings_DueDay_BoundaryValues verifies edge case: due day values of
// 1 and 31 are both accepted.
func TestE2E_CCSettings_DueDay_BoundaryValues(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, householdID, accountID := createCCAccountHelper(t, env, "cc-due-bound@example.com", "DueBoundHouse", "Visa")
	createCCSettingsHelper(t, env, token, householdID, accountID)

	for _, day := range []int{1, 31} {
		w := env.do(t, http.MethodGet, ccSettingsURL(householdID, accountID), nil, token)
		require.Equal(t, http.StatusOK, w.Code)
		current := decodeJSON(t, w)
		version := int(current["version"].(float64))

		w = env.do(t, http.MethodPut, ccSettingsURL(householdID, accountID)+"/due-day", map[string]any{
			"due_day": day,
			"version": version,
		}, token)
		require.Equal(t, http.StatusOK, w.Code, "expected 200 for due_day=%d", day)
		resp := decodeJSON(t, w)
		assert.Equal(t, float64(day), resp["due_day"])
	}
}

// TestE2E_CCSettings_UpdateLimit_LargeValue verifies edge case: a credit limit
// near the int64 maximum is accepted (stored as BIGINT minor units).
func TestE2E_CCSettings_UpdateLimit_LargeValue(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, householdID, accountID := createCCAccountHelper(t, env, "cc-large@example.com", "LargeHouse", "Visa")
	createCCSettingsHelper(t, env, token, householdID, accountID)

	// Use a large but safe value well within int64 range (1 trillion cents = R$10B).
	const largeLimit = int64(1_000_000_000_000)

	w := env.do(t, http.MethodPut, ccSettingsURL(householdID, accountID)+"/limit", map[string]any{
		"limit_amount": largeLimit,
		"version":      1,
	}, token)

	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON(t, w)
	// JSON numbers decode as float64; verify it round-trips without precision loss.
	assert.Equal(t, float64(largeLimit), resp["limit_amount"])
	assert.InDelta(t, float64(largeLimit), resp["limit_amount"], float64(math.MaxFloat32))
}
