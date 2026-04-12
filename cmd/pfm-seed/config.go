package main

import (
	"fmt"
	"os"

	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/config"
	"github.com/zambone/pfm-go/internal/platform/validate"
)

// seedConfig holds all settings required by the seed tool, loaded from environment variables.
// Validated in full before any database connection is opened.
type seedConfig struct {
	// Email is the seed user's login email. Env: SEED_EMAIL.
	Email string
	// DisplayName is the seed user's display name. Env: SEED_DISPLAY_NAME.
	DisplayName string
	// Password is the seed user's plaintext password (hashed before storage). Env: SEED_PASSWORD.
	Password string
	// HouseholdName is the name of the seed household. Env: SEED_HOUSEHOLD_NAME.
	HouseholdName string
	// DatabaseURL is the full PostgreSQL connection URL. Env: DATABASE_URL.
	DatabaseURL string
	// DatabaseSSLMode is the PostgreSQL SSL mode. Env: DATABASE_SSL_MODE. Default: "disable".
	DatabaseSSLMode string
}

// loadSeedConfig reads seed configuration from environment variables and validates all fields.
// Returns a ValidationError if any required field is missing or invalid — before any DB connection.
func loadSeedConfig() (*seedConfig, error) {
	cfg := &seedConfig{
		Email:           os.Getenv("SEED_EMAIL"),
		DisplayName:     os.Getenv("SEED_DISPLAY_NAME"),
		Password:        os.Getenv("SEED_PASSWORD"),
		HouseholdName:   os.Getenv("SEED_HOUSEHOLD_NAME"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		DatabaseSSLMode: envStrDefault("DATABASE_SSL_MODE", "disable"),
	}

	r := validate.NewResult()
	r.Field("email", cfg.Email, validate.Required, validate.Email)
	r.Field("display_name", cfg.DisplayName, validate.Required)
	r.Field("password", cfg.Password, validate.Required, validate.MinLen(8))
	r.Field("household_name", cfg.HouseholdName, validate.Required)
	r.Field("database_url", cfg.DatabaseURL, validate.Required)

	if err := r.Error(); err != nil {
		return nil, fmt.Errorf(message.ErrSeedConfig, err)
	}

	return cfg, nil
}

// platformCfg maps a seedConfig to the platform config struct that database.Open
// and database.NewPool expect. Only the DB-related fields are populated.
func platformCfg(cfg *seedConfig) *config.Config {
	return &config.Config{
		DatabaseURL:            cfg.DatabaseURL,
		DatabaseSSLMode:        cfg.DatabaseSSLMode,
		DBConnectTimeoutSec:    5,
		DBStartupRetries:       3,
		DBStartupRetryDelaySec: 2,
		DBMaxOpenConns:         5,
		DBMaxIdleConns:         2,
		DBConnMaxLifetimeSec:   300,
		DBConnMaxIdleTimeSec:   60,
	}
}

func envStrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
