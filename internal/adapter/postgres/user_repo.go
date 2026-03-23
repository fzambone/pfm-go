package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	domainuser "github.com/zambone/pfm-go/internal/domain/user"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/database"
)

// UserRepo implements domain/user.Repository using PostgreSQL via sqlc-generated queries.
type UserRepo struct {
	pool *pgxpool.Pool
}

// NewUserRepo creates a UserRepo backed by pool.
// Panics if pool is nil to catch misconfigured wiring at startup.
func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	if pool == nil {
		panic("postgres: NewUserRepo requires non-nil pool")
	}
	return &UserRepo{pool: pool}
}

// uuidToPgUUID converts a uuid.UUID to pgtype.UUID for use as a nullable DB column.
// Returns a NULL pgtype.UUID when id is uuid.Nil, since nullable self-referential FKs
// (e.g. created_by REFERENCES users(id)) must be NULL when no real user exists yet.
func uuidToPgUUID(id uuid.UUID) pgtype.UUID {
	if id == uuid.Nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

// pgUUIDToUUID converts a nullable pgtype.UUID to uuid.UUID.
// Returns uuid.Nil when the DB value is NULL.
func pgUUIDToUUID(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return p.Bytes
}

// pgTimestamptzToTime converts a pgtype.Timestamptz to time.Time.
// Returns zero time when the DB value is NULL.
func pgTimestamptzToTime(p pgtype.Timestamptz) time.Time {
	if !p.Valid {
		return time.Time{}
	}
	return p.Time
}

// isUniqueViolation reports whether err is a PostgreSQL unique-constraint violation
// for the given constraint name.
func isUniqueViolation(err error, constraintName string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == constraintName
}

// Create inserts a new user and returns the saved entity with server-assigned fields.
// Returns an error wrapping ErrUserEmailTaken when the email is already in use.
func (r *UserRepo) Create(ctx context.Context, input domainuser.RegisterInput, passwordHash string, callerID uuid.UUID) (domainuser.User, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "UserRepo.Create")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	row, err := New(db).CreateUser(ctx, CreateUserParams{
		Email:        input.Email,
		PasswordHash: passwordHash,
		DisplayName:  input.DisplayName,
		CreatedBy:    uuidToPgUUID(callerID),
		UpdatedBy:    uuidToPgUUID(callerID),
	})
	if err != nil {
		if isUniqueViolation(err, "users_email_unique_active") {
			err = fmt.Errorf(message.ErrUserCreate, message.ErrUserEmailTaken)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return domainuser.User{}, err
		}
		err = fmt.Errorf(message.ErrUserCreate, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return domainuser.User{}, err
	}

	span.SetStatus(codes.Ok, "")
	return domainuser.User{
		ID:           row.ID,
		Email:        row.Email,
		DisplayName:  row.DisplayName,
		PasswordHash: row.PasswordHash,
		Version:      int(row.Version),
		CreatedAt:    pgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:    pgTimestamptzToTime(row.UpdatedAt),
		CreatedBy:    pgUUIDToUUID(row.CreatedBy),
		UpdatedBy:    pgUUIDToUUID(row.UpdatedBy),
	}, nil
}

// FindByID returns the active (non-deleted) user with the given ID.
// Returns an error wrapping ErrUserNotFound when no matching active user exists.
func (r *UserRepo) FindByID(ctx context.Context, id uuid.UUID) (domainuser.User, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "UserRepo.FindByID")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	row, err := New(db).FindUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = fmt.Errorf(message.ErrUserFindByID, message.ErrUserNotFound)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return domainuser.User{}, err
		}
		err = fmt.Errorf(message.ErrUserFindByID, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return domainuser.User{}, err
	}

	span.SetStatus(codes.Ok, "")
	return domainuser.User{
		ID:           row.ID,
		Email:        row.Email,
		DisplayName:  row.DisplayName,
		PasswordHash: row.PasswordHash,
		Version:      int(row.Version),
		CreatedAt:    pgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:    pgTimestamptzToTime(row.UpdatedAt),
		CreatedBy:    pgUUIDToUUID(row.CreatedBy),
		UpdatedBy:    pgUUIDToUUID(row.UpdatedBy),
	}, nil
}

