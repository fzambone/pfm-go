-- +goose Up

-- The original credit_card_settings_account_id_key constraint is a full UNIQUE,
-- which blocks re-creating settings for an account after soft-deletion. Replace it
-- with a partial unique index that only covers non-deleted rows so that settings
-- can be recreated after deletion.
ALTER TABLE credit_card_settings
    DROP CONSTRAINT credit_card_settings_account_id_key;

CREATE UNIQUE INDEX credit_card_settings_account_id_active_idx
    ON credit_card_settings (account_id)
    WHERE deleted_at IS NULL;

-- +goose Down

DROP INDEX IF EXISTS credit_card_settings_account_id_active_idx;

ALTER TABLE credit_card_settings
    ADD CONSTRAINT credit_card_settings_account_id_key UNIQUE (account_id);
