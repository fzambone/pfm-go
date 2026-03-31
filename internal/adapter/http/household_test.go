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
	domainhouse "github.com/zambone/pfm-go/internal/domain/household"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/validate"
	"github.com/zambone/pfm-go/internal/types"
)

// --- Test helpers ---

var (
	houseID  = uuid.MustParse("00000000-0000-0000-0000-000000000010")
	memberID = uuid.MustParse("00000000-0000-0000-0000-000000000020")
	callerID = uuid.MustParse("00000000-0000-0000-0000-000000000099")
)

func testHousehold() domainhouse.Household {
	return domainhouse.Household{
		ID:        houseID,
		Name:      "Test House",
		Status:    types.StatusActive,
		Version:   1,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		CreatedBy: callerID,
		UpdatedBy: callerID,
	}
}

func testMembership() domainhouse.Membership {
	return domainhouse.Membership{
		HouseholdID: houseID,
		UserID:      memberID,
		Role:        types.RoleMember,
		InvitedBy:   callerID,
		JoinedAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

// --- CreateHouseholdHandler tests ---

type stubCreateHouseholdService struct {
	household domainhouse.Household
	err       error
}

func (s *stubCreateHouseholdService) Create(_ context.Context, _ domainhouse.CreateInput, _ uuid.UUID) (domainhouse.Household, error) {
	return s.household, s.err
}

// TestCreateHouseholdHandler_ValidRequest_Returns201 verifies AC1: creating a
// household returns 201 Created with the household and Location header.
func TestCreateHouseholdHandler_ValidRequest_Returns201(t *testing.T) {
	t.Parallel()

	svc := &stubCreateHouseholdService{household: testHousehold()}
	handler := pfmhttp.CreateHouseholdHandler(svc)

	body := `{"name":"Test House"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/households", strings.NewReader(body))
	r = r.WithContext(ctxWithUser(callerID))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Header().Get("Location"), houseID.String())

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "Test House", resp["name"])
}

// TestCreateHouseholdHandler_ValidationError_Returns400 verifies AC10.
func TestCreateHouseholdHandler_ValidationError_Returns400(t *testing.T) {
	t.Parallel()

	svc := &stubCreateHouseholdService{
		err: &validate.ValidationError{
			Violations: []validate.Violation{{Field: "name", Message: "is required"}},
		},
	}
	handler := pfmhttp.CreateHouseholdHandler(svc)

	body := `{"name":""}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/households", strings.NewReader(body))
	r = r.WithContext(ctxWithUser(callerID))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCreateHouseholdHandler_MalformedJSON_Returns400 verifies edge case.
func TestCreateHouseholdHandler_MalformedJSON_Returns400(t *testing.T) {
	t.Parallel()

	svc := &stubCreateHouseholdService{}
	handler := pfmhttp.CreateHouseholdHandler(svc)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/households", strings.NewReader("{bad"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCreateHouseholdHandler_NilService_Panics verifies nil guard.
func TestCreateHouseholdHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.CreateHouseholdHandler(nil) })
}

// --- GetHouseholdHandler tests ---

type stubGetHouseholdService struct {
	household domainhouse.Household
	err       error
}

func (s *stubGetHouseholdService) FindByID(_ context.Context, _ uuid.UUID) (domainhouse.Household, error) {
	return s.household, s.err
}

// TestGetHouseholdHandler_ValidID_Returns200 verifies AC2.
func TestGetHouseholdHandler_ValidID_Returns200(t *testing.T) {
	t.Parallel()

	svc := &stubGetHouseholdService{household: testHousehold()}
	handler := pfmhttp.GetHouseholdHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/households/"+houseID.String(), nil)
	r.SetPathValue("id", houseID.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "Test House", resp["name"])
}

// TestGetHouseholdHandler_NotFound_Returns404 verifies domain error mapping.
func TestGetHouseholdHandler_NotFound_Returns404(t *testing.T) {
	t.Parallel()

	svc := &stubGetHouseholdService{err: message.ErrHouseholdNotFound}
	handler := pfmhttp.GetHouseholdHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/households/"+uuid.Nil.String(), nil)
	r.SetPathValue("id", uuid.Nil.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestGetHouseholdHandler_InvalidUUID_Returns400 verifies edge case.
func TestGetHouseholdHandler_InvalidUUID_Returns400(t *testing.T) {
	t.Parallel()

	svc := &stubGetHouseholdService{}
	handler := pfmhttp.GetHouseholdHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/households/bad", nil)
	r.SetPathValue("id", "bad")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetHouseholdHandler_NilService_Panics verifies nil guard.
func TestGetHouseholdHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.GetHouseholdHandler(nil) })
}

// --- ListHouseholdsHandler tests ---

type stubListHouseholdsService struct {
	households []domainhouse.Household
	err        error
}

func (s *stubListHouseholdsService) ListForUser(_ context.Context, _ uuid.UUID) ([]domainhouse.Household, error) {
	return s.households, s.err
}

// TestListHouseholdsHandler_ReturnsArray verifies AC3.
func TestListHouseholdsHandler_ReturnsArray(t *testing.T) {
	t.Parallel()

	svc := &stubListHouseholdsService{households: []domainhouse.Household{testHousehold()}}
	handler := pfmhttp.ListHouseholdsHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/households", nil)
	r = r.WithContext(ctxWithUser(callerID))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp, 1)
	assert.Equal(t, "Test House", resp[0]["name"])
}

// TestListHouseholdsHandler_EmptyList_ReturnsEmptyArray verifies edge case:
// a user with no households gets [], not null.
func TestListHouseholdsHandler_EmptyList_ReturnsEmptyArray(t *testing.T) {
	t.Parallel()

	svc := &stubListHouseholdsService{households: []domainhouse.Household{}}
	handler := pfmhttp.ListHouseholdsHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/households", nil)
	r = r.WithContext(ctxWithUser(callerID))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	// Must be "[]" not "null"
	assert.Contains(t, w.Body.String(), "[]")
}

// TestListHouseholdsHandler_NilService_Panics verifies nil guard.
func TestListHouseholdsHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.ListHouseholdsHandler(nil) })
}

