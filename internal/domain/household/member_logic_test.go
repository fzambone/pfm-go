package household_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zambone/pfm-go/internal/adapter/postgres"
	"github.com/zambone/pfm-go/internal/domain/household"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/clock"
	"github.com/zambone/pfm-go/internal/platform/database"
)

// compile-time type checks — fail to compile if the types don't exist.
var (
	_ = household.NewUserInput{}
	_ = household.CreatedUser{}
	_ = household.CreatedMember{}
)

// --- Test helpers ---

// fakeUserCreator is a test double for the household.userCreator interface.
type fakeUserCreator struct {
	result household.CreatedUser
	err    error
}

func (f *fakeUserCreator) Create(_ context.Context, _ household.NewUserInput, _ uuid.UUID) (household.CreatedUser, error) {
	return f.result, f.err
}

var testTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func newMemberLogicEnv() (
	repo *postgres.FakeHouseholdRepository,
	users *fakeUserCreator,
	logic *household.HouseholdMemberLogic,
) {
	repo = postgres.NewFakeHouseholdRepository()
	users = &fakeUserCreator{}
	tx := database.NewFakeTransactor()
	cl := clock.NewFakeClock(testTime)
	logic = household.NewHouseholdMemberLogic(repo, users, tx, cl)
	return repo, users, logic
}

// --- Constructor guard tests ---

func TestNewHouseholdMemberLogic_NilRepo_Panics(t *testing.T) {
	t.Parallel()
	cl := clock.NewFakeClock(testTime)
	tx := database.NewFakeTransactor()
	assert.Panics(t, func() {
		household.NewHouseholdMemberLogic(nil, &fakeUserCreator{}, tx, cl)
	})
}

func TestNewHouseholdMemberLogic_NilUserCreator_Panics(t *testing.T) {
	t.Parallel()
	cl := clock.NewFakeClock(testTime)
	tx := database.NewFakeTransactor()
	assert.Panics(t, func() {
		household.NewHouseholdMemberLogic(postgres.NewFakeHouseholdRepository(), nil, tx, cl)
	})
}

func TestNewHouseholdMemberLogic_NilTransactor_Panics(t *testing.T) {
	t.Parallel()
	cl := clock.NewFakeClock(testTime)
	assert.Panics(t, func() {
		household.NewHouseholdMemberLogic(postgres.NewFakeHouseholdRepository(), &fakeUserCreator{}, nil, cl)
	})
}

func TestNewHouseholdMemberLogic_NilClock_Panics(t *testing.T) {
	t.Parallel()
	tx := database.NewFakeTransactor()
	assert.Panics(t, func() {
		household.NewHouseholdMemberLogic(postgres.NewFakeHouseholdRepository(), &fakeUserCreator{}, tx, nil)
	})
}

// --- CreateHouseholdUser tests ---

// TestCreateHouseholdUser_Success verifies the happy path: a new user is created
// and a MEMBER membership is added. The returned CreatedMember carries the user
// info and the membership with the correct household and user IDs.
func TestCreateHouseholdUser_Success(t *testing.T) {
	t.Parallel()

	repo, users, logic := newMemberLogicEnv()

	callerID := uuid.New()
	householdID := uuid.New()
	repo.AddHousehold(householdID, callerID)

	newUserID := uuid.New()
	users.result = household.CreatedUser{
		ID:          newUserID,
		Email:       "new@example.com",
		DisplayName: "New User",
	}

	input := household.NewUserInput{
		Email:       "new@example.com",
		DisplayName: "New User",
		Password:    "secret1234",
	}

	got, err := logic.CreateHouseholdUser(context.Background(), householdID, input, callerID)

	require.NoError(t, err)
	assert.Equal(t, newUserID, got.User.ID)
	assert.Equal(t, "new@example.com", got.User.Email)
	assert.Equal(t, "New User", got.User.DisplayName)
	assert.Equal(t, householdID, got.Membership.HouseholdID)
	assert.Equal(t, newUserID, got.Membership.UserID)
}

