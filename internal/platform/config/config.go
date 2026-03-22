package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/zambone/pfm-go/internal/message"
)

// Config holds all application settings loaded from environment variables.
// After loading, the struct is immutable - no exported setters.
type Config struct {
	// HTTPPort is the port the server listens on. Env: HTTP_PORT. Default: 8080.
	HTTPPort int
	// DatabaseHost is the PostgreSQL host. Env: DATABASE_HOST. Default: "localhost".
	DatabaseHost string
	// DatabasePort is the PostgreSQL port. Env: DATABASE_PORT. Default: 5432.
	DatabasePort int
	// DatabaseUser is the PostgreSQL user. Env: DATABASE_USER. Default: "pfm_user".
	DatabaseUser string
	// DatabasePassword is the PostgreSQL password. Env: DATABASE_PASSWORD. Default: "secret".
	DatabasePassword string
	// DatabaseName is the PostgreSQL database name. Env: DATABASE_NAME. Default: "pfm_dev".
	DatabaseName string
	// DatabaseSSLMode is the PostgreSQL SSL mode. Env: DATABASE_SSL_MODE. Default: "disable".
	DatabaseSSLMode string
	// DBMaxOpenConns is the maximum number of open connections. Env: DB_MAX_OPEN_CONNS. Default: 25.
	DBMaxOpenConns int
	// DBMaxIdleConns is the maximum number of idle connections. Env: DB_MAX_IDLE_CONNS. Default: 5.
	DBMaxIdleConns int
	// DBConnMaxLifetimeSec is the maximum connection lifetime in seconds. Env: DB_CONN_MAX_LIFETIME_SEC. Default: 300.
	DBConnMaxLifetimeSec int
	// DBConnMaxIdleTimeSec is the maximum idle connection lifetime in seconds. Env: DB_CONN_MAX_IDLE_TIME_SEC. Default: 60.
	DBConnMaxIdleTimeSec int
	// DBConnectTimeoutSec is the PostgreSQL connection timeout in seconds. Env: DB_CONNECT_TIMEOUT_SEC. Default: 5.
	DBConnectTimeoutSec int
	// DBStartupRetries is the number of ping retries at startup. Env: DB_STARTUP_RETRIES. Default: 5.
	DBStartupRetries int
	// DBStartupRetryDelaySec is the base delay between startup retries in seconds. Env: DB_STARTUP_RETRY_DELAY_SEC. Default: 2.
	DBStartupRetryDelaySec int
	// ShutdownTimeoutSec is the graceful shutdown timeout in seconds. Env: SHUTDOWN_TIMEOUT_SEC. Default: 15.
	ShutdownTimeoutSec int
	// LogLevel is the minimum log level. Env: LOG_LEVEL. Default: "info".
	LogLevel string
	// Debug enables debug mode. Env: DEBUG. Default: false.
	Debug bool
	// RequireOTEL flags whether OTEL_ENDPOINT is mandatory. Env: REQUIRE_OTEL. Default: false.
	RequireOTEL bool
	// OTELEndpoint is the OpenTelemetry collector endpoint. Env: OTEL_ENDPOINT. Default: none.
	OTELEndpoint string
	// TokenSecretKey is the hex-encoded 32-byte symmetric key for PASETO v4 tokens. Env: TOKEN_SECRET_KEY. No default.
	TokenSecretKey string
	// TokenExpirationSec is the token lifetime in seconds. Env: TOKEN_EXPIRATION_SEC. Default: 86400 (24 hours).
	TokenExpirationSec int
}

