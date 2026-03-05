-- +goose Up

CREATE TABLE users (
    id            UUID         NOT NULL DEFAULT uuidv7()  PRIMARY KEY,
    email         TEXT         NOT NULL,
    password_hash TEXT         NOT NULL,
    display_name  TEXT         NOT NULL,
    version       INTEGER      NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_by    UUID         REFERENCES users(id),
    updated_by    UUID         REFERENCES users(id),
    deleted_at    TIMESTAMPTZ
);

-- Case-insensitive uniqueness enforced only among active (non-deleted) users.
-- A partial index means deleted users' emails can be reclaimed by new registrations.
CREATE UNIQUE INDEX users_email_unique_active
    ON users (lower(email))
    WHERE deleted_at IS NULL;

-- +goose Down

DROP TABLE IF EXISTS users;