// TestCreateHouseholdUser_EmailTaken verifies that when userCreator returns
// ErrUserEmailTaken the error propagates and no membership is written.
func TestCreateHouseholdUser_EmailTaken(t *testing.T) {
	t.Parallel()

	repo, users, logic := newMemberLogicEnv()
	callerID := uuid.New()
	householdID := uuid.New()
	repo.AddHousehold(householdID, callerID)

	users.err = message.ErrUserEmailTaken

	_, err := logic.CreateHouseholdUser(context.Background(), householdID, household.NewUserInput{
		Email:    "taken@example.com",
		Password: "secret1234",
	}, callerID)

	require.Error(t, err)
	assert.ErrorIs(t, err, message.ErrUserEmailTaken)
}

// TestCreateHouseholdUser_UserCreatorError verifies that a generic user creation
// failure propagates and the operation is aborted.
func TestCreateHouseholdUser_UserCreatorError(t *testing.T) {
	t.Parallel()

	repo, users, logic := newMemberLogicEnv()
	callerID := uuid.New()
	householdID := uuid.New()
	repo.AddHousehold(householdID, callerID)

	users.err = errors.New("db connection lost")

	_, err := logic.CreateHouseholdUser(context.Background(), householdID, household.NewUserInput{
		Email:    "x@example.com",
		Password: "secret1234",
	}, callerID)

	require.Error(t, err)
}

// TestCreateHouseholdUser_MembershipAddFails verifies that when AddMember fails
// (user already a member), the error propagates.
func TestCreateHouseholdUser_MembershipAddFails(t *testing.T) {
	t.Parallel()

	repo, users, logic := newMemberLogicEnv()
	callerID := uuid.New()
	householdID := uuid.New()
	repo.AddHousehold(householdID, callerID)

	newUserID := uuid.New()
	users.result = household.CreatedUser{ID: newUserID}
	// Pre-seed the membership so AddMember returns ErrHouseholdMemberExists.
	repo.AddMemberDirect(householdID, newUserID, callerID)

	_, err := logic.CreateHouseholdUser(context.Background(), householdID, household.NewUserInput{
		Email:    "x@example.com",
		Password: "secret1234",
	}, callerID)

	require.Error(t, err)
	assert.ErrorIs(t, err, message.ErrHouseholdMemberExists)
}

// TestCreateHouseholdUser_Rollback_WhenMembershipFails verifies AC6: when user
// creation succeeds but AddMember fails, the transactor records a rollback (not a
// commit), meaning neither write persists in a real DB transaction.
func TestCreateHouseholdUser_Rollback_WhenMembershipFails(t *testing.T) {
	t.Parallel()

	repo := postgres.NewFakeHouseholdRepository()
	users := &fakeUserCreator{}
	tx := database.NewFakeTransactor()
	cl := clock.NewFakeClock(testTime)
	logic := household.NewHouseholdMemberLogic(repo, users, tx, cl)

	callerID := uuid.New()
	householdID := uuid.New()
	repo.AddHousehold(householdID, callerID)

	newUserID := uuid.New()
	users.result = household.CreatedUser{ID: newUserID}
	// Pre-seed membership to force AddMember to fail after user creation succeeds.
	repo.AddMemberDirect(householdID, newUserID, callerID)

	_, err := logic.CreateHouseholdUser(context.Background(), householdID, household.NewUserInput{
		Email:    "rollback@example.com",
		Password: "secret1234",
	}, callerID)

	require.Error(t, err)
	assert.False(t, tx.CommittedLastCall(), "transaction must not commit when AddMember fails")
}

// TestCreateHouseholdUser_Rollback_WhenUserCreationFails verifies AC7: when user
// creation itself fails, the transactor records a rollback — no partial write occurs.
func TestCreateHouseholdUser_Rollback_WhenUserCreationFails(t *testing.T) {
	t.Parallel()

	repo := postgres.NewFakeHouseholdRepository()
	users := &fakeUserCreator{err: errors.New("db unavailable")}
	tx := database.NewFakeTransactor()
	cl := clock.NewFakeClock(testTime)
	logic := household.NewHouseholdMemberLogic(repo, users, tx, cl)

	callerID := uuid.New()
	householdID := uuid.New()
	repo.AddHousehold(householdID, callerID)

	_, err := logic.CreateHouseholdUser(context.Background(), householdID, household.NewUserInput{
		Email:    "never@example.com",
		Password: "secret1234",
	}, callerID)

	require.Error(t, err)
	assert.False(t, tx.CommittedLastCall(), "transaction must not commit when user creation fails")
}
