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
	domaincc "github.com/zambone/pfm-go/internal/domain/creditcard"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/validate"
)

// --- Test helpers ---

var (
	ccAccountID = uuid.MustParse("00000000-0000-0000-0000-000000000040")
	ccSettingsID = uuid.MustParse("00000000-0000-0000-0000-000000000041")
	ccCaller     = uuid.MustParse("00000000-0000-0000-0000-000000000099")
)

func testSettings() domaincc.Settings {
	return domaincc.Settings{
		ID:          ccSettingsID,
		AccountID:   ccAccountID,
		ClosingDay:  15,
		DueDay:      25,
		LimitAmount: 500000, // R$5,000.00
		Version:     1,
		CreatedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		CreatedBy:   ccCaller,
		UpdatedBy:   ccCaller,
	}
}

// --- CreateCreditCardSettingsHandler tests ---

type FakeCreateCCSettingsService struct {
	settings domaincc.Settings
	err      error
}

func (s *FakeCreateCCSettingsService) Create(_ context.Context, _ uuid.UUID, _ domaincc.CreateInput, _ uuid.UUID) (domaincc.Settings, error) {
	return s.settings, s.err
}

// TestCreateCCSettingsHandler_ValidRequest_Returns201 verifies AC1.
func TestCreateCCSettingsHandler_ValidRequest_Returns201(t *testing.T) {
	t.Parallel()

	svc := &FakeCreateCCSettingsService{settings: testSettings()}
	handler := pfmhttp.CreateCreditCardSettingsHandler(svc)

	body := `{"closing_day":15,"due_day":25,"limit_amount":500000}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.SetPathValue("account_id", ccAccountID.String())
	r = r.WithContext(ctxWithUser(ccCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, float64(15), resp["closing_day"])
	assert.Equal(t, float64(25), resp["due_day"])
	assert.Equal(t, float64(500000), resp["limit_amount"])
}

// TestCreateCCSettingsHandler_NotCreditCard_ReturnsConflict verifies AC2.
func TestCreateCCSettingsHandler_NotCreditCard_ReturnsConflict(t *testing.T) {
	t.Parallel()

	svc := &FakeCreateCCSettingsService{err: message.ErrCreditCardSettingsNotCreditCard}
	handler := pfmhttp.CreateCreditCardSettingsHandler(svc)

	body := `{"closing_day":15,"due_day":25,"limit_amount":500000}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.SetPathValue("account_id", ccAccountID.String())
	r = r.WithContext(ctxWithUser(ccCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusConflict, w.Code)
}

// TestCreateCCSettingsHandler_AlreadyExists_Returns409 verifies AC3.
func TestCreateCCSettingsHandler_AlreadyExists_Returns409(t *testing.T) {
	t.Parallel()

	svc := &FakeCreateCCSettingsService{err: message.ErrCreditCardSettingsExists}
	handler := pfmhttp.CreateCreditCardSettingsHandler(svc)

	body := `{"closing_day":15,"due_day":25,"limit_amount":500000}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.SetPathValue("account_id", ccAccountID.String())
	r = r.WithContext(ctxWithUser(ccCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusConflict, w.Code)
}

// TestCreateCCSettingsHandler_ValidationError_Returns400 verifies AC10.
func TestCreateCCSettingsHandler_ValidationError_Returns400(t *testing.T) {
	t.Parallel()

	svc := &FakeCreateCCSettingsService{
		err: &validate.ValidationError{
			Violations: []validate.Violation{{Field: "closing_day", Message: "must be between 1 and 31"}},
		},
	}
	handler := pfmhttp.CreateCreditCardSettingsHandler(svc)

	body := `{"closing_day":0,"due_day":0,"limit_amount":0}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.SetPathValue("account_id", ccAccountID.String())
	r = r.WithContext(ctxWithUser(ccCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCreateCCSettingsHandler_MalformedJSON_Returns400 verifies edge case.
func TestCreateCCSettingsHandler_MalformedJSON_Returns400(t *testing.T) {
	t.Parallel()

	svc := &FakeCreateCCSettingsService{}
	handler := pfmhttp.CreateCreditCardSettingsHandler(svc)

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{bad"))
	r.SetPathValue("account_id", ccAccountID.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCreateCCSettingsHandler_NilService_Panics verifies nil guard.
func TestCreateCCSettingsHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.CreateCreditCardSettingsHandler(nil) })
}

// --- GetCreditCardSettingsHandler tests ---

type FakeGetCCSettingsService struct {
	settings domaincc.Settings
	err      error
}

func (s *FakeGetCCSettingsService) FindByAccountID(_ context.Context, _ uuid.UUID) (domaincc.Settings, error) {
	return s.settings, s.err
}

// TestGetCCSettingsHandler_ValidID_Returns200 verifies AC4.
func TestGetCCSettingsHandler_ValidID_Returns200(t *testing.T) {
	t.Parallel()

	svc := &FakeGetCCSettingsService{settings: testSettings()}
	handler := pfmhttp.GetCreditCardSettingsHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("account_id", ccAccountID.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, float64(15), resp["closing_day"])
}

// TestGetCCSettingsHandler_NotFound_Returns404 verifies AC5.
func TestGetCCSettingsHandler_NotFound_Returns404(t *testing.T) {
	t.Parallel()

	svc := &FakeGetCCSettingsService{err: message.ErrCreditCardSettingsNotFound}
	handler := pfmhttp.GetCreditCardSettingsHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("account_id", uuid.Nil.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestGetCCSettingsHandler_NilService_Panics verifies nil guard.
func TestGetCCSettingsHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.GetCreditCardSettingsHandler(nil) })
}

// --- UpdateClosingDayHandler tests ---

type FakeUpdateClosingDayService struct {
	settings domaincc.Settings
	err      error
}

func (s *FakeUpdateClosingDayService) UpdateClosingDay(_ context.Context, _ uuid.UUID, _ domaincc.UpdateClosingDayInput, _ int, _ uuid.UUID) (domaincc.Settings, error) {
	return s.settings, s.err
}

// TestUpdateClosingDayHandler_ValidRequest_Returns200 verifies AC6.
func TestUpdateClosingDayHandler_ValidRequest_Returns200(t *testing.T) {
	t.Parallel()

	updated := testSettings()
	updated.ClosingDay = 20
	svc := &FakeUpdateClosingDayService{settings: updated}
	handler := pfmhttp.UpdateClosingDayHandler(svc)

	body := `{"closing_day":20,"version":1}`
	r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	r.SetPathValue("account_id", ccAccountID.String())
	r = r.WithContext(ctxWithUser(ccCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, float64(20), resp["closing_day"])
}

// TestUpdateClosingDayHandler_VersionConflict_Returns409 verifies edge case.
func TestUpdateClosingDayHandler_VersionConflict_Returns409(t *testing.T) {
	t.Parallel()

	svc := &FakeUpdateClosingDayService{err: message.ErrCreditCardSettingsVersionConflict}
	handler := pfmhttp.UpdateClosingDayHandler(svc)

	body := `{"closing_day":20,"version":1}`
	r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	r.SetPathValue("account_id", ccAccountID.String())
	r = r.WithContext(ctxWithUser(ccCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusConflict, w.Code)
}

// TestUpdateClosingDayHandler_NilService_Panics verifies nil guard.
func TestUpdateClosingDayHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.UpdateClosingDayHandler(nil) })
}

// --- UpdateDueDayHandler tests ---

type FakeUpdateDueDayService struct {
	settings domaincc.Settings
	err      error
}

func (s *FakeUpdateDueDayService) UpdateDueDay(_ context.Context, _ uuid.UUID, _ domaincc.UpdateDueDayInput, _ int, _ uuid.UUID) (domaincc.Settings, error) {
	return s.settings, s.err
}

// TestUpdateDueDayHandler_ValidRequest_Returns200 verifies AC7.
func TestUpdateDueDayHandler_ValidRequest_Returns200(t *testing.T) {
	t.Parallel()

	updated := testSettings()
	updated.DueDay = 10
	svc := &FakeUpdateDueDayService{settings: updated}
	handler := pfmhttp.UpdateDueDayHandler(svc)

	body := `{"due_day":10,"version":1}`
	r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	r.SetPathValue("account_id", ccAccountID.String())
	r = r.WithContext(ctxWithUser(ccCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, float64(10), resp["due_day"])
}

// TestUpdateDueDayHandler_NilService_Panics verifies nil guard.
func TestUpdateDueDayHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.UpdateDueDayHandler(nil) })
}

