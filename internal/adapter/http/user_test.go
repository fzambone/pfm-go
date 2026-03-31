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
	domainuser "github.com/zambone/pfm-go/internal/domain/user"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/ctxutil"
	"github.com/zambone/pfm-go/internal/platform/validate"
)

// --- Stubs ---

// stubRegisterService is a test double for the registerService interface.
type stubRegisterService struct {
	user domainuser.User
	err  error
}

func (s *stubRegisterService) Register(_ context.Context, _ domainuser.RegisterInput, _ uuid.UUID) (domainuser.User, error) {
	return s.user, s.err
}

// testUser returns a User with all fields populated for test assertions.
func testUser() domainuser.User {
	return domainuser.User{
		ID:          uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Email:       "alice@example.com",
		DisplayName: "Alice",
		Version:     1,
		CreatedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		CreatedBy:   uuid.MustParse("00000000-0000-0000-0000-000000000099"),
		UpdatedBy:   uuid.MustParse("00000000-0000-0000-0000-000000000099"),
	}
}

// ctxWithUser returns a context with the given user ID set, simulating an
// authenticated request that has passed through the authn middleware.
func ctxWithUser(id uuid.UUID) context.Context {
	return ctxutil.WithUserID(context.Background(), id)
}

// --- RegisterHandler tests ---

// TestRegisterHandler_ValidRequest_Returns201 verifies AC1: a valid registration
// request returns 201 Created with the new user and a Location header.
func TestRegisterHandler_ValidRequest_Returns201(t *testing.T) {
	t.Parallel()

	svc := &stubRegisterService{user: testUser()}
	handler := pfmhttp.RegisterHandler(svc)

	body := `{"email":"alice@example.com","display_name":"Alice","password":"secret123"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Header().Get("Location"), testUser().ID.String())
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "alice@example.com", resp["email"])
	assert.Equal(t, "Alice", resp["display_name"])
	// PasswordHash must NEVER appear in the response
	assert.NotContains(t, w.Body.String(), "password_hash")
}

// TestRegisterHandler_EmailTaken_Returns409 verifies AC2: duplicate email
// returns 409 Conflict.
func TestRegisterHandler_EmailTaken_Returns409(t *testing.T) {
	t.Parallel()

	svc := &stubRegisterService{err: message.ErrUserEmailTaken}
	handler := pfmhttp.RegisterHandler(svc)

	body := `{"email":"taken@example.com","display_name":"Bob","password":"secret123"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusConflict, w.Code)
}

// TestRegisterHandler_MissingFields_Returns400 verifies AC10: missing required
// fields return 400 with per-field validation details.
func TestRegisterHandler_MissingFields_Returns400(t *testing.T) {
	t.Parallel()

	svc := &stubRegisterService{}
	handler := pfmhttp.RegisterHandler(svc)

	// All fields empty — domain logic returns ValidationError
	svc.err = &validate.ValidationError{
		Violations: []validate.Violation{
			{Field: "email", Message: "is required"},
			{Field: "display_name", Message: "is required"},
			{Field: "password", Message: "is required"},
		},
	}

	body := `{"email":"","display_name":"","password":""}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, message.MsgValidationFailed, resp["error"])
	fields, ok := resp["fields"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, fields, 3)
}

// TestRegisterHandler_MalformedJSON_Returns400 verifies edge case: unparseable
// body returns 400, not 500.
func TestRegisterHandler_MalformedJSON_Returns400(t *testing.T) {
	t.Parallel()

	svc := &stubRegisterService{}
	handler := pfmhttp.RegisterHandler(svc)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader("{bad"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestRegisterHandler_InternalError_Returns500 verifies AC6 (from #122):
// unexpected errors return 500 with generic message.
func TestRegisterHandler_InternalError_Returns500(t *testing.T) {
	t.Parallel()

	svc := &stubRegisterService{err: assert.AnError}
	handler := pfmhttp.RegisterHandler(svc)

	body := `{"email":"a@b.com","display_name":"X","password":"secret123"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestRegisterHandler_NilService_Panics verifies the nil guard fires at wiring time.
func TestRegisterHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.RegisterHandler(nil) })
}

// --- GetUserHandler tests ---

// stubGetUserService is a test double for the getUserService interface.
type stubGetUserService struct {
	user domainuser.User
	err  error
}

func (s *stubGetUserService) FindByID(_ context.Context, _ uuid.UUID) (domainuser.User, error) {
	return s.user, s.err
}

// TestGetUserHandler_ValidID_Returns200 verifies AC3: an authenticated user
// can retrieve a profile by ID, getting a 200 OK with the user.
func TestGetUserHandler_ValidID_Returns200(t *testing.T) {
	t.Parallel()

	svc := &stubGetUserService{user: testUser()}
	handler := pfmhttp.GetUserHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+testUser().ID.String(), nil)
	r.SetPathValue("id", testUser().ID.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "alice@example.com", resp["email"])
	assert.NotContains(t, w.Body.String(), "password_hash")
}

