//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	pfmdb "github.com/zambone/pfm-go/db"
	"github.com/zambone/pfm-go/internal/adapter/postgres"
	domainuser "github.com/zambone/pfm-go/internal/domain/user"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/config"
	"github.com/zambone/pfm-go/internal/platform/database"
)

// testPasswordHash is a valid-length placeholder hash used for insertion fixtures.
var integTestPasswordHash = strings.Repeat("x", 128)

// newTestPool spins up a Postgres container, runs all migrations, and returns
// a *pgxpool.Pool. Each call creates an isolated container — tests never share state.
func newTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	ctr, err := tcpostgres.Run(ctx,
		"postgres:18-alpine3.23",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	host, err := ctr.Host(ctx)
	require.NoError(t, err)
	mappedPort, err := ctr.MappedPort(ctx, "5432")
	require.NoError(t, err)

	cfg := &config.Config{
		DatabaseHost:           host,
		DatabasePort:           mappedPort.Int(),
		DatabaseName:           "testdb",
		DatabaseUser:           "testuser",
		DatabasePassword:       "testpass",
		DatabaseSSLMode:        "disable",
		DBConnectTimeoutSec:    5,
		DBStartupRetries:       3,
		DBStartupRetryDelaySec: 1,
		DBMaxOpenConns:         5,
		DBMaxIdleConns:         2,
		DBConnMaxLifetimeSec:   60,
		DBConnMaxIdleTimeSec:   30,
	}

	// Run migrations via database/sql (goose requirement).
	sqlDB, err := database.Open(ctx, cfg)
	require.NoError(t, err)
	sub, err := fs.Sub(pfmdb.Migrations, "migrations")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(ctx, sqlDB, sub))
	require.NoError(t, sqlDB.Close())

	// Re-connect with the pgx-native pool for application queries.
	pool, err := database.NewPool(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	return pool
}

// insertUser is a test helper that inserts a user row directly via SQL.
func insertUser(t *testing.T, pool *pgxpool.Pool, email, passwordHash string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		"INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3)",
		email, passwordHash, "Test User",
	)
	require.NoError(t, err)
}

// TestUserRepo_FindByEmail_ReturnsUser verifies the happy path:
// an existing active user is found and all fields are populated.
func TestUserRepo_FindByEmail_ReturnsUser(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewUserRepo(pool)

	insertUser(t, pool, "alice@example.com", integTestPasswordHash)

	u, err := repo.FindByEmail(ctx, "alice@example.com")

	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", u.Email)
	assert.Equal(t, integTestPasswordHash, u.PasswordHash)
	assert.NotEmpty(t, u.ID)
}

// TestUserRepo_FindByEmail_CaseInsensitive verifies that email lookup ignores case:
// "ALICE@EXAMPLE.COM" matches the record stored as "alice@example.com".
func TestUserRepo_FindByEmail_CaseInsensitive(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewUserRepo(pool)

	insertUser(t, pool, "alice@example.com", integTestPasswordHash)

	u, err := repo.FindByEmail(ctx, "ALICE@EXAMPLE.COM")

	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", u.Email)
}

// TestUserRepo_FindByEmail_NotFound_ReturnsInvalidCredentials verifies that
// a missing email returns an error wrapping ErrLoginInvalidCredentials.
func TestUserRepo_FindByEmail_NotFound_ReturnsInvalidCredentials(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewUserRepo(pool)

	_, err := repo.FindByEmail(ctx, "nobody@example.com")

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrLoginInvalidCredentials),
		"expected ErrLoginInvalidCredentials, got: %v", err)
}

// TestUserRepo_FindByEmail_SoftDeleted_ReturnsInvalidCredentials verifies AC4:
// a soft-deleted user is invisible to FindByEmail (deleted_at IS NULL in SQL).
func TestUserRepo_FindByEmail_SoftDeleted_ReturnsInvalidCredentials(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewUserRepo(pool)

	insertUser(t, pool, "deleted@example.com", integTestPasswordHash)
	_, err := pool.Exec(ctx,
		"UPDATE users SET deleted_at = NOW() WHERE email = $1",
		"deleted@example.com",
	)
	require.NoError(t, err)

	_, err = repo.FindByEmail(ctx, "deleted@example.com")

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrLoginInvalidCredentials),
		"soft-deleted user must return ErrLoginInvalidCredentials, got: %v", err)
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

