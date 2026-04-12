package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pfmhttp "github.com/zambone/pfm-go/internal/adapter/http"
	domainledger "github.com/zambone/pfm-go/internal/domain/ledger"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/validate"
	"github.com/zambone/pfm-go/internal/types"
)

// --- Test helpers ---

var (
	ledgerHouseID = uuid.MustParse("00000000-0000-0000-0000-000000000010")
	ledgerAcctID  = uuid.MustParse("00000000-0000-0000-0000-000000000030")
	ledgerAcctID2 = uuid.MustParse("00000000-0000-0000-0000-000000000031")
	ledgerTxnID   = uuid.MustParse("00000000-0000-0000-0000-000000000050")
	ledgerCaller  = uuid.MustParse("00000000-0000-0000-0000-000000000099")
)

func testTransaction() domainledger.Transaction {
	return domainledger.Transaction{
		ID:              ledgerTxnID,
		HouseholdID:     ledgerHouseID,
		Description:     "Grocery purchase",
		TransactionDate: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		CreatedAt:       time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
		CreatedBy:       ledgerCaller,
	}
}

func testEntries() []domainledger.Entry {
	return []domainledger.Entry{
		{
			ID:            uuid.MustParse("00000000-0000-0000-0000-000000000060"),
			TransactionID: ledgerTxnID,
			AccountID:     ledgerAcctID,
			EntryType:     types.EntryTypeDebit,
			Amount:        5000,
			CreatedAt:     time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
		},
		{
			ID:            uuid.MustParse("00000000-0000-0000-0000-000000000061"),
			TransactionID: ledgerTxnID,
			AccountID:     ledgerAcctID2,
			EntryType:     types.EntryTypeCredit,
			Amount:        5000,
			CreatedAt:     time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
		},
	}
}

// --- PostTransactionHandler tests ---

type FakePostTransactionService struct {
	txn     domainledger.Transaction
	entries []domainledger.Entry
	err     error
}

func (s *FakePostTransactionService) PostTransaction(_ context.Context, _ uuid.UUID, _ domainledger.PostTransactionInput, _ uuid.UUID) (domainledger.Transaction, []domainledger.Entry, error) {
	return s.txn, s.entries, s.err
}

