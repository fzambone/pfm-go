package money

import (
	"math"

	"github.com/zambone/pfm-go/internal/message"
)

func Add(a, b int64) (int64, error) {
	if b > 0 && a > math.MaxInt64-b {
		return 0, message.ErrMoneyOverflow
	}
	if b < 0 && a < math.MinInt64-b {
		return 0, message.ErrMoneyOverflow
	}
	return a + b, nil
}

func Sub(a, b int64) (int64, error) {
	return Add(a, -b)
}
