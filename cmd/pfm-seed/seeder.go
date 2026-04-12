package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	domainhousehold "github.com/zambone/pfm-go/internal/domain/household"
	domainuser "github.com/zambone/pfm-go/internal/domain/user"
	"github.com/zambone/pfm-go/internal/message"
)

// passwordHasher hashes a plaintext password. Structurally satisfied by authadapter.Argon2idHasher.
type passwordHasher interface {
	Hash(ctx context.Context, password string) (string, error)
}

// existenceChecker reports whether any active user row exists. Structurally satisfied by postgres.UserRepo.
type existenceChecker interface {
	AnyExists(ctx context.Context) (bool, error)
}

// userCreator inserts a new user row. Structurally satisfied by postgres.UserRepo.
type userCreator interface {
	Create(ctx context.Context, input domainuser.RegisterInput, passwordHash string, callerID uuid.UUID) (domainuser.User, error)
}

// householdCreator inserts a new household row and its founding ADMIN membership atomically.
// The callerID is used as both created_by and the founding member's user_id.
// Structurally satisfied by postgres.HouseholdRepo.
type householdCreator interface {
	Create(ctx context.Context, input domainhousehold.CreateInput, callerID uuid.UUID) (domainhousehold.Household, error)
}

// seederTransactor runs a function inside a database transaction.
// Structurally satisfied by database.PostgresTransactor.
type seederTransactor interface {
	RunAtomic(ctx context.Context, fn func(context.Context) error) error
}

// seedInput carries the validated credentials and household name for the bootstrap operation.
type seedInput struct {
	Email         string
	DisplayName   string
	Password      string // plaintext — hashed inside Run before any DB write
	HouseholdName string
}

// seedResult distinguishes "newly created" from "already seeded" (idempotent exit).
type seedResult struct {
	// AlreadySeeded is true when users already existed; no writes were performed.
	AlreadySeeded bool
	// UserID is the ID of the newly created seed user. Zero if AlreadySeeded.
	UserID uuid.UUID
	// HouseholdID is the ID of the newly created seed household. Zero if AlreadySeeded.
	HouseholdID uuid.UUID
}

// seeder orchestrates the atomic bootstrap of the first user and household.
type seeder struct {
	hasher     passwordHasher
	checker    existenceChecker
	users      userCreator
	households householdCreator
	tx         seederTransactor
}

// newSeeder constructs a seeder. Panics if any dependency is nil.
func newSeeder(
	hasher passwordHasher,
	checker existenceChecker,
	users userCreator,
	households householdCreator,
	tx seederTransactor,
) *seeder {
	if hasher == nil {
		panic("seed: newSeeder requires non-nil passwordHasher")
	}
	if checker == nil {
		panic("seed: newSeeder requires non-nil existenceChecker")
	}
	if users == nil {
		panic("seed: newSeeder requires non-nil userCreator")
	}
	if households == nil {
		panic("seed: newSeeder requires non-nil householdCreator")
	}
	if tx == nil {
		panic("seed: newSeeder requires non-nil seederTransactor")
	}
	return &seeder{
		hasher:     hasher,
		checker:    checker,
		users:      users,
		households: households,
		tx:         tx,
	}
}

// Run executes the bootstrap sequence. It is idempotent: if any user already exists,
// it returns seedResult{AlreadySeeded: true} without modifying the database.
//
// On success, the created user and household are linked via an ADMIN membership, all within
// a single database transaction. A failure at any step rolls back all writes.
func (s *seeder) Run(ctx context.Context, input seedInput) (seedResult, error) {
	exists, err := s.checker.AnyExists(ctx)
	if err != nil {
		return seedResult{}, fmt.Errorf(message.ErrSeedCheckExists, err)
	}
	if exists {
		return seedResult{AlreadySeeded: true}, nil
	}

	passwordHash, err := s.hasher.Hash(ctx, input.Password)
	if err != nil {
		return seedResult{}, fmt.Errorf(message.ErrSeedHashPassword, err)
	}

	var result seedResult

	if err := s.tx.RunAtomic(ctx, func(txCtx context.Context) error {
		user, err := s.users.Create(txCtx, domainuser.RegisterInput{
			Email:       input.Email,
			DisplayName: input.DisplayName,
		}, passwordHash, uuid.Nil)
		if err != nil {
			return fmt.Errorf(message.ErrSeedCreateUser, err)
		}

		// HouseholdRepo.Create inserts both the household row and the founding ADMIN
		// membership in one operation — no separate AddMember call needed.
		household, err := s.households.Create(txCtx, domainhousehold.CreateInput{
			Name: input.HouseholdName,
		}, user.ID)
		if err != nil {
			return fmt.Errorf(message.ErrSeedCreateHousehold, err)
		}

		result = seedResult{UserID: user.ID, HouseholdID: household.ID}
		return nil
	}); err != nil {
		return seedResult{}, fmt.Errorf(message.ErrSeedTransaction, err)
	}

	return result, nil
}
