package main

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainhousehold "github.com/zambone/pfm-go/internal/domain/household"
	domainuser "github.com/zambone/pfm-go/internal/domain/user"
)

// fakeHasher implements passwordHasher for tests.
type fakeHasher struct {
	hash string
	err  error
}

func (f *fakeHasher) Hash(_ context.Context, _ string) (string, error) {
	return f.hash, f.err
}

// fakeExistenceChecker implements existenceChecker for tests.
type fakeExistenceChecker struct {
	exists bool
	err    error
}

func (f *fakeExistenceChecker) AnyExists(_ context.Context) (bool, error) {
	return f.exists, f.err
}

// fakeUserCreator implements userCreator for tests.
type fakeUserCreator struct {
	user domainuser.User
	err  error
}

func (f *fakeUserCreator) Create(_ context.Context, _ domainuser.RegisterInput, _ string, _ uuid.UUID) (domainuser.User, error) {
	return f.user, f.err
}

// fakeHouseholdCreator implements householdCreator for tests.
type fakeHouseholdCreator struct {
	household domainhousehold.Household
	err       error
}

func (f *fakeHouseholdCreator) Create(_ context.Context, _ domainhousehold.CreateInput, _ uuid.UUID) (domainhousehold.Household, error) {
	return f.household, f.err
}

// fakeTransactor implements seederTransactor for tests. It executes fn inline (no real DB).
type fakeTransactor struct {
	err error // if non-nil, RunAtomic returns this error without calling fn
}

func (f *fakeTransactor) RunAtomic(ctx context.Context, fn func(context.Context) error) error {
	if f.err != nil {
		return f.err
	}
	return fn(ctx)
}

// seedTestEnv builds a Seeder with all fakes pre-wired for happy-path defaults.
func seedTestEnv() (*seeder, *fakeExistenceChecker, *fakeUserCreator, *fakeHouseholdCreator) {
	checker := &fakeExistenceChecker{exists: false}
	hasher := &fakeHasher{hash: "hashed-password"}
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	hhID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	creator := &fakeUserCreator{user: domainuser.User{ID: userID, Email: "alice@example.com"}}
	hhCreator := &fakeHouseholdCreator{household: domainhousehold.Household{ID: hhID, Name: "Test HH"}}
	tx := &fakeTransactor{}

	s := newSeeder(hasher, checker, creator, hhCreator, tx)
	return s, checker, creator, hhCreator
}

// TestSeeder_Run_Success verifies the happy path: all three writes occur and the result
// carries the created user and household IDs.
func TestSeeder_Run_Success(t *testing.T) {
	s, _, _, _ := seedTestEnv()

	result, err := s.Run(context.Background(), seedInput{
		Email:         "alice@example.com",
		DisplayName:   "Alice",
		Password:      "secret1234",
		HouseholdName: "Test HH",
	})

	require.NoError(t, err)
	assert.False(t, result.AlreadySeeded)
	assert.NotEqual(t, uuid.Nil, result.UserID)
	assert.NotEqual(t, uuid.Nil, result.HouseholdID)
}

// TestSeeder_Run_WhenAlreadySeeded verifies AC2: when users already exist, Run returns
// AlreadySeeded=true and makes no DB writes.
func TestSeeder_Run_WhenAlreadySeeded(t *testing.T) {
	s, checker, creator, hhCreator := seedTestEnv()
	checker.exists = true

	// Force errors on write operations to confirm they are never called.
	creator.err = errors.New("should not be called")
	hhCreator.err = errors.New("should not be called")

	result, err := s.Run(context.Background(), seedInput{
		Email:         "alice@example.com",
		DisplayName:   "Alice",
		Password:      "secret1234",
		HouseholdName: "Test HH",
	})

	require.NoError(t, err)
	assert.True(t, result.AlreadySeeded)
	assert.Equal(t, uuid.Nil, result.UserID)
	assert.Equal(t, uuid.Nil, result.HouseholdID)
}

// TestSeeder_Run_WhenExistenceCheckFails verifies that a DB error on the existence check
// is propagated immediately.
func TestSeeder_Run_WhenExistenceCheckFails(t *testing.T) {
	s, checker, _, _ := seedTestEnv()
	checker.err = errors.New("connection refused")

	_, err := s.Run(context.Background(), seedInput{
		Email:         "alice@example.com",
		DisplayName:   "Alice",
		Password:      "secret1234",
		HouseholdName: "Test HH",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

// TestSeeder_Run_WhenUserCreateFails verifies AC5: a failure inside the transaction
// causes RunAtomic to roll back. The error is propagated to the caller.
func TestSeeder_Run_WhenUserCreateFails(t *testing.T) {
	s, _, creator, _ := seedTestEnv()
	creator.err = errors.New("unique constraint violation")

	_, err := s.Run(context.Background(), seedInput{
		Email:         "alice@example.com",
		DisplayName:   "Alice",
		Password:      "secret1234",
		HouseholdName: "Test HH",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unique constraint violation")
}

// TestSeeder_Run_WhenHouseholdCreateFails verifies AC5: a household creation failure
// rolls back the entire transaction (user row must not persist).
func TestSeeder_Run_WhenHouseholdCreateFails(t *testing.T) {
	s, _, _, hhCreator := seedTestEnv()
	hhCreator.err = errors.New("household insert error")

	_, err := s.Run(context.Background(), seedInput{
		Email:         "alice@example.com",
		DisplayName:   "Alice",
		Password:      "secret1234",
		HouseholdName: "Test HH",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "household insert error")
}

