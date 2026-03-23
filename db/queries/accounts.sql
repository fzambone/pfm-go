-- name: CreateAccount :one
INSERT INTO accounts (household_id, name, account_type, currency_code, created_by, updated_by)
VALUES (sqlc.arg('household_id'), sqlc.arg('name'), sqlc.arg('account_type'),
        sqlc.arg('currency_code'), sqlc.arg('created_by'), sqlc.arg('updated_by'))
RETURNING id, household_id, name, account_type, currency_code, balance, status, version,
          created_at, updated_at, created_by, updated_by;

-- name: FindAccountByID :one
SELECT id, household_id, name, account_type, currency_code, balance, status, version,
       created_at, updated_at, created_by, updated_by
FROM accounts
WHERE id = sqlc.arg('id')
  AND deleted_at IS NULL;

-- name: ListAccountsForHousehold :many
SELECT id, household_id, name, account_type, currency_code, balance, status, version,
       created_at, updated_at, created_by, updated_by
FROM accounts
WHERE household_id = sqlc.arg('household_id')
  AND deleted_at IS NULL
ORDER BY name;

-- name: UpdateAccountName :one
UPDATE accounts
SET name       = sqlc.arg('name'),
    updated_at = NOW(),
    updated_by = sqlc.arg('updated_by'),
    version    = version + 1
WHERE id       = sqlc.arg('id')
  AND version  = sqlc.arg('expected_version')
  AND deleted_at IS NULL
RETURNING id, household_id, name, account_type, currency_code, balance, status, version,
          created_at, updated_at, created_by, updated_by;

-- name: UpdateAccountBalance :one
UPDATE accounts
SET balance    = sqlc.arg('balance'),
    updated_at = NOW(),
    updated_by = sqlc.arg('updated_by'),
    version    = version + 1
WHERE id       = sqlc.arg('id')
  AND version  = sqlc.arg('expected_version')
  AND deleted_at IS NULL
RETURNING id, household_id, name, account_type, currency_code, balance, status, version,
          created_at, updated_at, created_by, updated_by;

-- name: DeactivateAccount :exec
UPDATE accounts
SET deleted_at = NOW(),
    updated_at = NOW(),
    updated_by = sqlc.arg('updated_by'),
    status     = 'INACTIVE'
WHERE id       = sqlc.arg('id')
  AND deleted_at IS NULL;
