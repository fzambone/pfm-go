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

	"github.com/zambone/pfm-go/internal/domain/account"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/database"
	"github.com/zambone/pfm-go/internal/types"
)

// AccountRepo implements domain/account.Repository using PostgreSQL via sqlc-generated queries.
type AccountRepo struct {
	pool *pgxpool.Pool
}

// NewAccountRepo creates an AccountRepo backed by pool.
// Panics if pool is nil to catch misconfigured wiring at startup.
func NewAccountRepo(pool *pgxpool.Pool) *AccountRepo {
	if pool == nil {
		panic("postgres: NewAccountRepo requires non-nil pool")
	}
	return &AccountRepo{pool: pool}
}

// accountFromRow maps common account row fields to the domain entity.
func accountFromRow(
	id, householdID uuid.UUID,
	name, accountType, currencyCode string,
	balance int64,
	status string,
	version int32,
	createdAt, updatedAt pgtype.Timestamptz,
	createdBy, updatedBy pgtype.UUID,
) account.Account {
	return account.Account{
		ID:           id,
		HouseholdID:  householdID,
		Name:         name,
		AccountType:  types.AccountType(accountType),
		CurrencyCode: types.CurrencyCode(currencyCode),
		Balance:      balance,
		Status:       types.Status(status),
		Version:      int(version),
		CreatedAt:    pgTimestamptzToTime(createdAt),
		UpdatedAt:    pgTimestamptzToTime(updatedAt),
		CreatedBy:    pgUUIDToUUID(createdBy),
		UpdatedBy:    pgUUIDToUUID(updatedBy),
	}
}

// Create inserts a new account with zero balance.
// Returns an error wrapping ErrAccountNameTaken when the name conflicts.
func (r *AccountRepo) Create(ctx context.Context, householdID uuid.UUID, input account.CreateInput, callerID uuid.UUID) (account.Account, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "AccountRepo.Create")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	row, err := New(db).CreateAccount(ctx, CreateAccountParams{
		HouseholdID:  householdID,
		Name:         input.Name,
		AccountType:  string(input.AccountType),
		CurrencyCode: string(input.CurrencyCode),
		CreatedBy:    uuidToPgUUID(callerID),
		UpdatedBy:    uuidToPgUUID(callerID),
	})
	if err != nil {
		if isUniqueViolation(err, "accounts_name_unique_active") {
			err = fmt.Errorf(message.ErrAccountCreate, message.ErrAccountNameTaken)
		} else {
			err = fmt.Errorf(message.ErrAccountCreate, err)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return account.Account{}, err
	}

	span.SetStatus(codes.Ok, "")
	return accountFromRow(row.ID, row.HouseholdID, row.Name, row.AccountType, row.CurrencyCode,
		row.Balance, row.Status, row.Version, row.CreatedAt, row.UpdatedAt, row.CreatedBy, row.UpdatedBy), nil
}

// FindByID returns the active (non-deleted) account with the given ID.
// Returns an error wrapping ErrAccountNotFound when no matching active account exists.
func (r *AccountRepo) FindByID(ctx context.Context, id uuid.UUID) (account.Account, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "AccountRepo.FindByID")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	row, err := New(db).FindAccountByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = fmt.Errorf(message.ErrAccountFindByID, message.ErrAccountNotFound)
		} else {
			err = fmt.Errorf(message.ErrAccountFindByID, err)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return account.Account{}, err
	}

	span.SetStatus(codes.Ok, "")
	return accountFromRow(row.ID, row.HouseholdID, row.Name, row.AccountType, row.CurrencyCode,
		row.Balance, row.Status, row.Version, row.CreatedAt, row.UpdatedAt, row.CreatedBy, row.UpdatedBy), nil
}

// ListForHousehold returns all active accounts belonging to the given household.
func (r *AccountRepo) ListForHousehold(ctx context.Context, householdID uuid.UUID) ([]account.Account, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "AccountRepo.ListForHousehold")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	rows, err := New(db).ListAccountsForHousehold(ctx, householdID)
	if err != nil {
		err = fmt.Errorf(message.ErrAccountListForHouse, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	result := make([]account.Account, len(rows))
	for i, row := range rows {
		result[i] = accountFromRow(row.ID, row.HouseholdID, row.Name, row.AccountType, row.CurrencyCode,
			row.Balance, row.Status, row.Version, row.CreatedAt, row.UpdatedAt, row.CreatedBy, row.UpdatedBy)
	}

	span.SetStatus(codes.Ok, "")
	return result, nil
}

// UpdateName changes the name of the account.
// Returns an error wrapping ErrAccountVersionConflict when expectedVersion does not match.
// Returns an error wrapping ErrAccountNameTaken when the new name conflicts.
func (r *AccountRepo) UpdateName(ctx context.Context, id uuid.UUID, input account.UpdateNameInput, expectedVersion int, callerID uuid.UUID) (account.Account, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "AccountRepo.UpdateName")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	row, err := New(db).UpdateAccountName(ctx, UpdateAccountNameParams{
		ID:              id,
		Name:            input.Name,
		UpdatedBy:       uuidToPgUUID(callerID),
		ExpectedVersion: int32(expectedVersion),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = fmt.Errorf(message.ErrAccountUpdateName, message.ErrAccountVersionConflict)
		} else if isUniqueViolation(err, "accounts_name_unique_active") {
			err = fmt.Errorf(message.ErrAccountUpdateName, message.ErrAccountNameTaken)
		} else {
			err = fmt.Errorf(message.ErrAccountUpdateName, err)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return account.Account{}, err
	}

	span.SetStatus(codes.Ok, "")
	return accountFromRow(row.ID, row.HouseholdID, row.Name, row.AccountType, row.CurrencyCode,
		row.Balance, row.Status, row.Version, row.CreatedAt, row.UpdatedAt, row.CreatedBy, row.UpdatedBy), nil
}

// UpdateBalance sets the account balance to the new value.
// Returns an error wrapping ErrAccountVersionConflict when expectedVersion does not match.
func (r *AccountRepo) UpdateBalance(ctx context.Context, id uuid.UUID, input account.UpdateBalanceInput, expectedVersion int, callerID uuid.UUID) (account.Account, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "AccountRepo.UpdateBalance")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	row, err := New(db).UpdateAccountBalance(ctx, UpdateAccountBalanceParams{
		ID:              id,
		Balance:         input.Balance,
		UpdatedBy:       uuidToPgUUID(callerID),
		ExpectedVersion: int32(expectedVersion),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = fmt.Errorf(message.ErrAccountUpdateBalance, message.ErrAccountVersionConflict)
		} else {
			err = fmt.Errorf(message.ErrAccountUpdateBalance, err)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return account.Account{}, err
	}

	span.SetStatus(codes.Ok, "")
	return accountFromRow(row.ID, row.HouseholdID, row.Name, row.AccountType, row.CurrencyCode,
		row.Balance, row.Status, row.Version, row.CreatedAt, row.UpdatedAt, row.CreatedBy, row.UpdatedBy), nil
}

// Deactivate soft-deletes the account.
// Idempotent — deactivating an already-deactivated account is not an error.
func (r *AccountRepo) Deactivate(ctx context.Context, id uuid.UUID, callerID uuid.UUID) error {
	ctx, span := otel.Tracer("postgres").Start(ctx, "AccountRepo.Deactivate")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	if err := New(db).DeactivateAccount(ctx, DeactivateAccountParams{
		ID:        id,
		UpdatedBy: uuidToPgUUID(callerID),
	}); err != nil {
		err = fmt.Errorf(message.ErrAccountDeactivate, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}
