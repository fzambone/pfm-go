package http

import (
	"net/http"

	"github.com/zambone/pfm-go/api"
)

const scalarHTML = `<!DOCTYPE html>
<html>
<head>
  <title>PFM-Go API Documentation</title>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
</head>
<body>
  <script id="api-reference" data-url="/api/v1/openapi.yaml"></script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`

// DocsHandler returns an http.HandlerFunc that serves the interactive API
// documentation page using Scalar (loaded from CDN). The page references
// the OpenAPI spec served at /api/v1/openapi.yaml.
func DocsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(scalarHTML)) // best-effort
	}
}

// OpenAPIHandler returns an http.HandlerFunc that serves the raw OpenAPI
// specification as YAML. The spec is embedded at build time from api/swagger.yaml.
func OpenAPIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(api.SwaggerYAML) // best-effort
	}
}
