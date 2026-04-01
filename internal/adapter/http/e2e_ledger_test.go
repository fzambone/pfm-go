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

// ledgerSetup holds the common resources for ledger E2E tests.
type ledgerSetup struct {
	token      string
	householdID string
	checkingID  string
	savingsID   string
}

// createLedgerSetupHelper creates a household with two accounts (CHECKING and SAVINGS).
// Returns a ledgerSetup with token, householdID, checkingID, savingsID.
func createLedgerSetupHelper(t *testing.T, env *e2eEnv, email, householdName string) ledgerSetup {
	t.Helper()
	token, _, householdID := createHouseholdHelper(t, env, email, householdName)

	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name":          "Checking",
		"account_type":  "CHECKING",
		"currency_code": "BRL",
	}, token)
	require.Equal(t, http.StatusCreated, w.Code, "create checking account failed: %s", w.Body.String())
	checkingID := decodeJSON(t, w)["id"].(string)

	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name":          "Savings",
		"account_type":  "SAVINGS",
		"currency_code": "BRL",
	}, token)
	require.Equal(t, http.StatusCreated, w.Code, "create savings account failed: %s", w.Body.String())
	savingsID := decodeJSON(t, w)["id"].(string)

	return ledgerSetup{
		token:       token,
		householdID: householdID,
		checkingID:  checkingID,
		savingsID:   savingsID,
	}
}

// postTransactionHelper posts a balanced 2-entry transaction (debit from, credit to) and
// fails the test if the response is not 201. Returns the decoded response body.
func postTransactionHelper(t *testing.T, env *e2eEnv, setup ledgerSetup, debitAccountID, creditAccountID string, amount int64, description, date string) map[string]any {
	t.Helper()
	w := env.do(t, http.MethodPost, "/api/v1/households/"+setup.householdID+"/transactions", map[string]any{
		"description":      description,
		"transaction_date": date,
		"entries": []map[string]any{
			{"account_id": debitAccountID, "entry_type": "DEBIT", "amount": amount},
			{"account_id": creditAccountID, "entry_type": "CREDIT", "amount": amount},
		},
	}, setup.token)
	require.Equal(t, http.StatusCreated, w.Code, "post transaction failed: %s", w.Body.String())
	return decodeJSON(t, w)
}

// txnURL returns the transactions URL for a household.
func txnURL(householdID string) string {
	return "/api/v1/households/" + householdID + "/transactions"
}

// balanceURL returns the balance URL for an account within a household.
func balanceURL(householdID, accountID string) string {
	return "/api/v1/households/" + householdID + "/accounts/" + accountID + "/balance"
}

// --- Ledger Domain E2E Tests ---

// TestE2E_Ledger_PostTransaction_Success verifies AC1: a balanced transaction
// returns 201 with the transaction and all its entries.
func TestE2E_Ledger_PostTransaction_Success(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	setup := createLedgerSetupHelper(t, env, "ledger1@example.com", "Ledger House 1")

	w := env.do(t, http.MethodPost, txnURL(setup.householdID), map[string]any{
		"description":      "Transfer to savings",
		"transaction_date": "2026-03-15",
		"entries": []map[string]any{
			{"account_id": setup.checkingID, "entry_type": "DEBIT", "amount": 10000},
			{"account_id": setup.savingsID, "entry_type": "CREDIT", "amount": 10000},
		},
	}, setup.token)

	require.Equal(t, http.StatusCreated, w.Code)
	resp := decodeJSON(t, w)
	assert.NotEmpty(t, resp["id"])
	assert.Equal(t, setup.householdID, resp["household_id"])
	assert.Equal(t, "Transfer to savings", resp["description"])
	entries, ok := resp["entries"].([]any)
	require.True(t, ok)
	assert.Len(t, entries, 2)
}

