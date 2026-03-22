-- +goose Up

-- credit_card_settings stores billing cycle and limit details for CREDIT_CARD accounts.
-- There is exactly one row per account — enforced by the UNIQUE constraint on account_id.
-- ON DELETE CASCADE: settings are meaningless without their parent account.
CREATE TABLE credit_card_settings (
    id           UUID        NOT NULL DEFAULT uuidv7()  PRIMARY KEY,
    account_id   UUID        NOT NULL UNIQUE REFERENCES accounts(id) ON DELETE CASCADE,
    closing_day  INTEGER     NOT NULL CHECK (closing_day >= 1 AND closing_day <= 31),
    due_day      INTEGER     NOT NULL CHECK (due_day >= 1 AND due_day <= 31),
    limit_amount BIGINT      NOT NULL,
    version      INTEGER     NOT NULL DEFAULT 1,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by   UUID        REFERENCES users(id),
    updated_by   UUID        REFERENCES users(id),
    deleted_at   TIMESTAMPTZ
);

-- +goose Down

DROP TABLE IF EXISTS credit_card_settings;
