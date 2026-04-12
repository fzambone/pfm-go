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
	domainacct "github.com/zambone/pfm-go/internal/domain/account"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/validate"
	"github.com/zambone/pfm-go/internal/types"
)

// --- Test helpers ---

var (
	acctID      = uuid.MustParse("00000000-0000-0000-0000-000000000030")
	acctHouseID = uuid.MustParse("00000000-0000-0000-0000-000000000010")
	acctCaller  = uuid.MustParse("00000000-0000-0000-0000-000000000099")
)

func testAccount() domainacct.Account {
	return domainacct.Account{
		ID:           acctID,
		HouseholdID:  acctHouseID,
		Name:         "Checking",
		AccountType:  types.AccountTypeChecking,
		CurrencyCode: types.CurrencyBRL,
		Balance:      10000, // R$100.00
		Status:       types.StatusActive,
		Version:      1,
		CreatedAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		CreatedBy:    acctCaller,
		UpdatedBy:    acctCaller,
	}
}

// --- CreateAccountHandler tests ---

type FakeCreateAccountService struct {
	account domainacct.Account
	err     error
}

func (s *FakeCreateAccountService) Create(_ context.Context, _ uuid.UUID, _ domainacct.CreateInput, _ uuid.UUID) (domainacct.Account, error) {
	return s.account, s.err
}

