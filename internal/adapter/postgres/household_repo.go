package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	"github.com/zambone/pfm-go/internal/domain/household"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/database"
)

// HouseholdRepo implements domain/household.Repository using PostgreSQL via sqlc-generated queries.
type HouseholdRepo struct {
	pool *pgxpool.Pool
}

// NewHouseholdRepo creates a HouseholdRepo backed by pool.
// Panics if pool is nil to catch misconfigured wiring at startup.
func NewHouseholdRepo(pool *pgxpool.Pool) *HouseholdRepo {
	if pool == nil {
		panic("postgres: NewHouseholdRepo requires non-nil pool")
	}
	return &HouseholdRepo{pool: pool}
}

// householdFromRow maps a sqlc-generated household row to the domain entity.
func householdFromRow(id uuid.UUID, name, status string, version int32, createdAt, updatedAt pgtype.Timestamptz, createdBy, updatedBy pgtype.UUID) household.Household {
	return household.Household{
		ID:        id,
		Name:      name,
		Status:    household.Status(status),
		Version:   int(version),
		CreatedAt: pgTimestamptzToTime(createdAt),
		UpdatedAt: pgTimestamptzToTime(updatedAt),
		CreatedBy: pgUUIDToUUID(createdBy),
		UpdatedBy: pgUUIDToUUID(updatedBy),
	}
}

// membershipFromRow maps a sqlc-generated membership row to the domain entity.
func membershipFromRow(householdID, userID uuid.UUID, role string, invitedBy pgtype.UUID, joinedAt pgtype.Timestamptz) household.Membership {
	return household.Membership{
		HouseholdID: householdID,
		UserID:      userID,
		Role:        household.Role(role),
		InvitedBy:   pgUUIDToUUID(invitedBy),
		JoinedAt:    pgTimestamptzToTime(joinedAt),
	}
}

// Create inserts a new household and its founding admin membership within the current
// database connection (pool or transaction via DBTXFromContext).
// Returns an error wrapping the underlying cause on failure.
func (r *HouseholdRepo) Create(ctx context.Context, input household.CreateInput, callerID uuid.UUID) (household.Household, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "HouseholdRepo.Create")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	q := New(db)

	row, err := q.CreateHousehold(ctx, CreateHouseholdParams{
		Name:      input.Name,
		CreatedBy: uuidToPgUUID(callerID),
		UpdatedBy: uuidToPgUUID(callerID),
	})
	if err != nil {
		err = fmt.Errorf(message.ErrHouseholdCreate, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return household.Household{}, err
	}

	_, err = q.CreateHouseholdMember(ctx, CreateHouseholdMemberParams{
		HouseholdID: row.ID,
		UserID:      callerID,
		Role:        string(household.RoleAdmin),
		InvitedBy:   uuidToPgUUID(uuid.Nil),
	})
	if err != nil {
		err = fmt.Errorf(message.ErrHouseholdCreate, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return household.Household{}, err
	}

	span.SetStatus(codes.Ok, "")
	return householdFromRow(row.ID, row.Name, row.Status, row.Version, row.CreatedAt, row.UpdatedAt, row.CreatedBy, row.UpdatedBy), nil
}

// FindByID returns the active (non-deleted) household with the given ID.
// Returns an error wrapping ErrHouseholdNotFound when no matching active household exists.
func (r *HouseholdRepo) FindByID(ctx context.Context, id uuid.UUID) (household.Household, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "HouseholdRepo.FindByID")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	row, err := New(db).FindHouseholdByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = fmt.Errorf(message.ErrHouseholdFindByID, message.ErrHouseholdNotFound)
		} else {
			err = fmt.Errorf(message.ErrHouseholdFindByID, err)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return household.Household{}, err
	}

	span.SetStatus(codes.Ok, "")
	return householdFromRow(row.ID, row.Name, row.Status, row.Version, row.CreatedAt, row.UpdatedAt, row.CreatedBy, row.UpdatedBy), nil
}

