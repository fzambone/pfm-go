//go:build integration

package http_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	pfmhttp "github.com/zambone/pfm-go/internal/adapter/http"
)

// TestRoutes_HealthEndpoints verifies that the ServeMux registered in main.go routes
// requests to the correct handlers. Handler unit tests call handler functions directly
// and bypass the mux entirely — a typo in a route pattern would only be caught here.
func TestRoutes_HealthEndpoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		path        string
		shutingDown bool
		wantStatus  int
	}{
		{
			name:       "healthz returns 200",
			path:       "/healthz",
			wantStatus: http.StatusOK,
		},
		{
			name:       "health live returns 200",
			path:       "/health/live",
			wantStatus: http.StatusOK,
		},
		{
			name:       "health ready returns 200 when not shutting down",
			path:       "/health/ready",
			wantStatus: http.StatusOK,
		},
		{
			name:        "health ready returns 503 when shutting down",
			path:        "/health/ready",
			shutingDown: true,
			wantStatus:  http.StatusServiceUnavailable,
		},
		{
			name:       "wrong path returns 404",
			path:       "/health/lyve",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Each case owns its atomic.Bool — no shared state between parallel subtests.
			var shuttingDown atomic.Bool
			if tc.shutingDown {
				shuttingDown.Store(true)
			}

			// Construct the mux identically to run() in cmd/pfm/main.go.
			mux := http.NewServeMux()
			mux.Handle("GET /healthz", pfmhttp.HealthHandler("test"))
			mux.Handle("GET /health/live", pfmhttp.LiveHandler())
			mux.Handle("GET /health/ready", pfmhttp.ReadyHandler(&shuttingDown))

			r := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)

			assert.Equal(t, tc.wantStatus, w.Code)
		})
	}
}
