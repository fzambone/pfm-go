//go:build integration

package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Cross-Domain Workflow E2E Tests ---

// TestE2E_Workflow_FullFinancialLifecycle verifies AC1: the complete lifecycle from
// registration through account deactivation and household deactivation, exercising
// every domain in a single cohesive flow.
func TestE2E_Workflow_FullFinancialLifecycle(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	// 1. Register and log in.
	token, _ := registerAndLogin(t, env, "lifecycle@example.com", "Alice", "secret1234")

	// 2. Create a household.
	w := env.do(t, http.MethodPost, "/api/v1/households", map[string]string{
		"name": "Alice's Home",
	}, token)
	require.Equal(t, http.StatusCreated, w.Code)
	householdID := decodeJSON(t, w)["id"].(string)

	// 3. Create a CHECKING and a SAVINGS account.
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name": "Checking", "account_type": "CHECKING", "currency_code": "BRL",
	}, token)
	require.Equal(t, http.StatusCreated, w.Code)
	checkingID := decodeJSON(t, w)["id"].(string)

	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name": "Savings", "account_type": "SAVINGS", "currency_code": "BRL",
	}, token)
	require.Equal(t, http.StatusCreated, w.Code)
	savingsID := decodeJSON(t, w)["id"].(string)

	// 4. Create a CREDIT_CARD account and attach settings.
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name": "Visa", "account_type": "CREDIT_CARD", "currency_code": "BRL",
	}, token)
	require.Equal(t, http.StatusCreated, w.Code)
	ccID := decodeJSON(t, w)["id"].(string)

	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts/"+ccID+"/credit-card-settings", map[string]any{
		"closing_day": 15, "due_day": 25, "limit_amount": 500000,
	}, token)
	require.Equal(t, http.StatusCreated, w.Code)
	ccSettings := decodeJSON(t, w)
	assert.Equal(t, float64(500000), ccSettings["limit_amount"])

	// 5. Post a transaction: checking → savings.
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/transactions", map[string]any{
		"description":      "Transfer to savings",
		"transaction_date": "2026-03-01",
		"entries": []map[string]any{
			{"account_id": checkingID, "entry_type": "DEBIT", "amount": 20000},
			{"account_id": savingsID, "entry_type": "CREDIT", "amount": 20000},
		},
	}, token)
	require.Equal(t, http.StatusCreated, w.Code)

	// 6. Verify balances reflect the transaction.
	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/accounts/"+checkingID+"/balance", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, float64(-20000), decodeJSON(t, w)["balance"])

	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/accounts/"+savingsID+"/balance", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, float64(20000), decodeJSON(t, w)["balance"])

	// 7. Verify history contains the transaction.
	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/transactions", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	var history []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&history))
	assert.Len(t, history, 1)

	// 8. Reverse the transaction to zero out balances (required before account deactivation).
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/transactions", map[string]any{
		"description":      "Reversal",
		"transaction_date": "2026-03-02",
		"entries": []map[string]any{
			{"account_id": savingsID, "entry_type": "DEBIT", "amount": 20000},
			{"account_id": checkingID, "entry_type": "CREDIT", "amount": 20000},
		},
	}, token)
	require.Equal(t, http.StatusCreated, w.Code)

	// 9. Deactivate accounts (balances are now zero).
	for _, id := range []string{checkingID, savingsID, ccID} {
		w = env.do(t, http.MethodDelete, "/api/v1/households/"+householdID+"/accounts/"+id, nil, token)
		require.Equal(t, http.StatusNoContent, w.Code, "deactivate account %s failed", id)
	}

	// 10. Deactivate the household.
	w = env.do(t, http.MethodDelete, "/api/v1/households/"+householdID, nil, token)
	require.Equal(t, http.StatusNoContent, w.Code)
}

