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
	MsgEmail       = "must be a valid email address"
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

// Authorization middleware messages.
const (
	MsgAuthzUnauthenticated = "authentication required"
	MsgAuthzForbidden       = "you are not a member of this household"
	MsgAuthzNotFound        = "household not found"
	MsgAuthzBadRequest      = "invalid household ID in URL"
	MsgAuthzAdminRequired   = "admin role required"
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

// Household repository errors.
var (
	// ErrHouseholdNotFound is returned when a household lookup by ID finds no active record.
	ErrHouseholdNotFound = errors.New("household: not found")
	// ErrHouseholdVersionConflict is returned when an optimistic-lock update finds a stale version.
	ErrHouseholdVersionConflict = errors.New("household: version conflict")
	// ErrHouseholdMemberExists is returned when adding a member who already has an active membership.
	ErrHouseholdMemberExists = errors.New("household: member already exists")
	// ErrHouseholdMemberNotFound is returned when a membership lookup finds no active record.
	ErrHouseholdMemberNotFound = errors.New("household: member not found")
	// ErrHouseholdNotAdmin is returned when a non-ADMIN attempts an admin-only operation.
	ErrHouseholdNotAdmin = errors.New("household: caller is not an admin")
	// ErrHouseholdLastAdmin is returned when removing the last ADMIN would leave the household without one.
	ErrHouseholdLastAdmin = errors.New("household: cannot remove last admin")
)

// Household repository format strings (adapter layer).
const (
	ErrHouseholdCreate       = "household: create: %w"
	ErrHouseholdFindByID     = "household: find by id: %w"
	ErrHouseholdListForUser  = "household: list for user: %w"
	ErrHouseholdAddMember    = "household: add member: %w"
	ErrHouseholdRemoveMember = "household: remove member: %w"
	ErrHouseholdUpdateName   = "household: update name: %w"
	ErrHouseholdDeactivate       = "household: deactivate: %w"
	ErrHouseholdFindMembership   = "household: find membership: %w"
	ErrHouseholdListMembers      = "household: list members: %w"
)

// Household logic format strings (domain layer).
const (
	ErrHouseholdLogicCreate       = "household logic: create: %w"
	ErrHouseholdLogicFindByID     = "household logic: find by id: %w"
	ErrHouseholdLogicListForUser  = "household logic: list for user: %w"
	ErrHouseholdLogicAddMember    = "household logic: add member: %w"
	ErrHouseholdLogicRemoveMember = "household logic: remove member: %w"
	ErrHouseholdLogicUpdateName   = "household logic: update name: %w"
	ErrHouseholdLogicDeactivate   = "household logic: deactivate: %w"
	ErrHouseholdLogicCreateUser        = "household logic: create household user: %w"
	ErrHouseholdLogicCreateUserStep    = "household logic: create user step: %w"
	ErrHouseholdLogicAddMemberStep     = "household logic: add member step: %w"
)

// Account repository errors.
var (
	// ErrAccountNotFound is returned when an account lookup by ID finds no active record.
	ErrAccountNotFound = errors.New("account: not found")
	// ErrAccountVersionConflict is returned when an optimistic-lock update finds a stale version.
	ErrAccountVersionConflict = errors.New("account: version conflict")
	// ErrAccountNameTaken is returned when creating or renaming an account to a name already
	// used by another active account in the same household.
	ErrAccountNameTaken = errors.New("account: name already taken in household")
	// ErrAccountBalanceNotZero is returned when deactivating an account that still has a balance.
	ErrAccountBalanceNotZero = errors.New("account: balance must be zero to deactivate")
)

// Account repository format strings (adapter layer).
const (
	ErrAccountCreate         = "account: create: %w"
	ErrAccountFindByID       = "account: find by id: %w"
	ErrAccountListForHouse   = "account: list for household: %w"
	ErrAccountUpdateName     = "account: update name: %w"
	ErrAccountUpdateBalance  = "account: update balance: %w"
	ErrAccountDeactivate     = "account: deactivate: %w"
)

// Account logic format strings (domain layer).
const (
	ErrAccountLogicCreate        = "account logic: create: %w"
	ErrAccountLogicFindByID      = "account logic: find by id: %w"
	ErrAccountLogicListForHouse  = "account logic: list for household: %w"
	ErrAccountLogicUpdateName    = "account logic: update name: %w"
	ErrAccountLogicUpdateBalance = "account logic: update balance: %w"
	ErrAccountLogicDeactivate    = "account logic: deactivate: %w"
)

// Credit card settings repository errors.
var (
	// ErrCreditCardSettingsNotFound is returned when settings lookup finds no active record.
	ErrCreditCardSettingsNotFound = errors.New("credit card settings: not found")
	// ErrCreditCardSettingsExists is returned when creating settings for an account that already has them.
	ErrCreditCardSettingsExists = errors.New("credit card settings: already exist for account")
	// ErrCreditCardSettingsVersionConflict is returned when an optimistic-lock update finds a stale version.
	ErrCreditCardSettingsVersionConflict = errors.New("credit card settings: version conflict")
	// ErrCreditCardSettingsNotCreditCard is returned when creating settings for a non-credit-card account.
	ErrCreditCardSettingsNotCreditCard = errors.New("credit card settings: account is not a credit card")
)

// Credit card settings repository format strings (adapter layer).
const (
	ErrCCSettingsCreate         = "credit card settings: create: %w"
	ErrCCSettingsFindByAccount  = "credit card settings: find by account: %w"
	ErrCCSettingsUpdateClosing  = "credit card settings: update closing day: %w"
	ErrCCSettingsUpdateDueDay   = "credit card settings: update due day: %w"
	ErrCCSettingsUpdateLimit    = "credit card settings: update limit: %w"
	ErrCCSettingsDelete         = "credit card settings: delete: %w"
)

// Credit card settings logic format strings (domain layer).
const (
	ErrCCLogicCreate        = "credit card logic: create: %w"
	ErrCCLogicFindByAccount = "credit card logic: find by account: %w"
	ErrCCLogicUpdateClosing = "credit card logic: update closing day: %w"
	ErrCCLogicUpdateDueDay  = "credit card logic: update due day: %w"
	ErrCCLogicUpdateLimit   = "credit card logic: update limit: %w"
	ErrCCLogicDelete        = "credit card logic: delete: %w"
)

// Ledger repository errors.
var (
	// ErrLedgerUnbalanced is returned when a transaction's entries do not balance (debits != credits).
	ErrLedgerUnbalanced = errors.New("ledger: transaction entries do not balance")
)

// Ledger repository format strings (adapter layer).
const (
	ErrLedgerPostTransaction = "ledger: post transaction: %w"
	ErrLedgerGetBalance      = "ledger: get balance: %w"
	ErrLedgerGetHistory      = "ledger: get transaction history: %w"
)

// Ledger logic format strings (domain layer).
const (
	ErrLedgerLogicPost       = "ledger logic: post transaction: %w"
	ErrLedgerLogicGetBalance = "ledger logic: get balance: %w"
	ErrLedgerLogicGetHistory = "ledger logic: get history: %w"
)

// HTTP response messages (used by adapter/http response infrastructure).
const (
	MsgBadRequestBody   = "invalid request body"
	MsgNotFound         = "resource not found"
	MsgConflict         = "resource conflict"
	MsgInternalError    = "internal server error"
	MsgUnbalancedLedger = "transaction entries do not balance"
	MsgBalanceNotZero   = "account balance must be zero to deactivate"
	MsgNotCreditCard    = "account is not a credit card"
	MsgLastAdmin        = "cannot remove last admin"
)

// Startup errors (used by cmd/pfm/main.go composition root).
const (
	ErrRunLoadConfig = "load config: %w"
	ErrRunSeed       = "run seed: %w"
	ErrRunLogLevel   = "parse log level %q: %w"
	ErrRunTracerInit = "init tracer: %w"
	ErrRunOpenDB     = "open database: %w"
	ErrRunMigrate    = "run migrations: %w"
	ErrRunTokenKey   = "init token service: %w"
)

// Seed tool messages and errors (used by cmd/pfm-seed).
const (
	MsgSeedStarting        = "starting seed tool"
	MsgSeedAlreadySeeded   = "database already contains users — skipping seed"
	MsgSeedSuccess         = "seed complete"
	ErrSeedConfig          = "seed config: %w"
	ErrSeedCheckExists     = "seed: check existing users: %w"
	ErrSeedHashPassword    = "seed: hash password: %w"
	ErrSeedCreateUser      = "seed: create user: %w"
	ErrSeedCreateHousehold = "seed: create household: %w"
	ErrSeedAddMember       = "seed: add admin member: %w"
	ErrSeedTransaction     = "seed: transaction: %w"
)
