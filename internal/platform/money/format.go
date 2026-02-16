package money

import "fmt"

// FormatUSD converts a minor-unit amount to US Dollar display format.
// Example: FormatUSD(150099) returns "1,500.99".
func FormatUSD(amount int64) string {
	return formatAmount(amount, ',', '.')
}

// FormatBRL converts a minor-unit amount to Brazilian Real display format.
// Example: FormatBRL(150099) returns "1.500,99".
func FormatBRL(amount int64) string {
	return formatAmount(amount, '.', ',')
}

// FormatEUR converts a minor-unit amount to Brazilian Real display format.
// Example: FormatEUR(150099) returns "1.500,99".
func FormatEUR(amount int64) string {
	return formatAmount(amount, '.', ',')
}

func formatAmount(amount int64, thousandsSep byte, decimalSep byte) string {
	negative := amount < 0
	if negative {
		amount = -amount
	}

	whole := amount / 100
	fraction := amount % 100

	result := fmt.Sprintf("%d%c%02d", whole, decimalSep, fraction)
	result = addThousandsSeparator(result, thousandsSep, decimalSep)

	if negative {
		return "-" + result
	}
	return result
}

func addThousandsSeparator(s string, sep byte, decimal byte) string {
	dotIndex := len(s)
	for i, c := range s {
		if byte(c) == decimal {
			dotIndex = i
			break
		}
	}

	intPart := s[:dotIndex]
	decPart := s[dotIndex:]

	if len(intPart) <= 3 {
		return intPart + decPart
	}

	var result []byte
	for i, c := range intPart {
		remaining := len(intPart) - i
		if remaining%3 == 0 && i > 0 {
			result = append(result, sep)
		}
		result = append(result, byte(c))
	}

	return string(result) + decPart
}
