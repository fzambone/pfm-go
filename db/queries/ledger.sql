-- name: InsertTransaction :one
INSERT INTO transactions (household_id, description, transaction_date, created_by)
VALUES (sqlc.arg('household_id'), sqlc.arg('description'),
        sqlc.arg('transaction_date'), sqlc.arg('created_by'))
RETURNING id, household_id, description, transaction_date, created_at, created_by;

-- name: InsertLedgerEntry :one
INSERT INTO ledger_entries (transaction_id, account_id, entry_type, amount)
VALUES (sqlc.arg('transaction_id'), sqlc.arg('account_id'),
        sqlc.arg('entry_type'), sqlc.arg('amount'))
RETURNING id, transaction_id, account_id, entry_type, amount, created_at;

-- name: AdjustAccountBalance :exec
UPDATE accounts
SET balance    = balance + sqlc.arg('delta'),
    updated_at = NOW(),
    version    = version + 1
WHERE id       = sqlc.arg('id')
  AND deleted_at IS NULL;

-- name: GetAccountBalance :one
SELECT balance
FROM accounts
WHERE id = sqlc.arg('id')
  AND deleted_at IS NULL;

-- name: ListTransactionsForHousehold :many
SELECT id, household_id, description, transaction_date, created_at, created_by
FROM transactions
WHERE household_id = sqlc.arg('household_id')
ORDER BY created_at DESC
LIMIT sqlc.arg('query_limit') OFFSET sqlc.arg('query_offset');

-- name: ListEntriesForTransaction :many
SELECT id, transaction_id, account_id, entry_type, amount, created_at
FROM ledger_entries
WHERE transaction_id = sqlc.arg('transaction_id')
ORDER BY created_at;

-- name: ListTransactionIDsForAccount :many
SELECT DISTINCT transaction_id
FROM ledger_entries
WHERE account_id = sqlc.arg('account_id')
ORDER BY transaction_id;
