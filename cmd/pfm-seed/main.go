package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"

	authadapter "github.com/zambone/pfm-go/internal/adapter/auth"
	pgadapter "github.com/zambone/pfm-go/internal/adapter/postgres"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/database"
	pfmdb "github.com/zambone/pfm-go/db"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	cfg, err := loadSeedConfig()
	if err != nil {
		return fmt.Errorf(message.ErrSeedConfig, err)
	}

	slog.InfoContext(ctx, message.MsgSeedStarting)

	db, err := database.Open(ctx, platformCfg(cfg))
	if err != nil {
		return fmt.Errorf(message.ErrRunOpenDB, err)
	}
	defer func() { _ = db.Close() }() // best-effort: already committed or rolled back

	migrationsFS, err := fs.Sub(pfmdb.Migrations, "migrations")
	if err != nil {
		return fmt.Errorf(message.ErrMigrateSubFS, err)
	}
	if err := database.Migrate(ctx, db, migrationsFS); err != nil {
		return fmt.Errorf(message.ErrRunMigrate, err)
	}

	pool, err := database.NewPool(ctx, platformCfg(cfg))
	if err != nil {
		return fmt.Errorf(message.ErrDBNewPool, err)
	}
	defer pool.Close()

	hasher := authadapter.NewArgon2idHasher(authadapter.DefaultArgon2idParams())
	userRepo := pgadapter.NewUserRepo(pool)
	householdRepo := pgadapter.NewHouseholdRepo(pool)
	tx := database.NewPostgresTransactor(pool)

	s := newSeeder(hasher, userRepo, userRepo, householdRepo, tx)

	result, err := s.Run(ctx, seedInput{
		Email:         cfg.Email,
		DisplayName:   cfg.DisplayName,
		Password:      cfg.Password,
		HouseholdName: cfg.HouseholdName,
	})
	if err != nil {
		return fmt.Errorf(message.ErrSeedTransaction, err)
	}

	if result.AlreadySeeded {
		slog.InfoContext(ctx, message.MsgSeedAlreadySeeded)
		return nil
	}

	slog.InfoContext(ctx, message.MsgSeedSuccess,
		"user_id", result.UserID,
		"household_id", result.HouseholdID,
	)
	return nil
}