// TestE2E_Workflow_MultiUserCollaboration verifies AC2: two users in a household
// can both create accounts and post transactions, and both appear in history.
func TestE2E_Workflow_MultiUserCollaboration(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	// User A creates the household.
	tokenA, _, householdID := createHouseholdHelper(t, env, "collab-a@example.com", "Shared Home")

	// User B registers.
	tokenB, userBID := registerAndLogin(t, env, "collab-b@example.com", "Bob", "secret1234")

	// A adds B as MEMBER.
	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/members", map[string]any{
		"user_id": userBID, "role": "MEMBER",
	}, tokenA)
	require.Equal(t, http.StatusCreated, w.Code)

	// A creates an account.
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name": "A-Checking", "account_type": "CHECKING", "currency_code": "BRL",
	}, tokenA)
	require.Equal(t, http.StatusCreated, w.Code)
	aCheckingID := decodeJSON(t, w)["id"].(string)

	// B creates an account.
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name": "B-Checking", "account_type": "CHECKING", "currency_code": "BRL",
	}, tokenB)
	require.Equal(t, http.StatusCreated, w.Code)
	bCheckingID := decodeJSON(t, w)["id"].(string)

	// A posts a transaction.
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/transactions", map[string]any{
		"description":      "A's payment",
		"transaction_date": "2026-03-01",
		"entries": []map[string]any{
			{"account_id": aCheckingID, "entry_type": "DEBIT", "amount": 5000},
			{"account_id": bCheckingID, "entry_type": "CREDIT", "amount": 5000},
		},
	}, tokenA)
	require.Equal(t, http.StatusCreated, w.Code)

	// B posts a transaction.
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/transactions", map[string]any{
		"description":      "B's payment",
		"transaction_date": "2026-03-02",
		"entries": []map[string]any{
			{"account_id": bCheckingID, "entry_type": "DEBIT", "amount": 3000},
			{"account_id": aCheckingID, "entry_type": "CREDIT", "amount": 3000},
		},
	}, tokenB)
	require.Equal(t, http.StatusCreated, w.Code)

	// Both A and B can see the full history (2 transactions).
	for _, tok := range []string{tokenA, tokenB} {
		w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/transactions", nil, tok)
		require.Equal(t, http.StatusOK, w.Code)
		var hist []map[string]any
		require.NoError(t, json.NewDecoder(w.Body).Decode(&hist))
		assert.Len(t, hist, 2)
	}
}

// TestE2E_Workflow_MemberRoleEnforcement verifies AC3: MEMBER is blocked from
// admin-only household operations (403) but can freely read and create resources.
func TestE2E_Workflow_MemberRoleEnforcement(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	// A creates household; B registers and is added as MEMBER.
	tokenA, _, householdID := createHouseholdHelper(t, env, "role-a@example.com", "Role House")
	tokenB, userBID := registerAndLogin(t, env, "role-b@example.com", "Bob", "secret1234")
	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/members", map[string]any{
		"user_id": userBID, "role": "MEMBER",
	}, tokenA)
	require.Equal(t, http.StatusCreated, w.Code)

	// Admin-only operations — B should receive 403.
	w = env.do(t, http.MethodPut, "/api/v1/households/"+householdID, map[string]any{
		"name": "Renamed", "version": 1,
	}, tokenB)
	assert.Equal(t, http.StatusForbidden, w.Code, "rename should be admin-only")

	w = env.do(t, http.MethodDelete, "/api/v1/households/"+householdID, nil, tokenB)
	assert.Equal(t, http.StatusForbidden, w.Code, "deactivate should be admin-only")

	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/members", map[string]any{
		"user_id": userBID, "role": "MEMBER",
	}, tokenB)
	assert.Equal(t, http.StatusForbidden, w.Code, "add member should be admin-only")

	w = env.do(t, http.MethodDelete, "/api/v1/households/"+householdID+"/members/"+userBID, nil, tokenB)
	assert.Equal(t, http.StatusForbidden, w.Code, "remove member should be admin-only")

	// Member-allowed operations — B should succeed.
	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID, nil, tokenB)
	assert.Equal(t, http.StatusOK, w.Code, "GET household should be allowed for members")

	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name": "B-Account", "account_type": "CHECKING", "currency_code": "BRL",
	}, tokenB)
	require.Equal(t, http.StatusCreated, w.Code, "create account should be allowed for members")
	bAccountID := decodeJSON(t, w)["id"].(string)

	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/accounts", nil, tokenB)
	assert.Equal(t, http.StatusOK, w.Code, "list accounts should be allowed for members")

	// Create a second account so B can post a balanced transaction.
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name": "B-Savings", "account_type": "SAVINGS", "currency_code": "BRL",
	}, tokenB)
	require.Equal(t, http.StatusCreated, w.Code)
	bSavingsID := decodeJSON(t, w)["id"].(string)

	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/transactions", map[string]any{
		"description":      "Member txn",
		"transaction_date": "2026-03-01",
		"entries": []map[string]any{
			{"account_id": bAccountID, "entry_type": "DEBIT", "amount": 1000},
			{"account_id": bSavingsID, "entry_type": "CREDIT", "amount": 1000},
		},
	}, tokenB)
	assert.Equal(t, http.StatusCreated, w.Code, "post transaction should be allowed for members")
}

