package message

import "errors"

// Money errors.
var (
	ErrMoneyEmpty           = errors.New("money parser: empty string")
	ErrMoneyInvalidFormat   = errors.New("money parser: invalid format")
	ErrMoneyInvalidNumber   = errors.New("money parser: invalid number")
	ErrMoneyInvalidDecimal  = errors.New("money parser: expected 2 decimal digits")
	ErrMoneyOverflow        = errors.New("money arithmetic: overflow")
	ErrMoneyInvalidCurrency = errors.New("money currency: unsupported currency")
)

// Validation errors.
const (
	MsgRequired    = "is required"
	MsgMinLen      = "must be at least %d characters"
	MsgMaxLen      = "must be at most %d characters"
	MsgRange       = "must be between %d and %d"
	MsgOneOf       = "must be one of %v"
	MsgPositive    = "must be positive"
	MsgNonNegative = "must not be negative"
)

// Config errors.
const (
	ErrConfigParse       = "config parse %s: %w"
	ErrConfigPortRange   = "config %s: must be between 1 and 65535, got %d"
	ErrConfigInvalidBool = "must be true/false, 1/0, or yes/no"
	ErrConfigRequired    = "%s is required but not set"
)

// Observe log messages.
const (
	MsgLoggerReady         = "logger initialized"
	MsgStartupInfo         = "application starting"
	MsgTracerReady         = "tracer provider initialized"
	MsgTracerShutdown      = "tracer provider shut down"
	MsgServerStarting      = "http server starting"
	MsgServerStopped       = "http server stopped"
	MsgShuttingDown        = "shutting down"
	MsgTracerShutdownError = "tracer provider shutdown error"
)

// Server messages.
const (
	MsgServerError         = "server error"
	MsgServerShutdownError = "server shutdown error"
	MsgHealthNotReady      = "service not ready"
)

// Database log messages.
const (
	MsgDBConnecting = "connecting to database"
	MsgDBPingRetry  = "database ping failed, retrying"
	MsgDBReady      = "database connection pool ready"
	MsgDBClosed     = "database connection pool closed"
	MsgDBCloseError = "database connection pool close failed"
)

// Database errors.
const (
	ErrDBOpen        = "database: open driver: %w"
	ErrDBPing        = "database: ping after %d retries: %w"
	ErrDBContextDone = "database: context cancelled during startup: %w"
	ErrDBVerifyConn  = "database: verify connection: %w"
)

// Migration log messages.
const (
	MsgMigrationsRunning  = "running database migrations"
	MsgMigrationApplied   = "migration applied"
	MsgMigrationsComplete = "database migrations complete"
)

// Migration errors.
const (
	ErrMigrateSubFS    = "migrate: create sub filesystem: %w"
	ErrMigrateProvider = "migrate: create provider: %w"
	ErrMigrateUp       = "migrate: apply pending migrations: %w"
)

// Observe errors.
const (
	ErrTracerResource = "observe: build resource: %w"
	ErrTracerExporter = "observe: create OTLP exporter: %w"
)

// Pool errors.
const (
	ErrDBParsePoolConfig = "database: parse pool config: %w"
	ErrDBNewPool         = "database: create pool: %w"
)

// Transactor log messages.
const (
	MsgTransactorRollbackFailed = "transaction rollback failed after fn error"
)

// Transactor errors.
const (
	ErrTransactorBegin  = "transactor: begin transaction: %w"
	ErrTransactorCommit = "transactor: commit transaction: %w"
)

// Password hasher errors.
const (
	ErrHasherHash   = "hasher: hash password: %w"
	ErrHasherVerify = "hasher: verify password: %w"
)

// Token service errors.
var (
	ErrTokenExpired  = errors.New("token: expired")
	ErrTokenInvalid  = errors.New("token: invalid")
)

const (
	ErrTokenServiceIssue    = "token service: issue: %w"
	ErrTokenServiceValidate = "token service: validate: %w"
	ErrTokenServiceKeyDecode = "token service: decode key: %w"
	ErrTokenServiceKeyLength = "token service: key must be 32 bytes, got %d"
)

// Authentication middleware messages.
const (
	MsgAuthnMissingToken = "missing or malformed authorization header"
	MsgAuthnExpiredToken = "token expired"
	MsgAuthnInvalidToken = "invalid token"
)

// Login errors and messages.
var (
	ErrLoginInvalidCredentials = errors.New("login: invalid credentials")
)

const (
	MsgLoginInvalidCredentials = "invalid credentials"
	MsgLoginBadRequest         = "invalid request body"
	MsgValidationFailed        = "validation failed"
	ErrLoginFindUser           = "login: find user: %w"
	ErrLoginIssueToken         = "login: issue token: %w"
	ErrLoginVerifyPassword     = "login: verify password: %w"
)

// User repository errors.
var (
	// ErrUserNotFound is returned when a user lookup by ID finds no active record.
	ErrUserNotFound = errors.New("user: not found")
	// ErrUserVersionConflict is returned when an optimistic-lock update finds a stale version.
	ErrUserVersionConflict = errors.New("user: version conflict")
	// ErrUserEmailTaken is returned when Register finds an existing user with the same email.
	ErrUserEmailTaken = errors.New("user: email already taken")
)

// User repository format strings (adapter layer).
const (
	ErrUserFindByEmail    = "user: find by email: %w"
	ErrUserFindByID       = "user: find by id: %w"
	ErrUserCreate         = "user: create: %w"
	ErrUserUpdateProfile  = "user: update profile: %w"
	ErrUserChangePassword = "user: change password: %w"
	ErrUserDeactivate     = "user: deactivate: %w"
)

// User logic format strings (domain layer).
const (
	ErrUserLogicRegister       = "user logic: register: %w"
	ErrUserLogicFindByID       = "user logic: find by id: %w"
	ErrUserLogicUpdateProfile  = "user logic: update profile: %w"
	ErrUserLogicChangePassword = "user logic: change password: %w"
	ErrUserLogicDeactivate     = "user logic: deactivate: %w"
)

// Startup errors (used by cmd/pfm/main.go composition root).
const (
	ErrRunLoadConfig = "load config: %w"
	ErrRunLogLevel   = "parse log level %q: %w"
	ErrRunTracerInit = "init tracer: %w"
	ErrRunOpenDB     = "open database: %w"
	ErrRunMigrate    = "run migrations: %w"
	ErrRunTokenKey   = "init token service: %w"
)
