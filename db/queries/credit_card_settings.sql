-- name: CreateCreditCardSettings :one
INSERT INTO credit_card_settings (account_id, closing_day, due_day, limit_amount, created_by, updated_by)
VALUES (sqlc.arg('account_id'), sqlc.arg('closing_day'), sqlc.arg('due_day'),
        sqlc.arg('limit_amount'), sqlc.arg('created_by'), sqlc.arg('updated_by'))
RETURNING id, account_id, closing_day, due_day, limit_amount, version,
          created_at, updated_at, created_by, updated_by;

-- name: FindCreditCardSettingsByAccountID :one
SELECT id, account_id, closing_day, due_day, limit_amount, version,
       created_at, updated_at, created_by, updated_by
FROM credit_card_settings
WHERE account_id = sqlc.arg('account_id')
  AND deleted_at IS NULL;

-- name: UpdateCreditCardClosingDay :one
UPDATE credit_card_settings
SET closing_day = sqlc.arg('closing_day'),
    updated_at  = NOW(),
    updated_by  = sqlc.arg('updated_by'),
    version     = version + 1
WHERE account_id = sqlc.arg('account_id')
  AND version    = sqlc.arg('expected_version')
  AND deleted_at IS NULL
RETURNING id, account_id, closing_day, due_day, limit_amount, version,
          created_at, updated_at, created_by, updated_by;

-- name: UpdateCreditCardDueDay :one
UPDATE credit_card_settings
SET due_day    = sqlc.arg('due_day'),
    updated_at = NOW(),
    updated_by = sqlc.arg('updated_by'),
    version    = version + 1
WHERE account_id = sqlc.arg('account_id')
  AND version    = sqlc.arg('expected_version')
  AND deleted_at IS NULL
RETURNING id, account_id, closing_day, due_day, limit_amount, version,
          created_at, updated_at, created_by, updated_by;

-- name: UpdateCreditCardLimit :one
UPDATE credit_card_settings
SET limit_amount = sqlc.arg('limit_amount'),
    updated_at   = NOW(),
    updated_by   = sqlc.arg('updated_by'),
    version      = version + 1
WHERE account_id = sqlc.arg('account_id')
  AND version    = sqlc.arg('expected_version')
  AND deleted_at IS NULL
RETURNING id, account_id, closing_day, due_day, limit_amount, version,
          created_at, updated_at, created_by, updated_by;

-- name: DeleteCreditCardSettings :exec
UPDATE credit_card_settings
SET deleted_at = NOW(),
    updated_at = NOW(),
    updated_by = sqlc.arg('updated_by')
WHERE account_id = sqlc.arg('account_id')
  AND deleted_at IS NULL;
