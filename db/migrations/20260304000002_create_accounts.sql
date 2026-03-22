-- +goose Up

CREATE TABLE accounts (
    id            UUID        NOT NULL DEFAULT uuidv7()  PRIMARY KEY,
    household_id  UUID        NOT NULL REFERENCES households(id),
    name          TEXT        NOT NULL,
    account_type  TEXT        NOT NULL
                              CHECK (account_type IN ('CHECKING', 'SAVINGS', 'CREDIT_CARD', 'INVESTMENT')),
    currency_code TEXT        NOT NULL
                              CHECK (currency_code IN ('USD', 'BRL', 'EUR')),
    balance       BIGINT      NOT NULL DEFAULT 0,
    status        TEXT        NOT NULL DEFAULT 'ACTIVE'
                              CHECK (status IN ('ACTIVE', 'INACTIVE')),
    version       INTEGER     NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by    UUID        REFERENCES users(id),
    updated_by    UUID        REFERENCES users(id),
    deleted_at    TIMESTAMPTZ
);

-- Name uniqueness is scoped to the household and is case-insensitive.
-- The partial index means soft-deleted accounts do not block name reuse.
-- No ON DELETE CASCADE on household_id — accounts must be explicitly managed.
CREATE UNIQUE INDEX accounts_name_unique_active
    ON accounts (household_id, lower(name))
    WHERE deleted_at IS NULL;

-- +goose Down

DROP TABLE IF EXISTS accounts;
