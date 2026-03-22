-- name: FindUserByEmail :one
SELECT
    id,
    email,
    password_hash,
    deleted_at
FROM users
WHERE lower(email) = lower(sqlc.arg('email'))
  AND deleted_at IS NULL
LIMIT 1;