// Load reads configuration from environment variables, applying defaults for local development.
func Load() (*Config, error) {
	httpPort, err := envPort("HTTP_PORT", 8080)
	if err != nil {
		return nil, err
	}

	dbPort, err := envPort("DATABASE_PORT", 5432)
	if err != nil {
		return nil, err
	}

	dbMaxOpenConns, err := envInt("DB_MAX_OPEN_CONNS", 25)
	if err != nil {
		return nil, err
	}
	dbMaxIdleConns, err := envInt("DB_MAX_IDLE_CONNS", 5)
	if err != nil {
		return nil, err
	}
	dbConnMaxLifetimeSec, err := envInt("DB_CONN_MAX_LIFETIME_SEC", 300)
	if err != nil {
		return nil, err
	}
	dbConnMaxIdleTimeSec, err := envInt("DB_CONN_MAX_IDLE_TIME_SEC", 60)
	if err != nil {
		return nil, err
	}
	dbConnectTimeoutSec, err := envInt("DB_CONNECT_TIMEOUT_SEC", 5)
	if err != nil {
		return nil, err
	}
	dbStartupRetries, err := envInt("DB_STARTUP_RETRIES", 5)
	if err != nil {
		return nil, err
	}
	dbStartupRetryDelaySec, err := envInt("DB_STARTUP_RETRY_DELAY_SEC", 2)
	if err != nil {
		return nil, err
	}

	shutdownTimeout, err := envInt("SHUTDOWN_TIMEOUT_SEC", 15)
	if err != nil {
		return nil, err
	}

	debug, err := envBool("DEBUG", false)
	if err != nil {
		return nil, err
	}

	otelEndpoint := os.Getenv("OTEL_ENDPOINT")
	requireOTEL, err := envBool("REQUIRE_OTEL", false)
	if err != nil {
		return nil, err
	}
	if requireOTEL && otelEndpoint == "" {
		return nil, fmt.Errorf(message.ErrConfigRequired, "OTEL_ENDPOINT")
	}

	tokenExpirationSec, err := envInt("TOKEN_EXPIRATION_SEC", 86400)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		HTTPPort:               httpPort,
		DatabaseHost:           envStr("DATABASE_HOST", "localhost"),
		DatabasePort:           dbPort,
		DatabaseUser:           envStr("DATABASE_USER", "pfm_user"),
		DatabasePassword:       envStr("DATABASE_PASSWORD", "secret"),
		DatabaseName:           envStr("DATABASE_NAME", "pfm_dev"),
		DatabaseSSLMode:        envStr("DATABASE_SSL_MODE", "disable"),
		DBMaxOpenConns:         dbMaxOpenConns,
		DBMaxIdleConns:         dbMaxIdleConns,
		DBConnMaxLifetimeSec:   dbConnMaxLifetimeSec,
		DBConnMaxIdleTimeSec:   dbConnMaxIdleTimeSec,
		DBConnectTimeoutSec:    dbConnectTimeoutSec,
		DBStartupRetries:       dbStartupRetries,
		DBStartupRetryDelaySec: dbStartupRetryDelaySec,
		ShutdownTimeoutSec:     shutdownTimeout,
		LogLevel:               envStr("LOG_LEVEL", "info"),
		Debug:                  debug,
		RequireOTEL:            requireOTEL,
		OTELEndpoint:           otelEndpoint,
		TokenSecretKey:         os.Getenv("TOKEN_SECRET_KEY"),
		TokenExpirationSec:     tokenExpirationSec,
	}

	return cfg, nil
}

func (c *Config) String() string {
	return fmt.Sprintf(
		"HTTPPort=%d DatabaseHost=%s DatabasePort=%d DatabaseUser=%s DatabasePassword=**** "+
			"DatabaseName=%s DatabaseSSLMode=%s DBMaxOpenConns=%d DBMaxIdleConns=%d DBConnMaxLifetimeSec=%d "+
			"DBConnMaxIdleTimeSec=%d DBConnectTimeoutSec=%d DBStartupRetries=%d DBStartupRetryDelaySec=%d "+
			"ShutdownTimeoutSec=%d LogLevel=%s Debug=%t RequireOTEL=%t OTELEndpoint=%s "+
			"TokenSecretKey=**** TokenExpirationSec=%d",
		c.HTTPPort, c.DatabaseHost, c.DatabasePort, c.DatabaseUser,
		c.DatabaseName, c.DatabaseSSLMode, c.DBMaxOpenConns, c.DBMaxIdleConns, c.DBConnMaxLifetimeSec,
		c.DBConnMaxIdleTimeSec, c.DBConnectTimeoutSec, c.DBStartupRetries, c.DBStartupRetryDelaySec,
		c.ShutdownTimeoutSec, c.LogLevel, c.Debug, c.RequireOTEL, c.OTELEndpoint,
		c.TokenExpirationSec,
	)
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf(message.ErrConfigParse, key, err)
	}
	return n, nil
}

func envPort(key string, fallback int) (int, error) {
	n, err := envInt(key, fallback)
	if err != nil {
		return 0, err
	}
	if n < 1 || n > 65535 {
		return 0, fmt.Errorf(message.ErrConfigPortRange, key, n)
	}
	return n, nil
}

func envBool(key string, fallback bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	switch strings.ToLower(v) {
	case "true", "1", "yes":
		return true, nil
	case "false", "0", "no":
		return false, nil
	default:
		return false, fmt.Errorf(message.ErrConfigParse, key, fmt.Errorf(message.ErrConfigInvalidBool))
	}
}
