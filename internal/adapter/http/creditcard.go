package http

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	domaincc "github.com/zambone/pfm-go/internal/domain/creditcard"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/ctxutil"
)

// settingsResponse is the JSON representation of credit card settings.
type settingsResponse struct {
	ID          string `json:"id"`
	AccountID   string `json:"account_id"`
	ClosingDay  int    `json:"closing_day"`
	DueDay      int    `json:"due_day"`
	LimitAmount int64  `json:"limit_amount"`
	Version     int    `json:"version"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// toSettingsResponse maps domain Settings to its JSON-safe representation.
func toSettingsResponse(s domaincc.Settings) settingsResponse {
	return settingsResponse{
		ID:          s.ID.String(),
		AccountID:   s.AccountID.String(),
		ClosingDay:  s.ClosingDay,
		DueDay:      s.DueDay,
		LimitAmount: s.LimitAmount,
		Version:     s.Version,
		CreatedAt:   s.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   s.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// --- CreateCreditCardSettings ---

// createCCSettingsService is the subset of SettingsLogic required by CreateCreditCardSettingsHandler.
type createCCSettingsService interface {
	Create(ctx context.Context, accountID uuid.UUID, input domaincc.CreateInput, callerID uuid.UUID) (domaincc.Settings, error)
}

type createCCSettingsRequest struct {
	ClosingDay  int   `json:"closing_day"`
	DueDay      int   `json:"due_day"`
	LimitAmount int64 `json:"limit_amount"`
}

// CreateCreditCardSettingsHandler returns an http.HandlerFunc that creates credit card
// settings for an account. The account ID is read from path parameter "account_id".
// Panics if svc is nil.
//
// @Summary Create credit card settings
// @Tags credit-card-settings
// @Accept json
// @Produce json
// @Param household_id path string true "Household ID (UUID)"
// @Param account_id path string true "Account ID (UUID)"
// @Param body body createCCSettingsRequest true "Settings input"
// @Success 201 {object} settingsResponse
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Security BearerAuth
// @Router /api/v1/households/{household_id}/accounts/{account_id}/credit-card-settings [post]
func CreateCreditCardSettingsHandler(svc createCCSettingsService) http.HandlerFunc {
	if svc == nil {
		panic("http: CreateCreditCardSettingsHandler requires non-nil createCCSettingsService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, err := parseUUID(r.PathValue("account_id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		var req createCCSettingsRequest
		if err := DecodeBody(r, &req); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgBadRequestBody})
			return
		}

		cID, _ := ctxutil.UserID(r.Context())

		s, err := svc.Create(r.Context(), accountID, domaincc.CreateInput{
			ClosingDay:  req.ClosingDay,
			DueDay:      req.DueDay,
			LimitAmount: req.LimitAmount,
		}, cID)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		WriteCreated(w, "", toSettingsResponse(s))
	}
}

// --- GetCreditCardSettings ---

// getCCSettingsService is the subset of SettingsLogic required by GetCreditCardSettingsHandler.
type getCCSettingsService interface {
	FindByAccountID(ctx context.Context, accountID uuid.UUID) (domaincc.Settings, error)
}

// GetCreditCardSettingsHandler returns an http.HandlerFunc that retrieves credit card
// settings by account ID. Panics if svc is nil.
//
// @Summary Get credit card settings
// @Tags credit-card-settings
// @Produce json
// @Param household_id path string true "Household ID (UUID)"
// @Param account_id path string true "Account ID (UUID)"
// @Success 200 {object} settingsResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /api/v1/households/{household_id}/accounts/{account_id}/credit-card-settings [get]
func GetCreditCardSettingsHandler(svc getCCSettingsService) http.HandlerFunc {
	if svc == nil {
		panic("http: GetCreditCardSettingsHandler requires non-nil getCCSettingsService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, err := parseUUID(r.PathValue("account_id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		s, err := svc.FindByAccountID(r.Context(), accountID)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, toSettingsResponse(s))
	}
}

// --- UpdateClosingDay ---

// updateClosingDayService is the subset of SettingsLogic required by UpdateClosingDayHandler.
type updateClosingDayService interface {
	UpdateClosingDay(ctx context.Context, accountID uuid.UUID, input domaincc.UpdateClosingDayInput, expectedVersion int, callerID uuid.UUID) (domaincc.Settings, error)
}

type updateClosingDayRequest struct {
	ClosingDay int `json:"closing_day"`
	Version    int `json:"version"`
}

// UpdateClosingDayHandler returns an http.HandlerFunc that updates the billing cycle
// closing day for credit card settings. Panics if svc is nil.
//
// @Summary Update closing day
// @Tags credit-card-settings
// @Accept json
// @Produce json
// @Param household_id path string true "Household ID (UUID)"
// @Param account_id path string true "Account ID (UUID)"
// @Param body body updateClosingDayRequest true "Closing day input"
// @Success 200 {object} settingsResponse
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Security BearerAuth
// @Router /api/v1/households/{household_id}/accounts/{account_id}/credit-card-settings/closing-day [put]
func UpdateClosingDayHandler(svc updateClosingDayService) http.HandlerFunc {
	if svc == nil {
		panic("http: UpdateClosingDayHandler requires non-nil updateClosingDayService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, err := parseUUID(r.PathValue("account_id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		var req updateClosingDayRequest
		if err := DecodeBody(r, &req); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgBadRequestBody})
			return
		}

		cID, _ := ctxutil.UserID(r.Context())

		s, err := svc.UpdateClosingDay(r.Context(), accountID, domaincc.UpdateClosingDayInput{
			ClosingDay: req.ClosingDay,
		}, req.Version, cID)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, toSettingsResponse(s))
	}
}

// --- UpdateDueDay ---

// updateDueDayService is the subset of SettingsLogic required by UpdateDueDayHandler.
type updateDueDayService interface {
	UpdateDueDay(ctx context.Context, accountID uuid.UUID, input domaincc.UpdateDueDayInput, expectedVersion int, callerID uuid.UUID) (domaincc.Settings, error)
}

type updateDueDayRequest struct {
	DueDay  int `json:"due_day"`
	Version int `json:"version"`
}

// UpdateDueDayHandler returns an http.HandlerFunc that updates the payment due day
// for credit card settings. Panics if svc is nil.
//
// @Summary Update due day
// @Tags credit-card-settings
// @Accept json
// @Produce json
// @Param household_id path string true "Household ID (UUID)"
// @Param account_id path string true "Account ID (UUID)"
// @Param body body updateDueDayRequest true "Due day input"
// @Success 200 {object} settingsResponse
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Security BearerAuth
// @Router /api/v1/households/{household_id}/accounts/{account_id}/credit-card-settings/due-day [put]
func UpdateDueDayHandler(svc updateDueDayService) http.HandlerFunc {
	if svc == nil {
		panic("http: UpdateDueDayHandler requires non-nil updateDueDayService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, err := parseUUID(r.PathValue("account_id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		var req updateDueDayRequest
		if err := DecodeBody(r, &req); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgBadRequestBody})
			return
		}

		cID, _ := ctxutil.UserID(r.Context())

		s, err := svc.UpdateDueDay(r.Context(), accountID, domaincc.UpdateDueDayInput{
			DueDay: req.DueDay,
		}, req.Version, cID)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, toSettingsResponse(s))
	}
}

// --- UpdateCreditLimit ---

// updateCreditLimitService is the subset of SettingsLogic required by UpdateCreditLimitHandler.
type updateCreditLimitService interface {
	UpdateLimit(ctx context.Context, accountID uuid.UUID, input domaincc.UpdateLimitInput, expectedVersion int, callerID uuid.UUID) (domaincc.Settings, error)
}

type updateCreditLimitRequest struct {
	LimitAmount int64 `json:"limit_amount"`
	Version     int   `json:"version"`
}

// UpdateCreditLimitHandler returns an http.HandlerFunc that updates the credit limit.
// Panics if svc is nil.
//
// @Summary Update credit limit
// @Tags credit-card-settings
// @Accept json
// @Produce json
// @Param household_id path string true "Household ID (UUID)"
// @Param account_id path string true "Account ID (UUID)"
// @Param body body updateCreditLimitRequest true "Limit input"
// @Success 200 {object} settingsResponse
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Security BearerAuth
// @Router /api/v1/households/{household_id}/accounts/{account_id}/credit-card-settings/limit [put]
func UpdateCreditLimitHandler(svc updateCreditLimitService) http.HandlerFunc {
	if svc == nil {
		panic("http: UpdateCreditLimitHandler requires non-nil updateCreditLimitService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, err := parseUUID(r.PathValue("account_id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		var req updateCreditLimitRequest
		if err := DecodeBody(r, &req); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgBadRequestBody})
			return
		}

		cID, _ := ctxutil.UserID(r.Context())

		s, err := svc.UpdateLimit(r.Context(), accountID, domaincc.UpdateLimitInput{
			LimitAmount: req.LimitAmount,
		}, req.Version, cID)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, toSettingsResponse(s))
	}
}

// --- DeleteCreditCardSettings ---

// deleteCCSettingsService is the subset of SettingsLogic required by DeleteCreditCardSettingsHandler.
type deleteCCSettingsService interface {
	Delete(ctx context.Context, accountID uuid.UUID, callerID uuid.UUID) error
}

// DeleteCreditCardSettingsHandler returns an http.HandlerFunc that removes credit card
// settings from an account. Panics if svc is nil.
//
// @Summary Delete credit card settings
// @Tags credit-card-settings
// @Param household_id path string true "Household ID (UUID)"
// @Param account_id path string true "Account ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /api/v1/households/{household_id}/accounts/{account_id}/credit-card-settings [delete]
func DeleteCreditCardSettingsHandler(svc deleteCCSettingsService) http.HandlerFunc {
	if svc == nil {
		panic("http: DeleteCreditCardSettingsHandler requires non-nil deleteCCSettingsService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, err := parseUUID(r.PathValue("account_id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		cID, _ := ctxutil.UserID(r.Context())

		if err := svc.Delete(r.Context(), accountID, cID); err != nil {
			handleDomainError(w, r, err)
			return
		}

		WriteNoContent(w)
	}
}
