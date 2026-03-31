package http

import (
	"context"
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
//
// @Summary Authenticate and obtain a token
// @Tags auth
// @Accept json
// @Produce json
// @Param body body loginRequest true "Login credentials"
// @Success 200 {object} loginResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /auth/login [post]
func LoginHandler(svc loginService) http.HandlerFunc {
	if svc == nil {
		panic("http: LoginHandler requires non-nil loginService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := DecodeBody(r, &req); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgLoginBadRequest})
			return
		}

		v := validate.NewResult()
		v.Field("email", req.Email, validate.Required)
		v.Field("password", req.Password, validate.Required)
		if err := v.Error(); err != nil {
			var ve *validate.ValidationError
			errors.As(err, &ve)
			WriteValidationError(w, ve)
			return
		}

		result, err := svc.Login(r.Context(), req.Email, req.Password)
		if err != nil {
			if errors.Is(err, message.ErrLoginInvalidCredentials) {
				WriteError(w, err)
				return
			}
			slog.ErrorContext(r.Context(), message.MsgServerError, "error", err)
			WriteError(w, err)
			return
		}

		WriteJSON(w, http.StatusOK, loginResponse{
			Token:     result.Token,
			ExpiresAt: result.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
}

