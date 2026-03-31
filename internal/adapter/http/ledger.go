package http

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	domainledger "github.com/zambone/pfm-go/internal/domain/ledger"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/ctxutil"
	"github.com/zambone/pfm-go/internal/types"
)

// transactionResponse is the JSON representation of a posted transaction with entries.
type transactionResponse struct {
	ID              string          `json:"id"`
	HouseholdID     string          `json:"household_id"`
	Description     string          `json:"description"`
	TransactionDate string          `json:"transaction_date"`
	CreatedAt       string          `json:"created_at"`
	Entries         []entryResponse `json:"entries"`
}

// entryResponse is the JSON representation of a ledger entry.
type entryResponse struct {
	ID            string `json:"id"`
	TransactionID string `json:"transaction_id"`
	AccountID     string `json:"account_id"`
	EntryType     string `json:"entry_type"`
	Amount        int64  `json:"amount"`
	CreatedAt     string `json:"created_at"`
}

// toEntryResponse maps a domain Entry to its JSON-safe representation.
func toEntryResponse(e domainledger.Entry) entryResponse {
	return entryResponse{
		ID:            e.ID.String(),
		TransactionID: e.TransactionID.String(),
		AccountID:     e.AccountID.String(),
		EntryType:     string(e.EntryType),
		Amount:        e.Amount,
		CreatedAt:     e.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// toTransactionResponse maps a domain Transaction and its entries to JSON.
func toTransactionResponse(t domainledger.Transaction, entries []domainledger.Entry) transactionResponse {
	entryResps := make([]entryResponse, 0, len(entries))
	for _, e := range entries {
		entryResps = append(entryResps, toEntryResponse(e))
	}
	return transactionResponse{
		ID:              t.ID.String(),
		HouseholdID:     t.HouseholdID.String(),
		Description:     t.Description,
		TransactionDate: t.TransactionDate.Format("2006-01-02"),
		CreatedAt:       t.CreatedAt.UTC().Format(time.RFC3339),
		Entries:         entryResps,
	}
}

// toTransactionWithEntriesResponse maps a TransactionWithEntries to JSON.
func toTransactionWithEntriesResponse(twe domainledger.TransactionWithEntries) transactionResponse {
	return toTransactionResponse(twe.Transaction, twe.Entries)
}

// balanceResponse is the JSON representation of an account balance query result.
type balanceResponse struct {
	AccountID string `json:"account_id"`
	Balance   int64  `json:"balance"`
}

// --- PostTransaction ---

// postTransactionService is the subset of LedgerLogic required by PostTransactionHandler.
type postTransactionService interface {
	PostTransaction(ctx context.Context, householdID uuid.UUID, input domainledger.PostTransactionInput, callerID uuid.UUID) (domainledger.Transaction, []domainledger.Entry, error)
}

type postTransactionRequest struct {
	Description     string              `json:"description"`
	TransactionDate string              `json:"transaction_date"` // YYYY-MM-DD
	Entries         []entryInputRequest `json:"entries"`
}

type entryInputRequest struct {
	AccountID string `json:"account_id"`
	EntryType string `json:"entry_type"`
	Amount    int64  `json:"amount"`
}

// PostTransactionHandler returns an http.HandlerFunc that posts a balanced
// double-entry transaction. The household ID is read from path parameter "household_id".
// Panics if svc is nil.
func PostTransactionHandler(svc postTransactionService) http.HandlerFunc {
	if svc == nil {
		panic("http: PostTransactionHandler requires non-nil postTransactionService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		householdID, err := parseUUID(r.PathValue("household_id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		var req postTransactionRequest
		if err := DecodeBody(r, &req); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgBadRequestBody})
			return
		}

		txnDate, err := time.Parse("2006-01-02", req.TransactionDate)
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgBadRequestBody})
			return
		}

		entries := make([]domainledger.EntryInput, 0, len(req.Entries))
		for _, e := range req.Entries {
			acctID, err := parseUUID(e.AccountID)
			if err != nil {
				WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgBadRequestBody})
				return
			}
			entries = append(entries, domainledger.EntryInput{
				AccountID: acctID,
				EntryType: types.EntryType(e.EntryType),
				Amount:    e.Amount,
			})
		}

		cID, _ := ctxutil.UserID(r.Context())

		txn, txnEntries, err := svc.PostTransaction(r.Context(), householdID, domainledger.PostTransactionInput{
			Description:     req.Description,
			TransactionDate: txnDate,
			Entries:         entries,
		}, cID)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		WriteCreated(w, "", toTransactionResponse(txn, txnEntries))
	}
}

// --- GetBalance ---

// getBalanceService is the subset of LedgerLogic required by GetBalanceHandler.
type getBalanceService interface {
	GetBalance(ctx context.Context, accountID uuid.UUID) (int64, error)
}

// GetBalanceHandler returns an http.HandlerFunc that returns the current balance
// for an account. The account ID is read from path parameter "account_id".
// Panics if svc is nil.
func GetBalanceHandler(svc getBalanceService) http.HandlerFunc {
	if svc == nil {
		panic("http: GetBalanceHandler requires non-nil getBalanceService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, err := parseUUID(r.PathValue("account_id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		balance, err := svc.GetBalance(r.Context(), accountID)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, balanceResponse{
			AccountID: accountID.String(),
			Balance:   balance,
		})
	}
}

// --- GetTransactionHistory ---

// getTransactionHistoryService is the subset of LedgerLogic required by GetTransactionHistoryHandler.
type getTransactionHistoryService interface {
	GetTransactionHistory(ctx context.Context, householdID uuid.UUID, query domainledger.HistoryQuery) ([]domainledger.TransactionWithEntries, error)
}

// GetTransactionHistoryHandler returns an http.HandlerFunc that lists transactions
// for a household with optional filtering. Query parameters: account_id, limit, offset.
// Panics if svc is nil.
func GetTransactionHistoryHandler(svc getTransactionHistoryService) http.HandlerFunc {
	if svc == nil {
		panic("http: GetTransactionHistoryHandler requires non-nil getTransactionHistoryService")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		householdID, err := parseUUID(r.PathValue("household_id"))
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgAuthzBadRequest})
			return
		}

		query := domainledger.HistoryQuery{}

		// Optional account_id filter
		if acctStr := r.URL.Query().Get("account_id"); acctStr != "" {
			acctID, err := parseUUID(acctStr)
			if err != nil {
				WriteJSON(w, http.StatusBadRequest, map[string]string{"error": message.MsgBadRequestBody})
				return
			}
			query.AccountID = acctID
		}

		// Optional limit
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			limit, err := strconv.Atoi(limitStr)
			if err == nil && limit > 0 {
				query.Limit = limit
			}
		}

		// Optional offset
		if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
			offset, err := strconv.Atoi(offsetStr)
			if err == nil && offset >= 0 {
				query.Offset = offset
			}
		}

		history, err := svc.GetTransactionHistory(r.Context(), householdID, query)
		if err != nil {
			handleDomainError(w, r, err)
			return
		}

		// Ensure empty slice serializes as [] not null.
		resp := make([]transactionResponse, 0, len(history))
		for _, twe := range history {
			resp = append(resp, toTransactionWithEntriesResponse(twe))
		}

		WriteJSON(w, http.StatusOK, resp)
	}
}