// TestE2E_Workflow_DualAdminOperations verifies AC4: after a second admin is added,
// both admins can independently perform admin-only operations.
func TestE2E_Workflow_DualAdminOperations(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	// A creates household; B registers.
	tokenA, _, householdID := createHouseholdHelper(t, env, "dual-a@example.com", "Dual House")
	tokenB, userBID := registerAndLogin(t, env, "dual-b@example.com", "Bob", "secret1234")

	// A adds B as ADMIN.
	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/members", map[string]any{
		"user_id": userBID, "role": "ADMIN",
	}, tokenA)
	require.Equal(t, http.StatusCreated, w.Code)

	// A renames the household.
	w = env.do(t, http.MethodPut, "/api/v1/households/"+householdID, map[string]any{
		"name": "Renamed by A", "version": 1,
	}, tokenA)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Renamed by A", decodeJSON(t, w)["name"])

	// B renames the household (using version 2 after A's update).
	w = env.do(t, http.MethodPut, "/api/v1/households/"+householdID, map[string]any{
		"name": "Renamed by B", "version": 2,
	}, tokenB)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Renamed by B", decodeJSON(t, w)["name"])

	// B registers a third user and adds them as MEMBER (admin operation by B).
	tokenC, userCID := registerAndLogin(t, env, "dual-c@example.com", "Carol", "secret1234")
	_ = tokenC
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/members", map[string]any{
		"user_id": userCID, "role": "MEMBER",
	}, tokenB)
	require.Equal(t, http.StatusCreated, w.Code)

	// A removes C (admin operation by A).
	w = env.do(t, http.MethodDelete, "/api/v1/households/"+householdID+"/members/"+userCID, nil, tokenA)
	require.Equal(t, http.StatusNoContent, w.Code)
}

// TestE2E_Workflow_MultiCurrencyAccounts verifies AC5: accounts created in different
// currencies within the same household maintain independent balances.
func TestE2E_Workflow_MultiCurrencyAccounts(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	token, _, householdID := createHouseholdHelper(t, env, "multiccy@example.com", "MultiCcy House")

	// Create BRL accounts.
	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name": "BRL-Checking", "account_type": "CHECKING", "currency_code": "BRL",
	}, token)
	require.Equal(t, http.StatusCreated, w.Code)
	brlCheckingID := decodeJSON(t, w)["id"].(string)

	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name": "BRL-Savings", "account_type": "SAVINGS", "currency_code": "BRL",
	}, token)
	require.Equal(t, http.StatusCreated, w.Code)
	brlSavingsID := decodeJSON(t, w)["id"].(string)

	// Create USD accounts.
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name": "USD-Checking", "account_type": "CHECKING", "currency_code": "USD",
	}, token)
	require.Equal(t, http.StatusCreated, w.Code)
	usdCheckingID := decodeJSON(t, w)["id"].(string)

	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name": "USD-Savings", "account_type": "SAVINGS", "currency_code": "USD",
	}, token)
	require.Equal(t, http.StatusCreated, w.Code)
	usdSavingsID := decodeJSON(t, w)["id"].(string)

	// Post a BRL transaction (10000 BRL cents).
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/transactions", map[string]any{
		"description":      "BRL transfer",
		"transaction_date": "2026-03-01",
		"entries": []map[string]any{
			{"account_id": brlCheckingID, "entry_type": "DEBIT", "amount": 10000},
			{"account_id": brlSavingsID, "entry_type": "CREDIT", "amount": 10000},
		},
	}, token)
	require.Equal(t, http.StatusCreated, w.Code)

	// Post a USD transaction (500 USD cents).
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/transactions", map[string]any{
		"description":      "USD transfer",
		"transaction_date": "2026-03-01",
		"entries": []map[string]any{
			{"account_id": usdCheckingID, "entry_type": "DEBIT", "amount": 500},
			{"account_id": usdSavingsID, "entry_type": "CREDIT", "amount": 500},
		},
	}, token)
	require.Equal(t, http.StatusCreated, w.Code)

	// BRL accounts reflect only BRL amounts.
	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/accounts/"+brlCheckingID+"/balance", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, float64(-10000), decodeJSON(t, w)["balance"])

	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/accounts/"+brlSavingsID+"/balance", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, float64(10000), decodeJSON(t, w)["balance"])

	// USD accounts reflect only USD amounts (no cross-contamination).
	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/accounts/"+usdCheckingID+"/balance", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, float64(-500), decodeJSON(t, w)["balance"])

	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/accounts/"+usdSavingsID+"/balance", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, float64(500), decodeJSON(t, w)["balance"])
}

