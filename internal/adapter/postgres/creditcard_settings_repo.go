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

	"github.com/zambone/pfm-go/internal/domain/creditcard"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/database"
)

// CreditCardSettingsRepo implements domain/creditcard.Repository using PostgreSQL.
type CreditCardSettingsRepo struct {
	pool *pgxpool.Pool
}

// NewCreditCardSettingsRepo creates a CreditCardSettingsRepo backed by pool.
// Panics if pool is nil to catch misconfigured wiring at startup.
func NewCreditCardSettingsRepo(pool *pgxpool.Pool) *CreditCardSettingsRepo {
	if pool == nil {
		panic("postgres: NewCreditCardSettingsRepo requires non-nil pool")
	}
	return &CreditCardSettingsRepo{pool: pool}
}

// settingsFromRow maps sqlc-generated row fields to the domain entity.
func settingsFromRow(
	id, accountID uuid.UUID,
	closingDay, dueDay int32,
	limitAmount int64,
	version int32,
	createdAt, updatedAt pgtype.Timestamptz,
	createdBy, updatedBy pgtype.UUID,
) creditcard.Settings {
	return creditcard.Settings{
		ID:          id,
		AccountID:   accountID,
		ClosingDay:  int(closingDay),
		DueDay:      int(dueDay),
		LimitAmount: limitAmount,
		Version:     int(version),
		CreatedAt:   pgTimestamptzToTime(createdAt),
		UpdatedAt:   pgTimestamptzToTime(updatedAt),
		CreatedBy:   pgUUIDToUUID(createdBy),
		UpdatedBy:   pgUUIDToUUID(updatedBy),
	}
}

// Create inserts new credit card settings for the given account.
// Returns an error wrapping ErrCreditCardSettingsExists when the account already has settings.
func (r *CreditCardSettingsRepo) Create(ctx context.Context, accountID uuid.UUID, input creditcard.CreateInput, callerID uuid.UUID) (creditcard.Settings, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "CreditCardSettingsRepo.Create")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	row, err := New(db).CreateCreditCardSettings(ctx, CreateCreditCardSettingsParams{
		AccountID:   accountID,
		ClosingDay:  int32(input.ClosingDay),
		DueDay:      int32(input.DueDay),
		LimitAmount: input.LimitAmount,
		CreatedBy:   uuidToPgUUID(callerID),
		UpdatedBy:   uuidToPgUUID(callerID),
	})
	if err != nil {
		if isUniqueViolation(err, "credit_card_settings_account_id_key") {
			err = fmt.Errorf(message.ErrCCSettingsCreate, message.ErrCreditCardSettingsExists)
		} else {
			err = fmt.Errorf(message.ErrCCSettingsCreate, err)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return creditcard.Settings{}, err
	}

	span.SetStatus(codes.Ok, "")
	return settingsFromRow(row.ID, row.AccountID, row.ClosingDay, row.DueDay,
		row.LimitAmount, row.Version, row.CreatedAt, row.UpdatedAt, row.CreatedBy, row.UpdatedBy), nil
}

// FindByAccountID returns the active settings for the given account.
// Returns an error wrapping ErrCreditCardSettingsNotFound when no settings exist.
func (r *CreditCardSettingsRepo) FindByAccountID(ctx context.Context, accountID uuid.UUID) (creditcard.Settings, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "CreditCardSettingsRepo.FindByAccountID")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	row, err := New(db).FindCreditCardSettingsByAccountID(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = fmt.Errorf(message.ErrCCSettingsFindByAccount, message.ErrCreditCardSettingsNotFound)
		} else {
			err = fmt.Errorf(message.ErrCCSettingsFindByAccount, err)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return creditcard.Settings{}, err
	}

	span.SetStatus(codes.Ok, "")
	return settingsFromRow(row.ID, row.AccountID, row.ClosingDay, row.DueDay,
		row.LimitAmount, row.Version, row.CreatedAt, row.UpdatedAt, row.CreatedBy, row.UpdatedBy), nil
}