// integCallerID is uuid.Nil, which maps to SQL NULL via uuidToPgUUID.
// This models the bootstrap scenario: the first user self-registers with no creator.
var integCallerID = uuid.Nil

// integRegisterInput returns a RegisterInput with sensible defaults.
func integRegisterInput(email string) domainuser.RegisterInput {
	return domainuser.RegisterInput{
		Email:       email,
		DisplayName: "Test User",
		Password:    "correct-horse-battery-staple",
	}
}

// TestUserRepo_Create_StoresAndReturnsUser verifies the happy path:
// the created user has a server-assigned ID, correct fields, and Version = 1.
func TestUserRepo_Create_StoresAndReturnsUser(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewUserRepo(pool)

	u, err := repo.Create(ctx, integRegisterInput("create@example.com"), integTestPasswordHash, integCallerID)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, u.ID)
	assert.Equal(t, "create@example.com", u.Email)
	assert.Equal(t, "Test User", u.DisplayName)
	assert.Equal(t, integTestPasswordHash, u.PasswordHash)
	assert.Equal(t, 1, u.Version)
	assert.False(t, u.CreatedAt.IsZero())
	assert.Equal(t, integCallerID, u.CreatedBy)
}

// TestUserRepo_Create_EmailAlreadyTaken verifies that inserting a duplicate email
// returns an error wrapping ErrUserEmailTaken.
func TestUserRepo_Create_EmailAlreadyTaken(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewUserRepo(pool)

	insertUser(t, pool, "taken@example.com", integTestPasswordHash)

	_, err := repo.Create(ctx, integRegisterInput("taken@example.com"), integTestPasswordHash, integCallerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrUserEmailTaken),
		"expected ErrUserEmailTaken, got: %v", err)
}

// ---------------------------------------------------------------------------
// FindByID
// ---------------------------------------------------------------------------

// TestUserRepo_FindByID_ReturnsUser verifies that an inserted user is returned by ID.
func TestUserRepo_FindByID_ReturnsUser(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewUserRepo(pool)

	created, err := repo.Create(ctx, integRegisterInput("findbyid@example.com"), integTestPasswordHash, integCallerID)
	require.NoError(t, err)

	u, err := repo.FindByID(ctx, created.ID)

	require.NoError(t, err)
	assert.Equal(t, created.ID, u.ID)
	assert.Equal(t, "findbyid@example.com", u.Email)
	assert.Equal(t, 1, u.Version)
}

// TestUserRepo_FindByID_NotFound verifies that a missing ID returns ErrUserNotFound.
func TestUserRepo_FindByID_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewUserRepo(pool)

	_, err := repo.FindByID(ctx, uuid.MustParse("00000000-0000-0000-0000-000000000099"))

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrUserNotFound),
		"expected ErrUserNotFound, got: %v", err)
}

// TestUserRepo_FindByID_SoftDeleted_NotFound verifies that a deactivated user
// is invisible to FindByID.
func TestUserRepo_FindByID_SoftDeleted_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewUserRepo(pool)

	created, err := repo.Create(ctx, integRegisterInput("deleted2@example.com"), integTestPasswordHash, integCallerID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "UPDATE users SET deleted_at = NOW() WHERE id = $1", created.ID)
	require.NoError(t, err)

	_, err = repo.FindByID(ctx, created.ID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrUserNotFound),
		"soft-deleted user must not be findable by ID, got: %v", err)
}

// ---------------------------------------------------------------------------
// UpdateProfile
// ---------------------------------------------------------------------------

