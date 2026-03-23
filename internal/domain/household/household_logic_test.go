package household_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zambone/pfm-go/internal/adapter/postgres"
	"github.com/zambone/pfm-go/internal/domain/household"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/clock"
	"github.com/zambone/pfm-go/internal/platform/database"
	"github.com/zambone/pfm-go/internal/platform/validate"
	"github.com/zambone/pfm-go/internal/types"
)

// newHouseholdLogic returns a HouseholdLogic with a fresh fake repo, fake transactor,
// and fake clock. The callerID is returned for convenience.
func newHouseholdLogic() (*household.HouseholdLogic, *postgres.FakeHouseholdRepository, uuid.UUID) {
	repo := postgres.NewFakeHouseholdRepository()
	tx := database.NewFakeTransactor()
	clk := clock.NewFakeClock(fixedTime)
	callerID := testOwnerID
	logic := household.NewHouseholdLogic(repo, tx, clk)
	return logic, repo, callerID
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestHouseholdLogic_Create_HappyPath(t *testing.T) {
	logic, _, callerID := newHouseholdLogic()

	h, err := logic.Create(context.Background(), household.CreateInput{Name: "Home"}, callerID)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, h.ID)
	assert.Equal(t, "Home", h.Name)
	assert.Equal(t, types.StatusActive, h.Status)
	assert.Equal(t, 1, h.Version)
}

func TestHouseholdLogic_Create_EmptyName_FailsValidation(t *testing.T) {
	logic, _, callerID := newHouseholdLogic()

	_, err := logic.Create(context.Background(), household.CreateInput{Name: ""}, callerID)

	require.Error(t, err)
	var ve *validate.ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %v", err)
}

func TestHouseholdLogic_Create_CallerBecomesAdmin(t *testing.T) {
	logic, repo, callerID := newHouseholdLogic()

	h, err := logic.Create(context.Background(), household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	m, err := repo.FindMembership(context.Background(), h.ID, callerID)
	require.NoError(t, err)
	assert.Equal(t, types.RoleAdmin, m.Role)
}

// ---------------------------------------------------------------------------
// FindByID
// ---------------------------------------------------------------------------

func TestHouseholdLogic_FindByID_HappyPath(t *testing.T) {
	logic, _, callerID := newHouseholdLogic()

	created, err := logic.Create(context.Background(), household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	found, err := logic.FindByID(context.Background(), created.ID)

	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
}

func TestHouseholdLogic_FindByID_NotFound(t *testing.T) {
	logic, _, _ := newHouseholdLogic()

	_, err := logic.FindByID(context.Background(), uuid.New())

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrHouseholdNotFound))
}

// ---------------------------------------------------------------------------
// ListForUser
// ---------------------------------------------------------------------------

func TestHouseholdLogic_ListForUser_ReturnsOnlyCallerHouseholds(t *testing.T) {
	logic, _, callerID := newHouseholdLogic()

	_, err := logic.Create(context.Background(), household.CreateInput{Name: "H1"}, callerID)
	require.NoError(t, err)
	_, err = logic.Create(context.Background(), household.CreateInput{Name: "H2"}, callerID)
	require.NoError(t, err)

	list, err := logic.ListForUser(context.Background(), callerID)

	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestHouseholdLogic_ListForUser_EmptyForUnknownUser(t *testing.T) {
	logic, _, callerID := newHouseholdLogic()

	_, err := logic.Create(context.Background(), household.CreateInput{Name: "H1"}, callerID)
	require.NoError(t, err)

	list, err := logic.ListForUser(context.Background(), uuid.New())

	require.NoError(t, err)
	assert.Empty(t, list)
}

// ---------------------------------------------------------------------------
// AddMember
// ---------------------------------------------------------------------------

func TestHouseholdLogic_AddMember_AdminCanAdd(t *testing.T) {
	logic, _, callerID := newHouseholdLogic()
	newMember := uuid.New()

	h, err := logic.Create(context.Background(), household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	m, err := logic.AddMember(context.Background(), h.ID, household.AddMemberInput{
		UserID: newMember,
		Role:   types.RoleMember,
	}, callerID)

	require.NoError(t, err)
	assert.Equal(t, newMember, m.UserID)
	assert.Equal(t, types.RoleMember, m.Role)
}

func TestHouseholdLogic_AddMember_NonAdminDenied(t *testing.T) {
	logic, repo, callerID := newHouseholdLogic()
	memberUser := uuid.New()
	anotherUser := uuid.New()

	h, err := logic.Create(context.Background(), household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	// Add memberUser as MEMBER (not admin).
	_, err = repo.AddMember(context.Background(), h.ID, household.AddMemberInput{
		UserID: memberUser,
		Role:   types.RoleMember,
	}, callerID)
	require.NoError(t, err)

	// memberUser tries to add anotherUser — should be denied.
	_, err = logic.AddMember(context.Background(), h.ID, household.AddMemberInput{
		UserID: anotherUser,
		Role:   types.RoleMember,
	}, memberUser)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrHouseholdNotAdmin))
}

func TestHouseholdLogic_AddMember_DuplicateReturnsError(t *testing.T) {
	logic, _, callerID := newHouseholdLogic()
	newMember := uuid.New()

	h, err := logic.Create(context.Background(), household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	_, err = logic.AddMember(context.Background(), h.ID, household.AddMemberInput{
		UserID: newMember,
		Role:   types.RoleMember,
	}, callerID)
	require.NoError(t, err)

	_, err = logic.AddMember(context.Background(), h.ID, household.AddMemberInput{
		UserID: newMember,
		Role:   types.RoleMember,
	}, callerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrHouseholdMemberExists))
}

// ---------------------------------------------------------------------------
// RemoveMember
// ---------------------------------------------------------------------------

func TestHouseholdLogic_RemoveMember_AdminCanRemove(t *testing.T) {
	logic, _, callerID := newHouseholdLogic()
	member := uuid.New()

	h, err := logic.Create(context.Background(), household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)
	_, err = logic.AddMember(context.Background(), h.ID, household.AddMemberInput{
		UserID: member,
		Role:   types.RoleMember,
	}, callerID)
	require.NoError(t, err)

	err = logic.RemoveMember(context.Background(), h.ID, member, callerID)
	require.NoError(t, err)

	list, err := logic.ListForUser(context.Background(), member)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestHouseholdLogic_RemoveMember_NonAdminDenied(t *testing.T) {
	logic, repo, callerID := newHouseholdLogic()
	memberUser := uuid.New()
	anotherMember := uuid.New()

	h, err := logic.Create(context.Background(), household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)
	_, err = repo.AddMember(context.Background(), h.ID, household.AddMemberInput{
		UserID: memberUser, Role: types.RoleMember,
	}, callerID)
	require.NoError(t, err)
	_, err = repo.AddMember(context.Background(), h.ID, household.AddMemberInput{
		UserID: anotherMember, Role: types.RoleMember,
	}, callerID)
	require.NoError(t, err)

	err = logic.RemoveMember(context.Background(), h.ID, anotherMember, memberUser)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrHouseholdNotAdmin))
}

