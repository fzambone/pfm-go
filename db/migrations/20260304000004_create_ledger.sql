-- +goose Up

-- transactions groups related ledger entries into a single business event.
-- Immutable: no updated_at, updated_by, version, or deleted_at.
CREATE TABLE transactions (
    id               UUID        NOT NULL DEFAULT uuidv7()  PRIMARY KEY,
    household_id     UUID        NOT NULL REFERENCES households(id),
    description      TEXT        NOT NULL,
    transaction_date DATE        NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by       UUID        REFERENCES users(id)
);

-- ledger_entries records individual debit/credit lines within a transaction.
-- Immutable: no updated_at, updated_by, version, or deleted_at.
-- Amount is always positive — entry_type (DEBIT/CREDIT) determines direction.
CREATE TABLE ledger_entries (
    id             UUID        NOT NULL DEFAULT uuidv7()  PRIMARY KEY,
    transaction_id UUID        NOT NULL REFERENCES transactions(id),
    account_id     UUID        NOT NULL REFERENCES accounts(id),
    entry_type     TEXT        NOT NULL CHECK (entry_type IN ('DEBIT', 'CREDIT')),
    amount         BIGINT      NOT NULL CHECK (amount > 0),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down

DROP TABLE IF EXISTS ledger_entries;
DROP TABLE IF EXISTS transactions;