// --- AddMemberHandler tests ---

type stubAddMemberService struct {
	membership domainhouse.Membership
	err        error
}

func (s *stubAddMemberService) AddMember(_ context.Context, _ uuid.UUID, _ domainhouse.AddMemberInput, _ uuid.UUID) (domainhouse.Membership, error) {
	return s.membership, s.err
}

// TestAddMemberHandler_ValidRequest_Returns201 verifies AC4.
func TestAddMemberHandler_ValidRequest_Returns201(t *testing.T) {
	t.Parallel()

	svc := &stubAddMemberService{membership: testMembership()}
	handler := pfmhttp.AddMemberHandler(svc)

	body := `{"user_id":"00000000-0000-0000-0000-000000000020","role":"MEMBER"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/households/"+houseID.String()+"/members", strings.NewReader(body))
	r.SetPathValue("id", houseID.String())
	r = r.WithContext(ctxWithUser(callerID))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, memberID.String(), resp["user_id"])
	assert.Equal(t, "MEMBER", resp["role"])
}

// TestAddMemberHandler_AlreadyMember_Returns409 verifies AC5.
func TestAddMemberHandler_AlreadyMember_Returns409(t *testing.T) {
	t.Parallel()

	svc := &stubAddMemberService{err: message.ErrHouseholdMemberExists}
	handler := pfmhttp.AddMemberHandler(svc)

	body := `{"user_id":"00000000-0000-0000-0000-000000000020","role":"MEMBER"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/households/"+houseID.String()+"/members", strings.NewReader(body))
	r.SetPathValue("id", houseID.String())
	r = r.WithContext(ctxWithUser(callerID))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusConflict, w.Code)
}

// TestAddMemberHandler_MalformedJSON_Returns400 verifies edge case.
func TestAddMemberHandler_MalformedJSON_Returns400(t *testing.T) {
	t.Parallel()

	svc := &stubAddMemberService{}
	handler := pfmhttp.AddMemberHandler(svc)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/households/"+houseID.String()+"/members", strings.NewReader("{bad"))
	r.SetPathValue("id", houseID.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestAddMemberHandler_NilService_Panics verifies nil guard.
func TestAddMemberHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.AddMemberHandler(nil) })
}

// --- RemoveMemberHandler tests ---

type stubRemoveMemberService struct {
	err error
}

func (s *stubRemoveMemberService) RemoveMember(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ uuid.UUID) error {
	return s.err
}

// TestRemoveMemberHandler_Success_Returns204 verifies AC6.
func TestRemoveMemberHandler_Success_Returns204(t *testing.T) {
	t.Parallel()

	svc := &stubRemoveMemberService{}
	handler := pfmhttp.RemoveMemberHandler(svc)

	r := httptest.NewRequest(http.MethodDelete, "/api/v1/households/"+houseID.String()+"/members/"+memberID.String(), nil)
	r.SetPathValue("id", houseID.String())
	r.SetPathValue("user_id", memberID.String())
	r = r.WithContext(ctxWithUser(callerID))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

// TestRemoveMemberHandler_LastAdmin_ReturnsConflict verifies AC7.
func TestRemoveMemberHandler_LastAdmin_ReturnsConflict(t *testing.T) {
	t.Parallel()

	svc := &stubRemoveMemberService{err: message.ErrHouseholdLastAdmin}
	handler := pfmhttp.RemoveMemberHandler(svc)

	r := httptest.NewRequest(http.MethodDelete, "/api/v1/households/"+houseID.String()+"/members/"+callerID.String(), nil)
	r.SetPathValue("id", houseID.String())
	r.SetPathValue("user_id", callerID.String())
	r = r.WithContext(ctxWithUser(callerID))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusConflict, w.Code)
}

