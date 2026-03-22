package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zambone/pfm-go/internal/adapter/auth"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/middleware"
	"github.com/zambone/pfm-go/internal/platform/clock"
	"github.com/zambone/pfm-go/internal/platform/ctxutil"
)

var (
	fixedTime  = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	testUserID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
)

// okHandler is a test handler that records whether it was called and
// returns the user ID from context as a JSON body.
func okHandler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := ctxutil.UserID(r.Context())
		if !ok {
			http.Error(w, "no user id in context", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"user_id": userID.String()}) // best-effort
	})
}

// errorBody decodes a JSON error response body into a map.
func errorBody(t *testing.T, w *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	var body map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	return body
}

// newSvc builds a FakeTokenService pre-loaded with a valid token for testUserID.
func newSvc(t *testing.T) (*auth.FakeTokenService, string) {
	t.Helper()
	clk := clock.NewFakeClock(fixedTime)
	svc := auth.NewFakeTokenService(clk)
	token, err := svc.Issue(context.Background(), testUserID, time.Hour)
	require.NoError(t, err)
	return svc, token
}

// TestAuthnMiddleware_ValidToken_InjectsUserID verifies AC1:
// a valid Bearer token causes the user ID to be injected into context.
func TestAuthnMiddleware_ValidToken_InjectsUserID(t *testing.T) {
	svc, token := newSvc(t)
	handler := middleware.Authn(svc)(okHandler(t))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	body := errorBody(t, w)
	assert.Equal(t, testUserID.String(), body["user_id"])
}

// TestAuthnMiddleware_NoAuthHeader_Returns401 verifies AC3:
// a request with no Authorization header is rejected with 401.
func TestAuthnMiddleware_NoAuthHeader_Returns401(t *testing.T) {
	svc, _ := newSvc(t)
	handler := middleware.Authn(svc)(okHandler(t))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, message.MsgAuthnMissingToken, errorBody(t, w)["error"])
}

// TestAuthnMiddleware_InvalidToken_Returns401 verifies AC2:
// an invalid token is rejected with 401 and a generic error message.
func TestAuthnMiddleware_InvalidToken_Returns401(t *testing.T) {
	svc, _ := newSvc(t)
	handler := middleware.Authn(svc)(okHandler(t))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer not-a-real-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, message.MsgAuthnInvalidToken, errorBody(t, w)["error"])
}

// TestAuthnMiddleware_ExpiredToken_Returns401 verifies AC4:
// an expired token is rejected with 401 and a specific expiry message.
func TestAuthnMiddleware_ExpiredToken_Returns401(t *testing.T) {
	clk := clock.NewFakeClock(fixedTime)
	svc := auth.NewFakeTokenService(clk)
	token, err := svc.Issue(context.Background(), testUserID, time.Hour)
	require.NoError(t, err)

	// Advance clock past expiry.
	clk.Advance(2 * time.Hour)

	handler := middleware.Authn(svc)(okHandler(t))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, message.MsgAuthnExpiredToken, errorBody(t, w)["error"])
}

// TestAuthnMiddleware_MalformedHeader_Returns401 verifies edge cases:
// headers missing the Bearer prefix or with extra whitespace are rejected.
func TestAuthnMiddleware_MalformedHeader_Returns401(t *testing.T) {
	svc, token := newSvc(t)
	handler := middleware.Authn(svc)(okHandler(t))

	cases := []struct {
		name   string
		header string
	}{
		{"no bearer prefix", "Token " + token},
		{"bare token", token},
		{"empty bearer", "Bearer "},
		{"two tokens", "Bearer " + token + " extra"},
		{"bearer lowercase", "bearer " + token},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set("Authorization", tc.header)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, r)

			assert.Equal(t, http.StatusUnauthorized, w.Code, "expected 401 for header: %q", tc.header)
			assert.Equal(t, message.MsgAuthnMissingToken, errorBody(t, w)["error"])
		})
	}
}

// TestAuthn_PanicsOnNilTokenValidator verifies that passing nil panics at
// wiring time rather than at request time.
func TestAuthn_PanicsOnNilTokenValidator(t *testing.T) {
	assert.Panics(t, func() {
		middleware.Authn(nil)
	})
}

// TestAuthnMiddleware_AppliedToRoute_NextReceivesContext verifies AC5:
// a mux-registered route wrapped with the middleware delivers authenticated context.
func TestAuthnMiddleware_AppliedToRoute_NextReceivesContext(t *testing.T) {
	svc, token := newSvc(t)

	mux := http.NewServeMux()
	mux.Handle("GET /protected", middleware.Authn(svc)(okHandler(t)))

	r := httptest.NewRequest(http.MethodGet, "/protected", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	body := errorBody(t, w)
	assert.Equal(t, testUserID.String(), body["user_id"])
}

// BenchmarkAuthnMiddleware_ValidRequest measures the per-request cost of the
// middleware on the happy path — runs on every authenticated API request.
func BenchmarkAuthnMiddleware_ValidRequest(b *testing.B) {
	svc, token := func() (*auth.FakeTokenService, string) {
		clk := clock.NewFakeClock(fixedTime)
		svc := auth.NewFakeTokenService(clk)
		tok, err := svc.Issue(context.Background(), testUserID, time.Hour)
		if err != nil {
			b.Fatal(err)
		}
		return svc, tok
	}()

	handler := middleware.Authn(svc)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+token)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	}
}
