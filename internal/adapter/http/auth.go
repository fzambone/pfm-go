package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	domainuser "github.com/zambone/pfm-go/internal/domain/user"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/validate"
)

// loginService is the subset of domain/user.LoginLogic required by LoginHandler.
// Defined locally to follow interface segregation and keep the HTTP layer decoupled
// from the concrete domain type.
type loginService interface {
	Login(ctx context.Context, email, password string) (domainuser.LoginResult, error)
}

// loginRequest is the expected JSON body for POST /auth/login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// loginResponse is the JSON body returned on successful authentication.
type loginResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"` // RFC 3339 UTC
}

// LoginHandler returns an http.HandlerFunc that handles POST /auth/login.
// It decodes credentials from the request body, delegates to svc, and writes
// a JSON response. Panics if svc is nil.
func LoginHandler(svc loginService) http.HandlerFunc {
	if svc == nil {
		panic("http: LoginHandler requires non-nil loginService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, message.MsgLoginBadRequest)
			return
		}

		v := validate.NewResult()
		v.Field("email", req.Email, validate.Required)
		v.Field("password", req.Password, validate.Required)
		if err := v.Error(); err != nil {
			var ve *validate.ValidationError
			errors.As(err, &ve)
			writeJSONValidationError(w, ve)
			return
		}

		result, err := svc.Login(r.Context(), req.Email, req.Password)
		if err != nil {
			if errors.Is(err, message.ErrLoginInvalidCredentials) {
				writeJSONError(w, http.StatusUnauthorized, message.MsgLoginInvalidCredentials)
				return
			}
			slog.ErrorContext(r.Context(), message.MsgServerError, "error", err)
			writeJSONError(w, http.StatusInternalServerError, message.MsgServerError)
			return
		}

		w.Header().Set(headerContentType, mimeJSON)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(loginResponse{ // best-effort
			Token:     result.Token,
			ExpiresAt: result.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
}

// writeJSONError writes {"error": msg} with the given HTTP status.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set(headerContentType, mimeJSON)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg}) // best-effort
}

// writeJSONValidationError writes a 400 response with per-field validation details.
func writeJSONValidationError(w http.ResponseWriter, ve *validate.ValidationError) {
	fields := make(map[string]string, len(ve.Violations))
	for _, v := range ve.Violations {
		fields[v.Field] = v.Message
	}
	w.Header().Set(headerContentType, mimeJSON)
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]any{ // best-effort
		"error":  message.MsgValidationFailed,
		"fields": fields,
	})
}