// ListForUser returns all active households where the given user has an active membership.
func (r *HouseholdRepo) ListForUser(ctx context.Context, userID uuid.UUID) ([]household.Household, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "HouseholdRepo.ListForUser")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	rows, err := New(db).ListHouseholdsForUser(ctx, userID)
	if err != nil {
		err = fmt.Errorf(message.ErrHouseholdListForUser, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	result := make([]household.Household, len(rows))
	for i, row := range rows {
		result[i] = householdFromRow(row.ID, row.Name, row.Status, row.Version, row.CreatedAt, row.UpdatedAt, row.CreatedBy, row.UpdatedBy)
	}

	span.SetStatus(codes.Ok, "")
	return result, nil
}

// FindMembership returns the active membership for a user in a household.
// Returns an error wrapping ErrHouseholdMemberNotFound when no active membership exists.
func (r *HouseholdRepo) FindMembership(ctx context.Context, householdID uuid.UUID, userID uuid.UUID) (household.Membership, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "HouseholdRepo.FindMembership")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	row, err := New(db).FindMembership(ctx, FindMembershipParams{
		HouseholdID: householdID,
		UserID:      userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = fmt.Errorf(message.ErrHouseholdFindMembership, message.ErrHouseholdMemberNotFound)
		} else {
			err = fmt.Errorf(message.ErrHouseholdFindMembership, err)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return household.Membership{}, err
	}

	span.SetStatus(codes.Ok, "")
	return membershipFromRow(row.HouseholdID, row.UserID, row.Role, row.InvitedBy, row.JoinedAt), nil
}

// ListMembers returns all active memberships for the given household.
func (r *HouseholdRepo) ListMembers(ctx context.Context, householdID uuid.UUID) ([]household.Membership, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "HouseholdRepo.ListMembers")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	rows, err := New(db).ListMembers(ctx, householdID)
	if err != nil {
		err = fmt.Errorf(message.ErrHouseholdListMembers, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	result := make([]household.Membership, len(rows))
	for i, row := range rows {
		result[i] = membershipFromRow(row.HouseholdID, row.UserID, row.Role, row.InvitedBy, row.JoinedAt)
	}

	span.SetStatus(codes.Ok, "")
	return result, nil
}

// AddMember adds a new membership to the household.
// Returns an error wrapping ErrHouseholdMemberExists if the user already has an active membership.
func (r *HouseholdRepo) AddMember(ctx context.Context, householdID uuid.UUID, input household.AddMemberInput, callerID uuid.UUID) (household.Membership, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "HouseholdRepo.AddMember")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	row, err := New(db).CreateHouseholdMember(ctx, CreateHouseholdMemberParams{
		HouseholdID: householdID,
		UserID:      input.UserID,
		Role:        string(input.Role),
		InvitedBy:   uuidToPgUUID(callerID),
	})
	if err != nil {
		if isUniqueViolation(err, "household_members_pkey") {
			err = fmt.Errorf(message.ErrHouseholdAddMember, message.ErrHouseholdMemberExists)
		} else {
			err = fmt.Errorf(message.ErrHouseholdAddMember, err)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return household.Membership{}, err
	}

	span.SetStatus(codes.Ok, "")
	return membershipFromRow(row.HouseholdID, row.UserID, row.Role, row.InvitedBy, row.JoinedAt), nil
}

// RemoveMember soft-deletes the membership for the given user in the household.
// Idempotent — removing an already-removed or non-existent membership is not an error.
func (r *HouseholdRepo) RemoveMember(ctx context.Context, householdID uuid.UUID, userID uuid.UUID, callerID uuid.UUID) error {
	ctx, span := otel.Tracer("postgres").Start(ctx, "HouseholdRepo.RemoveMember")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	if err := New(db).RemoveHouseholdMember(ctx, RemoveHouseholdMemberParams{
		HouseholdID: householdID,
		UserID:      userID,
	}); err != nil {
		err = fmt.Errorf(message.ErrHouseholdRemoveMember, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

// UpdateName changes the name of the household.
// Returns an error wrapping ErrHouseholdVersionConflict when expectedVersion does not match.
func (r *HouseholdRepo) UpdateName(ctx context.Context, id uuid.UUID, input household.UpdateNameInput, expectedVersion int, callerID uuid.UUID) (household.Household, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "HouseholdRepo.UpdateName")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	row, err := New(db).UpdateHouseholdName(ctx, UpdateHouseholdNameParams{
		ID:              id,
		Name:            input.Name,
		UpdatedBy:       uuidToPgUUID(callerID),
		ExpectedVersion: int32(expectedVersion),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = fmt.Errorf(message.ErrHouseholdUpdateName, message.ErrHouseholdVersionConflict)
		} else {
			err = fmt.Errorf(message.ErrHouseholdUpdateName, err)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return household.Household{}, err
	}

	span.SetStatus(codes.Ok, "")
	return householdFromRow(row.ID, row.Name, row.Status, row.Version, row.CreatedAt, row.UpdatedAt, row.CreatedBy, row.UpdatedBy), nil
}

// Deactivate soft-deletes the household.
// Idempotent — deactivating an already-deactivated household is not an error.
func (r *HouseholdRepo) Deactivate(ctx context.Context, id uuid.UUID, callerID uuid.UUID) error {
	ctx, span := otel.Tracer("postgres").Start(ctx, "HouseholdRepo.Deactivate")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	if err := New(db).DeactivateHousehold(ctx, DeactivateHouseholdParams{
		ID:        id,
		UpdatedBy: uuidToPgUUID(callerID),
	}); err != nil {
		err = fmt.Errorf(message.ErrHouseholdDeactivate, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}