// TestRemoveMemberHandler_MemberNotFound_Returns404 verifies edge case.
func TestRemoveMemberHandler_MemberNotFound_Returns404(t *testing.T) {
	t.Parallel()

	svc := &stubRemoveMemberService{err: message.ErrHouseholdMemberNotFound}
	handler := pfmhttp.RemoveMemberHandler(svc)

	r := httptest.NewRequest(http.MethodDelete, "/api/v1/households/"+houseID.String()+"/members/"+uuid.Nil.String(), nil)
	r.SetPathValue("id", houseID.String())
	r.SetPathValue("user_id", uuid.Nil.String())
	r = r.WithContext(ctxWithUser(callerID))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestRemoveMemberHandler_NilService_Panics verifies nil guard.
func TestRemoveMemberHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.RemoveMemberHandler(nil) })
}

// --- UpdateHouseholdNameHandler tests ---

type stubUpdateHouseholdNameService struct {
	household domainhouse.Household
	err       error
}

func (s *stubUpdateHouseholdNameService) UpdateName(_ context.Context, _ uuid.UUID, _ domainhouse.UpdateNameInput, _ int, _ uuid.UUID) (domainhouse.Household, error) {
	return s.household, s.err
}

// TestUpdateHouseholdNameHandler_ValidRequest_Returns200 verifies AC8.
func TestUpdateHouseholdNameHandler_ValidRequest_Returns200(t *testing.T) {
	t.Parallel()

	updated := testHousehold()
	updated.Name = "New Name"
	svc := &stubUpdateHouseholdNameService{household: updated}
	handler := pfmhttp.UpdateHouseholdNameHandler(svc)

	body := `{"name":"New Name","version":1}`
	r := httptest.NewRequest(http.MethodPut, "/api/v1/households/"+houseID.String(), strings.NewReader(body))
	r.SetPathValue("id", houseID.String())
	r = r.WithContext(ctxWithUser(callerID))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "New Name", resp["name"])
}

// TestUpdateHouseholdNameHandler_StaleVersion_Returns409 verifies version conflict.
func TestUpdateHouseholdNameHandler_StaleVersion_Returns409(t *testing.T) {
	t.Parallel()

	svc := &stubUpdateHouseholdNameService{err: message.ErrHouseholdVersionConflict}
	handler := pfmhttp.UpdateHouseholdNameHandler(svc)

	body := `{"name":"New Name","version":1}`
	r := httptest.NewRequest(http.MethodPut, "/api/v1/households/"+houseID.String(), strings.NewReader(body))
	r.SetPathValue("id", houseID.String())
	r = r.WithContext(ctxWithUser(callerID))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusConflict, w.Code)
}

// TestUpdateHouseholdNameHandler_MalformedJSON_Returns400 verifies edge case.
func TestUpdateHouseholdNameHandler_MalformedJSON_Returns400(t *testing.T) {
	t.Parallel()

	svc := &stubUpdateHouseholdNameService{}
	handler := pfmhttp.UpdateHouseholdNameHandler(svc)

	r := httptest.NewRequest(http.MethodPut, "/api/v1/households/"+houseID.String(), strings.NewReader("{bad"))
	r.SetPathValue("id", houseID.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestUpdateHouseholdNameHandler_NilService_Panics verifies nil guard.
func TestUpdateHouseholdNameHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.UpdateHouseholdNameHandler(nil) })
}

// --- DeactivateHouseholdHandler tests ---

type stubDeactivateHouseholdService struct {
	err error
}

func (s *stubDeactivateHouseholdService) Deactivate(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return s.err
}

// TestDeactivateHouseholdHandler_Success_Returns204 verifies AC9.
func TestDeactivateHouseholdHandler_Success_Returns204(t *testing.T) {
	t.Parallel()

	svc := &stubDeactivateHouseholdService{}
	handler := pfmhttp.DeactivateHouseholdHandler(svc)

	r := httptest.NewRequest(http.MethodDelete, "/api/v1/households/"+houseID.String(), nil)
	r.SetPathValue("id", houseID.String())
	r = r.WithContext(ctxWithUser(callerID))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

// TestDeactivateHouseholdHandler_NotFound_Returns404 verifies error mapping.
func TestDeactivateHouseholdHandler_NotFound_Returns404(t *testing.T) {
	t.Parallel()

	svc := &stubDeactivateHouseholdService{err: message.ErrHouseholdNotFound}
	handler := pfmhttp.DeactivateHouseholdHandler(svc)

	r := httptest.NewRequest(http.MethodDelete, "/api/v1/households/"+uuid.Nil.String(), nil)
	r.SetPathValue("id", uuid.Nil.String())
	r = r.WithContext(ctxWithUser(callerID))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestDeactivateHouseholdHandler_InvalidUUID_Returns400 verifies edge case.
func TestDeactivateHouseholdHandler_InvalidUUID_Returns400(t *testing.T) {
	t.Parallel()

	svc := &stubDeactivateHouseholdService{}
	handler := pfmhttp.DeactivateHouseholdHandler(svc)

	r := httptest.NewRequest(http.MethodDelete, "/api/v1/households/bad", nil)
	r.SetPathValue("id", "bad")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestDeactivateHouseholdHandler_NilService_Panics verifies nil guard.
func TestDeactivateHouseholdHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.DeactivateHouseholdHandler(nil) })
}
