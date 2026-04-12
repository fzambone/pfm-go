package household

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/types"
)

// HouseholdMemberLogic handles atomic user-creation-within-a-household operations.
// It is separate from HouseholdLogic so that its constructor can accept a
// userCreator without altering the existing HouseholdLogic constructor signature.
type HouseholdMemberLogic struct {
	repo  Repository
	users userCreator
	tx    transactor
	cl    clocker
}

// NewHouseholdMemberLogic creates a HouseholdMemberLogic. Panics if any dependency is nil.
func NewHouseholdMemberLogic(repo Repository, users userCreator, tx transactor, cl clocker) *HouseholdMemberLogic {
	if repo == nil {
		panic("household: NewHouseholdMemberLogic requires non-nil Repository")
	}
	if users == nil {
		panic("household: NewHouseholdMemberLogic requires non-nil userCreator")
	}
	if tx == nil {
		panic("household: NewHouseholdMemberLogic requires non-nil transactor")
	}
	if cl == nil {
		panic("household: NewHouseholdMemberLogic requires non-nil clocker")
	}
	return &HouseholdMemberLogic{repo: repo, users: users, tx: tx, cl: cl}
}

// CreateHouseholdUser creates a new user and immediately adds them as a MEMBER of
// the given household. Both writes happen inside a single database transaction —
// if either fails, both are rolled back.
//
// The callerID must be an admin of the target household; this is enforced by the
// adminGuard middleware before the request reaches this method.
func (l *HouseholdMemberLogic) CreateHouseholdUser(ctx context.Context, householdID uuid.UUID, input NewUserInput, callerID uuid.UUID) (CreatedMember, error) {
	var result CreatedMember

	if err := l.tx.RunAtomic(ctx, func(txCtx context.Context) error {
		// Step 1 — create the user (validation + hashing happen inside the adapter).
		created, err := l.users.Create(txCtx, input, callerID)
		if err != nil {
			return err
		}

		// Step 2 — add the new user as a MEMBER of the household.
		membership, err := l.repo.AddMember(txCtx, householdID, AddMemberInput{
			UserID: created.ID,
			Role:   types.RoleMember,
		}, callerID)
		if err != nil {
			return err
		}

		result = CreatedMember{User: created, Membership: membership}
		return nil
	}); err != nil {
		return CreatedMember{}, fmt.Errorf(message.ErrHouseholdLogicCreateUser, err)
	}
	return result, nil
}
