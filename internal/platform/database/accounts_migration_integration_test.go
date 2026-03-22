//go:build integration

package database

import (
	"context"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAccountsMigration_DefaultsOnCreate verifies AC1: a new account has name, type,
// currency, zero balance, version=1, and status='ACTIVE'.
func TestAccountsMigration_DefaultsOnCreate(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t, ctx)
	require.NoError(t, Migrate(ctx, db, usersMigrationsFS(t)))

	var userID, householdID string
	err := db.QueryRowContext(ctx,
		"INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3) RETURNING id::text",
		"alice@example.com", testPasswordHash, "Alice",
	).Scan(&userID)
	require.NoError(t, err)

	err = db.QueryRowContext(ctx,
		"INSERT INTO households (name, created_by, updated_by) VALUES ($1, $2::uuid, $2::uuid) RETURNING id::text",
		"Personal", userID,
	).Scan(&householdID)
	require.NoError(t, err)

	var status, accountType, currencyCode string
	var balance int64
	var version int
	var createdOK, updatedOK bool

	err = db.QueryRowContext(ctx,
		`INSERT INTO accounts (household_id, name, account_type, currency_code, created_by, updated_by)
		 VALUES ($1::uuid, $2, $3, $4, $5::uuid, $5::uuid)
		 RETURNING status, account_type, currency_code, balance, version,
		           created_at IS NOT NULL, updated_at IS NOT NULL`,
		householdID, "Checking", "CHECKING", "USD", userID,
	).Scan(&status, &accountType, &currencyCode, &balance, &version, &createdOK, &updatedOK)

	require.NoError(t, err)
	assert.Equal(t, "ACTIVE", status, "default status must be ACTIVE")
	assert.Equal(t, "CHECKING", accountType)
	assert.Equal(t, "USD", currencyCode)
	assert.Equal(t, int64(0), balance, "initial balance must be zero")
	assert.Equal(t, 1, version, "version must default to 1")
	assert.True(t, createdOK, "created_at must be set automatically")
	assert.True(t, updatedOK, "updated_at must be set automatically")
}

// TestAccountsMigration_NameUniqueWithinHousehold verifies AC2: two accounts in the same
// household cannot share the same name.
func TestAccountsMigration_NameUniqueWithinHousehold(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t, ctx)
	require.NoError(t, Migrate(ctx, db, usersMigrationsFS(t)))

	var userID, householdID string
	err := db.QueryRowContext(ctx,
		"INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3) RETURNING id::text",
		"bob@example.com", testPasswordHash, "Bob",
	).Scan(&userID)
	require.NoError(t, err)

	err = db.QueryRowContext(ctx,
		"INSERT INTO households (name, created_by, updated_by) VALUES ($1, $2::uuid, $2::uuid) RETURNING id::text",
		"Bob's Household", userID,
	).Scan(&householdID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		"INSERT INTO accounts (household_id, name, account_type, currency_code, created_by, updated_by) VALUES ($1::uuid, $2, $3, $4, $5::uuid, $5::uuid)",
		householdID, "Savings", "SAVINGS", "USD", userID,
	)
	require.NoError(t, err)

	// Same name (case-insensitive) in same household must be rejected.
	_, err = db.ExecContext(ctx,
		"INSERT INTO accounts (household_id, name, account_type, currency_code, created_by, updated_by) VALUES ($1::uuid, $2, $3, $4, $5::uuid, $5::uuid)",
		householdID, "SAVINGS", "CHECKING", "BRL", userID,
	)
	assert.Error(t, err, "duplicate name in same household must be rejected")
}

// TestAccountsMigration_NameAllowedAcrossHouseholds verifies AC3: two accounts in different
// households may share the same name.
func TestAccountsMigration_NameAllowedAcrossHouseholds(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t, ctx)
	require.NoError(t, Migrate(ctx, db, usersMigrationsFS(t)))

	var user1ID, user2ID, household1ID, household2ID string
	err := db.QueryRowContext(ctx,
		"INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3) RETURNING id::text",
		"carol@example.com", testPasswordHash, "Carol",
	).Scan(&user1ID)
	require.NoError(t, err)

	err = db.QueryRowContext(ctx,
		"INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3) RETURNING id::text",
		"dave@example.com", testPasswordHash, "Dave",
	).Scan(&user2ID)
	require.NoError(t, err)

	err = db.QueryRowContext(ctx,
		"INSERT INTO households (name, created_by, updated_by) VALUES ($1, $2::uuid, $2::uuid) RETURNING id::text",
		"Carol's Household", user1ID,
	).Scan(&household1ID)
	require.NoError(t, err)

	err = db.QueryRowContext(ctx,
		"INSERT INTO households (name, created_by, updated_by) VALUES ($1, $2::uuid, $2::uuid) RETURNING id::text",
		"Dave's Household", user2ID,
	).Scan(&household2ID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		"INSERT INTO accounts (household_id, name, account_type, currency_code, created_by, updated_by) VALUES ($1::uuid, $2, $3, $4, $5::uuid, $5::uuid)",
		household1ID, "Personal Savings", "SAVINGS", "USD", user1ID,
	)
	require.NoError(t, err)

	// Same name in a different household must succeed.
	_, err = db.ExecContext(ctx,
		"INSERT INTO accounts (household_id, name, account_type, currency_code, created_by, updated_by) VALUES ($1::uuid, $2, $3, $4, $5::uuid, $5::uuid)",
		household2ID, "Personal Savings", "SAVINGS", "BRL", user2ID,
	)
	assert.NoError(t, err, "same name in a different household must be allowed")
}

