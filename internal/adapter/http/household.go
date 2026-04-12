package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	domainhouse "github.com/zambone/pfm-go/internal/domain/household"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/ctxutil"
	"github.com/zambone/pfm-go/internal/platform/validate"
	"github.com/zambone/pfm-go/internal/types"
)

// householdResponse is the JSON representation of a household.
type householdResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Version   int    `json:"version"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// toHouseholdResponse maps a domain Household to its JSON-safe representation.
func toHouseholdResponse(h domainhouse.Household) householdResponse {
	return householdResponse{
		ID:        h.ID.String(),
		Name:      h.Name,
		Status:    string(h.Status),
		Version:   h.Version,
		CreatedAt: h.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: h.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// membershipResponse is the JSON representation of a household membership.
type membershipResponse struct {
	HouseholdID string `json:"household_id"`
	UserID      string `json:"user_id"`
	Role        string `json:"role"`
	InvitedBy   string `json:"invited_by"`
	JoinedAt    string `json:"joined_at"`
}

// toMembershipResponse maps a domain Membership to its JSON-safe representation.
func toMembershipResponse(m domainhouse.Membership) membershipResponse {
	return membershipResponse{
		HouseholdID: m.HouseholdID.String(),
		UserID:      m.UserID.String(),
		Role:        string(m.Role),
		InvitedBy:   m.InvitedBy.String(),
		JoinedAt:    m.JoinedAt.UTC().Format(time.RFC3339),
	}
}

// --- CreateHousehold ---

// createHouseholdService is the subset of HouseholdLogic required by CreateHouseholdHandler.
type createHouseholdService interface {
	Create(ctx context.Context, input domainhouse.CreateInput, callerID uuid.UUID) (domainhouse.Household, error)
}

type createHouseholdRequest struct {
	Name string `json:"name"`
}

// CreateHouseholdHandler returns an http.HandlerFunc that creates a new household.
// The caller is automatically added as an ADMIN member. Panics if svc is nil.
//
// @Summary Create a household
// @Tags households
// @Accept json
// @Produce json
// @Param body body createHouseholdRequest true "Household input"
// @Success 201 {object} householdResponse
// @Failure 400 {object} map[string]string
// @Security BearerAuth
// @Router /api/v1/households [post]
func CreateHouseholdHandler(svc createHouseholdService) http.HandlerFunc {
	if svc == nil {
		panic("http: CreateHouseholdHandler requires non-nil createHouseholdService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var req createHouseholdRequest
		if err := DecodeBody(r, &req); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgBadRequestBody})
			return
		}

		callerID, _ := ctxutil.UserID(r.Context())

		h, err := svc.Create(r.Context(), domainhouse.CreateInput{
			Name: req.Name,
		}, callerID)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		location := fmt.Sprintf("/api/v1/households/%s", h.ID)
		WriteCreated(w, location, toHouseholdResponse(h))
	}
}

// --- GetHousehold ---

// getHouseholdService is the subset of HouseholdLogic required by GetHouseholdHandler.
type getHouseholdService interface {
	FindByID(ctx context.Context, id uuid.UUID) (domainhouse.Household, error)
}

// GetHouseholdHandler returns an http.HandlerFunc that retrieves a household by ID.
// The household ID is read from the URL path parameter "household_id". Panics if svc is nil.
//
// @Summary Get household by ID
// @Tags households
// @Produce json
// @Param household_id path string true "Household ID (UUID)"
// @Success 200 {object} householdResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /api/v1/households/{household_id} [get]
func GetHouseholdHandler(svc getHouseholdService) http.HandlerFunc {
	if svc == nil {
		panic("http: GetHouseholdHandler requires non-nil getHouseholdService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseUUID(r.PathValue("household_id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		h, err := svc.FindByID(r.Context(), id)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, toHouseholdResponse(h))
	}
}

// --- ListHouseholds ---

// listHouseholdsService is the subset of HouseholdLogic required by ListHouseholdsHandler.
type listHouseholdsService interface {
	ListForUser(ctx context.Context, userID uuid.UUID) ([]domainhouse.Household, error)
}

// ListHouseholdsHandler returns an http.HandlerFunc that lists all households
// for the authenticated user. Panics if svc is nil.
//
// @Summary List households for the authenticated user
// @Tags households
// @Produce json
// @Success 200 {array} householdResponse
// @Security BearerAuth
// @Router /api/v1/households [get]
func ListHouseholdsHandler(svc listHouseholdsService) http.HandlerFunc {
	if svc == nil {
		panic("http: ListHouseholdsHandler requires non-nil listHouseholdsService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := ctxutil.UserID(r.Context())

		households, err := svc.ListForUser(r.Context(), userID)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		// Ensure empty slice serializes as [] not null.
		resp := make([]householdResponse, 0, len(households))
		for _, h := range households {
			resp = append(resp, toHouseholdResponse(h))
		}

		WriteJSON(w, http.StatusOK, resp)
	}
}

// --- AddMember ---

// addMemberService is the subset of HouseholdLogic required by AddMemberHandler.
type addMemberService interface {
	AddMember(ctx context.Context, householdID uuid.UUID, input domainhouse.AddMemberInput, callerID uuid.UUID) (domainhouse.Membership, error)
}

type addMemberRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// AddMemberHandler returns an http.HandlerFunc that adds a member to a household.
// The household ID is read from the URL path parameter "id". Panics if svc is nil.
//
// @Summary Add a member to a household
// @Tags members
// @Accept json
// @Produce json
// @Param household_id path string true "Household ID (UUID)"
// @Param body body addMemberRequest true "Member input"
// @Success 201 {object} membershipResponse
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Security BearerAuth
// @Router /api/v1/households/{household_id}/members [post]
func AddMemberHandler(svc addMemberService) http.HandlerFunc {
	if svc == nil {
		panic("http: AddMemberHandler requires non-nil addMemberService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		householdID, err := parseUUID(r.PathValue("household_id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		var req addMemberRequest
		if err := DecodeBody(r, &req); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgBadRequestBody})
			return
		}

		userID, err := parseUUID(req.UserID)
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgBadRequestBody})
			return
		}

		cID, _ := ctxutil.UserID(r.Context())

		m, err := svc.AddMember(r.Context(), householdID, domainhouse.AddMemberInput{
			UserID: userID,
			Role:   types.Role(req.Role),
		}, cID)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		WriteCreated(w, "", toMembershipResponse(m))
	}
}

// --- RemoveMember ---

// removeMemberService is the subset of HouseholdLogic required by RemoveMemberHandler.
type removeMemberService interface {
	RemoveMember(ctx context.Context, householdID uuid.UUID, userID uuid.UUID, callerID uuid.UUID) error
}

// RemoveMemberHandler returns an http.HandlerFunc that removes a member from a household.
// The household ID and user ID are read from URL path parameters "id" and "user_id".
// Panics if svc is nil.
//
// @Summary Remove a member from a household
// @Tags members
// @Param household_id path string true "Household ID (UUID)"
// @Param user_id path string true "User ID to remove (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Security BearerAuth
// @Router /api/v1/households/{household_id}/members/{user_id} [delete]
func RemoveMemberHandler(svc removeMemberService) http.HandlerFunc {
	if svc == nil {
		panic("http: RemoveMemberHandler requires non-nil removeMemberService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		householdID, err := parseUUID(r.PathValue("household_id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		userID, err := parseUUID(r.PathValue("user_id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		cID, _ := ctxutil.UserID(r.Context())

		if err := svc.RemoveMember(r.Context(), householdID, userID, cID); err != nil {
			handleDomainError(w, r, err)
			return
		}

		WriteNoContent(w)
	}
}

// --- UpdateHouseholdName ---

// updateHouseholdNameService is the subset of HouseholdLogic required by UpdateHouseholdNameHandler.
type updateHouseholdNameService interface {
	UpdateName(ctx context.Context, id uuid.UUID, input domainhouse.UpdateNameInput, expectedVersion int, callerID uuid.UUID) (domainhouse.Household, error)
}

type updateHouseholdNameRequest struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
}

// UpdateHouseholdNameHandler returns an http.HandlerFunc that renames a household.
// The household ID is read from the URL path parameter "id". Panics if svc is nil.
//
// @Summary Update household name
// @Tags households
// @Accept json
// @Produce json
// @Param household_id path string true "Household ID (UUID)"
// @Param body body updateHouseholdNameRequest true "Name update input"
// @Success 200 {object} householdResponse
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Security BearerAuth
// @Router /api/v1/households/{household_id} [put]
func UpdateHouseholdNameHandler(svc updateHouseholdNameService) http.HandlerFunc {
	if svc == nil {
		panic("http: UpdateHouseholdNameHandler requires non-nil updateHouseholdNameService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseUUID(r.PathValue("household_id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		var req updateHouseholdNameRequest
		if err := DecodeBody(r, &req); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgBadRequestBody})
			return
		}

		cID, _ := ctxutil.UserID(r.Context())

		h, err := svc.UpdateName(r.Context(), id, domainhouse.UpdateNameInput{
			Name: req.Name,
		}, req.Version, cID)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, toHouseholdResponse(h))
	}
}

// --- CreateHouseholdUser ---

// createHouseholdUserService is the narrow interface required by CreateHouseholdUserHandler.
// Structurally satisfied by HouseholdMemberLogic.
type createHouseholdUserService interface {
	CreateHouseholdUser(ctx context.Context, householdID uuid.UUID, input domainhouse.NewUserInput, callerID uuid.UUID) (domainhouse.CreatedMember, error)
}

// createHouseholdUserRequest carries the caller-supplied fields for creating a new user
// within a household.
type createHouseholdUserRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

// createdHouseholdUserResponse is the JSON representation of a newly created household user.
// It deliberately omits sensitive fields (PasswordHash) and internal audit fields.
type createdHouseholdUserResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

// CreateHouseholdUserHandler returns an http.HandlerFunc that creates a new user and
// atomically adds them as a MEMBER of the given household. Requires an authenticated
// caller who is an admin of the household (enforced by adminGuard middleware upstream).
// On success it writes a 201 Created response with the new user and a Location header.
// Panics if svc is nil.
//
// @Summary Create a user within a household
// @Tags households
// @Accept json
// @Produce json
// @Param household_id path string true "Household ID (UUID)"
// @Param body body createHouseholdUserRequest true "New user input"
// @Success 201 {object} createdHouseholdUserResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Security BearerAuth
// @Router /api/v1/households/{household_id}/users [post]
func CreateHouseholdUserHandler(svc createHouseholdUserService) http.HandlerFunc {
	if svc == nil {
		panic("http: CreateHouseholdUserHandler requires non-nil createHouseholdUserService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		householdID, err := parseUUID(r.PathValue("household_id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		var req createHouseholdUserRequest
		if err := DecodeBody(r, &req); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgBadRequestBody})
			return
		}

		v := validate.NewResult()
		v.Field("email", req.Email, validate.Required)
		v.Field("password", req.Password, validate.Required)
		if err := v.Error(); err != nil {
			var ve *validate.ValidationError
			if errors.As(err, &ve) {
				WriteValidationError(w, ve)
				return
			}
		}

		callerID, _ := ctxutil.UserID(r.Context())

		created, err := svc.CreateHouseholdUser(r.Context(), householdID, domainhouse.NewUserInput{
			Email:       req.Email,
			DisplayName: req.DisplayName,
			Password:    req.Password,
		}, callerID)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		location := fmt.Sprintf("/api/v1/users/%s", created.User.ID)
		WriteCreated(w, location, createdHouseholdUserResponse{
			ID:          created.User.ID.String(),
			Email:       created.User.Email,
			DisplayName: created.User.DisplayName,
		})
	}
}

// --- DeactivateHousehold ---

// deactivateHouseholdService is the subset of HouseholdLogic required by DeactivateHouseholdHandler.
type deactivateHouseholdService interface {
	Deactivate(ctx context.Context, id uuid.UUID, callerID uuid.UUID) error
}

// DeactivateHouseholdHandler returns an http.HandlerFunc that soft-deletes a household.
// The household ID is read from the URL path parameter "id". Panics if svc is nil.
//
// @Summary Deactivate a household
// @Tags households
// @Param household_id path string true "Household ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /api/v1/households/{household_id} [delete]
func DeactivateHouseholdHandler(svc deactivateHouseholdService) http.HandlerFunc {
	if svc == nil {
		panic("http: DeactivateHouseholdHandler requires non-nil deactivateHouseholdService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseUUID(r.PathValue("household_id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		cID, _ := ctxutil.UserID(r.Context())

		if err := svc.Deactivate(r.Context(), id, cID); err != nil {
			handleDomainError(w, r, err)
			return
		}

		WriteNoContent(w)
	}
}
