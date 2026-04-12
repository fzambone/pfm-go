package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zambone/pfm-go/internal/platform/validate"
)

func TestLoadSeedConfig_WhenAllEnvVarsSet_ReturnsConfig(t *testing.T) {
	t.Setenv("SEED_EMAIL", "alice@example.com")
	t.Setenv("SEED_DISPLAY_NAME", "Alice")
	t.Setenv("SEED_PASSWORD", "secret1234")
	t.Setenv("SEED_HOUSEHOLD_NAME", "Alice's Household")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")

	cfg, err := loadSeedConfig()

	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", cfg.Email)
	assert.Equal(t, "Alice", cfg.DisplayName)
	assert.Equal(t, "secret1234", cfg.Password)
	assert.Equal(t, "Alice's Household", cfg.HouseholdName)
	assert.Equal(t, "postgres://user:pass@localhost/db", cfg.DatabaseURL)
}

func TestLoadSeedConfig_WhenMissingEmail_ReturnsValidationError(t *testing.T) {
	t.Setenv("SEED_EMAIL", "")
	t.Setenv("SEED_DISPLAY_NAME", "Alice")
	t.Setenv("SEED_PASSWORD", "secret1234")
	t.Setenv("SEED_HOUSEHOLD_NAME", "Alice's Household")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")

	_, err := loadSeedConfig()

	require.Error(t, err)
	var ve *validate.ValidationError
	require.True(t, errors.As(err, &ve), "expected ValidationError, got %T: %v", err, err)
	assertFieldViolated(t, ve, "email")
}

func TestLoadSeedConfig_WhenMissingDisplayName_ReturnsValidationError(t *testing.T) {
	t.Setenv("SEED_EMAIL", "alice@example.com")
	t.Setenv("SEED_DISPLAY_NAME", "")
	t.Setenv("SEED_PASSWORD", "secret1234")
	t.Setenv("SEED_HOUSEHOLD_NAME", "Alice's Household")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")

	_, err := loadSeedConfig()

	require.Error(t, err)
	var ve *validate.ValidationError
	require.True(t, errors.As(err, &ve))
	assertFieldViolated(t, ve, "display_name")
}

func TestLoadSeedConfig_WhenPasswordTooShort_ReturnsValidationError(t *testing.T) {
	t.Setenv("SEED_EMAIL", "alice@example.com")
	t.Setenv("SEED_DISPLAY_NAME", "Alice")
	t.Setenv("SEED_PASSWORD", "short")
	t.Setenv("SEED_HOUSEHOLD_NAME", "Alice's Household")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")

	_, err := loadSeedConfig()

	require.Error(t, err)
	var ve *validate.ValidationError
	require.True(t, errors.As(err, &ve))
	assertFieldViolated(t, ve, "password")
}

func TestLoadSeedConfig_WhenHouseholdNameEmpty_ReturnsValidationError(t *testing.T) {
	t.Setenv("SEED_EMAIL", "alice@example.com")
	t.Setenv("SEED_DISPLAY_NAME", "Alice")
	t.Setenv("SEED_PASSWORD", "secret1234")
	t.Setenv("SEED_HOUSEHOLD_NAME", "")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")

	_, err := loadSeedConfig()

	require.Error(t, err)
	var ve *validate.ValidationError
	require.True(t, errors.As(err, &ve))
	assertFieldViolated(t, ve, "household_name")
}

func TestLoadSeedConfig_WhenDatabaseURLMissing_ReturnsValidationError(t *testing.T) {
	t.Setenv("SEED_EMAIL", "alice@example.com")
	t.Setenv("SEED_DISPLAY_NAME", "Alice")
	t.Setenv("SEED_PASSWORD", "secret1234")
	t.Setenv("SEED_HOUSEHOLD_NAME", "Alice's Household")
	t.Setenv("DATABASE_URL", "")

	_, err := loadSeedConfig()

	require.Error(t, err)
	var ve *validate.ValidationError
	require.True(t, errors.As(err, &ve))
	assertFieldViolated(t, ve, "database_url")
}

func TestLoadSeedConfig_WhenEmailInvalid_ReturnsValidationError(t *testing.T) {
	t.Setenv("SEED_EMAIL", "notanemail")
	t.Setenv("SEED_DISPLAY_NAME", "Alice")
	t.Setenv("SEED_PASSWORD", "secret1234")
	t.Setenv("SEED_HOUSEHOLD_NAME", "Alice's Household")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")

	_, err := loadSeedConfig()

	require.Error(t, err)
	var ve *validate.ValidationError
	require.True(t, errors.As(err, &ve))
	assertFieldViolated(t, ve, "email")
}

// assertFieldViolated is a test helper that checks at least one violation targets the given field.
func assertFieldViolated(t *testing.T, ve *validate.ValidationError, field string) {
	t.Helper()
	for _, v := range ve.Violations {
		if v.Field == field {
			return
		}
	}
	t.Errorf("expected violation for field %q but got %v", field, ve.Violations)
}