// TestGetUserHandler_NotFound_Returns404 verifies AC4: a non-existent user ID
// returns 404 Not Found.
func TestGetUserHandler_NotFound_Returns404(t *testing.T) {
	t.Parallel()

	svc := &stubGetUserService{err: message.ErrUserNotFound}
	handler := pfmhttp.GetUserHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+uuid.Nil.String(), nil)
	r.SetPathValue("id", uuid.Nil.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestGetUserHandler_InvalidUUID_Returns400 verifies edge case: a malformed UUID
// in the path returns 400, not 500.
func TestGetUserHandler_InvalidUUID_Returns400(t *testing.T) {
	t.Parallel()

	svc := &stubGetUserService{}
	handler := pfmhttp.GetUserHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/users/not-a-uuid", nil)
	r.SetPathValue("id", "not-a-uuid")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGetUserHandler_NilService_Panics verifies the nil guard fires at wiring time.
func TestGetUserHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.GetUserHandler(nil) })
}

// --- UpdateProfileHandler tests ---

// stubUpdateProfileService is a test double for the updateProfileService interface.
type stubUpdateProfileService struct {
	user domainuser.User
	err  error
}

func (s *stubUpdateProfileService) UpdateProfile(_ context.Context, _ uuid.UUID, _ domainuser.UpdateProfileInput, _ int, _ uuid.UUID) (domainuser.User, error) {
	return s.user, s.err
}

// TestUpdateProfileHandler_ValidRequest_Returns200 verifies AC5: a valid update
// with correct version returns 200 OK with the updated user.
func TestUpdateProfileHandler_ValidRequest_Returns200(t *testing.T) {
	t.Parallel()

	updated := testUser()
	updated.DisplayName = "Alice Updated"
	svc := &stubUpdateProfileService{user: updated}
	handler := pfmhttp.UpdateProfileHandler(svc)

	body := `{"display_name":"Alice Updated","version":1}`
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/"+testUser().ID.String(), strings.NewReader(body))
	r.SetPathValue("id", testUser().ID.String())
	r = r.WithContext(ctxWithUser(testUser().ID))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "Alice Updated", resp["display_name"])
}

// TestUpdateProfileHandler_StaleVersion_Returns409 verifies AC6: a stale version
// returns 409 Conflict.
func TestUpdateProfileHandler_StaleVersion_Returns409(t *testing.T) {
	t.Parallel()

	svc := &stubUpdateProfileService{err: message.ErrUserVersionConflict}
	handler := pfmhttp.UpdateProfileHandler(svc)

	body := `{"display_name":"New Name","version":1}`
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/"+testUser().ID.String(), strings.NewReader(body))
	r.SetPathValue("id", testUser().ID.String())
	r = r.WithContext(ctxWithUser(testUser().ID))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusConflict, w.Code)
}

