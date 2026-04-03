package config_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/config"
)

func TestLoad_DefaultValues(t *testing.T) {
	cfg, err := config.Load()
	assert.NoError(t, err)

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"HTTP port", cfg.HTTPPort, 8080},
		{"database host", cfg.DatabaseHost, "localhost"},
		{"database port", cfg.DatabasePort, 5432},
		{"database user", cfg.DatabaseUser, "pfm_user"},
		{"database name", cfg.DatabaseName, "pfm_dev"},
		{"database SSL mode", cfg.DatabaseSSLMode, "disable"},
		{"db max open conns", cfg.DBMaxOpenConns, 25},
		{"db max idle conns", cfg.DBMaxIdleConns, 5},
		{"db conn max lifetime sec", cfg.DBConnMaxLifetimeSec, 300},
		{"db conn max idle time sec", cfg.DBConnMaxIdleTimeSec, 60},
		{"db connect timeout sec", cfg.DBConnectTimeoutSec, 5},
		{"db startup retries", cfg.DBStartupRetries, 5},
		{"db startup retry delay sec", cfg.DBStartupRetryDelaySec, 2},
		{"shutdown timeout seconds", cfg.ShutdownTimeoutSec, 15},
		{"log level", cfg.LogLevel, "info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.got)
		})
	}
}

func TestLoad_EnvironmentOverridesDefaults(t *testing.T) {
	tests := []struct {
		name   string
		envKey string
		envVal string
		get    func(config2 *config.Config) any
		want   any
	}{
		{"HTTP port", "HTTP_PORT", "9090", func(c *config.Config) any { return c.HTTPPort }, 9090},
		{"database host", "DATABASE_HOST", "db.example.com", func(c *config.Config) any { return c.DatabaseHost }, "db.example.com"},
		{"database port", "DATABASE_PORT", "5433", func(c *config.Config) any { return c.DatabasePort }, 5433},
		{"database user", "DATABASE_USER", "admin", func(c *config.Config) any { return c.DatabaseUser }, "admin"},
		{"database password", "DATABASE_PASSWORD", "supersecret", func(c *config.Config) any { return c.DatabasePassword }, "supersecret"},
		{"database name", "DATABASE_NAME", "pfm_prod", func(c *config.Config) any { return c.DatabaseName }, "pfm_prod"},
		{"database SSL mode", "DATABASE_SSL_MODE", "require", func(c *config.Config) any { return c.DatabaseSSLMode }, "require"},
		{"db max open conns", "DB_MAX_OPEN_CONNS", "50", func(c *config.Config) any { return c.DBMaxOpenConns }, 50},
		{"db max idle conns", "DB_MAX_IDLE_CONNS", "10", func(c *config.Config) any { return c.DBMaxIdleConns }, 10},
		{"db conn max lifetime sec", "DB_CONN_MAX_LIFETIME_SEC", "600", func(c *config.Config) any { return c.DBConnMaxLifetimeSec }, 600},
		{"db conn max idle time sec", "DB_CONN_MAX_IDLE_TIME_SEC", "120", func(c *config.Config) any { return c.DBConnMaxIdleTimeSec }, 120},
		{"db connect timeout sec", "DB_CONNECT_TIMEOUT_SEC", "10", func(c *config.Config) any { return c.DBConnectTimeoutSec }, 10},
		{"db startup retries", "DB_STARTUP_RETRIES", "3", func(c *config.Config) any { return c.DBStartupRetries }, 3},
		{"db startup retry delay sec", "DB_STARTUP_RETRY_DELAY_SEC", "5", func(c *config.Config) any { return c.DBStartupRetryDelaySec }, 5},
		{"shutdown timeout", "SHUTDOWN_TIMEOUT_SEC", "30", func(c *config.Config) any { return c.ShutdownTimeoutSec }, 30},
		{"log level", "LOG_LEVEL", "debug", func(c *config.Config) any { return c.LogLevel }, "debug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envKey, tt.envVal)

			cfg, err := config.Load()

			assert.NoError(t, err)
			assert.Equal(t, tt.want, tt.get(cfg))
		})
	}
}

