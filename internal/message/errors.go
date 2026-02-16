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
