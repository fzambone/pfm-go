package money

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/zambone/pfm-go/internal/message"
)

// ParseUSD converts a USD display string to minor units (cents).
// Example: ParseUSD("1,500.99") returns 150099.
func ParseUSD(s string) (int64, error) {
	return parseAmount(s, ',', '.')
}

// ParseBRL coverts a BRL display string to minor units (centavos).
// Example: ParseBRL("1.500,99") returns 150099.
func ParseBRL(s string) (int64, error) {
	return parseAmount(s, '.', ',')
}

// ParseEUR coverts a BRL display string to minor units (cents).
// Example: ParseEUR("1.500,99") returns 150099.
func ParseEUR(s string) (int64, error) {
	return parseAmount(s, '.', ',')
}

func parseAmount(s string, thousandsSep byte, decimalSep byte) (int64, error) {
	if s == "" {
		return 0, message.ErrMoneyEmpty
	}

	negative := s[0] == '-'
	if negative {
		s = s[1:]
	}

	// Remove thousands separators
	s = strings.ReplaceAll(s, string(thousandsSep), "")

	// Split on decimal separator
	parts := strings.Split(s, string(decimalSep))
	if len(parts) > 2 {
		return 0, wrapError(message.ErrMoneyInvalidFormat, s)
	}

	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, wrapError(message.ErrMoneyInvalidNumber, s)
	}

	var fraction int64
	if len(parts) == 2 {
		frac := parts[1]
		if len(frac) != 2 {
			return 0, fmt.Errorf("%w:, got %d", message.ErrMoneyInvalidDecimal, len(frac))
		}
		fraction, err = strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, wrapError(message.ErrMoneyInvalidNumber, frac)
		}
	}

	result := whole*100 + fraction
	if negative {
		result = -result
	}
	return result, nil
}

func wrapError(sentinel error, detail string) error {
	return fmt.Errorf("%w: %q", sentinel, detail)
}
