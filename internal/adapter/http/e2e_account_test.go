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

// createAccountHelper registers and logs in a user, creates a household, then
// creates an account within that household. Returns token, householdID, and accountID.
func createAccountHelper(t *testing.T, env *e2eEnv, email, householdName, accountName string) (token, householdID, accountID string) {
	t.Helper()
	token, _, householdID = createHouseholdHelper(t, env, email, householdName)
	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name":          accountName,
		"account_type":  "CHECKING",
		"currency_code": "BRL",
	}, token)
	require.Equal(t, http.StatusCreated, w.Code, "create account failed: %s", w.Body.String())
	a := decodeJSON(t, w)
	accountID = a["id"].(string)
	return token, householdID, accountID
}

// --- Account Domain E2E Tests ---

// TestE2E_Account_Create_Success verifies AC1: valid input returns 201 with
// server-assigned ID and zero balance.
func TestE2E_Account_Create_Success(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _, householdID := createHouseholdHelper(t, env, "a1@example.com", "House A1")

	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name":          "Checking",
		"account_type":  "CHECKING",
		"currency_code": "BRL",
	}, token)

	require.Equal(t, http.StatusCreated, w.Code)
	resp := decodeJSON(t, w)
	assert.NotEmpty(t, resp["id"])
	assert.Equal(t, "Checking", resp["name"])
	assert.Equal(t, "CHECKING", resp["account_type"])
	assert.Equal(t, "BRL", resp["currency_code"])
	assert.Equal(t, float64(0), resp["balance"])
	assert.NotEmpty(t, w.Header().Get("Location"))
}

// TestE2E_Account_Create_DuplicateName verifies AC2: duplicate name in the same
// household returns 409.
func TestE2E_Account_Create_DuplicateName(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _, householdID := createHouseholdHelper(t, env, "a2@example.com", "House A2")

	// First creation succeeds.
	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name":          "Dup",
		"account_type":  "CHECKING",
		"currency_code": "BRL",
	}, token)
	require.Equal(t, http.StatusCreated, w.Code)

	// Second creation with same name → 409.
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name":          "Dup",
		"account_type":  "CHECKING",
		"currency_code": "BRL",
	}, token)
	require.Equal(t, http.StatusConflict, w.Code)
}

// TestE2E_Account_Create_ValidationError_EmptyName verifies AC3: empty name returns 400.
func TestE2E_Account_Create_ValidationError_EmptyName(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _, householdID := createHouseholdHelper(t, env, "a3a@example.com", "House A3a")

	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name":          "",
		"account_type":  "CHECKING",
		"currency_code": "BRL",
	}, token)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestE2E_Account_Create_ValidationError_InvalidType verifies AC3: invalid account
// type returns 400.
func TestE2E_Account_Create_ValidationError_InvalidType(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _, householdID := createHouseholdHelper(t, env, "a3b@example.com", "House A3b")

	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name":          "Acct",
		"account_type":  "INVALID_TYPE",
		"currency_code": "BRL",
	}, token)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestE2E_Account_Create_ValidationError_InvalidCurrency verifies AC3: invalid
// currency code returns 400.
func TestE2E_Account_Create_ValidationError_InvalidCurrency(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _, householdID := createHouseholdHelper(t, env, "a3c@example.com", "House A3c")

	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name":          "Acct",
		"account_type":  "CHECKING",
		"currency_code": "XXX",
	}, token)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestE2E_Account_GetByID verifies AC4: member retrieves an account by ID and
// receives correct data.
func TestE2E_Account_GetByID(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, householdID, accountID := createAccountHelper(t, env, "a4@example.com", "House A4", "Checking")

	w := env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/accounts/"+accountID, nil, token)

	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON(t, w)
	assert.Equal(t, accountID, resp["id"])
	assert.Equal(t, "Checking", resp["name"])
	assert.Equal(t, householdID, resp["household_id"])
}

