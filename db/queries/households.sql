-- name: CreateHousehold :one
INSERT INTO households (name, created_by, updated_by)
VALUES (sqlc.arg('name'), sqlc.arg('created_by'), sqlc.arg('updated_by'))
RETURNING id, name, status, version, created_at, updated_at, created_by, updated_by;

-- name: FindHouseholdByID :one
SELECT id, name, status, version, created_at, updated_at, created_by, updated_by
FROM households
WHERE id = sqlc.arg('id')
  AND deleted_at IS NULL;

-- name: ListHouseholdsForUser :many
SELECT h.id, h.name, h.status, h.version, h.created_at, h.updated_at, h.created_by, h.updated_by
FROM households h
JOIN household_members hm ON hm.household_id = h.id
WHERE hm.user_id = sqlc.arg('user_id')
  AND hm.deleted_at IS NULL
  AND h.deleted_at IS NULL;

-- name: UpdateHouseholdName :one
UPDATE households
SET name       = sqlc.arg('name'),
    updated_at = NOW(),
    updated_by = sqlc.arg('updated_by'),
    version    = version + 1
WHERE id       = sqlc.arg('id')
  AND version  = sqlc.arg('expected_version')
  AND deleted_at IS NULL
RETURNING id, name, status, version, created_at, updated_at, created_by, updated_by;

-- name: DeactivateHousehold :exec
UPDATE households
SET deleted_at = NOW(),
    updated_at = NOW(),
    updated_by = sqlc.arg('updated_by'),
    status     = 'INACTIVE'
WHERE id       = sqlc.arg('id')
  AND deleted_at IS NULL;

-- name: CreateHouseholdMember :one
INSERT INTO household_members (household_id, user_id, role, invited_by)
VALUES (sqlc.arg('household_id'), sqlc.arg('user_id'), sqlc.arg('role'), sqlc.arg('invited_by'))
RETURNING household_id, user_id, role, invited_by, joined_at;

-- name: FindMembership :one
SELECT household_id, user_id, role, invited_by, joined_at
FROM household_members
WHERE household_id = sqlc.arg('household_id')
  AND user_id      = sqlc.arg('user_id')
  AND deleted_at IS NULL;

-- name: ListMembers :many
SELECT household_id, user_id, role, invited_by, joined_at
FROM household_members
WHERE household_id = sqlc.arg('household_id')
  AND deleted_at IS NULL;

-- name: RemoveHouseholdMember :exec
UPDATE household_members
SET deleted_at = NOW()
WHERE household_id = sqlc.arg('household_id')
  AND user_id      = sqlc.arg('user_id')
  AND deleted_at IS NULL;