func TestHouseholdLogic_RemoveMember_LastAdminDenied(t *testing.T) {
	logic, _, callerID := newHouseholdLogic()

	h, err := logic.Create(context.Background(), household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	err = logic.RemoveMember(context.Background(), h.ID, callerID, callerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrHouseholdLastAdmin))
}

// ---------------------------------------------------------------------------
// UpdateName
// ---------------------------------------------------------------------------

func TestHouseholdLogic_UpdateName_HappyPath(t *testing.T) {
	logic, _, callerID := newHouseholdLogic()

	created, err := logic.Create(context.Background(), household.CreateInput{Name: "Old"}, callerID)
	require.NoError(t, err)

	updated, err := logic.UpdateName(context.Background(), created.ID,
		household.UpdateNameInput{Name: "New"}, created.Version, callerID)

	require.NoError(t, err)
	assert.Equal(t, "New", updated.Name)
	assert.Equal(t, created.Version+1, updated.Version)
}

func TestHouseholdLogic_UpdateName_EmptyName_FailsValidation(t *testing.T) {
	logic, _, callerID := newHouseholdLogic()

	created, err := logic.Create(context.Background(), household.CreateInput{Name: "Old"}, callerID)
	require.NoError(t, err)

	_, err = logic.UpdateName(context.Background(), created.ID,
		household.UpdateNameInput{Name: ""}, created.Version, callerID)

	require.Error(t, err)
	var ve *validate.ValidationError
	assert.True(t, errors.As(err, &ve))
}

func TestHouseholdLogic_UpdateName_VersionConflict(t *testing.T) {
	logic, _, callerID := newHouseholdLogic()

	created, err := logic.Create(context.Background(), household.CreateInput{Name: "Old"}, callerID)
	require.NoError(t, err)

	_, err = logic.UpdateName(context.Background(), created.ID,
		household.UpdateNameInput{Name: "New"}, created.Version+99, callerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrHouseholdVersionConflict))
}

// ---------------------------------------------------------------------------
// Deactivate
// ---------------------------------------------------------------------------

func TestHouseholdLogic_Deactivate_HappyPath(t *testing.T) {
	logic, _, callerID := newHouseholdLogic()

	created, err := logic.Create(context.Background(), household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	err = logic.Deactivate(context.Background(), created.ID, callerID)
	require.NoError(t, err)

	_, err = logic.FindByID(context.Background(), created.ID)
	assert.True(t, errors.Is(err, message.ErrHouseholdNotFound))
}

func TestHouseholdLogic_Deactivate_IsIdempotent(t *testing.T) {
	logic, _, callerID := newHouseholdLogic()

	created, err := logic.Create(context.Background(), household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	require.NoError(t, logic.Deactivate(context.Background(), created.ID, callerID))
	assert.NoError(t, logic.Deactivate(context.Background(), created.ID, callerID))
}
