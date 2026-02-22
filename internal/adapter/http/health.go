package http

import (
	"encoding/json"
	"net/http"
	"sync/atomic"

	"github.com/zambone/pfm-go/internal/message"
)

const (
	headerContentType = "Content-Type"
	mimeJSON          = "application/json"
)

// HealthHandler returns an http.HandlerFunc that responds to GET /healthz
// With a JSON body containing the application status and version.
func HealthHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set(headerContentType, mimeJSON)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{ // best-effort
			"status":  "ok",
			"version": version,
		})
	}
}

// LiveHandler returns an http.HandlerFunc that responds to GET /health/live.
// It always returns 200 ok as long as the process is running.
func LiveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set(headerContentType, mimeJSON)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{ // best-effort
			"status": "ok",
		})
	}
}

// ReadyHandler returns an http.HandlerFunc that responds to GET /health/ready.
// It returns 503 Service Unavailable when the application is shutting down.
func ReadyHandler(shuttingDown *atomic.Bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set(headerContentType, mimeJSON)
		if shuttingDown.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status":  "error",
				"message": message.MsgHealthNotReady,
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{ // best-effort
			"status": "ok",
		})
	}
}
