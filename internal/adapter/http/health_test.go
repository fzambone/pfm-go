package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pfmhttp "github.com/zambone/pfm-go/internal/adapter/http"
	"github.com/zambone/pfm-go/internal/message"
)

func TestHealthHandler_Get_ReturnsOk(t *testing.T) {
	handler := pfmhttp.HealthHandler("v1.2.3")

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
	assert.Equal(t, "v1.2.3", body["version"])
}

func TestHealthHandler_NoGet_ReturnsMethodNotAllowed(t *testing.T) {
	handler := pfmhttp.HealthHandler("V1.2.3")

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/healthz", nil)
			rec := httptest.NewRecorder()
			handler(rec, req)

			assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
		})
	}
}

func TestHealthHandler_Get_SetContentTypeJSON(t *testing.T) {
	handler := pfmhttp.HealthHandler("v1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

func TestLiveHandler_Get_ReturnsOk(t *testing.T) {
	handler := pfmhttp.LiveHandler()

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
	assert.Empty(t, body["version"]) // liveness carries no version field
}

func TestLiveHandler_NonGet_ReturnsMethodNotAllowed(t *testing.T) {
	handler := pfmhttp.LiveHandler()

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/health/live", nil)
			rec := httptest.NewRecorder()
			handler(rec, req)

			assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
		})
	}
}

func TestLiveHandler_Get_SetsContentTypeJSON(t *testing.T) {
	handler := pfmhttp.LiveHandler()

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

func TestReadyHandler_WhenReady_ReturnsOk(t *testing.T) {
	var shuttingDown atomic.Bool
	handler := pfmhttp.ReadyHandler(&shuttingDown)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
}

func TestReadyHandler_WhenShuttingDown_ReturnsServiceUnavailable(t *testing.T) {
	var shuttingDown atomic.Bool
	shuttingDown.Store(true)
	handler := pfmhttp.ReadyHandler(&shuttingDown)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "error", body["status"])
	assert.Equal(t, message.MsgHealthNotReady, body["message"])
}

func TestReadyHandler_NonGet_ReturnsMethodNotAllowed(t *testing.T) {
	var shuttingDown atomic.Bool
	handler := pfmhttp.ReadyHandler(&shuttingDown)

	methods := []string{http.MethodPost, http.MethodDelete, http.MethodPut, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/health/ready", nil)
			rec := httptest.NewRecorder()
			handler(rec, req)

			assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
		})
	}
}

func TestReadyHandler_WhenReady_SetsContentTypeJSON(t *testing.T) {
	var shuttingDown atomic.Bool
	handler := pfmhttp.ReadyHandler(&shuttingDown)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

func TestReadyHandler_WhenShuttingDown_SetsContentTypeJSON(t *testing.T) {
	var shuttingDown atomic.Bool
	shuttingDown.Store(true)
	handler := pfmhttp.ReadyHandler(&shuttingDown)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}
