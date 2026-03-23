package household_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/zambone/pfm-go/internal/domain/household"
)

var (
	fixedTime       = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	testHouseholdID = uuid.MustParse("00000000-0000-0000-0000-000000000010")
	testOwnerID     = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	testMemberID    = uuid.MustParse("00000000-0000-0000-0000-000000000002")
)

// householdFactory returns a Household with all required fields set to non-zero defaults.
// Individual tests override only the fields relevant to their scenario:
//
//	h := householdFactory(func(h *household.Household) { h.Name = "Custom" })
func householdFactory(overrides ...func(*household.Household)) household.Household {
	h := household.Household{
		ID:        testHouseholdID,
		Name:      "Test Household",
		Status:    household.StatusActive,
		Version:   1,
		CreatedAt: fixedTime,
		UpdatedAt: fixedTime,
		CreatedBy: testOwnerID,
		UpdatedBy: testOwnerID,
	}
	for _, o := range overrides {
		o(&h)
	}
	return h
}

// membershipFactory returns a Membership with sensible defaults (RoleAdmin for the owner).
func membershipFactory(overrides ...func(*household.Membership)) household.Membership {
	m := household.Membership{
		HouseholdID: testHouseholdID,
		UserID:      testOwnerID,
		Role:        household.RoleAdmin,
		InvitedBy:   uuid.Nil,
		JoinedAt:    fixedTime,
	}
	for _, o := range overrides {
		o(&m)
	}
	return m
}

// createInputFactory returns a valid CreateInput with sensible defaults.
func createInputFactory(overrides ...func(*household.CreateInput)) household.CreateInput {
	in := household.CreateInput{
		Name: "New Household",
	}
	for _, o := range overrides {
		o(&in)
	}
	return in
}

// updateNameInputFactory returns a valid UpdateNameInput with sensible defaults.
func updateNameInputFactory(overrides ...func(*household.UpdateNameInput)) household.UpdateNameInput {
	in := household.UpdateNameInput{
		Name: "Updated Household",
	}
	for _, o := range overrides {
		o(&in)
	}
	return in
}

// addMemberInputFactory returns a valid AddMemberInput with sensible defaults.
func addMemberInputFactory(overrides ...func(*household.AddMemberInput)) household.AddMemberInput {
	in := household.AddMemberInput{
		UserID: testMemberID,
		Role:   household.RoleMember,
	}
	for _, o := range overrides {
		o(&in)
	}
	return in
}

// TestFactories_ProduceValidDefaults verifies that each factory returns fully
// populated values when called with no overrides.
func TestFactories_ProduceValidDefaults(t *testing.T) {
	t.Run("householdFactory has non-zero fields", func(t *testing.T) {
		h := householdFactory()

		assert.NotEqual(t, uuid.Nil, h.ID)
		assert.NotEmpty(t, h.Name)
		assert.Equal(t, household.StatusActive, h.Status)
		assert.Equal(t, 1, h.Version)
		assert.False(t, h.CreatedAt.IsZero())
		assert.False(t, h.UpdatedAt.IsZero())
		assert.NotEqual(t, uuid.Nil, h.CreatedBy)
		assert.NotEqual(t, uuid.Nil, h.UpdatedBy)
	})

	t.Run("membershipFactory defaults to RoleAdmin", func(t *testing.T) {
		m := membershipFactory()

		assert.NotEqual(t, uuid.Nil, m.HouseholdID)
		assert.NotEqual(t, uuid.Nil, m.UserID)
		assert.Equal(t, household.RoleAdmin, m.Role)
		assert.False(t, m.JoinedAt.IsZero())
	})

	t.Run("createInputFactory has non-empty name", func(t *testing.T) {
		in := createInputFactory()

		assert.NotEmpty(t, in.Name)
	})

	t.Run("householdFactory override applies", func(t *testing.T) {
		h := householdFactory(func(h *household.Household) { h.Name = "Custom" })

		assert.Equal(t, "Custom", h.Name)
	})

	t.Run("membershipFactory override applies", func(t *testing.T) {
		m := membershipFactory(func(m *household.Membership) { m.Role = household.RoleMember })

		assert.Equal(t, household.RoleMember, m.Role)
	})

	t.Run("updateNameInputFactory has non-empty name", func(t *testing.T) {
		in := updateNameInputFactory()

		assert.NotEmpty(t, in.Name)
	})

	t.Run("addMemberInputFactory defaults to RoleMember", func(t *testing.T) {
		in := addMemberInputFactory()

		assert.NotEqual(t, uuid.Nil, in.UserID)
		assert.Equal(t, household.RoleMember, in.Role)
	})

	t.Run("householdFactory fixedTime is 2026-01-01", func(t *testing.T) {
		h := householdFactory()

		assert.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), h.CreatedAt)
	})
}
