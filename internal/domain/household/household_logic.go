package household

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/validate"
	"github.com/zambone/pfm-go/internal/types"
)

// transactor abstracts atomic multi-table operations.
// Structurally satisfied by platform/database.Transactor.
type transactor interface {
	RunAtomic(ctx context.Context, fn func(ctx context.Context) error) error
}

// clocker abstracts the current time.
// Structurally satisfied by platform/clock.Clock.
type clocker interface {
	Now() time.Time
}

// HouseholdLogic orchestrates the household lifecycle: creation, membership management,
// name updates, and deactivation.
type HouseholdLogic struct {
	repo Repository
	tx   transactor
	clk  clocker
}

// NewHouseholdLogic constructs a HouseholdLogic. Panics if any dependency is nil.
func NewHouseholdLogic(repo Repository, tx transactor, clk clocker) *HouseholdLogic {
	if repo == nil {
		panic("household: NewHouseholdLogic requires non-nil repo")
	}
	if tx == nil {
		panic("household: NewHouseholdLogic requires non-nil tx")
	}
	if clk == nil {
		panic("household: NewHouseholdLogic requires non-nil clk")
	}
	return &HouseholdLogic{repo: repo, tx: tx, clk: clk}
}

// Create validates input and persists a new household with the caller as its founding ADMIN
// member. Both the household and the membership are created atomically via the Transactor.
func (l *HouseholdLogic) Create(ctx context.Context, input CreateInput, callerID uuid.UUID) (Household, error) {
	r := validate.NewResult()
	r.Field("name", input.Name, validate.Required, validate.MinLen(2), validate.MaxLen(100))
	if err := r.Error(); err != nil {
		return Household{}, err
	}

	var created Household
	if err := l.tx.RunAtomic(ctx, func(txCtx context.Context) error {
		h, err := l.repo.Create(txCtx, input, callerID)
		if err != nil {
			return err
		}
		created = h
		return nil
	}); err != nil {
		return Household{}, fmt.Errorf(message.ErrHouseholdLogicCreate, err)
	}
	return created, nil
}

// FindByID returns the active household with the given ID.
func (l *HouseholdLogic) FindByID(ctx context.Context, id uuid.UUID) (Household, error) {
	h, err := l.repo.FindByID(ctx, id)
	if err != nil {
		return Household{}, fmt.Errorf(message.ErrHouseholdLogicFindByID, err)
	}
	return h, nil
}

// ListForUser returns all active households where the user has an active membership.
func (l *HouseholdLogic) ListForUser(ctx context.Context, userID uuid.UUID) ([]Household, error) {
	list, err := l.repo.ListForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf(message.ErrHouseholdLogicListForUser, err)
	}
	return list, nil
}

// AddMember validates the caller is an ADMIN of the household, then adds the new member.
func (l *HouseholdLogic) AddMember(ctx context.Context, householdID uuid.UUID, input AddMemberInput, callerID uuid.UUID) (Membership, error) {
	callerMembership, err := l.repo.FindMembership(ctx, householdID, callerID)
	if err != nil {
		if errors.Is(err, message.ErrHouseholdMemberNotFound) {
			return Membership{}, fmt.Errorf(message.ErrHouseholdLogicAddMember, message.ErrHouseholdNotAdmin)
		}
		return Membership{}, fmt.Errorf(message.ErrHouseholdLogicAddMember, err)
	}
	if callerMembership.Role != types.RoleAdmin {
		return Membership{}, fmt.Errorf(message.ErrHouseholdLogicAddMember, message.ErrHouseholdNotAdmin)
	}

	m, err := l.repo.AddMember(ctx, householdID, input, callerID)
	if err != nil {
		return Membership{}, fmt.Errorf(message.ErrHouseholdLogicAddMember, err)
	}
	return m, nil
}

// RemoveMember validates the caller is an ADMIN and that removing the target would not
// leave the household without any admins, then soft-deletes the membership.
func (l *HouseholdLogic) RemoveMember(ctx context.Context, householdID uuid.UUID, userID uuid.UUID, callerID uuid.UUID) error {
	callerMembership, err := l.repo.FindMembership(ctx, householdID, callerID)
	if err != nil {
		if errors.Is(err, message.ErrHouseholdMemberNotFound) {
			return fmt.Errorf(message.ErrHouseholdLogicRemoveMember, message.ErrHouseholdNotAdmin)
		}
		return fmt.Errorf(message.ErrHouseholdLogicRemoveMember, err)
	}
	if callerMembership.Role != types.RoleAdmin {
		return fmt.Errorf(message.ErrHouseholdLogicRemoveMember, message.ErrHouseholdNotAdmin)
	}

	// Check if removing the target would leave the household without any admins.
	members, err := l.repo.ListMembers(ctx, householdID)
	if err != nil {
		return fmt.Errorf(message.ErrHouseholdLogicRemoveMember, err)
	}
	adminCount := 0
	for _, m := range members {
		if m.Role == types.RoleAdmin {
			adminCount++
		}
	}

	// Look up the target's role to see if they're an admin.
	targetMembership, err := l.repo.FindMembership(ctx, householdID, userID)
	if err != nil {
		if errors.Is(err, message.ErrHouseholdMemberNotFound) {
			return nil // already removed — idempotent
		}
		return fmt.Errorf(message.ErrHouseholdLogicRemoveMember, err)
	}
	if targetMembership.Role == types.RoleAdmin && adminCount <= 1 {
		return fmt.Errorf(message.ErrHouseholdLogicRemoveMember, message.ErrHouseholdLastAdmin)
	}

	if err := l.repo.RemoveMember(ctx, householdID, userID, callerID); err != nil {
		return fmt.Errorf(message.ErrHouseholdLogicRemoveMember, err)
	}
	return nil
}

// UpdateName validates input and updates the household name with optimistic concurrency.
func (l *HouseholdLogic) UpdateName(ctx context.Context, id uuid.UUID, input UpdateNameInput, expectedVersion int, callerID uuid.UUID) (Household, error) {
	r := validate.NewResult()
	r.Field("name", input.Name, validate.Required, validate.MinLen(2), validate.MaxLen(100))
	if err := r.Error(); err != nil {
		return Household{}, err
	}

	h, err := l.repo.UpdateName(ctx, id, input, expectedVersion, callerID)
	if err != nil {
		return Household{}, fmt.Errorf(message.ErrHouseholdLogicUpdateName, err)
	}
	return h, nil
}

// Deactivate soft-deletes the household.
func (l *HouseholdLogic) Deactivate(ctx context.Context, id uuid.UUID, callerID uuid.UUID) error {
	if err := l.repo.Deactivate(ctx, id, callerID); err != nil {
		return fmt.Errorf(message.ErrHouseholdLogicDeactivate, err)
	}
	return nil
}