// TestCreateAccountHandler_ValidRequest_Returns201 verifies AC1.
func TestCreateAccountHandler_ValidRequest_Returns201(t *testing.T) {
	t.Parallel()

	svc := &FakeCreateAccountService{account: testAccount()}
	handler := pfmhttp.CreateAccountHandler(svc)

	body := `{"name":"Checking","account_type":"CHECKING","currency_code":"BRL"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/households/"+acctHouseID.String()+"/accounts", strings.NewReader(body))
	r.SetPathValue("household_id", acctHouseID.String())
	r = r.WithContext(ctxWithUser(acctCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Header().Get("Location"), acctID.String())

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "Checking", resp["name"])
	assert.Equal(t, "CHECKING", resp["account_type"])
	assert.Equal(t, "BRL", resp["currency_code"])
}

// TestCreateAccountHandler_NameTaken_Returns409 verifies AC2.
func TestCreateAccountHandler_NameTaken_Returns409(t *testing.T) {
	t.Parallel()

	svc := &FakeCreateAccountService{err: message.ErrAccountNameTaken}
	handler := pfmhttp.CreateAccountHandler(svc)

	body := `{"name":"Dup","account_type":"CHECKING","currency_code":"BRL"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/households/"+acctHouseID.String()+"/accounts", strings.NewReader(body))
	r.SetPathValue("household_id", acctHouseID.String())
	r = r.WithContext(ctxWithUser(acctCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusConflict, w.Code)
}

// TestCreateAccountHandler_ValidationError_Returns400 verifies AC10.
func TestCreateAccountHandler_ValidationError_Returns400(t *testing.T) {
	t.Parallel()

	svc := &FakeCreateAccountService{
		err: &validate.ValidationError{
			Violations: []validate.Violation{{Field: "name", Message: "is required"}},
		},
	}
	handler := pfmhttp.CreateAccountHandler(svc)

	body := `{"name":"","account_type":"","currency_code":""}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/households/"+acctHouseID.String()+"/accounts", strings.NewReader(body))
	r.SetPathValue("household_id", acctHouseID.String())
	r = r.WithContext(ctxWithUser(acctCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCreateAccountHandler_MalformedJSON_Returns400 verifies edge case.
func TestCreateAccountHandler_MalformedJSON_Returns400(t *testing.T) {
	t.Parallel()

	svc := &FakeCreateAccountService{}
	handler := pfmhttp.CreateAccountHandler(svc)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/households/"+acctHouseID.String()+"/accounts", strings.NewReader("{bad"))
	r.SetPathValue("household_id", acctHouseID.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCreateAccountHandler_NilService_Panics verifies nil guard.
func TestCreateAccountHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.CreateAccountHandler(nil) })
}

// --- GetAccountHandler tests ---

type FakeGetAccountService struct {
	account domainacct.Account
	err     error
}

func (s *FakeGetAccountService) FindByID(_ context.Context, _ uuid.UUID) (domainacct.Account, error) {
	return s.account, s.err
}

// TestGetAccountHandler_ValidID_Returns200 verifies AC3.
func TestGetAccountHandler_ValidID_Returns200(t *testing.T) {
	t.Parallel()

	svc := &FakeGetAccountService{account: testAccount()}
	handler := pfmhttp.GetAccountHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/households/"+acctHouseID.String()+"/accounts/"+acctID.String(), nil)
	r.SetPathValue("id", acctID.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "Checking", resp["name"])
}

// TestGetAccountHandler_NotFound_Returns404 verifies AC4.
func TestGetAccountHandler_NotFound_Returns404(t *testing.T) {
	t.Parallel()

	svc := &FakeGetAccountService{err: message.ErrAccountNotFound}
	handler := pfmhttp.GetAccountHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("id", uuid.Nil.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestGetAccountHandler_InvalidUUID_Returns400 verifies edge case.
func TestGetAccountHandler_InvalidUUID_Returns400(t *testing.T) {
	t.Parallel()

	svc := &FakeGetAccountService{}
	handler := pfmhttp.GetAccountHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("id", "bad")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetAccountHandler_NilService_Panics verifies nil guard.
func TestGetAccountHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.GetAccountHandler(nil) })
}

// --- ListAccountsHandler tests ---

type FakeListAccountsService struct {
	accounts []domainacct.Account
	err      error
}

func (s *FakeListAccountsService) ListForHousehold(_ context.Context, _ uuid.UUID) ([]domainacct.Account, error) {
	return s.accounts, s.err
}

// TestListAccountsHandler_ReturnsArray verifies AC5.
func TestListAccountsHandler_ReturnsArray(t *testing.T) {
	t.Parallel()

	svc := &FakeListAccountsService{accounts: []domainacct.Account{testAccount()}}
	handler := pfmhttp.ListAccountsHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("household_id", acctHouseID.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp, 1)
}

// TestListAccountsHandler_EmptyList_ReturnsEmptyArray verifies edge case.
func TestListAccountsHandler_EmptyList_ReturnsEmptyArray(t *testing.T) {
	t.Parallel()

	svc := &FakeListAccountsService{accounts: []domainacct.Account{}}
	handler := pfmhttp.ListAccountsHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("household_id", acctHouseID.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "[]")
}

// TestListAccountsHandler_NilService_Panics verifies nil guard.
func TestListAccountsHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.ListAccountsHandler(nil) })
}

// --- UpdateAccountNameHandler tests ---

type FakeUpdateAccountNameService struct {
	account domainacct.Account
	err     error
}

func (s *FakeUpdateAccountNameService) UpdateName(_ context.Context, _ uuid.UUID, _ domainacct.UpdateNameInput, _ int, _ uuid.UUID) (domainacct.Account, error) {
	return s.account, s.err
}

// TestUpdateAccountNameHandler_ValidRequest_Returns200 verifies AC6.
func TestUpdateAccountNameHandler_ValidRequest_Returns200(t *testing.T) {
	t.Parallel()

	updated := testAccount()
	updated.Name = "Savings"
	svc := &FakeUpdateAccountNameService{account: updated}
	handler := pfmhttp.UpdateAccountNameHandler(svc)

	body := `{"name":"Savings","version":1}`
	r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	r.SetPathValue("id", acctID.String())
	r = r.WithContext(ctxWithUser(acctCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "Savings", resp["name"])
}

// TestUpdateAccountNameHandler_VersionConflict_Returns409 verifies AC6 error path.
func TestUpdateAccountNameHandler_VersionConflict_Returns409(t *testing.T) {
	t.Parallel()

	svc := &FakeUpdateAccountNameService{err: message.ErrAccountVersionConflict}
	handler := pfmhttp.UpdateAccountNameHandler(svc)

	body := `{"name":"X","version":1}`
	r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	r.SetPathValue("id", acctID.String())
	r = r.WithContext(ctxWithUser(acctCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusConflict, w.Code)
}

// TestUpdateAccountNameHandler_MalformedJSON_Returns400 verifies edge case.
func TestUpdateAccountNameHandler_MalformedJSON_Returns400(t *testing.T) {
	t.Parallel()

	svc := &FakeUpdateAccountNameService{}
	handler := pfmhttp.UpdateAccountNameHandler(svc)

	r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader("{bad"))
	r.SetPathValue("id", acctID.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestUpdateAccountNameHandler_NilService_Panics verifies nil guard.
func TestUpdateAccountNameHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.UpdateAccountNameHandler(nil) })
}

// --- UpdateAccountBalanceHandler tests ---

type FakeUpdateAccountBalanceService struct {
	account domainacct.Account
	err     error
}

func (s *FakeUpdateAccountBalanceService) UpdateBalance(_ context.Context, _ uuid.UUID, _ domainacct.UpdateBalanceInput, _ int, _ uuid.UUID) (domainacct.Account, error) {
	return s.account, s.err
}

// TestUpdateAccountBalanceHandler_ValidRequest_Returns200 verifies AC7.
func TestUpdateAccountBalanceHandler_ValidRequest_Returns200(t *testing.T) {
	t.Parallel()

	updated := testAccount()
	updated.Balance = 50000
	svc := &FakeUpdateAccountBalanceService{account: updated}
	handler := pfmhttp.UpdateAccountBalanceHandler(svc)

	body := `{"balance":50000,"version":1}`
	r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	r.SetPathValue("id", acctID.String())
	r = r.WithContext(ctxWithUser(acctCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, float64(50000), resp["balance"])
}

// TestUpdateAccountBalanceHandler_MalformedJSON_Returns400 verifies edge case.
func TestUpdateAccountBalanceHandler_MalformedJSON_Returns400(t *testing.T) {
	t.Parallel()

	svc := &FakeUpdateAccountBalanceService{}
	handler := pfmhttp.UpdateAccountBalanceHandler(svc)

	r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader("{bad"))
	r.SetPathValue("id", acctID.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestUpdateAccountBalanceHandler_NilService_Panics verifies nil guard.
func TestUpdateAccountBalanceHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.UpdateAccountBalanceHandler(nil) })
}

// --- DeactivateAccountHandler tests ---

type FakeDeactivateAccountService struct {
	err error
}

func (s *FakeDeactivateAccountService) Deactivate(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return s.err
}

// TestDeactivateAccountHandler_Success_Returns204 verifies AC8.
func TestDeactivateAccountHandler_Success_Returns204(t *testing.T) {
	t.Parallel()

	svc := &FakeDeactivateAccountService{}
	handler := pfmhttp.DeactivateAccountHandler(svc)

	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r.SetPathValue("id", acctID.String())
	r = r.WithContext(ctxWithUser(acctCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

// TestDeactivateAccountHandler_BalanceNotZero_ReturnsConflict verifies AC9.
func TestDeactivateAccountHandler_BalanceNotZero_ReturnsConflict(t *testing.T) {
	t.Parallel()

	svc := &FakeDeactivateAccountService{err: message.ErrAccountBalanceNotZero}
	handler := pfmhttp.DeactivateAccountHandler(svc)

	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r.SetPathValue("id", acctID.String())
	r = r.WithContext(ctxWithUser(acctCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusConflict, w.Code)
}

// TestDeactivateAccountHandler_NotFound_Returns404 verifies error mapping.
func TestDeactivateAccountHandler_NotFound_Returns404(t *testing.T) {
	t.Parallel()

	svc := &FakeDeactivateAccountService{err: message.ErrAccountNotFound}
	handler := pfmhttp.DeactivateAccountHandler(svc)

	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r.SetPathValue("id", uuid.Nil.String())
	r = r.WithContext(ctxWithUser(acctCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestDeactivateAccountHandler_NilService_Panics verifies nil guard.
func TestDeactivateAccountHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.DeactivateAccountHandler(nil) })
}
