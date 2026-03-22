-- +goose Up

CREATE TABLE households (
    id          UUID        NOT NULL DEFAULT uuidv7()             PRIMARY KEY,
    name        TEXT        NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'ACTIVE'
                            CHECK (status IN ('ACTIVE', 'INACTIVE')),
    version     INTEGER     NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by  UUID        REFERENCES users(id),
    updated_by  UUID        REFERENCES users(id),
    deleted_at  TIMESTAMPTZ
);

-- household_members is a join table: it uses joined_at/invited_by rather than the full audit set.
-- The composite primary key (household_id, user_id) ensures a user cannot be a member twice.
-- ON DELETE CASCADE is a safety net for hard deletes — the application always soft-deletes.
-- role is restricted to ADMIN or MEMBER by a CHECK constraint.
CREATE TABLE household_members (
    household_id UUID        NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    user_id      UUID        NOT NULL REFERENCES users(id)      ON DELETE CASCADE,
    role         TEXT        NOT NULL CHECK (role IN ('ADMIN', 'MEMBER')),
    invited_by   UUID        REFERENCES users(id),
    joined_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ,
    PRIMARY KEY (household_id, user_id)
);

-- +goose Down

DROP TABLE IF EXISTS household_members;
DROP TABLE IF EXISTS households;
