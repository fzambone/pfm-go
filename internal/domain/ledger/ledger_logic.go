package ledger

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/validate"
	"github.com/zambone/pfm-go/internal/types"
)

// transactor abstracts atomic multi-table operations.
// Structurally satisfied by platform/database.Transactor.
type transactor interface {
	RunAtomic(ctx context.Context, fn func(ctx context.Context) error) error
}

// clocker abstracts the current time.
// Structurally satisfied by platform/clock.Clock.
type clocker interface {
	Now() time.Time
}

// LedgerLogic orchestrates double-entry transaction posting, balance queries,
// and transaction history. It enforces the critical invariant: every transaction
// must have balanced entries (total debits = total credits).
type LedgerLogic struct {
	repo Repository
	tx   transactor
	clk  clocker
}

// NewLedgerLogic constructs a LedgerLogic. Panics if any dependency is nil.
func NewLedgerLogic(repo Repository, tx transactor, clk clocker) *LedgerLogic {
	if repo == nil {
		panic("ledger: NewLedgerLogic requires non-nil repo")
	}
	if tx == nil {
		panic("ledger: NewLedgerLogic requires non-nil tx")
	}
	if clk == nil {
		panic("ledger: NewLedgerLogic requires non-nil clk")
	}
	return &LedgerLogic{repo: repo, tx: tx, clk: clk}
}

// PostTransaction validates the input, enforces the balanced-entries invariant,
// and persists the transaction with its entries atomically via the Transactor.
func (l *LedgerLogic) PostTransaction(ctx context.Context, householdID uuid.UUID, input PostTransactionInput, callerID uuid.UUID) (Transaction, []Entry, error) {
	// Validate description and entry count.
	r := validate.NewResult()
	r.Field("description", input.Description, validate.Required)
	r.Field("entries", len(input.Entries), validate.Positive)
	if err := r.Error(); err != nil {
		return Transaction{}, nil, err
	}

	// Validate each entry's amount is positive.
	for i, e := range input.Entries {
		er := validate.NewResult()
		er.Field(fmt.Sprintf("entries[%d].amount", i), e.Amount, validate.Positive)
		if err := er.Error(); err != nil {
			return Transaction{}, nil, err
		}
	}

	// Enforce balanced-entries invariant: total debits must equal total credits.
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
		return Transaction{}, nil, fmt.Errorf(message.ErrLedgerLogicPost, message.ErrLedgerUnbalanced)
	}

	// Persist atomically via Transactor.
	var postedTx Transaction
	var postedEntries []Entry
	if err := l.tx.RunAtomic(ctx, func(txCtx context.Context) error {
		tx, entries, err := l.repo.PostTransaction(txCtx, householdID, input, callerID)
		if err != nil {
			return err
		}
		postedTx = tx
		postedEntries = entries
		return nil
	}); err != nil {
		return Transaction{}, nil, fmt.Errorf(message.ErrLedgerLogicPost, err)
	}
	return postedTx, postedEntries, nil
}

// GetBalance returns the current balance for the given account.
func (l *LedgerLogic) GetBalance(ctx context.Context, accountID uuid.UUID) (int64, error) {
	bal, err := l.repo.GetBalance(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf(message.ErrLedgerLogicGetBalance, err)
	}
	return bal, nil
}

// GetTransactionHistory returns transactions with their entries, filtered and paginated.
func (l *LedgerLogic) GetTransactionHistory(ctx context.Context, householdID uuid.UUID, query HistoryQuery) ([]TransactionWithEntries, error) {
	history, err := l.repo.GetTransactionHistory(ctx, householdID, query)
	if err != nil {
		return nil, fmt.Errorf(message.ErrLedgerLogicGetHistory, err)
	}
	return history, nil
}