// UpdateClosingDay changes the billing cycle closing day.
// Returns an error wrapping ErrCreditCardSettingsVersionConflict when version mismatches.
func (r *CreditCardSettingsRepo) UpdateClosingDay(ctx context.Context, accountID uuid.UUID, input creditcard.UpdateClosingDayInput, expectedVersion int, callerID uuid.UUID) (creditcard.Settings, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "CreditCardSettingsRepo.UpdateClosingDay")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	row, err := New(db).UpdateCreditCardClosingDay(ctx, UpdateCreditCardClosingDayParams{
		AccountID:       accountID,
		ClosingDay:      int32(input.ClosingDay),
		UpdatedBy:       uuidToPgUUID(callerID),
		ExpectedVersion: int32(expectedVersion),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = fmt.Errorf(message.ErrCCSettingsUpdateClosing, message.ErrCreditCardSettingsVersionConflict)
		} else {
			err = fmt.Errorf(message.ErrCCSettingsUpdateClosing, err)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return creditcard.Settings{}, err
	}

	span.SetStatus(codes.Ok, "")
	return settingsFromRow(row.ID, row.AccountID, row.ClosingDay, row.DueDay,
		row.LimitAmount, row.Version, row.CreatedAt, row.UpdatedAt, row.CreatedBy, row.UpdatedBy), nil
}

// UpdateDueDay changes the payment due day.
// Returns an error wrapping ErrCreditCardSettingsVersionConflict when version mismatches.
func (r *CreditCardSettingsRepo) UpdateDueDay(ctx context.Context, accountID uuid.UUID, input creditcard.UpdateDueDayInput, expectedVersion int, callerID uuid.UUID) (creditcard.Settings, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "CreditCardSettingsRepo.UpdateDueDay")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	row, err := New(db).UpdateCreditCardDueDay(ctx, UpdateCreditCardDueDayParams{
		AccountID:       accountID,
		DueDay:          int32(input.DueDay),
		UpdatedBy:       uuidToPgUUID(callerID),
		ExpectedVersion: int32(expectedVersion),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = fmt.Errorf(message.ErrCCSettingsUpdateDueDay, message.ErrCreditCardSettingsVersionConflict)
		} else {
			err = fmt.Errorf(message.ErrCCSettingsUpdateDueDay, err)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return creditcard.Settings{}, err
	}

	span.SetStatus(codes.Ok, "")
	return settingsFromRow(row.ID, row.AccountID, row.ClosingDay, row.DueDay,
		row.LimitAmount, row.Version, row.CreatedAt, row.UpdatedAt, row.CreatedBy, row.UpdatedBy), nil
}

// UpdateLimit changes the credit limit.
// Returns an error wrapping ErrCreditCardSettingsVersionConflict when version mismatches.
func (r *CreditCardSettingsRepo) UpdateLimit(ctx context.Context, accountID uuid.UUID, input creditcard.UpdateLimitInput, expectedVersion int, callerID uuid.UUID) (creditcard.Settings, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "CreditCardSettingsRepo.UpdateLimit")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	row, err := New(db).UpdateCreditCardLimit(ctx, UpdateCreditCardLimitParams{
		AccountID:       accountID,
		LimitAmount:     input.LimitAmount,
		UpdatedBy:       uuidToPgUUID(callerID),
		ExpectedVersion: int32(expectedVersion),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = fmt.Errorf(message.ErrCCSettingsUpdateLimit, message.ErrCreditCardSettingsVersionConflict)
		} else {
			err = fmt.Errorf(message.ErrCCSettingsUpdateLimit, err)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return creditcard.Settings{}, err
	}

	span.SetStatus(codes.Ok, "")
	return settingsFromRow(row.ID, row.AccountID, row.ClosingDay, row.DueDay,
		row.LimitAmount, row.Version, row.CreatedAt, row.UpdatedAt, row.CreatedBy, row.UpdatedBy), nil
}

// Delete soft-deletes the settings for the given account.
// Idempotent — deleting already-deleted settings is not an error.
func (r *CreditCardSettingsRepo) Delete(ctx context.Context, accountID uuid.UUID, callerID uuid.UUID) error {
	ctx, span := otel.Tracer("postgres").Start(ctx, "CreditCardSettingsRepo.Delete")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	if err := New(db).DeleteCreditCardSettings(ctx, DeleteCreditCardSettingsParams{
		AccountID: accountID,
		UpdatedBy: uuidToPgUUID(callerID),
	}); err != nil {
		err = fmt.Errorf(message.ErrCCSettingsDelete, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}