func TestLoad_InvalidIntegerValue_ReturnsError(t *testing.T) {
	tests := []struct {
		name   string
		envKey string
	}{
		{"HTTP port", "HTTP_PORT"},
		{"database port", "DATABASE_PORT"},
		{"shutdown timeout", "SHUTDOWN_TIMEOUT_SEC"},
		{"db max open conns", "DB_MAX_OPEN_CONNS"},
		{"db max idle conns", "DB_MAX_IDLE_CONNS"},
		{"db conn max lifetime sec", "DB_CONN_MAX_LIFETIME_SEC"},
		{"db conn max idle time sec", "DB_CONN_MAX_IDLE_TIME_SEC"},
		{"db connect timeout sec", "DB_CONNECT_TIMEOUT_SEC"},
		{"db startup retries", "DB_STARTUP_RETRIES"},
		{"db startup retry delay sec", "DB_STARTUP_RETRY_DELAY_SEC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envKey, "not-a-number")

			_, err := config.Load()

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.envKey)
		})
	}
}

func TestLoad_InvalidPortRange_ReturnsError(t *testing.T) {
	tests := []struct {
		name   string
		envKey string
		value  string
	}{
		{"HTTP port zero", "HTTP_PORT", "0"},
		{"HTTP port negative", "HTTP_PORT", "-1"},
		{"HTTP port too high", "HTTP_PORT", "65536"},
		{"database port zero", "DATABASE_PORT", "0"},
		{"database port too high", "DATABASE_PORT", "65536"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envKey, tt.value)

			_, err := config.Load()

			assert.Error(t, err)
		})
	}
}

func TestLoad_EmptyStringTreatedAsNotSet(t *testing.T) {
	t.Setenv("DATABASE_HOST", "")

	cfg, err := config.Load()

	assert.NoError(t, err)
	assert.Equal(t, "localhost", cfg.DatabaseHost)
}

func TestLoad_BooleanParsing(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"true", "true", true},
		{"false", "false", false},
		{"TRUE", "TRUE", true},
		{"1", "1", true},
		{"0", "0", false},
		{"yes", "yes", true},
		{"no", "no", false},
		{"YES", "YES", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DEBUG", tt.value)

			cfg, err := config.Load()

			assert.NoError(t, err)
			assert.Equal(t, tt.want, cfg.Debug)
		})
	}
}

func TestLoad_InvalidBooleanValue_ReturnsError(t *testing.T) {
	t.Setenv("DEBUG", "maybe")

	_, err := config.Load()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), message.ErrConfigInvalidBool)
}

func TestConfig_StringDoesNotLeakPassword(t *testing.T) {
	t.Setenv("DATABASE_PASSWORD", "super-secret-123")
	cfg, err := config.Load()

	assert.NoError(t, err)
	assert.NotContains(t, cfg.String(), "super-secret-123")
	assert.Contains(t, cfg.String(), "****")
}

func TestLoad_DatabaseURL_EmptyByDefault(t *testing.T) {
	cfg, err := config.Load()

	assert.NoError(t, err)
	assert.Empty(t, cfg.DatabaseURL)
}

func TestLoad_DatabaseURL_LoadedFromEnv(t *testing.T) {
	url := "postgres://user:pass@host:5432/dbname?sslmode=require"
	t.Setenv("DATABASE_URL", url)

	cfg, err := config.Load()

	assert.NoError(t, err)
	assert.Equal(t, url, cfg.DatabaseURL)
}

func TestConfig_StringDoesNotLeakDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:secret@host:5432/db")
	cfg, err := config.Load()

	assert.NoError(t, err)
	assert.NotContains(t, cfg.String(), "secret")
}

func TestLoad_OTELEndpointHasNoDefault(t *testing.T) {
	cfg, err := config.Load()

	assert.NoError(t, err)
	assert.Empty(t, cfg.OTELEndpoint)
}

func TestLoad_OTELEndpointFromEnv(t *testing.T) {
	endpoint := "http://collector:4317"
	t.Setenv("OTEL_ENDPOINT", endpoint)

	cfg, err := config.Load()

	assert.NoError(t, err)
	assert.Equal(t, endpoint, cfg.OTELEndpoint)
}

func TestLoad_RequiredFieldMissing_ReturnsError(t *testing.T) {
	t.Setenv("REQUIRE_OTEL", "true")

	_, err := config.Load()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OTEL_ENDPOINT")
	assert.Contains(t, err.Error(), fmt.Sprintf(message.ErrConfigRequired, "OTEL_ENDPOINT"))
}
