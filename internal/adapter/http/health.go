package http

import (
	"encoding/json"
	"net/http"
)

// HealthHandler returns an http.HandlerFunc that responds to GET /healthz
// With a JSON body containing the application status and version.
func HealthHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{ // best-effort
			"status":  "ok",
			"version": version,
		})
	}
}
