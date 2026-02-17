package validate

import (
	"fmt"

	"github.com/zambone/pfm-go/internal/message"
)

// Required rejects empty strings, zero values, and nil pointers.
func Required(value any) string {
	switch v := value.(type) {
	case string:
		if v == "" {
			return message.MsgRequired
		}
	case int:
		if v == 0 {
			return message.MsgRequired
		}
	case int64:
		if v == 0 {
			return message.MsgRequired
		}
	case nil:
		return message.MsgRequired
	}
	return ""
}

// MinLen rejects strings shorter than n characters (Unicode-aware).
func MinLen(n int) Rule {
	return func(value any) string {
		if s, ok := value.(string); ok {
			if len([]rune(s)) < n {
				return fmt.Sprintf(message.MsgMinLen, n)
			}
		}
		return ""
	}
}

// MaxLen rejects strings longer than n characters (Unicode-aware).
func MaxLen(n int) Rule {
	return func(value any) string {
		if s, ok := value.(string); ok {
			if len([]rune(s)) > n {
				return fmt.Sprintf(message.MsgMaxLen, n)
			}
		}
		return ""
	}
}

// Range rejects integers outside the given min/max bounds (inclusive).
func Range(min, max int) Rule {
	return func(value any) string {
		switch v := value.(type) {
		case int:
			if v < min || v > max {
				return fmt.Sprintf(message.MsgRange, min, max)
			}
		case int64:
			if v < int64(min) || v > int64(max) {
				return fmt.Sprintf(message.MsgRange, min, max)
			}
		}
		return ""
	}
}

// OneOf rejects values not in the allowed set.
func OneOf(allowed ...string) Rule {
	return func(value any) string {
		if s, ok := value.(string); ok {
			for _, a := range allowed {
				if s == a {
					return ""
				}

			}
			return fmt.Sprintf(message.MsgOneOf, allowed)
		}
		return ""
	}
}

// Positive rejects values that are zero or negative.
func Positive(value any) string {
	switch v := value.(type) {
	case int:
		if v <= 0 {
			return message.MsgPositive
		}
	case int64:
		if v <= 0 {
			return message.MsgPositive
		}
	}
	return ""
}

// NonNegative rejects values that are negative only.
func NonNegative(value any) string {
	switch v := value.(type) {
	case int:
		if v < 0 {
			return message.MsgNonNegative
		}
	case int64:
		if v < 0 {
			return message.MsgNonNegative
		}
	}
	return ""
}