// TestE2E_Account_GetByID_NotFound verifies AC5: non-existent account ID returns 404.
func TestE2E_Account_GetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _, householdID := createHouseholdHelper(t, env, "a5@example.com", "House A5")

	w := env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/accounts/"+uuid.New().String(), nil, token)
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestE2E_Account_List verifies AC6: listing accounts returns all active accounts
// for the household.
func TestE2E_Account_List(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _, householdID := createHouseholdHelper(t, env, "a6@example.com", "House A6")

	// Create two accounts.
	env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name": "Checking", "account_type": "CHECKING", "currency_code": "BRL",
	}, token)
	env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name": "Savings", "account_type": "SAVINGS", "currency_code": "BRL",
	}, token)

	w := env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/accounts", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	list := decodeJSONArray(t, w)
	assert.Len(t, list, 2)
}

// TestE2E_Account_List_Empty verifies AC7: a household with no accounts returns [].
func TestE2E_Account_List_Empty(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _, householdID := createHouseholdHelper(t, env, "a7@example.com", "House A7")

	w := env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/accounts", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	list := decodeJSONArray(t, w)
	assert.Empty(t, list)
}

// TestE2E_Account_UpdateName verifies AC8: updating an account name with the
// correct version returns 200 with the updated name and incremented version.
func TestE2E_Account_UpdateName(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, householdID, accountID := createAccountHelper(t, env, "a8@example.com", "House A8", "Old Name")

	w := env.do(t, http.MethodPut, "/api/v1/households/"+householdID+"/accounts/"+accountID+"/name", map[string]any{
		"name": "New Name", "version": 1,
	}, token)

	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON(t, w)
	assert.Equal(t, "New Name", resp["name"])
	assert.Equal(t, float64(2), resp["version"])
}

// TestE2E_Account_UpdateName_StaleVersion verifies AC9: a stale version on name
// update returns 409.
func TestE2E_Account_UpdateName_StaleVersion(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, householdID, accountID := createAccountHelper(t, env, "a9@example.com", "House A9", "Acct")

	// First update succeeds (version 1 → 2).
	env.do(t, http.MethodPut, "/api/v1/households/"+householdID+"/accounts/"+accountID+"/name", map[string]any{
		"name": "V2", "version": 1,
	}, token)

	// Second update with stale version 1 → 409.
	w := env.do(t, http.MethodPut, "/api/v1/households/"+householdID+"/accounts/"+accountID+"/name", map[string]any{
		"name": "V3", "version": 1,
	}, token)
	require.Equal(t, http.StatusConflict, w.Code)
}

// TestE2E_Account_UpdateBalance verifies AC10: updating an account balance with
// the correct version returns 200 with the new balance.
func TestE2E_Account_UpdateBalance(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, householdID, accountID := createAccountHelper(t, env, "a10@example.com", "House A10", "Checking")

	w := env.do(t, http.MethodPut, "/api/v1/households/"+householdID+"/accounts/"+accountID+"/balance", map[string]any{
		"balance": 75000, "version": 1,
	}, token)

	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON(t, w)
	assert.Equal(t, float64(75000), resp["balance"])
	assert.Equal(t, float64(2), resp["version"])
}

