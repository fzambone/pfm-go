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