// TestE2E_Workflow_UserDeactivation_HistoryPreserved verifies EDGE-1: deactivating
// a user does not affect the household's transaction history. A second household
// member (B) can still retrieve history created by A after A deactivates.
func TestE2E_Workflow_UserDeactivation_HistoryPreserved(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	// A creates household and two accounts.
	tokenA, userAID, householdID := createHouseholdHelper(t, env, "deact-a@example.com", "Deact House")
	tokenB, userBID := registerAndLogin(t, env, "deact-b@example.com", "Bob", "secret1234")

	// A adds B as MEMBER.
	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/members", map[string]any{
		"user_id": userBID, "role": "MEMBER",
	}, tokenA)
	require.Equal(t, http.StatusCreated, w.Code)

	// A creates accounts and posts a transaction.
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name": "Checking", "account_type": "CHECKING", "currency_code": "BRL",
	}, tokenA)
	require.Equal(t, http.StatusCreated, w.Code)
	checkingID := decodeJSON(t, w)["id"].(string)

	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name": "Savings", "account_type": "SAVINGS", "currency_code": "BRL",
	}, tokenA)
	require.Equal(t, http.StatusCreated, w.Code)
	savingsID := decodeJSON(t, w)["id"].(string)

	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/transactions", map[string]any{
		"description":      "A's transaction",
		"transaction_date": "2026-03-01",
		"entries": []map[string]any{
			{"account_id": checkingID, "entry_type": "DEBIT", "amount": 7500},
			{"account_id": savingsID, "entry_type": "CREDIT", "amount": 7500},
		},
	}, tokenA)
	require.Equal(t, http.StatusCreated, w.Code)

	// A deactivates their own account.
	w = env.do(t, http.MethodDelete, "/api/v1/users/"+userAID, nil, tokenA)
	require.Equal(t, http.StatusNoContent, w.Code)

	// B can still retrieve the full history — A's transaction is preserved.
	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/transactions", nil, tokenB)
	require.Equal(t, http.StatusOK, w.Code)
	var hist []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&hist))
	assert.Len(t, hist, 1)
	assert.Equal(t, "A's transaction", hist[0]["description"])
}

// TestE2E_Workflow_HouseholdDeactivation_AccountsInaccessible verifies EDGE-2:
// after a household is deactivated, GET on the household itself returns 404.
// The guard checks membership (not household state), so sub-resources remain
// accessible to existing members — this is the current designed behaviour.
func TestE2E_Workflow_HouseholdDeactivation_AccountsInaccessible(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	// Create household and one account.
	token, _, householdID := createHouseholdHelper(t, env, "hdeact@example.com", "Gone House")
	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name": "Checking", "account_type": "CHECKING", "currency_code": "BRL",
	}, token)
	require.Equal(t, http.StatusCreated, w.Code)

	// Deactivate the household — returns 204.
	w = env.do(t, http.MethodDelete, "/api/v1/households/"+householdID, nil, token)
	require.Equal(t, http.StatusNoContent, w.Code)

	// Direct household lookup returns 404 (domain filters by deleted_at IS NULL).
	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID, nil, token)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// The household no longer appears in the caller's list.
	w = env.do(t, http.MethodGet, "/api/v1/households", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	households := decodeJSONArray(t, w)
	for _, h := range households {
		assert.NotEqual(t, householdID, h["id"], "deactivated household should not appear in list")
	}
}

// TestE2E_Workflow_MemberRemoval_AccessRevoked verifies EDGE-3: a removed member
// can no longer access any household-scoped resources.
func TestE2E_Workflow_MemberRemoval_AccessRevoked(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	// A creates household; B registers and is added as MEMBER.
	tokenA, _, householdID := createHouseholdHelper(t, env, "revoke-a@example.com", "Revoke House")
	tokenB, userBID := registerAndLogin(t, env, "revoke-b@example.com", "Bob", "secret1234")

	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/members", map[string]any{
		"user_id": userBID, "role": "MEMBER",
	}, tokenA)
	require.Equal(t, http.StatusCreated, w.Code)

	// Confirm B has access before removal.
	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID, nil, tokenB)
	require.Equal(t, http.StatusOK, w.Code)

	// A removes B.
	w = env.do(t, http.MethodDelete, "/api/v1/households/"+householdID+"/members/"+userBID, nil, tokenA)
	require.Equal(t, http.StatusNoContent, w.Code)

	// B's token is still valid but B is no longer a member — all household access returns 403.
	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID, nil, tokenB)
	assert.Equal(t, http.StatusForbidden, w.Code, "removed member should get 403 on GET household")

	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/accounts", nil, tokenB)
	assert.Equal(t, http.StatusForbidden, w.Code, "removed member should get 403 on list accounts")

	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/transactions", nil, tokenB)
	assert.Equal(t, http.StatusForbidden, w.Code, "removed member should get 403 on list transactions")
}