// TestPostTransactionHandler_ValidRequest_Returns201 verifies AC1.
func TestPostTransactionHandler_ValidRequest_Returns201(t *testing.T) {
	t.Parallel()

	svc := &FakePostTransactionService{txn: testTransaction(), entries: testEntries()}
	handler := pfmhttp.PostTransactionHandler(svc)

	body := `{
		"description":"Grocery purchase",
		"transaction_date":"2026-03-15",
		"entries":[
			{"account_id":"00000000-0000-0000-0000-000000000030","entry_type":"DEBIT","amount":5000},
			{"account_id":"00000000-0000-0000-0000-000000000031","entry_type":"CREDIT","amount":5000}
		]
	}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.SetPathValue("household_id", ledgerHouseID.String())
	r = r.WithContext(ctxWithUser(ledgerCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "Grocery purchase", resp["description"])

	entries, ok := resp["entries"].([]any)
	require.True(t, ok)
	assert.Len(t, entries, 2)
}

// TestPostTransactionHandler_Unbalanced_Returns422 verifies AC2.
func TestPostTransactionHandler_Unbalanced_Returns422(t *testing.T) {
	t.Parallel()

	svc := &FakePostTransactionService{err: message.ErrLedgerUnbalanced}
	handler := pfmhttp.PostTransactionHandler(svc)

	body := `{
		"description":"Bad txn",
		"transaction_date":"2026-03-15",
		"entries":[
			{"account_id":"00000000-0000-0000-0000-000000000030","entry_type":"DEBIT","amount":5000}
		]
	}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.SetPathValue("household_id", ledgerHouseID.String())
	r = r.WithContext(ctxWithUser(ledgerCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// TestPostTransactionHandler_ValidationError_Returns400 verifies AC7.
func TestPostTransactionHandler_ValidationError_Returns400(t *testing.T) {
	t.Parallel()

	svc := &FakePostTransactionService{
		err: &validate.ValidationError{
			Violations: []validate.Violation{{Field: "description", Message: "is required"}},
		},
	}
	handler := pfmhttp.PostTransactionHandler(svc)

	body := `{"description":"","transaction_date":"2026-03-15","entries":[]}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.SetPathValue("household_id", ledgerHouseID.String())
	r = r.WithContext(ctxWithUser(ledgerCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestPostTransactionHandler_MalformedJSON_Returns400 verifies edge case.
func TestPostTransactionHandler_MalformedJSON_Returns400(t *testing.T) {
	t.Parallel()

	svc := &FakePostTransactionService{}
	handler := pfmhttp.PostTransactionHandler(svc)

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{bad"))
	r.SetPathValue("household_id", ledgerHouseID.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestPostTransactionHandler_NilService_Panics verifies nil guard.
func TestPostTransactionHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.PostTransactionHandler(nil) })
}

// --- GetBalanceHandler tests ---

type FakeGetBalanceService struct {
	balance int64
	err     error
}

func (s *FakeGetBalanceService) GetBalance(_ context.Context, _ uuid.UUID) (int64, error) {
	return s.balance, s.err
}

// TestGetBalanceHandler_ValidID_Returns200 verifies AC3.
func TestGetBalanceHandler_ValidID_Returns200(t *testing.T) {
	t.Parallel()

	svc := &FakeGetBalanceService{balance: 15000}
	handler := pfmhttp.GetBalanceHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("account_id", ledgerAcctID.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, float64(15000), resp["balance"])
	assert.Equal(t, ledgerAcctID.String(), resp["account_id"])
}

// TestGetBalanceHandler_ZeroBalance_Returns200 verifies edge case: account with
// no entries returns zero balance, not error.
func TestGetBalanceHandler_ZeroBalance_Returns200(t *testing.T) {
	t.Parallel()

	svc := &FakeGetBalanceService{balance: 0}
	handler := pfmhttp.GetBalanceHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("account_id", ledgerAcctID.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, float64(0), resp["balance"])
}

// TestGetBalanceHandler_NotFound_Returns404 verifies AC6.
func TestGetBalanceHandler_NotFound_Returns404(t *testing.T) {
	t.Parallel()

	svc := &FakeGetBalanceService{err: message.ErrAccountNotFound}
	handler := pfmhttp.GetBalanceHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("account_id", uuid.Nil.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestGetBalanceHandler_InvalidUUID_Returns400 verifies edge case.
func TestGetBalanceHandler_InvalidUUID_Returns400(t *testing.T) {
	t.Parallel()

	svc := &FakeGetBalanceService{}
	handler := pfmhttp.GetBalanceHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("account_id", "bad")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetBalanceHandler_NilService_Panics verifies nil guard.
func TestGetBalanceHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.GetBalanceHandler(nil) })
}

// --- GetTransactionHistoryHandler tests ---

type FakeGetHistoryService struct {
	history []domainledger.TransactionWithEntries
	err     error
}

func (s *FakeGetHistoryService) GetTransactionHistory(_ context.Context, _ uuid.UUID, _ domainledger.HistoryQuery) ([]domainledger.TransactionWithEntries, error) {
	return s.history, s.err
}

// TestGetHistoryHandler_ReturnsArray verifies AC4.
func TestGetHistoryHandler_ReturnsArray(t *testing.T) {
	t.Parallel()

	svc := &FakeGetHistoryService{
		history: []domainledger.TransactionWithEntries{
			{Transaction: testTransaction(), Entries: testEntries()},
		},
	}
	handler := pfmhttp.GetTransactionHistoryHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("household_id", ledgerHouseID.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp, 1)
	assert.Equal(t, "Grocery purchase", resp[0]["description"])
}

// TestGetHistoryHandler_EmptyList_ReturnsEmptyArray verifies edge case.
func TestGetHistoryHandler_EmptyList_ReturnsEmptyArray(t *testing.T) {
	t.Parallel()

	svc := &FakeGetHistoryService{history: []domainledger.TransactionWithEntries{}}
	handler := pfmhttp.GetTransactionHistoryHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("household_id", ledgerHouseID.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "[]")
}

// TestGetHistoryHandler_WithQueryParams verifies AC5: filtering parameters.
func TestGetHistoryHandler_WithQueryParams(t *testing.T) {
	t.Parallel()

	svc := &FakeGetHistoryService{history: []domainledger.TransactionWithEntries{}}
	handler := pfmhttp.GetTransactionHistoryHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/?account_id="+ledgerAcctID.String()+"&limit=10&offset=5", nil)
	r.SetPathValue("household_id", ledgerHouseID.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	// Should not error — query params are optional and parsed gracefully
	require.Equal(t, http.StatusOK, w.Code)
}

// TestGetHistoryHandler_NilService_Panics verifies nil guard.
func TestGetHistoryHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.GetTransactionHistoryHandler(nil) })
}
