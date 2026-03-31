package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	domainuser "github.com/zambone/pfm-go/internal/domain/user"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/ctxutil"
	"github.com/zambone/pfm-go/internal/platform/validate"
)

// userResponse is the JSON representation of a user. It deliberately excludes
// PasswordHash so that sensitive data never reaches the wire.
type userResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Version     int    `json:"version"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// toUserResponse maps a domain User to its JSON-safe response representation.
func toUserResponse(u domainuser.User) userResponse {
	return userResponse{
		ID:          u.ID.String(),
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Version:     u.Version,
		CreatedAt:   u.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   u.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// handleDomainError writes the appropriate HTTP error response for a domain error.
// It checks for ValidationError first (400), then delegates to WriteError for all
// other domain errors (404, 409, 500, etc.). Unexpected errors are logged at ERROR.
func handleDomainError(w http.ResponseWriter, r *http.Request, err error) {
	var ve *validate.ValidationError
	if errors.As(err, &ve) {
		WriteValidationError(w, ve)
		return
	}

	// Log unexpected errors (5xx) at ERROR; known domain errors need no logging
	// because they represent expected business outcomes (not found, conflict, etc.).
	status, _ := MapError(err)
	if status >= http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), message.MsgServerError, "error", err)
	}

	WriteError(w, err)
}

// parseUUID parses a UUID string from a path parameter. Returns an error
// if the value is not a valid UUID, which handlers translate to 400 Bad Request.
func parseUUID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse uuid %q: %w", value, err)
	}
	return id, nil
}

// --- Register ---

// registerService is the subset of UserLogic required by RegisterHandler.
type registerService interface {
	Register(ctx context.Context, input domainuser.RegisterInput, callerID uuid.UUID) (domainuser.User, error)
}

type registerRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

// RegisterHandler returns an http.HandlerFunc that handles user registration.
// On success it writes a 201 Created response with the new user and a Location header.
// Panics if svc is nil.
//
// @Summary Register a new user
// @Tags users
// @Accept json
// @Produce json
// @Param body body registerRequest true "Registration input"
// @Success 201 {object} userResponse
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/v1/users [post]
func RegisterHandler(svc registerService) http.HandlerFunc {
	if svc == nil {
		panic("http: RegisterHandler requires non-nil registerService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerRequest
		if err := DecodeBody(r, &req); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgBadRequestBody})
			return
		}

		// callerID: for self-registration there is no authenticated user yet,
		// so we use uuid.Nil as a sentinel meaning "self-created".
		callerID, _ := ctxutil.UserID(r.Context())

		u, err := svc.Register(r.Context(), domainuser.RegisterInput{
			Email:       req.Email,
			DisplayName: req.DisplayName,
			Password:    req.Password,
		}, callerID)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		location := fmt.Sprintf("/api/v1/users/%s", u.ID)
		WriteCreated(w, location, toUserResponse(u))
	}
}

// --- GetUser ---

// getUserService is the subset of UserLogic required by GetUserHandler.
type getUserService interface {
	FindByID(ctx context.Context, id uuid.UUID) (domainuser.User, error)
}

// GetUserHandler returns an http.HandlerFunc that retrieves a user by ID.
// The user ID is read from the URL path parameter "id". Panics if svc is nil.
//
// @Summary Get user by ID
// @Tags users
// @Produce json
// @Param id path string true "User ID (UUID)"
// @Success 200 {object} userResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /api/v1/users/{id} [get]
func GetUserHandler(svc getUserService) http.HandlerFunc {
	if svc == nil {
		panic("http: GetUserHandler requires non-nil getUserService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseUUID(r.PathValue("id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		u, err := svc.FindByID(r.Context(), id)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, toUserResponse(u))
	}
}

// --- ChangePassword ---

// changePasswordService is the subset of UserLogic required by ChangePasswordHandler.
type changePasswordService interface {
	ChangePassword(ctx context.Context, id uuid.UUID, input domainuser.ChangePasswordInput, expectedVersion int, callerID uuid.UUID) (domainuser.User, error)
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
	Version     int    `json:"version"`
}

// ChangePasswordHandler returns an http.HandlerFunc that changes a user's password.
// The user ID is read from the URL path parameter "id". Panics if svc is nil.
//
// @Summary Change user password
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID (UUID)"
// @Param body body changePasswordRequest true "Password change input"
// @Success 200 {object} userResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Security BearerAuth
// @Router /api/v1/users/{id}/password [put]
func ChangePasswordHandler(svc changePasswordService) http.HandlerFunc {
	if svc == nil {
		panic("http: ChangePasswordHandler requires non-nil changePasswordService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseUUID(r.PathValue("id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		var req changePasswordRequest
		if err := DecodeBody(r, &req); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgBadRequestBody})
			return
		}

		callerID, _ := ctxutil.UserID(r.Context())

		u, err := svc.ChangePassword(r.Context(), id, domainuser.ChangePasswordInput{
			OldPassword: req.OldPassword,
			NewPassword: req.NewPassword,
		}, req.Version, callerID)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, toUserResponse(u))
	}
}

// --- Deactivate ---

// deactivateUserService is the subset of UserLogic required by DeactivateUserHandler.
type deactivateUserService interface {
	Deactivate(ctx context.Context, id uuid.UUID, callerID uuid.UUID) error
}

// DeactivateUserHandler returns an http.HandlerFunc that soft-deletes a user.
// The user ID is read from the URL path parameter "id". Panics if svc is nil.
//
// @Summary Deactivate a user
// @Tags users
// @Param id path string true "User ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /api/v1/users/{id} [delete]
func DeactivateUserHandler(svc deactivateUserService) http.HandlerFunc {
	if svc == nil {
		panic("http: DeactivateUserHandler requires non-nil deactivateUserService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseUUID(r.PathValue("id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		callerID, _ := ctxutil.UserID(r.Context())

		if err := svc.Deactivate(r.Context(), id, callerID); err != nil {
			handleDomainError(w, r, err)
			return
		}

		WriteNoContent(w)
	}
}

// --- UpdateProfile ---

// updateProfileService is the subset of UserLogic required by UpdateProfileHandler.
type updateProfileService interface {
	UpdateProfile(ctx context.Context, id uuid.UUID, input domainuser.UpdateProfileInput, expectedVersion int, callerID uuid.UUID) (domainuser.User, error)
}

type updateProfileRequest struct {
	DisplayName string `json:"display_name"`
	Version     int    `json:"version"`
}

// UpdateProfileHandler returns an http.HandlerFunc that updates a user's profile.
// The user ID is read from the URL path parameter "id". The caller ID is read from
// the request context (set by authn middleware). Panics if svc is nil.
//
// @Summary Update user profile
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID (UUID)"
// @Param body body updateProfileRequest true "Profile update input"
// @Success 200 {object} userResponse
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Security BearerAuth
// @Router /api/v1/users/{id} [put]
func UpdateProfileHandler(svc updateProfileService) http.HandlerFunc {
	if svc == nil {
		panic("http: UpdateProfileHandler requires non-nil updateProfileService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseUUID(r.PathValue("id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		var req updateProfileRequest
		if err := DecodeBody(r, &req); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgBadRequestBody})
			return
		}

		callerID, _ := ctxutil.UserID(r.Context())

		u, err := svc.UpdateProfile(r.Context(), id, domainuser.UpdateProfileInput{
			DisplayName: req.DisplayName,
		}, req.Version, callerID)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, toUserResponse(u))
	}
}