// TestE2E_Ledger_PostTransaction_Unbalanced verifies AC2: a transaction where
// total debits ≠ total credits returns 422.
func TestE2E_Ledger_PostTransaction_Unbalanced(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	setup := createLedgerSetupHelper(t, env, "ledger2@example.com", "Ledger House 2")

	w := env.do(t, http.MethodPost, txnURL(setup.householdID), map[string]any{
		"description":      "Bad transfer",
		"transaction_date": "2026-03-15",
		"entries": []map[string]any{
			{"account_id": setup.checkingID, "entry_type": "DEBIT", "amount": 10000},
			{"account_id": setup.savingsID, "entry_type": "CREDIT", "amount": 5000},
		},
	}, setup.token)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// TestE2E_Ledger_PostTransaction_ValidationError verifies AC3: a transaction with
// an empty description returns 400.
func TestE2E_Ledger_PostTransaction_ValidationError_EmptyDescription(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	setup := createLedgerSetupHelper(t, env, "ledger3a@example.com", "Ledger House 3a")

	w := env.do(t, http.MethodPost, txnURL(setup.householdID), map[string]any{
		"description":      "",
		"transaction_date": "2026-03-15",
		"entries": []map[string]any{
			{"account_id": setup.checkingID, "entry_type": "DEBIT", "amount": 10000},
			{"account_id": setup.savingsID, "entry_type": "CREDIT", "amount": 10000},
		},
	}, setup.token)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestE2E_Ledger_PostTransaction_ValidationError_NoEntries verifies AC3: a transaction
// with no entries returns 400.
func TestE2E_Ledger_PostTransaction_ValidationError_NoEntries(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	setup := createLedgerSetupHelper(t, env, "ledger3b@example.com", "Ledger House 3b")

	w := env.do(t, http.MethodPost, txnURL(setup.householdID), map[string]any{
		"description":      "No entries",
		"transaction_date": "2026-03-15",
		"entries":          []map[string]any{},
	}, setup.token)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestE2E_Ledger_GetBalance_AfterMultipleTransactions verifies AC4: the balance
// reflects the cumulative net of all debits and credits.
func TestE2E_Ledger_GetBalance_AfterMultipleTransactions(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	setup := createLedgerSetupHelper(t, env, "ledger4@example.com", "Ledger House 4")

	// Two transfers: checking -10000, savings +10000, then checking -5000, savings +5000.
	postTransactionHelper(t, env, setup, setup.checkingID, setup.savingsID, 10000, "Transfer 1", "2026-03-01")
	postTransactionHelper(t, env, setup, setup.checkingID, setup.savingsID, 5000, "Transfer 2", "2026-03-02")

	w := env.do(t, http.MethodGet, balanceURL(setup.householdID, setup.checkingID), nil, setup.token)
	require.Equal(t, http.StatusOK, w.Code)
	bal := decodeJSON(t, w)
	assert.Equal(t, float64(-15000), bal["balance"])

	w = env.do(t, http.MethodGet, balanceURL(setup.householdID, setup.savingsID), nil, setup.token)
	require.Equal(t, http.StatusOK, w.Code)
	bal = decodeJSON(t, w)
	assert.Equal(t, float64(15000), bal["balance"])
}

// TestE2E_Ledger_GetBalance_NoEntries_ReturnsZero verifies AC5: querying the balance
// for an account with no entries returns zero (not an error).
func TestE2E_Ledger_GetBalance_NoEntries_ReturnsZero(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	setup := createLedgerSetupHelper(t, env, "ledger5@example.com", "Ledger House 5")

	w := env.do(t, http.MethodGet, balanceURL(setup.householdID, setup.checkingID), nil, setup.token)

	require.Equal(t, http.StatusOK, w.Code)
	bal := decodeJSON(t, w)
	assert.Equal(t, float64(0), bal["balance"])
}

// TestE2E_Ledger_GetHistory_ReturnsAllTransactions verifies AC6: history returns all
// transactions with their entries for the household.
func TestE2E_Ledger_GetHistory_ReturnsAllTransactions(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	setup := createLedgerSetupHelper(t, env, "ledger6@example.com", "Ledger House 6")

	postTransactionHelper(t, env, setup, setup.checkingID, setup.savingsID, 10000, "Txn 1", "2026-03-01")
	postTransactionHelper(t, env, setup, setup.checkingID, setup.savingsID, 5000, "Txn 2", "2026-03-02")

	w := env.do(t, http.MethodGet, txnURL(setup.householdID), nil, setup.token)

	require.Equal(t, http.StatusOK, w.Code)
	var history []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&history))
	assert.Len(t, history, 2)
	for _, txn := range history {
		entries, ok := txn["entries"].([]any)
		require.True(t, ok)
		assert.Len(t, entries, 2)
	}
}

// TestE2E_Ledger_GetHistory_FilterByAccountID verifies AC7: the account_id query
// parameter filters the history to transactions involving that account only.
func TestE2E_Ledger_GetHistory_FilterByAccountID(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	setup := createLedgerSetupHelper(t, env, "ledger7@example.com", "Ledger House 7")

	// Create a third account — only used in one transaction.
	w := env.do(t, http.MethodPost, "/api/v1/households/"+setup.householdID+"/accounts", map[string]string{
		"name":          "Other",
		"account_type":  "SAVINGS",
		"currency_code": "BRL",
	}, setup.token)
	require.Equal(t, http.StatusCreated, w.Code)
	otherID := decodeJSON(t, w)["id"].(string)

	// Transaction A: checking ↔ savings (involves checkingID).
	postTransactionHelper(t, env, setup, setup.checkingID, setup.savingsID, 10000, "Txn A", "2026-03-01")
	// Transaction B: checking ↔ other (involves checkingID and otherID, not savingsID).
	postTransactionHelper(t, env, setup, setup.checkingID, otherID, 3000, "Txn B", "2026-03-02")

	// Filter by savingsID — only transaction A should appear.
	w = env.do(t, http.MethodGet, txnURL(setup.householdID)+"?account_id="+setup.savingsID, nil, setup.token)
	require.Equal(t, http.StatusOK, w.Code)
	var filtered []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&filtered))
	assert.Len(t, filtered, 1)
	assert.Equal(t, "Txn A", filtered[0]["description"])
}