// FindByEmail returns the active (non-deleted) user whose email matches (case-insensitive).
// Returns an error wrapping message.ErrLoginInvalidCredentials when no matching user exists.
func (r *UserRepo) FindByEmail(ctx context.Context, email string) (domainuser.User, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "UserRepo.FindByEmail")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	row, err := New(db).FindUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = fmt.Errorf(message.ErrUserFindByEmail, message.ErrLoginInvalidCredentials)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return domainuser.User{}, err
		}
		err = fmt.Errorf(message.ErrUserFindByEmail, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return domainuser.User{}, err
	}

	span.SetStatus(codes.Ok, "")
	return domainuser.User{
		ID:           row.ID,
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
	}, nil
}

// UpdateProfile changes the display name of the user identified by id.
// Returns an error wrapping ErrUserVersionConflict when expectedVersion does not match.
func (r *UserRepo) UpdateProfile(ctx context.Context, id uuid.UUID, input domainuser.UpdateProfileInput, expectedVersion int, callerID uuid.UUID) (domainuser.User, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "UserRepo.UpdateProfile")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	row, err := New(db).UpdateUserProfile(ctx, UpdateUserProfileParams{
		ID:              id,
		DisplayName:     input.DisplayName,
		UpdatedBy:       uuidToPgUUID(callerID),
		ExpectedVersion: int32(expectedVersion),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = fmt.Errorf(message.ErrUserUpdateProfile, message.ErrUserVersionConflict)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return domainuser.User{}, err
		}
		err = fmt.Errorf(message.ErrUserUpdateProfile, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return domainuser.User{}, err
	}

	span.SetStatus(codes.Ok, "")
	return domainuser.User{
		ID:           row.ID,
		Email:        row.Email,
		DisplayName:  row.DisplayName,
		PasswordHash: row.PasswordHash,
		Version:      int(row.Version),
		CreatedAt:    pgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:    pgTimestamptzToTime(row.UpdatedAt),
		CreatedBy:    pgUUIDToUUID(row.CreatedBy),
		UpdatedBy:    pgUUIDToUUID(row.UpdatedBy),
	}, nil
}

// ChangePassword replaces the password hash of the user identified by id.
// Returns an error wrapping ErrUserVersionConflict when expectedVersion does not match.
func (r *UserRepo) ChangePassword(ctx context.Context, id uuid.UUID, newHash string, expectedVersion int, callerID uuid.UUID) (domainuser.User, error) {
	ctx, span := otel.Tracer("postgres").Start(ctx, "UserRepo.ChangePassword")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	row, err := New(db).ChangeUserPassword(ctx, ChangeUserPasswordParams{
		ID:              id,
		PasswordHash:    newHash,
		UpdatedBy:       uuidToPgUUID(callerID),
		ExpectedVersion: int32(expectedVersion),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = fmt.Errorf(message.ErrUserChangePassword, message.ErrUserVersionConflict)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return domainuser.User{}, err
		}
		err = fmt.Errorf(message.ErrUserChangePassword, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return domainuser.User{}, err
	}

	span.SetStatus(codes.Ok, "")
	return domainuser.User{
		ID:           row.ID,
		Email:        row.Email,
		DisplayName:  row.DisplayName,
		PasswordHash: row.PasswordHash,
		Version:      int(row.Version),
		CreatedAt:    pgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:    pgTimestamptzToTime(row.UpdatedAt),
		CreatedBy:    pgUUIDToUUID(row.CreatedBy),
		UpdatedBy:    pgUUIDToUUID(row.UpdatedBy),
	}, nil
}

// Deactivate soft-deletes the user. Idempotent — deactivating an already-deactivated
// or non-existent user is not an error.
func (r *UserRepo) Deactivate(ctx context.Context, id uuid.UUID, callerID uuid.UUID) error {
	ctx, span := otel.Tracer("postgres").Start(ctx, "UserRepo.Deactivate")
	defer span.End()

	db := database.DBTXFromContext(ctx, r.pool)
	if err := New(db).DeactivateUser(ctx, DeactivateUserParams{
		ID:        id,
		UpdatedBy: uuidToPgUUID(callerID),
	}); err != nil {
		err = fmt.Errorf(message.ErrUserDeactivate, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}
