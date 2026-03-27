package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	"github.com/zambone/pfm-go/internal/domain/ledger"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/database"
	"github.com/zambone/pfm-go/internal/types"
)

// timeToPgDate converts a time.Time to pgtype.Date for use as a DATE column.
func timeToPgDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

// pgDateToTime converts a pgtype.Date to time.Time.
// Returns zero time when the DB value is NULL.
func pgDateToTime(d pgtype.Date) time.Time {
	if !d.Valid {
		return time.Time{}
	}
	return d.Time
}

// LedgerRepo implements domain/ledger.Repository using PostgreSQL via sqlc-generated queries.
type LedgerRepo struct {
	pool *pgxpool.Pool
}

// NewLedgerRepo creates a LedgerRepo backed by pool.
// Panics if pool is nil to catch misconfigured wiring at startup.
func NewLedgerRepo(pool *pgxpool.Pool) *LedgerRepo {
	if pool == nil {
		panic("postgres: NewLedgerRepo requires non-nil pool")
	}
	return &LedgerRepo{pool: pool}
}

// PostTransaction validates balance, inserts the transaction row, all entry rows,
// and adjusts each affected account's balance — all within the current DB connection
// (transaction provided by the logic layer via Transactor).
func (r *LedgerRepo) PostTransaction(ctx context.Context, householdID uuid.UUID, input ledger.PostTransactionInput, callerID uuid.UUID) (ledger.Transaction, []ledger.Entry, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "LedgerRepo.PostTransaction")
	defer span.End()

	// Verify balance before any writes.
	var debitSum, creditSum int64
	for _, e := range input.Entries {
		switch e.EntryType {
		case types.EntryTypeDebit:
			debitSum += e.Amount
		case types.EntryTypeCredit:
			creditSum += e.Amount
		}
	}
	if debitSum != creditSum {
		err := fmt.Errorf(message.ErrLedgerPostTransaction, message.ErrLedgerUnbalanced)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return ledger.Transaction{}, nil, err
	}

	db := database.DBTXFromContext(ctx, r.pool)
	q := New(db)

	// Insert transaction.
	txRow, err := q.InsertTransaction(ctx, InsertTransactionParams{
		HouseholdID:     householdID,
		Description:     input.Description,
		TransactionDate: timeToPgDate(input.TransactionDate),
		CreatedBy:       uuidToPgUUID(callerID),
	})
	if err != nil {
		err = fmt.Errorf(message.ErrLedgerPostTransaction, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return ledger.Transaction{}, nil, err
	}

	// Insert entries and adjust balances.
	entries := make([]ledger.Entry, len(input.Entries))
	for i, ei := range input.Entries {
		entryRow, err := q.InsertLedgerEntry(ctx, InsertLedgerEntryParams{
			TransactionID: txRow.ID,
			AccountID:     ei.AccountID,
			EntryType:     string(ei.EntryType),
			Amount:        ei.Amount,
		})
		if err != nil {
			err = fmt.Errorf(message.ErrLedgerPostTransaction, err)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return ledger.Transaction{}, nil, err
		}

		entries[i] = ledger.Entry{
			ID:            entryRow.ID,
			TransactionID: entryRow.TransactionID,
			AccountID:     entryRow.AccountID,
			EntryType:     types.EntryType(entryRow.EntryType),
			Amount:        entryRow.Amount,
			CreatedAt:     pgTimestamptzToTime(entryRow.CreatedAt),
		}

		// Adjust account balance: credits increase, debits decrease.
		var delta int64
		switch ei.EntryType {
		case types.EntryTypeCredit:
			delta = ei.Amount
		case types.EntryTypeDebit:
			delta = -ei.Amount
		}
		if err := q.AdjustAccountBalance(ctx, AdjustAccountBalanceParams{
			ID:    ei.AccountID,
			Delta: delta,
		}); err != nil {
			err = fmt.Errorf(message.ErrLedgerPostTransaction, err)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return ledger.Transaction{}, nil, err
		}
	}

	span.SetStatus(codes.Ok, "")
	return ledger.Transaction{
		ID:              txRow.ID,
		HouseholdID:     txRow.HouseholdID,
		Description:     txRow.Description,
		TransactionDate: pgDateToTime(txRow.TransactionDate),
		CreatedAt:       pgTimestamptzToTime(txRow.CreatedAt),
		CreatedBy:       pgUUIDToUUID(txRow.CreatedBy),
	}, entries, nil
}

// GetBalance returns the current balance for the given account from the accounts table.
func (r *LedgerRepo) GetBalance(ctx context.Context, accountID uuid.UUID) (int64, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "LedgerRepo.GetBalance")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	bal, err := New(db).GetAccountBalance(ctx, accountID)
	if err != nil {
		err = fmt.Errorf(message.ErrLedgerGetBalance, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, err
	}

	span.SetStatus(codes.Ok, "")
	return bal, nil
}

// GetTransactionHistory returns transactions with their entries for the given household,
// optionally filtered by account, with pagination.
func (r *LedgerRepo) GetTransactionHistory(ctx context.Context, householdID uuid.UUID, query ledger.HistoryQuery) ([]ledger.TransactionWithEntries, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "LedgerRepo.GetTransactionHistory")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	q := New(db)

	// If filtering by account, get the transaction IDs first.
	var txFilter map[uuid.UUID]bool
	if query.AccountID != uuid.Nil {
		txIDs, err := q.ListTransactionIDsForAccount(ctx, query.AccountID)
		if err != nil {
			err = fmt.Errorf(message.ErrLedgerGetHistory, err)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		txFilter = make(map[uuid.UUID]bool, len(txIDs))
		for _, id := range txIDs {
			txFilter[id] = true
		}
	}

	limit := int32(query.Limit)
	if limit <= 0 {
		limit = 50
	}
	txRows, err := q.ListTransactionsForHousehold(ctx, ListTransactionsForHouseholdParams{
		HouseholdID: householdID,
		QueryLimit:  limit,
		QueryOffset: int32(query.Offset),
	})
	if err != nil {
		err = fmt.Errorf(message.ErrLedgerGetHistory, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	var results []ledger.TransactionWithEntries
	for _, txRow := range txRows {
		// Apply account filter if set.
		if txFilter != nil && !txFilter[txRow.ID] {
			continue
		}

		entryRows, err := q.ListEntriesForTransaction(ctx, txRow.ID)
		if err != nil {
			err = fmt.Errorf(message.ErrLedgerGetHistory, err)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}

		entries := make([]ledger.Entry, len(entryRows))
		for i, er := range entryRows {
			entries[i] = ledger.Entry{
				ID:            er.ID,
				TransactionID: er.TransactionID,
				AccountID:     er.AccountID,
				EntryType:     types.EntryType(er.EntryType),
				Amount:        er.Amount,
				CreatedAt:     pgTimestamptzToTime(er.CreatedAt),
			}
		}

		results = append(results, ledger.TransactionWithEntries{
			Transaction: ledger.Transaction{
				ID:              txRow.ID,
				HouseholdID:     txRow.HouseholdID,
				Description:     txRow.Description,
				TransactionDate: pgDateToTime(txRow.TransactionDate),
				CreatedAt:       pgTimestamptzToTime(txRow.CreatedAt),
				CreatedBy:       pgUUIDToUUID(txRow.CreatedBy),
			},
			Entries: entries,
		})
	}

	span.SetStatus(codes.Ok, "")
	return results, nil
}