// TestUserRepo_UpdateProfile_UpdatesDisplayName verifies the happy path:
// display name is updated and version increments.
func TestUserRepo_UpdateProfile_UpdatesDisplayName(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewUserRepo(pool)

	created, err := repo.Create(ctx, integRegisterInput("profile@example.com"), integTestPasswordHash, integCallerID)
	require.NoError(t, err)

	updated, err := repo.UpdateProfile(ctx, created.ID,
		domainuser.UpdateProfileInput{DisplayName: "New Name"},
		created.Version, integCallerID)

	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.DisplayName)
	assert.Equal(t, created.Version+1, updated.Version)
}

// TestUserRepo_UpdateProfile_VersionConflict verifies that a stale version
// returns ErrUserVersionConflict.
func TestUserRepo_UpdateProfile_VersionConflict(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewUserRepo(pool)

	created, err := repo.Create(ctx, integRegisterInput("conflict@example.com"), integTestPasswordHash, integCallerID)
	require.NoError(t, err)

	_, err = repo.UpdateProfile(ctx, created.ID,
		domainuser.UpdateProfileInput{DisplayName: "X"},
		created.Version+99, integCallerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrUserVersionConflict),
		"expected ErrUserVersionConflict, got: %v", err)
}

// ---------------------------------------------------------------------------
// ChangePassword
// ---------------------------------------------------------------------------

// TestUserRepo_ChangePassword_UpdatesHash verifies the happy path:
// the password hash is replaced and version increments.
func TestUserRepo_ChangePassword_UpdatesHash(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewUserRepo(pool)

	created, err := repo.Create(ctx, integRegisterInput("pwd@example.com"), integTestPasswordHash, integCallerID)
	require.NoError(t, err)

	newHash := strings.Repeat("y", 128)
	updated, err := repo.ChangePassword(ctx, created.ID, newHash, created.Version, integCallerID)

	require.NoError(t, err)
	assert.Equal(t, newHash, updated.PasswordHash)
	assert.Equal(t, created.Version+1, updated.Version)
}

// TestUserRepo_ChangePassword_VersionConflict verifies that a stale version
// returns ErrUserVersionConflict.
func TestUserRepo_ChangePassword_VersionConflict(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewUserRepo(pool)

	created, err := repo.Create(ctx, integRegisterInput("pwdconflict@example.com"), integTestPasswordHash, integCallerID)
	require.NoError(t, err)

	_, err = repo.ChangePassword(ctx, created.ID, strings.Repeat("z", 128), created.Version+99, integCallerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrUserVersionConflict),
		"expected ErrUserVersionConflict, got: %v", err)
}

// ---------------------------------------------------------------------------
// Deactivate
// ---------------------------------------------------------------------------

// TestUserRepo_Deactivate_SoftDeletesUser verifies that a deactivated user
// is no longer findable by ID or email.
func TestUserRepo_Deactivate_SoftDeletesUser(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewUserRepo(pool)

	created, err := repo.Create(ctx, integRegisterInput("deactivate@example.com"), integTestPasswordHash, integCallerID)
	require.NoError(t, err)

	err = repo.Deactivate(ctx, created.ID, integCallerID)
	require.NoError(t, err)

	_, err = repo.FindByID(ctx, created.ID)
	assert.True(t, errors.Is(err, message.ErrUserNotFound),
		"deactivated user must not be findable by ID")

	_, err = repo.FindByEmail(ctx, "deactivate@example.com")
	assert.True(t, errors.Is(err, message.ErrLoginInvalidCredentials),
		"deactivated user must not be findable by email")
}

// TestUserRepo_Deactivate_IsIdempotent verifies that deactivating an already-
// deactivated user does not return an error.
func TestUserRepo_Deactivate_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewUserRepo(pool)

	created, err := repo.Create(ctx, integRegisterInput("idemp@example.com"), integTestPasswordHash, integCallerID)
	require.NoError(t, err)

	require.NoError(t, repo.Deactivate(ctx, created.ID, integCallerID))
	assert.NoError(t, repo.Deactivate(ctx, created.ID, integCallerID), "second deactivate must not error")
}
