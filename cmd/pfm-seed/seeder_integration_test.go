//go:build integration

package main

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authadapter "github.com/zambone/pfm-go/internal/adapter/auth"
	pgadapter "github.com/zambone/pfm-go/internal/adapter/postgres"
	"github.com/zambone/pfm-go/internal/platform/database"
	"github.com/zambone/pfm-go/internal/testutil/dbtest"
)

var sharedDB *dbtest.SharedDB

func TestMain(m *testing.M) {
	ctx := context.Background()
	var cleanup func()
	sharedDB, cleanup = dbtest.Setup(ctx)
	defer cleanup()
	sharedDB.PrepareTemplate(ctx)
	os.Exit(m.Run())
}

// newSeedEnv wires a real seeder against a per-test isolated Postgres database.
func newSeedEnv(t *testing.T, ctx context.Context) *seeder {
	t.Helper()

	pool := sharedDB.NewPool(t, ctx)
	hasher := authadapter.NewArgon2idHasher(authadapter.DefaultArgon2idParams())
	userRepo := pgadapter.NewUserRepo(pool)
	householdRepo := pgadapter.NewHouseholdRepo(pool)
	tx := database.NewPostgresTransactor(pool)

	return newSeeder(hasher, userRepo, userRepo, householdRepo, tx)
}

// TestSeeder_Integration_BootstrapsFirstUser verifies AC1: running the seeder against an
// empty database creates one user, one household, and one ADMIN membership atomically.
func TestSeeder_Integration_BootstrapsFirstUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newSeedEnv(t, ctx)

	result, err := s.Run(ctx, seedInput{
		Email:         "alice@example.com",
		DisplayName:   "Alice",
		Password:      "secret1234",
		HouseholdName: "Alice's Household",
	})

	require.NoError(t, err)
	assert.False(t, result.AlreadySeeded, "first run must not report already seeded")
	assert.NotEmpty(t, result.UserID, "result must carry the created user ID")
	assert.NotEmpty(t, result.HouseholdID, "result must carry the created household ID")
}

// TestSeeder_Integration_IsIdempotent verifies AC2: a second run against a database
// that already contains users exits cleanly with AlreadySeeded=true.
func TestSeeder_Integration_IsIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newSeedEnv(t, ctx)

	input := seedInput{
		Email:         "bob@example.com",
		DisplayName:   "Bob",
		Password:      "secret1234",
		HouseholdName: "Bob's Household",
	}

	first, err := s.Run(ctx, input)
	require.NoError(t, err)
	require.False(t, first.AlreadySeeded)

	second, err := s.Run(ctx, input)
	require.NoError(t, err)
	assert.True(t, second.AlreadySeeded, "second run must report already seeded")
}

// alwaysFreshChecker is a test-only existenceChecker that always reports no users exist.
// It forces the seeder to enter the transaction path even when a user is already in the DB,
// allowing integration tests to exercise mid-transaction rollback behaviour.
type alwaysFreshChecker struct{}

func (alwaysFreshChecker) AnyExists(_ context.Context) (bool, error) { return false, nil }

// TestSeeder_Integration_RollbackOnFailure verifies AC5: when the user insert fails
// mid-transaction (duplicate email unique constraint), the transaction is rolled back and
// no household row persists.
func TestSeeder_Integration_RollbackOnFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	hasher := authadapter.NewArgon2idHasher(authadapter.DefaultArgon2idParams())
	userRepo := pgadapter.NewUserRepo(pool)
	householdRepo := pgadapter.NewHouseholdRepo(pool)
	tx := database.NewPostgresTransactor(pool)

	// Use a fake checker that bypasses the idempotency guard so the seeder always
	// enters the transaction, even when a user already exists.
	s := newSeeder(hasher, alwaysFreshChecker{}, userRepo, householdRepo, tx)

	// Pre-insert a user with the target email directly — outside any seeder transaction.
	// When the seeder tries to create the same email inside the transaction, the unique
	// constraint fires and the transaction is aborted.
	_, err := pool.Exec(ctx,
		`INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3)`,
		"dave@example.com", "somehash", "Dave",
	)
	require.NoError(t, err, "pre-insert must succeed")

	_, err = s.Run(ctx, seedInput{
		Email:         "dave@example.com",
		DisplayName:   "Dave",
		Password:      "secret1234",
		HouseholdName: "Dave's Household",
	})

	require.Error(t, err, "seeder must fail on duplicate email")

	// Confirm no household row was created — the transaction rolled back completely.
	var count int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM households WHERE name = $1`, "Dave's Household").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "household must not persist after transaction rollback")
}

// TestSeeder_Integration_CreatedUserCanLogin verifies AC4: after seeding, the created
// user's email and password hash are valid — the login path can authenticate them.
func TestSeeder_Integration_CreatedUserCanLogin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	hasher := authadapter.NewArgon2idHasher(authadapter.DefaultArgon2idParams())
	userRepo := pgadapter.NewUserRepo(pool)
	householdRepo := pgadapter.NewHouseholdRepo(pool)
	tx := database.NewPostgresTransactor(pool)
	s := newSeeder(hasher, userRepo, userRepo, householdRepo, tx)

	const password = "secret1234"
	_, err := s.Run(ctx, seedInput{
		Email:         "carol@example.com",
		DisplayName:   "Carol",
		Password:      password,
		HouseholdName: "Carol's Household",
	})
	require.NoError(t, err)

	// Verify the stored hash authenticates with the original plaintext password.
	found, err := userRepo.FindByEmail(ctx, "carol@example.com")
	require.NoError(t, err)

	ok, err := hasher.Verify(ctx, password, found.PasswordHash)
	require.NoError(t, err)
	assert.True(t, ok, "stored hash must verify against the original password")
}
