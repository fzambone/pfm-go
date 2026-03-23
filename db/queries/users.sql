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

-- name: CreateUser :one
INSERT INTO users (email, password_hash, display_name, created_by, updated_by)
VALUES (sqlc.arg('email'), sqlc.arg('password_hash'), sqlc.arg('display_name'),
        sqlc.arg('created_by'), sqlc.arg('updated_by'))
RETURNING id, email, display_name, password_hash, version, created_at, updated_at, created_by, updated_by;

-- name: FindUserByID :one
SELECT id, email, display_name, password_hash, version, created_at, updated_at, created_by, updated_by
FROM users
WHERE id = sqlc.arg('id')
  AND deleted_at IS NULL;

-- name: UpdateUserProfile :one
UPDATE users
SET display_name = sqlc.arg('display_name'),
    updated_at   = NOW(),
    updated_by   = sqlc.arg('updated_by'),
    version      = version + 1
WHERE id         = sqlc.arg('id')
  AND version    = sqlc.arg('expected_version')
  AND deleted_at IS NULL
RETURNING id, email, display_name, password_hash, version, created_at, updated_at, created_by, updated_by;

-- name: ChangeUserPassword :one
UPDATE users
SET password_hash = sqlc.arg('password_hash'),
    updated_at    = NOW(),
    updated_by    = sqlc.arg('updated_by'),
    version       = version + 1
WHERE id          = sqlc.arg('id')
  AND version     = sqlc.arg('expected_version')
  AND deleted_at  IS NULL
RETURNING id, email, display_name, password_hash, version, created_at, updated_at, created_by, updated_by;

-- name: DeactivateUser :exec
UPDATE users
SET deleted_at = NOW(),
    updated_at = NOW(),
    updated_by = sqlc.arg('updated_by')
WHERE id       = sqlc.arg('id')
  AND deleted_at IS NULL;
