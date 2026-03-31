package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pfmhttp "github.com/zambone/pfm-go/internal/adapter/http"
)

// TestDocsHandler_ReturnsHTML verifies AC4: a developer visiting /docs gets
// interactive API documentation.
func TestDocsHandler_ReturnsHTML(t *testing.T) {
	t.Parallel()

	handler := pfmhttp.DocsHandler()
	r := httptest.NewRequest(http.MethodGet, "/docs", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "scalar")
	assert.Contains(t, w.Body.String(), "/api/v1/openapi.yaml")
}

// TestOpenAPIHandler_ReturnsYAML verifies AC1: the spec is served as YAML
// and contains expected API metadata.
func TestOpenAPIHandler_ReturnsYAML(t *testing.T) {
	t.Parallel()

	handler := pfmhttp.OpenAPIHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/openapi.yaml", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/yaml", w.Header().Get("Content-Type"))

	body := w.Body.String()
	// Verify it's a valid OpenAPI spec with expected content.
	assert.Contains(t, body, "PFM-Go API")
	assert.Contains(t, body, "/api/v1/users")
	assert.Contains(t, body, "/api/v1/households")
	assert.Contains(t, body, "/api/v1/households/{household_id}/accounts")
	assert.Contains(t, body, "/api/v1/households/{household_id}/transactions")
	assert.Contains(t, body, "credit-card-settings")
	assert.Contains(t, body, "BearerAuth")
}

// TestOpenAPIHandler_ContainsAllEndpoints verifies AC1: every registered
// endpoint has a corresponding OpenAPI operation.
func TestOpenAPIHandler_ContainsAllEndpoints(t *testing.T) {
	t.Parallel()

	handler := pfmhttp.OpenAPIHandler()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/openapi.yaml", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	body := w.Body.String()

	// Every domain endpoint path must appear in the spec.
	paths := []string{
		"/auth/login",
		"/api/v1/users",
		"/api/v1/users/{id}",
		"/api/v1/users/{id}/password",
		"/api/v1/households",
		"/api/v1/households/{household_id}",
		"/api/v1/households/{household_id}/members",
		"/api/v1/households/{household_id}/members/{user_id}",
		"/api/v1/households/{household_id}/accounts",
		"/api/v1/households/{household_id}/accounts/{id}",
		"/api/v1/households/{household_id}/accounts/{id}/name",
		"/api/v1/households/{household_id}/accounts/{id}/balance",
		"/api/v1/households/{household_id}/accounts/{account_id}/credit-card-settings",
		"/api/v1/households/{household_id}/accounts/{account_id}/credit-card-settings/closing-day",
		"/api/v1/households/{household_id}/accounts/{account_id}/credit-card-settings/due-day",
		"/api/v1/households/{household_id}/accounts/{account_id}/credit-card-settings/limit",
		"/api/v1/households/{household_id}/accounts/{account_id}/balance",
		"/api/v1/households/{household_id}/transactions",
	}

	for _, p := range paths {
		assert.Contains(t, body, p, "missing endpoint: %s", p)
	}
}

// TestDocsEndpoints_ReachableViaRouter verifies that /docs and /api/v1/openapi.yaml
// are registered in the router and reachable.
func TestDocsEndpoints_ReachableViaRouter(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	pfmhttp.RegisterRoutes(mux, buildTestDeps())

	tests := []struct {
		path       string
		wantStatus int
		wantCT     string
	}{
		{"/docs", http.StatusOK, "text/html; charset=utf-8"},
		{"/api/v1/openapi.yaml", http.StatusOK, "application/yaml"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)

			require.Equal(t, tc.wantStatus, w.Code)
			assert.Equal(t, tc.wantCT, w.Header().Get("Content-Type"))
		})
	}
}
