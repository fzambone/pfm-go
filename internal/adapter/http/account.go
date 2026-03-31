package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	domainacct "github.com/zambone/pfm-go/internal/domain/account"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/ctxutil"
	"github.com/zambone/pfm-go/internal/types"
)

// accountResponse is the JSON representation of an account.
type accountResponse struct {
	ID           string `json:"id"`
	HouseholdID  string `json:"household_id"`
	Name         string `json:"name"`
	AccountType  string `json:"account_type"`
	CurrencyCode string `json:"currency_code"`
	Balance      int64  `json:"balance"`
	Status       string `json:"status"`
	Version      int    `json:"version"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// toAccountResponse maps a domain Account to its JSON-safe representation.
func toAccountResponse(a domainacct.Account) accountResponse {
	return accountResponse{
		ID:           a.ID.String(),
		HouseholdID:  a.HouseholdID.String(),
		Name:         a.Name,
		AccountType:  string(a.AccountType),
		CurrencyCode: string(a.CurrencyCode),
		Balance:      a.Balance,
		Status:       string(a.Status),
		Version:      a.Version,
		CreatedAt:    a.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    a.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// --- CreateAccount ---

// createAccountService is the subset of AccountLogic required by CreateAccountHandler.
type createAccountService interface {
	Create(ctx context.Context, householdID uuid.UUID, input domainacct.CreateInput, callerID uuid.UUID) (domainacct.Account, error)
}

type createAccountRequest struct {
	Name         string `json:"name"`
	AccountType  string `json:"account_type"`
	CurrencyCode string `json:"currency_code"`
}

// CreateAccountHandler returns an http.HandlerFunc that creates a new account
// within a household. The household ID is read from path parameter "household_id".
// Panics if svc is nil.
func CreateAccountHandler(svc createAccountService) http.HandlerFunc {
	if svc == nil {
		panic("http: CreateAccountHandler requires non-nil createAccountService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		householdID, err := parseUUID(r.PathValue("household_id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		var req createAccountRequest
		if err := DecodeBody(r, &req); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgBadRequestBody})
			return
		}

		cID, _ := ctxutil.UserID(r.Context())

		a, err := svc.Create(r.Context(), householdID, domainacct.CreateInput{
			Name:         req.Name,
			AccountType:  types.AccountType(req.AccountType),
			CurrencyCode: types.CurrencyCode(req.CurrencyCode),
		}, cID)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		location := fmt.Sprintf("/api/v1/households/%s/accounts/%s", householdID, a.ID)
		WriteCreated(w, location, toAccountResponse(a))
	}
}

// --- GetAccount ---

// getAccountService is the subset of AccountLogic required by GetAccountHandler.
type getAccountService interface {
	FindByID(ctx context.Context, id uuid.UUID) (domainacct.Account, error)
}

// GetAccountHandler returns an http.HandlerFunc that retrieves an account by ID.
// Panics if svc is nil.
func GetAccountHandler(svc getAccountService) http.HandlerFunc {
	if svc == nil {
		panic("http: GetAccountHandler requires non-nil getAccountService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseUUID(r.PathValue("id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		a, err := svc.FindByID(r.Context(), id)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, toAccountResponse(a))
	}
}

// --- ListAccounts ---

// listAccountsService is the subset of AccountLogic required by ListAccountsHandler.
type listAccountsService interface {
	ListForHousehold(ctx context.Context, householdID uuid.UUID) ([]domainacct.Account, error)
}

// ListAccountsHandler returns an http.HandlerFunc that lists all active accounts
// for a household. Panics if svc is nil.
func ListAccountsHandler(svc listAccountsService) http.HandlerFunc {
	if svc == nil {
		panic("http: ListAccountsHandler requires non-nil listAccountsService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		householdID, err := parseUUID(r.PathValue("household_id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		accounts, err := svc.ListForHousehold(r.Context(), householdID)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		// Ensure empty slice serializes as [] not null.
		resp := make([]accountResponse, 0, len(accounts))
		for _, a := range accounts {
			resp = append(resp, toAccountResponse(a))
		}

		WriteJSON(w, http.StatusOK, resp)
	}
}

// --- UpdateAccountName ---

// updateAccountNameService is the subset of AccountLogic required by UpdateAccountNameHandler.
type updateAccountNameService interface {
	UpdateName(ctx context.Context, id uuid.UUID, input domainacct.UpdateNameInput, expectedVersion int, callerID uuid.UUID) (domainacct.Account, error)
}

type updateAccountNameRequest struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
}

// UpdateAccountNameHandler returns an http.HandlerFunc that renames an account.
// Panics if svc is nil.
func UpdateAccountNameHandler(svc updateAccountNameService) http.HandlerFunc {
	if svc == nil {
		panic("http: UpdateAccountNameHandler requires non-nil updateAccountNameService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseUUID(r.PathValue("id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		var req updateAccountNameRequest
		if err := DecodeBody(r, &req); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgBadRequestBody})
			return
		}

		cID, _ := ctxutil.UserID(r.Context())

		a, err := svc.UpdateName(r.Context(), id, domainacct.UpdateNameInput{
			Name: req.Name,
		}, req.Version, cID)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, toAccountResponse(a))
	}
}

// --- UpdateAccountBalance ---

// updateAccountBalanceService is the subset of AccountLogic required by UpdateAccountBalanceHandler.
type updateAccountBalanceService interface {
	UpdateBalance(ctx context.Context, id uuid.UUID, input domainacct.UpdateBalanceInput, expectedVersion int, callerID uuid.UUID) (domainacct.Account, error)
}

type updateAccountBalanceRequest struct {
	Balance int64 `json:"balance"`
	Version int   `json:"version"`
}

// UpdateAccountBalanceHandler returns an http.HandlerFunc that updates an account balance.
// Panics if svc is nil.
func UpdateAccountBalanceHandler(svc updateAccountBalanceService) http.HandlerFunc {
	if svc == nil {
		panic("http: UpdateAccountBalanceHandler requires non-nil updateAccountBalanceService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseUUID(r.PathValue("id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		var req updateAccountBalanceRequest
		if err := DecodeBody(r, &req); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgBadRequestBody})
			return
		}

		cID, _ := ctxutil.UserID(r.Context())

		a, err := svc.UpdateBalance(r.Context(), id, domainacct.UpdateBalanceInput{
			Balance: req.Balance,
		}, req.Version, cID)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, toAccountResponse(a))
	}
}

// --- DeactivateAccount ---

// deactivateAccountService is the subset of AccountLogic required by DeactivateAccountHandler.
type deactivateAccountService interface {
	Deactivate(ctx context.Context, id uuid.UUID, callerID uuid.UUID) error
}

// DeactivateAccountHandler returns an http.HandlerFunc that soft-deletes an account.
// Panics if svc is nil.
func DeactivateAccountHandler(svc deactivateAccountService) http.HandlerFunc {
	if svc == nil {
		panic("http: DeactivateAccountHandler requires non-nil deactivateAccountService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseUUID(r.PathValue("id"))
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
