package money_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/money"
)

func TestFormatUSD_StandardAmount(t *testing.T) {
	got := money.FormatUSD(150099)

	assert.Equal(t, "1,500.99", got)
}

func TestFormatUSD_Zero(t *testing.T) {
	got := money.FormatUSD(0)

	assert.Equal(t, "0.00", got)
}

func TestFormatUSD_NegativeAmount(t *testing.T) {
	got := money.FormatUSD(-150099)

	assert.Equal(t, "-1,500.99", got)
}

func TestFormatUSD_SmallAmount(t *testing.T) {
	got := money.FormatUSD(99)

	assert.Equal(t, "0.99", got)
}

func TestFormatBRL_StandardAmount(t *testing.T) {
	got := money.FormatBRL(150099)

	assert.Equal(t, "1.500,99", got)
}

func TestFormatEUR_StandardAmount(t *testing.T) {
	got := money.FormatEUR(150099)

	assert.Equal(t, "1.500,99", got)
}

func TestParseUSD_StandardAmount(t *testing.T) {
	got, err := money.ParseUSD("1,500.99")

	assert.NoError(t, err)
	assert.Equal(t, int64(150099), got)
}

func TestParseUSD_Zero(t *testing.T) {
	got, err := money.ParseUSD("0.00")

	assert.NoError(t, err)
	assert.Equal(t, int64(0), got)
}

func TestParseUSD_NegativeAmount(t *testing.T) {
	got, err := money.ParseUSD("-1,500.99")

	assert.NoError(t, err)
	assert.Equal(t, int64(-150099), got)
}

func TestParseUSD_SmallAmount(t *testing.T) {
	got, err := money.ParseUSD("0.99")

	assert.NoError(t, err)
	assert.Equal(t, int64(99), got)
}

func TestParseUSD_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  error
	}{
		{"empty string", "", message.ErrMoneyEmpty},
		{"non-numeric input", "abc", message.ErrMoneyInvalidNumber},
		{"too many decimals", "1.500.99", message.ErrMoneyInvalidFormat},
		{"one decimal digit", "15.9", message.ErrMoneyInvalidDecimal},
		{"three decimal digits", "15.999", message.ErrMoneyInvalidDecimal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := money.ParseUSD(tt.input)

			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestSplit_EvenDivision(t *testing.T) {
	got := money.Split(10000, 4)

	assert.Equal(t, []int64{2500, 2500, 2500, 2500}, got)
}

func TestSplit_UnevenDivision_RemainderToFirst(t *testing.T) {
	got := money.Split(10000, 3)

	assert.Equal(t, []int64{3334, 3333, 3333}, got)
}

func TestAdd_StandardAmount(t *testing.T) {
	got, err := money.Add(10000, 5000)

	assert.NoError(t, err)
	assert.Equal(t, int64(15000), got)
}

func TestAdd_Overflow_ReturnsError(t *testing.T) {
	_, err := money.Add(math.MaxInt64, 1)

	assert.Error(t, err)
	assert.ErrorIs(t, err, message.ErrMoneyOverflow)
}

func TestSub_StandardAmount(t *testing.T) {
	got, err := money.Sub(10000, 3000)

	assert.NoError(t, err)
	assert.Equal(t, int64(7000), got)
}

func TestValidateCurrency_ValidCodes(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{"USD", "USD"},
		{"BRL", "BRL"},
		{"EUR", "EUR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, money.ValidateCurrency(tt.code))
		})
	}
}

func TestValidateCurrency_InvalidCode(t *testing.T) {
	err := money.ValidateCurrency("GBP")

	assert.ErrorIs(t, err, message.ErrMoneyInvalidCurrency)
}
