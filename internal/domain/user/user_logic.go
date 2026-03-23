package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/validate"
)

// passwordHasher abstracts both hashing and verification of passwords.
// Structurally satisfied by adapter/auth.FakeHasher and adapter/auth.Argon2idHasher.
// Distinct from passwordVerifier (Verify-only) used by LoginLogic.
type passwordHasher interface {
	Hash(ctx context.Context, password string) (string, error)
	Verify(ctx context.Context, password, hash string) (bool, error)
}

// UserLogic orchestrates the user lifecycle: registration, profile updates,
// password changes, and deactivation.
type UserLogic struct {
	repo   Repository
	hasher passwordHasher
	clk    clocker
}

// NewUserLogic constructs a UserLogic. Panics if any dependency is nil.
func NewUserLogic(repo Repository, hasher passwordHasher, clk clocker) *UserLogic {
	if repo == nil {
		panic("user: NewUserLogic requires non-nil repo")
	}
	if hasher == nil {
		panic("user: NewUserLogic requires non-nil hasher")
	}
	if clk == nil {
		panic("user: NewUserLogic requires non-nil clk")
	}
	return &UserLogic{repo: repo, hasher: hasher, clk: clk}
}

// Register validates input, hashes the password, and creates a new user.
// Returns a ValidationError if input is invalid.
// Returns an error wrapping ErrUserEmailTaken if the email is already registered.
func (l *UserLogic) Register(ctx context.Context, input RegisterInput, callerID uuid.UUID) (User, error) {
	r := validate.NewResult()
	r.Field("email", input.Email, validate.Required)
	r.Field("display_name", input.DisplayName, validate.Required, validate.MinLen(2), validate.MaxLen(100))
	r.Field("password", input.Password, validate.Required, validate.MinLen(8))
	if err := r.Error(); err != nil {
		return User{}, err
	}

	hash, err := l.hasher.Hash(ctx, input.Password)
	if err != nil {
		return User{}, fmt.Errorf(message.ErrUserLogicRegister, err)
	}

	u, err := l.repo.Create(ctx, input, hash, callerID)
	if err != nil {
		if errors.Is(err, message.ErrUserEmailTaken) {
			return User{}, fmt.Errorf(message.ErrUserLogicRegister, message.ErrUserEmailTaken)
		}
		return User{}, fmt.Errorf(message.ErrUserLogicRegister, err)
	}
	return u, nil
}

// FindByID returns the active user with the given ID.
// Returns an error wrapping ErrUserNotFound when no matching active user exists.
func (l *UserLogic) FindByID(ctx context.Context, id uuid.UUID) (User, error) {
	u, err := l.repo.FindByID(ctx, id)
	if err != nil {
		return User{}, fmt.Errorf(message.ErrUserLogicFindByID, err)
	}
	return u, nil
}

// UpdateProfile validates input and updates the display name of the user identified by id.
// Returns a ValidationError if the new display name is invalid.
// Returns an error wrapping ErrUserVersionConflict when expectedVersion does not match.
func (l *UserLogic) UpdateProfile(ctx context.Context, id uuid.UUID, input UpdateProfileInput, expectedVersion int, callerID uuid.UUID) (User, error) {
	r := validate.NewResult()
	r.Field("display_name", input.DisplayName, validate.Required, validate.MinLen(2), validate.MaxLen(100))
	if err := r.Error(); err != nil {
		return User{}, err
	}

	u, err := l.repo.UpdateProfile(ctx, id, input, expectedVersion, callerID)
	if err != nil {
		return User{}, fmt.Errorf(message.ErrUserLogicUpdateProfile, err)
	}
	return u, nil
}

// ChangePassword verifies the old password then replaces the hash with a new one.
// Returns an error wrapping ErrLoginInvalidCredentials if the old password is wrong.
// Returns an error wrapping ErrUserVersionConflict when expectedVersion does not match.
func (l *UserLogic) ChangePassword(ctx context.Context, id uuid.UUID, input ChangePasswordInput, expectedVersion int, callerID uuid.UUID) (User, error) {
	r := validate.NewResult()
	r.Field("old_password", input.OldPassword, validate.Required)
	r.Field("new_password", input.NewPassword, validate.Required, validate.MinLen(8))
	if err := r.Error(); err != nil {
		return User{}, err
	}

	current, err := l.repo.FindByID(ctx, id)
	if err != nil {
		return User{}, fmt.Errorf(message.ErrUserLogicChangePassword, err)
	}

	ok, err := l.hasher.Verify(ctx, input.OldPassword, current.PasswordHash)
	if err != nil {
		return User{}, fmt.Errorf(message.ErrUserLogicChangePassword, err)
	}
	if !ok {
		return User{}, fmt.Errorf(message.ErrUserLogicChangePassword, message.ErrLoginInvalidCredentials)
	}

	newHash, err := l.hasher.Hash(ctx, input.NewPassword)
	if err != nil {
		return User{}, fmt.Errorf(message.ErrUserLogicChangePassword, err)
	}

	u, err := l.repo.ChangePassword(ctx, id, newHash, expectedVersion, callerID)
	if err != nil {
		return User{}, fmt.Errorf(message.ErrUserLogicChangePassword, err)
	}
	return u, nil
}

// Deactivate soft-deletes the user identified by id.
func (l *UserLogic) Deactivate(ctx context.Context, id uuid.UUID, callerID uuid.UUID) error {
	if err := l.repo.Deactivate(ctx, id, callerID); err != nil {
		return fmt.Errorf(message.ErrUserLogicDeactivate, err)
	}
	return nil
}