// TestE2E_Ledger_GetHistory_Pagination verifies AC8: limit and offset query
// parameters are respected.
func TestE2E_Ledger_GetHistory_Pagination(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	setup := createLedgerSetupHelper(t, env, "ledger8@example.com", "Ledger House 8")

	// Post 3 transactions.
	for i := range 3 {
		postTransactionHelper(t, env, setup, setup.checkingID, setup.savingsID, int64(1000*(i+1)), "Txn", "2026-03-01")
	}

	// limit=2 → 2 results.
	w := env.do(t, http.MethodGet, txnURL(setup.householdID)+"?limit=2", nil, setup.token)
	require.Equal(t, http.StatusOK, w.Code)
	var page1 []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&page1))
	assert.Len(t, page1, 2)

	// limit=2&offset=2 → 1 result (the third).
	w = env.do(t, http.MethodGet, txnURL(setup.householdID)+"?limit=2&offset=2", nil, setup.token)
	require.Equal(t, http.StatusOK, w.Code)
	var page2 []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&page2))
	assert.Len(t, page2, 1)
}

// TestE2E_Ledger_GetHistory_Empty verifies AC9: a household with no transactions
// returns an empty array (not null, not 404).
func TestE2E_Ledger_GetHistory_Empty(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	setup := createLedgerSetupHelper(t, env, "ledger9@example.com", "Ledger House 9")

	w := env.do(t, http.MethodGet, txnURL(setup.householdID), nil, setup.token)

	require.Equal(t, http.StatusOK, w.Code)
	var history []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&history))
	assert.NotNil(t, history)
	assert.Empty(t, history)
}

