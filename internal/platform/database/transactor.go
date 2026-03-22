package database

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	"github.com/zambone/pfm-go/internal/message"
)

// Transactor executes a function atomically within a database transaction.
// Domain logic calls RunAtomic and receives a derived context carrying the active
// transaction. The domain never sees pgx.Tx — only context.Context.
type Transactor interface {
	RunAtomic(ctx context.Context, fn func(ctx context.Context) error) error
}

// DBTX is satisfied by both *pgxpool.Pool and pgx.Tx.
// Repository implementations accept DBTX to transparently participate in transactions
// without knowing whether a transaction is active.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// txContextKey is the unexported key used to store pgx.Tx in a context.
type txContextKey struct{}

// WithTxContext returns a new context carrying tx.
// Called by PostgresTransactor before invoking the atomic function.
func WithTxContext(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

// TxFromContext returns the pgx.Tx stored in ctx, if any.
// Repository methods call this to check for an active transaction.
func TxFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txContextKey{}).(pgx.Tx)
	return tx, ok
}

// DBTXFromContext returns the pgx.Tx from ctx if present, otherwise returns pool.
// Repositories call this at the top of every method to get the correct executor,
// transparently participating in a transaction when one is active.
func DBTXFromContext(ctx context.Context, pool *pgxpool.Pool) DBTX {
	if tx, ok := TxFromContext(ctx); ok {
		return tx
	}
	return pool
}

// PostgresTransactor implements Transactor using a pgx connection pool.
type PostgresTransactor struct {
	pool *pgxpool.Pool
}

// NewPostgresTransactor creates a PostgresTransactor backed by pool.
// Panics if pool is nil to catch misconfigured wiring at startup.
func NewPostgresTransactor(pool *pgxpool.Pool) *PostgresTransactor {
	if pool == nil {
		panic("database: NewPostgresTransactor requires a non-nil pool")
	}
	return &PostgresTransactor{pool: pool}
}

// RunAtomic executes fn within a single database transaction.
//
// Nested calls: if ctx already carries a transaction, fn is called directly without
// starting a new transaction — inner calls participate in the outer transaction.
//
// On panic inside fn: the transaction is rolled back and the panic is re-raised so
// the caller's stack trace is preserved.
//
// On error from fn: the transaction is rolled back and the error is returned.
//
// On success: the transaction is committed.
func (t *PostgresTransactor) RunAtomic(ctx context.Context, fn func(ctx context.Context) error) error {
	// Nested call: already inside a transaction — participate without starting a new one.
	if _, ok := TxFromContext(ctx); ok {
		return fn(ctx)
	}

	ctx, span := otel.Tracer("database").Start(ctx, "Transactor.RunAtomic")
	defer span.End()

	tx, err := t.pool.Begin(ctx)
	if err != nil {
		err = fmt.Errorf(message.ErrTransactorBegin, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	// Rollback on panic: executed before span.End() due to LIFO defer order.
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx) // best-effort: rollback before re-panic
			panic(p)             // re-raise with original value to preserve stack trace
		}
	}()

	txCtx := WithTxContext(ctx, tx)
	if err = fn(txCtx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			slog.WarnContext(ctx, message.MsgTransactorRollbackFailed, "error", rbErr)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("transactor: run atomic: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		err = fmt.Errorf(message.ErrTransactorCommit, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}