// TestAccountsMigration_SoftDeleteReleasesName verifies AC4: after an account is soft-deleted,
// its name becomes available for reuse within the same household.
func TestAccountsMigration_SoftDeleteReleasesName(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t, ctx)
	require.NoError(t, Migrate(ctx, db, usersMigrationsFS(t)))

	var userID, householdID string
	err := db.QueryRowContext(ctx,
		"INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3) RETURNING id::text",
		"eve@example.com", testPasswordHash, "Eve",
	).Scan(&userID)
	require.NoError(t, err)

	err = db.QueryRowContext(ctx,
		"INSERT INTO households (name, created_by, updated_by) VALUES ($1, $2::uuid, $2::uuid) RETURNING id::text",
		"Eve's Household", userID,
	).Scan(&householdID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		"INSERT INTO accounts (household_id, name, account_type, currency_code, created_by, updated_by) VALUES ($1::uuid, $2, $3, $4, $5::uuid, $5::uuid)",
		householdID, "Old Checking", "CHECKING", "USD", userID,
	)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		"UPDATE accounts SET deleted_at = NOW() WHERE household_id = $1::uuid AND name = $2",
		householdID, "Old Checking",
	)
	require.NoError(t, err)

	// Same name must now be accepted after soft-delete.
	_, err = db.ExecContext(ctx,
		"INSERT INTO accounts (household_id, name, account_type, currency_code, created_by, updated_by) VALUES ($1::uuid, $2, $3, $4, $5::uuid, $5::uuid)",
		householdID, "Old Checking", "CHECKING", "EUR", userID,
	)
	assert.NoError(t, err, "name must be reusable after soft-delete")
}

// TestAccountsMigration_AccountTypeEnforced verifies AC5: only valid account_type values
// are accepted; any other value is rejected by the CHECK constraint.
func TestAccountsMigration_AccountTypeEnforced(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t, ctx)
	require.NoError(t, Migrate(ctx, db, usersMigrationsFS(t)))

	var userID, householdID string
	err := db.QueryRowContext(ctx,
		"INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3) RETURNING id::text",
		"frank@example.com", testPasswordHash, "Frank",
	).Scan(&userID)
	require.NoError(t, err)

	err = db.QueryRowContext(ctx,
		"INSERT INTO households (name, created_by, updated_by) VALUES ($1, $2::uuid, $2::uuid) RETURNING id::text",
		"Frank's Household", userID,
	).Scan(&householdID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		"INSERT INTO accounts (household_id, name, account_type, currency_code, created_by, updated_by) VALUES ($1::uuid, $2, $3, $4, $5::uuid, $5::uuid)",
		householdID, "Bad Account", "BROKERAGE", "USD", userID,
	)
	assert.Error(t, err, "invalid account_type must be rejected by CHECK constraint")
}

// TestAccountsMigration_CurrencyCodeEnforced verifies AC6: only USD, BRL, and EUR are
// valid currency codes; any other value is rejected by the CHECK constraint.
func TestAccountsMigration_CurrencyCodeEnforced(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t, ctx)
	require.NoError(t, Migrate(ctx, db, usersMigrationsFS(t)))

	var userID, householdID string
	err := db.QueryRowContext(ctx,
		"INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3) RETURNING id::text",
		"grace@example.com", testPasswordHash, "Grace",
	).Scan(&userID)
	require.NoError(t, err)

	err = db.QueryRowContext(ctx,
		"INSERT INTO households (name, created_by, updated_by) VALUES ($1, $2::uuid, $2::uuid) RETURNING id::text",
		"Grace's Household", userID,
	).Scan(&householdID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		"INSERT INTO accounts (household_id, name, account_type, currency_code, created_by, updated_by) VALUES ($1::uuid, $2, $3, $4, $5::uuid, $5::uuid)",
		householdID, "Bad Currency", "SAVINGS", "GBP", userID,
	)
	assert.Error(t, err, "invalid currency_code must be rejected by CHECK constraint")
}

// TestAccountsMigration_RollbackRemovesTable verifies AC7: rolling back the accounts
// migration drops the table and all constraints cleanly.
func TestAccountsMigration_RollbackRemovesTable(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t, ctx)
	sub := usersMigrationsFS(t)
	require.NoError(t, Migrate(ctx, db, sub))

	var exists bool
	err := db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'accounts')",
	).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "accounts table must exist after Up migration")

	provider, err := goose.NewProvider(goose.DialectPostgres, db, sub,
		goose.WithLogger(goose.NopLogger()),
	)
	require.NoError(t, err)

	// Roll back only the accounts migration (version 20260304000002 → 20260304000001).
	_, err = provider.DownTo(ctx, 20260304000001)
	require.NoError(t, err)

	err = db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'accounts')",
	).Scan(&exists)
	require.NoError(t, err)
	assert.False(t, exists, "accounts table must be absent after rollback")
}
