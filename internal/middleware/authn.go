// Package middleware provides HTTP middleware for the pfm-go application.
package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/ctxutil"
)

const (
	headerAuthorization = "Authorization"
	bearerPrefix        = "Bearer "
	headerContentType   = "Content-Type"
	mimeJSON            = "application/json"
)

// tokenValidator is the subset of port/auth.TokenService required by Authn.
// Accepting an interface defined here (rather than importing port/auth) follows
// interface segregation: the middleware only needs Validate, not Issue.
type tokenValidator interface {
	Validate(ctx context.Context, token string) (uuid.UUID, error)
}

// Authn returns middleware that enforces Bearer token authentication.
// It extracts the token from the Authorization header, validates it with tv,
// and injects the authenticated user ID into the request context via ctxutil.
// Any failure returns 401 Unauthorized with a JSON error body before the next
// handler is called. Panics if tv is nil.
func Authn(tv tokenValidator) func(http.Handler) http.Handler {
	if tv == nil {
		panic("middleware: Authn requires non-nil tokenValidator")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := extractBearer(r)
			if !ok {
				writeJSON(w, http.StatusUnauthorized, message.MsgAuthnMissingToken)
				return
			}

			userID, err := tv.Validate(r.Context(), token)
			if err != nil {
				if errors.Is(err, message.ErrTokenExpired) {
					writeJSON(w, http.StatusUnauthorized, message.MsgAuthnExpiredToken)
					return
				}
				writeJSON(w, http.StatusUnauthorized, message.MsgAuthnInvalidToken)
				return
			}

			next.ServeHTTP(w, r.WithContext(ctxutil.WithUserID(r.Context(), userID)))
		})
	}
}

// extractBearer parses the Authorization header and returns the raw token.
// Returns ("", false) if the header is absent, missing the "Bearer " prefix,
// contains whitespace in the token, or is otherwise malformed.
func extractBearer(r *http.Request) (string, bool) {
	h := r.Header.Get(headerAuthorization)
	if h == "" {
		return "", false
	}
	// Require exact "Bearer " prefix — scheme is case-sensitive per convention.
	if !strings.HasPrefix(h, bearerPrefix) {
		return "", false
	}
	token := h[len(bearerPrefix):]
	// Reject empty tokens and tokens containing whitespace (e.g. "tok1 tok2").
	if token == "" || strings.ContainsAny(token, " \t") {
		return "", false
	}
	return token, true
}

// writeJSON writes a JSON body of the form {"error": "<msg>"} with the given status code.
func writeJSON(w http.ResponseWriter, status int, msg string) {
	w.Header().Set(headerContentType, mimeJSON)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg}) // best-effort
}