// TestE2E_Account_Deactivate verifies AC11: deactivating an account with zero
// balance returns 204 and subsequent GET returns 404.
func TestE2E_Account_Deactivate(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, householdID, accountID := createAccountHelper(t, env, "a11@example.com", "House A11", "Checking")

	w := env.do(t, http.MethodDelete, "/api/v1/households/"+householdID+"/accounts/"+accountID, nil, token)
	require.Equal(t, http.StatusNoContent, w.Code)

	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/accounts/"+accountID, nil, token)
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestE2E_Account_Deactivate_BalanceNotZero verifies AC12: deactivating an account
// with a non-zero balance returns 409.
func TestE2E_Account_Deactivate_BalanceNotZero(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, householdID, accountID := createAccountHelper(t, env, "a12@example.com", "House A12", "Checking")

	// Set a non-zero balance.
	w := env.do(t, http.MethodPut, "/api/v1/households/"+householdID+"/accounts/"+accountID+"/balance", map[string]any{
		"balance": 5000, "version": 1,
	}, token)
	require.Equal(t, http.StatusOK, w.Code)
	updated := decodeJSON(t, w)

	// Deactivate with non-zero balance → 409.
	w = env.do(t, http.MethodDelete, "/api/v1/households/"+householdID+"/accounts/"+accountID, nil, token)
	require.Equal(t, http.StatusConflict, w.Code)

	// Account is still accessible.
	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/accounts/"+accountID, nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON(t, w)
	assert.Equal(t, updated["balance"], resp["balance"])
}

// TestE2E_Account_Create_DifferentTypes verifies edge case: all four account types
// are accepted.
func TestE2E_Account_Create_DifferentTypes(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _, householdID := createHouseholdHelper(t, env, "a-types@example.com", "TypesHouse")

	types := []string{"CHECKING", "SAVINGS", "CREDIT_CARD", "INVESTMENT"}
	for _, at := range types {
		w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
			"name":          at + " Account",
			"account_type":  at,
			"currency_code": "BRL",
		}, token)
		require.Equal(t, http.StatusCreated, w.Code, "expected 201 for type %s: %s", at, w.Body.String())
		resp := decodeJSON(t, w)
		assert.Equal(t, at, resp["account_type"])
	}
}

// TestE2E_Account_Create_DifferentCurrencies verifies edge case: USD, BRL, and EUR
// are all accepted.
func TestE2E_Account_Create_DifferentCurrencies(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _, householdID := createHouseholdHelper(t, env, "a-currencies@example.com", "CurrenciesHouse")

	currencies := []string{"USD", "BRL", "EUR"}
	for _, cc := range currencies {
		w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
			"name":          cc + " Account",
			"account_type":  "CHECKING",
			"currency_code": cc,
		}, token)
		require.Equal(t, http.StatusCreated, w.Code, "expected 201 for currency %s: %s", cc, w.Body.String())
		resp := decodeJSON(t, w)
		assert.Equal(t, cc, resp["currency_code"])
	}
}

// TestE2E_Account_List_AfterDeactivate verifies edge case: listing accounts after
// deactivating one returns only the active accounts.
func TestE2E_Account_List_AfterDeactivate(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, _, householdID := createHouseholdHelper(t, env, "a-listdeact@example.com", "ListDeactHouse")

	// Create two accounts.
	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name": "Keep", "account_type": "CHECKING", "currency_code": "BRL",
	}, token)
	require.Equal(t, http.StatusCreated, w.Code)

	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name": "Remove", "account_type": "SAVINGS", "currency_code": "BRL",
	}, token)
	require.Equal(t, http.StatusCreated, w.Code)
	removeID := decodeJSON(t, w)["id"].(string)

	// Deactivate one.
	w = env.do(t, http.MethodDelete, "/api/v1/households/"+householdID+"/accounts/"+removeID, nil, token)
	require.Equal(t, http.StatusNoContent, w.Code)

	// List should only contain the active account.
	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/accounts", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	list := decodeJSONArray(t, w)
	assert.Len(t, list, 1)
	assert.Equal(t, "Keep", list[0]["name"])
}

// TestE2E_Account_UpdateBalance_Negative verifies edge case: the domain accepts a
// negative balance (representing an overdraft or owed amount).
func TestE2E_Account_UpdateBalance_Negative(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	token, householdID, accountID := createAccountHelper(t, env, "a-neg@example.com", "NegHouse", "Checking")

	w := env.do(t, http.MethodPut, "/api/v1/households/"+householdID+"/accounts/"+accountID+"/balance", map[string]any{
		"balance": -10000, "version": 1,
	}, token)

	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON(t, w)
	assert.Equal(t, float64(-10000), resp["balance"])
}
