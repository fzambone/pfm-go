package money

import "github.com/zambone/pfm-go/internal/message"

var supportedCurrencies = map[string]bool{
	"USD": true,
	"BRL": true,
	"EUR": true,
}

// ValidateCurrency checks if the given currency code is supported.
func ValidateCurrency(code string) error {
	if !supportedCurrencies[code] {
		return message.ErrMoneyInvalidCurrency
	}
	return nil
}
