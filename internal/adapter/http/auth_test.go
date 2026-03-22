package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pfmhttp "github.com/zambone/pfm-go/internal/adapter/http"
	domainuser "github.com/zambone/pfm-go/internal/domain/user"
	"github.com/zambone/pfm-go/internal/message"
)

// stubLoginService is a test double for the loginService interface.
// It returns whatever result/error is configured.
type stubLoginService struct {
	result domainuser.LoginResult
	err    error
}

func (s *stubLoginService) Login(_ context.Context, _, _ string) (domainuser.LoginResult, error) {
	return s.result, s.err
}

// loginBody builds a POST /auth/login JSON body.
func loginBody(email, password string) *bytes.Buffer {
	b, _ := json.Marshal(map[string]string{"email": email, "password": password})
	return bytes.NewBuffer(b)
}

// decodeBody decodes the JSON response body into a map.
func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	return body
}

var fixedExpiry = time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)

// TestLoginHandler_ValidCredentials_Returns200 verifies AC1:
// a successful login returns 200 with token and expires_at in the body.
func TestLoginHandler_ValidCredentials_Returns200(t *testing.T) {
	svc := &stubLoginService{result: domainuser.LoginResult{Token: "tok123", ExpiresAt: fixedExpiry}}
	handler := pfmhttp.LoginHandler(svc)

	r := httptest.NewRequest(http.MethodPost, "/auth/login", loginBody("u@example.com", "pass"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	body := decodeBody(t, w)
	assert.Equal(t, "tok123", body["token"])
	assert.Equal(t, "2026-01-01T01:00:00Z", body["expires_at"])
}

// TestLoginHandler_InvalidCredentials_Returns401 verifies AC2 and AC3:
// ErrLoginInvalidCredentials from the service produces a 401 with a generic message.
func TestLoginHandler_InvalidCredentials_Returns401(t *testing.T) {
	svc := &stubLoginService{
		err: fmt.Errorf("login: find user: %w", message.ErrLoginInvalidCredentials),
	}
	handler := pfmhttp.LoginHandler(svc)

	r := httptest.NewRequest(http.MethodPost, "/auth/login", loginBody("x@example.com", "wrong"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	body := decodeBody(t, w)
	assert.Equal(t, message.MsgLoginInvalidCredentials, body["error"])
}

// TestLoginHandler_EmptyEmail_Returns400 verifies that validation happens at the
// handler boundary — an empty email is rejected with 400 before calling the service.
func TestLoginHandler_EmptyEmail_Returns400(t *testing.T) {
	svc := &stubLoginService{} // must not be called
	handler := pfmhttp.LoginHandler(svc)

	r := httptest.NewRequest(http.MethodPost, "/auth/login", loginBody("", "pass"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
	body := decodeBody(t, w)
	assert.Equal(t, message.MsgValidationFailed, body["error"])
	fields, ok := body["fields"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "is required", fields["email"])
}

// TestLoginHandler_EmptyPassword_Returns400 verifies that an empty password
// is rejected at the handler boundary with 400.
func TestLoginHandler_EmptyPassword_Returns400(t *testing.T) {
	svc := &stubLoginService{} // must not be called
	handler := pfmhttp.LoginHandler(svc)

	r := httptest.NewRequest(http.MethodPost, "/auth/login", loginBody("u@example.com", ""))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
	body := decodeBody(t, w)
	assert.Equal(t, message.MsgValidationFailed, body["error"])
	fields, ok := body["fields"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "is required", fields["password"])
}

// TestLoginHandler_BothFieldsEmpty_Returns400WithTwoViolations verifies that
// validation collects all violations in a single pass (fail-all, not fail-fast).
func TestLoginHandler_BothFieldsEmpty_Returns400WithTwoViolations(t *testing.T) {
	svc := &stubLoginService{} // must not be called
	handler := pfmhttp.LoginHandler(svc)

	r := httptest.NewRequest(http.MethodPost, "/auth/login", loginBody("", ""))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
	body := decodeBody(t, w)
	fields, ok := body["fields"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, fields, 2, "both email and password violations must be reported")
}

// TestLoginHandler_MalformedJSON_Returns400 verifies that an unparseable body
// is rejected with 400 before the service is called.
func TestLoginHandler_MalformedJSON_Returns400(t *testing.T) {
	svc := &stubLoginService{}
	handler := pfmhttp.LoginHandler(svc)

	r := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader("{bad json}"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
	body := decodeBody(t, w)
	assert.Equal(t, message.MsgLoginBadRequest, body["error"])
}

// TestLoginHandler_InfraError_Returns500 verifies that an unexpected infrastructure
// error (DB down, token service failure) returns 500 with a generic server error.
func TestLoginHandler_InfraError_Returns500(t *testing.T) {
	svc := &stubLoginService{err: errors.New("connection refused")}
	handler := pfmhttp.LoginHandler(svc)

	r := httptest.NewRequest(http.MethodPost, "/auth/login", loginBody("u@example.com", "pass"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	body := decodeBody(t, w)
	assert.Equal(t, message.MsgServerError, body["error"])
}

// TestLoginHandler_NilService_Panics verifies the nil guard fires at wiring time.
func TestLoginHandler_NilService_Panics(t *testing.T) {
	assert.Panics(t, func() {
		pfmhttp.LoginHandler(nil)
	})
}

// BenchmarkLoginHandler_ValidRequest measures the per-request cost of the handler
// on the happy path — JSON decode + validation + service call + JSON encode.
func BenchmarkLoginHandler_ValidRequest(b *testing.B) {
	svc := &stubLoginService{result: domainuser.LoginResult{Token: "tok", ExpiresAt: fixedExpiry}}
	handler := pfmhttp.LoginHandler(svc)
	body := loginBody("u@example.com", "pass")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		r := httptest.NewRequest(http.MethodPost, "/auth/login", body)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	}
}