// TestE2E_Ledger_SplitTransaction_ThreeEntries verifies the edge case: a transaction
// with 3 entries (split expense across 3 accounts) balances correctly.
func TestE2E_Ledger_SplitTransaction_ThreeEntries(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	setup := createLedgerSetupHelper(t, env, "ledger-split@example.com", "Split House")

	// Create a third account to receive part of the split.
	w := env.do(t, http.MethodPost, "/api/v1/households/"+setup.householdID+"/accounts", map[string]string{
		"name":          "Expense",
		"account_type":  "CHECKING",
		"currency_code": "BRL",
	}, setup.token)
	require.Equal(t, http.StatusCreated, w.Code)
	expenseID := decodeJSON(t, w)["id"].(string)

	// 1 debit (checking -12000) → 2 credits (savings +7000, expense +5000).
	w = env.do(t, http.MethodPost, txnURL(setup.householdID), map[string]any{
		"description":      "Split purchase",
		"transaction_date": "2026-03-15",
		"entries": []map[string]any{
			{"account_id": setup.checkingID, "entry_type": "DEBIT", "amount": 12000},
			{"account_id": setup.savingsID, "entry_type": "CREDIT", "amount": 7000},
			{"account_id": expenseID, "entry_type": "CREDIT", "amount": 5000},
		},
	}, setup.token)

	require.Equal(t, http.StatusCreated, w.Code)
	resp := decodeJSON(t, w)
	entries, ok := resp["entries"].([]any)
	require.True(t, ok)
	assert.Len(t, entries, 3)

	// Verify balances.
	w = env.do(t, http.MethodGet, balanceURL(setup.householdID, setup.checkingID), nil, setup.token)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, float64(-12000), decodeJSON(t, w)["balance"])

	w = env.do(t, http.MethodGet, balanceURL(setup.householdID, setup.savingsID), nil, setup.token)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, float64(7000), decodeJSON(t, w)["balance"])

	w = env.do(t, http.MethodGet, balanceURL(setup.householdID, expenseID), nil, setup.token)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, float64(5000), decodeJSON(t, w)["balance"])
}

// TestE2E_Ledger_RunningBalance_Cumulative verifies the edge case: balance is
// cumulative across multiple transactions on the same accounts.
func TestE2E_Ledger_RunningBalance_Cumulative(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	setup := createLedgerSetupHelper(t, env, "ledger-cumul@example.com", "Cumul House")

	postTransactionHelper(t, env, setup, setup.checkingID, setup.savingsID, 10000, "Txn 1", "2026-03-01")
	postTransactionHelper(t, env, setup, setup.savingsID, setup.checkingID, 3000, "Txn 2", "2026-03-02")
	postTransactionHelper(t, env, setup, setup.checkingID, setup.savingsID, 2000, "Txn 3", "2026-03-03")

	// checking: -10000 + 3000 - 2000 = -9000.
	w := env.do(t, http.MethodGet, balanceURL(setup.householdID, setup.checkingID), nil, setup.token)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, float64(-9000), decodeJSON(t, w)["balance"])

	// savings: +10000 - 3000 + 2000 = +9000.
	w = env.do(t, http.MethodGet, balanceURL(setup.householdID, setup.savingsID), nil, setup.token)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, float64(9000), decodeJSON(t, w)["balance"])
}

// TestE2E_Ledger_Balance_DebitOnly_IsNegative verifies the edge case: an account
// that only has debit entries has a negative balance.
func TestE2E_Ledger_Balance_DebitOnly_IsNegative(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	setup := createLedgerSetupHelper(t, env, "ledger-debit@example.com", "Debit House")

	// checking is debited; savings receives the credit — checking balance is negative.
	postTransactionHelper(t, env, setup, setup.checkingID, setup.savingsID, 5000, "Debit only", "2026-03-01")

	w := env.do(t, http.MethodGet, balanceURL(setup.householdID, setup.checkingID), nil, setup.token)
	require.Equal(t, http.StatusOK, w.Code)
	bal := decodeJSON(t, w)
	assert.Less(t, bal["balance"].(float64), float64(0))
	assert.Equal(t, float64(-5000), bal["balance"])
}

// TestE2E_Ledger_TransactionDate_Format verifies the edge case: transaction_date
// is stored and returned in YYYY-MM-DD format.
func TestE2E_Ledger_TransactionDate_Format(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)
	setup := createLedgerSetupHelper(t, env, "ledger-date@example.com", "Date House")

	const txnDate = "2026-01-15"
	resp := postTransactionHelper(t, env, setup, setup.checkingID, setup.savingsID, 1000, "Date check", txnDate)

	assert.Equal(t, txnDate, resp["transaction_date"])
}