// --- UpdateCreditLimitHandler tests ---

type FakeUpdateCreditLimitService struct {
	settings domaincc.Settings
	err      error
}

func (s *FakeUpdateCreditLimitService) UpdateLimit(_ context.Context, _ uuid.UUID, _ domaincc.UpdateLimitInput, _ int, _ uuid.UUID) (domaincc.Settings, error) {
	return s.settings, s.err
}

// TestUpdateCreditLimitHandler_ValidRequest_Returns200 verifies AC8.
func TestUpdateCreditLimitHandler_ValidRequest_Returns200(t *testing.T) {
	t.Parallel()

	updated := testSettings()
	updated.LimitAmount = 1000000
	svc := &FakeUpdateCreditLimitService{settings: updated}
	handler := pfmhttp.UpdateCreditLimitHandler(svc)

	body := `{"limit_amount":1000000,"version":1}`
	r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	r.SetPathValue("account_id", ccAccountID.String())
	r = r.WithContext(ctxWithUser(ccCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, float64(1000000), resp["limit_amount"])
}

// TestUpdateCreditLimitHandler_NilService_Panics verifies nil guard.
func TestUpdateCreditLimitHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.UpdateCreditLimitHandler(nil) })
}

// --- DeleteCreditCardSettingsHandler tests ---

type FakeDeleteCCSettingsService struct {
	err error
}

func (s *FakeDeleteCCSettingsService) Delete(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return s.err
}

// TestDeleteCCSettingsHandler_Success_Returns204 verifies AC9.
func TestDeleteCCSettingsHandler_Success_Returns204(t *testing.T) {
	t.Parallel()

	svc := &FakeDeleteCCSettingsService{}
	handler := pfmhttp.DeleteCreditCardSettingsHandler(svc)

	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r.SetPathValue("account_id", ccAccountID.String())
	r = r.WithContext(ctxWithUser(ccCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

// TestDeleteCCSettingsHandler_NotFound_Returns404 verifies error mapping.
func TestDeleteCCSettingsHandler_NotFound_Returns404(t *testing.T) {
	t.Parallel()

	svc := &FakeDeleteCCSettingsService{err: message.ErrCreditCardSettingsNotFound}
	handler := pfmhttp.DeleteCreditCardSettingsHandler(svc)

	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r.SetPathValue("account_id", uuid.Nil.String())
	r = r.WithContext(ctxWithUser(ccCaller))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestDeleteCCSettingsHandler_NilService_Panics verifies nil guard.
func TestDeleteCCSettingsHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.DeleteCreditCardSettingsHandler(nil) })
}
