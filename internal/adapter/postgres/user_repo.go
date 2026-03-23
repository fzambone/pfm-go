package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
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
