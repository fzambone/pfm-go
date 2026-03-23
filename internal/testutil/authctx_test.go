package testutil_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/zambone/pfm-go/internal/platform/ctxutil"
	"github.com/zambone/pfm-go/internal/testutil"
	"github.com/zambone/pfm-go/internal/types"
)

func TestAuthenticatedContext_HasUserID(t *testing.T) {
	userID := uuid.New()

	ctx := testutil.AuthenticatedContext(userID)

	got, ok := ctxutil.UserID(ctx)
	assert.True(t, ok)
	assert.Equal(t, userID, got)
}

func TestAuthenticatedContext_NoHouseholdID(t *testing.T) {
	ctx := testutil.AuthenticatedContext(uuid.New())

	_, ok := ctxutil.HouseholdID(ctx)
	assert.False(t, ok, "authenticated-only context should not have household ID")
}

func TestAuthenticatedContext_NoRole(t *testing.T) {
	ctx := testutil.AuthenticatedContext(uuid.New())

	_, ok := ctxutil.Role(ctx)
	assert.False(t, ok, "authenticated-only context should not have role")
}

func TestAuthorizedContext_HasAllValues(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()
	role := types.RoleAdmin

	ctx := testutil.AuthorizedContext(userID, householdID, role)

	gotUser, ok := ctxutil.UserID(ctx)
	assert.True(t, ok)
	assert.Equal(t, userID, gotUser)

	gotHousehold, ok := ctxutil.HouseholdID(ctx)
	assert.True(t, ok)
	assert.Equal(t, householdID, gotHousehold)

	gotRole, ok := ctxutil.Role(ctx)
	assert.True(t, ok)
	assert.Equal(t, role, gotRole)
}

func TestAuthorizedContext_MemberRole(t *testing.T) {
	ctx := testutil.AuthorizedContext(uuid.New(), uuid.New(), types.RoleMember)

	gotRole, ok := ctxutil.Role(ctx)
	assert.True(t, ok)
	assert.Equal(t, types.RoleMember, gotRole)
}