// TestUpdateProfileHandler_ValidationError_Returns400 verifies AC10: missing
// display_name returns 400 with field-level details.
func TestUpdateProfileHandler_ValidationError_Returns400(t *testing.T) {
	t.Parallel()

	svc := &stubUpdateProfileService{
		err: &validate.ValidationError{
			Violations: []validate.Violation{{Field: "display_name", Message: "is required"}},
		},
	}
	handler := pfmhttp.UpdateProfileHandler(svc)

	body := `{"display_name":"","version":1}`
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/"+testUser().ID.String(), strings.NewReader(body))
	r.SetPathValue("id", testUser().ID.String())
	r = r.WithContext(ctxWithUser(testUser().ID))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestUpdateProfileHandler_MalformedJSON_Returns400 verifies edge case.
func TestUpdateProfileHandler_MalformedJSON_Returns400(t *testing.T) {
	t.Parallel()

	svc := &stubUpdateProfileService{}
	handler := pfmhttp.UpdateProfileHandler(svc)

	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/"+testUser().ID.String(), strings.NewReader("{bad"))
	r.SetPathValue("id", testUser().ID.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestUpdateProfileHandler_NilService_Panics verifies the nil guard.
func TestUpdateProfileHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.UpdateProfileHandler(nil) })
}

// --- ChangePasswordHandler tests ---

// stubChangePasswordService is a test double for the changePasswordService interface.
type stubChangePasswordService struct {
	user domainuser.User
	err  error
}

func (s *stubChangePasswordService) ChangePassword(_ context.Context, _ uuid.UUID, _ domainuser.ChangePasswordInput, _ int, _ uuid.UUID) (domainuser.User, error) {
	return s.user, s.err
}

// TestChangePasswordHandler_ValidRequest_Returns200 verifies AC7: a valid password
// change with correct old password returns 200 OK.
func TestChangePasswordHandler_ValidRequest_Returns200(t *testing.T) {
	t.Parallel()

	svc := &stubChangePasswordService{user: testUser()}
	handler := pfmhttp.ChangePasswordHandler(svc)

	body := `{"old_password":"oldpass123","new_password":"newpass123","version":1}`
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/"+testUser().ID.String()+"/password", strings.NewReader(body))
	r.SetPathValue("id", testUser().ID.String())
	r = r.WithContext(ctxWithUser(testUser().ID))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.NotContains(t, w.Body.String(), "password_hash")
}

// TestChangePasswordHandler_WrongPassword_Returns401 verifies AC8: incorrect
// old password returns the appropriate error response.
func TestChangePasswordHandler_WrongPassword_Returns401(t *testing.T) {
	t.Parallel()

	svc := &stubChangePasswordService{err: message.ErrLoginInvalidCredentials}
	handler := pfmhttp.ChangePasswordHandler(svc)

	body := `{"old_password":"wrong","new_password":"newpass123","version":1}`
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/"+testUser().ID.String()+"/password", strings.NewReader(body))
	r.SetPathValue("id", testUser().ID.String())
	r = r.WithContext(ctxWithUser(testUser().ID))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestChangePasswordHandler_ValidationError_Returns400 verifies AC10: missing
// fields return 400.
func TestChangePasswordHandler_ValidationError_Returns400(t *testing.T) {
	t.Parallel()

	svc := &stubChangePasswordService{
		err: &validate.ValidationError{
			Violations: []validate.Violation{
				{Field: "old_password", Message: "is required"},
				{Field: "new_password", Message: "is required"},
			},
		},
	}
	handler := pfmhttp.ChangePasswordHandler(svc)

	body := `{"old_password":"","new_password":"","version":1}`
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/"+testUser().ID.String()+"/password", strings.NewReader(body))
	r.SetPathValue("id", testUser().ID.String())
	r = r.WithContext(ctxWithUser(testUser().ID))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestChangePasswordHandler_MalformedJSON_Returns400 verifies edge case.
func TestChangePasswordHandler_MalformedJSON_Returns400(t *testing.T) {
	t.Parallel()

	svc := &stubChangePasswordService{}
	handler := pfmhttp.ChangePasswordHandler(svc)

	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/"+testUser().ID.String()+"/password", strings.NewReader("{bad"))
	r.SetPathValue("id", testUser().ID.String())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestChangePasswordHandler_NilService_Panics verifies the nil guard.
func TestChangePasswordHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.ChangePasswordHandler(nil) })
}

// --- DeactivateUserHandler tests ---

// stubDeactivateUserService is a test double for the deactivateUserService interface.
type stubDeactivateUserService struct {
	err error
}

func (s *stubDeactivateUserService) Deactivate(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return s.err
}

// TestDeactivateUserHandler_Success_Returns204 verifies AC9: deactivating a user
// returns 204 No Content with an empty body.
func TestDeactivateUserHandler_Success_Returns204(t *testing.T) {
	t.Parallel()

	svc := &stubDeactivateUserService{}
	handler := pfmhttp.DeactivateUserHandler(svc)

	r := httptest.NewRequest(http.MethodDelete, "/api/v1/users/"+testUser().ID.String(), nil)
	r.SetPathValue("id", testUser().ID.String())
	r = r.WithContext(ctxWithUser(testUser().ID))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

// TestDeactivateUserHandler_NotFound_Returns404 verifies edge case: deactivating
// a non-existent user returns 404.
func TestDeactivateUserHandler_NotFound_Returns404(t *testing.T) {
	t.Parallel()

	svc := &stubDeactivateUserService{err: message.ErrUserNotFound}
	handler := pfmhttp.DeactivateUserHandler(svc)

	r := httptest.NewRequest(http.MethodDelete, "/api/v1/users/"+uuid.Nil.String(), nil)
	r.SetPathValue("id", uuid.Nil.String())
	r = r.WithContext(ctxWithUser(testUser().ID))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestDeactivateUserHandler_InvalidUUID_Returns400 verifies edge case.
func TestDeactivateUserHandler_InvalidUUID_Returns400(t *testing.T) {
	t.Parallel()

	svc := &stubDeactivateUserService{}
	handler := pfmhttp.DeactivateUserHandler(svc)

	r := httptest.NewRequest(http.MethodDelete, "/api/v1/users/not-a-uuid", nil)
	r.SetPathValue("id", "not-a-uuid")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestDeactivateUserHandler_NilService_Panics verifies the nil guard.
func TestDeactivateUserHandler_NilService_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { pfmhttp.DeactivateUserHandler(nil) })
}
