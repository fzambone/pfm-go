package validate_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zambone/pfm-go/internal/platform/validate"
)

func TestField_Required_RejectsEmptyString(t *testing.T) {
	r := validate.NewResult()

	r.Field("name", "", validate.Required)

	assert.True(t, r.HasViolations())
	assert.Equal(t, 1, len(r.Violations()))
	assert.Equal(t, "name", r.Violations()[0].Field)
}

func TestField_MultipleRules_CollectsAllViolations(t *testing.T) {
	r := validate.NewResult()

	r.Field("name", "", validate.Required, validate.MinLen(3))

	assert.Equal(t, 2, len(r.Violations()))
}

func TestField_MaxLen_RejectsLongString(t *testing.T) {
	r := validate.NewResult()

	r.Field("name", "this is way too long", validate.MaxLen(10))

	assert.True(t, r.HasViolations())
	assert.Equal(t, "name", r.Violations()[0].Field)
}

func TestField_Range_RejectsOutOfBounds(t *testing.T) {
	tests := []struct {
		name  string
		value int
		min   int
		max   int
	}{
		{"below minimum", 0, 1, 31},
		{"above maximum", 32, 1, 31},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := validate.NewResult()

			r.Field("closing_day", tt.value, validate.Range(tt.min, tt.max))

			assert.True(t, r.HasViolations())
		})
	}
}

func TestField_OneOf_RejectsInvalidValue(t *testing.T) {
	r := validate.NewResult()

	r.Field("account_type", "WALLET", validate.OneOf("CHECKING", "SAVINGS", "CREDIT_CARD", "INVESTMENT"))

	assert.True(t, r.HasViolations())
}

func TestField_Positive_RejectsZeroAndNegative(t *testing.T) {
	tests := []struct {
		name  string
		value int64
	}{
		{"zero", 0},
		{"negative", -100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := validate.NewResult()

			r.Field("amount", tt.value, validate.Positive)

			assert.True(t, r.HasViolations())
		})
	}
}

func TestField_NonNegative_AllowsZeroRejectsNegative(t *testing.T) {
	r := validate.NewResult()
	r.Field("balance", int64(0), validate.NonNegative)
	assert.False(t, r.HasViolations())

	r2 := validate.NewResult()
	r2.Field("balance", int64(-1), validate.NonNegative)
	assert.True(t, r2.HasViolations())
}

func TestResult_Error_ReturnsNilWhenNoViolations(t *testing.T) {
	r := validate.NewResult()

	assert.NoError(t, r.Error())
}

func TestResult_Field_ReturnsValidationErrorWhenViolations(t *testing.T) {
	r := validate.NewResult()
	r.Field("name", "", validate.Required)

	err := r.Error()

	assert.Error(t, err)
	var validationErr *validate.ValidationError
	assert.ErrorAs(t, err, &validationErr)
	assert.Equal(t, 1, len(validationErr.Violations))
}

func TestField_NestedPath_IncludesFullPath(t *testing.T) {
	r := validate.NewResult()

	r.Field("credit_card_settings.closing_day", 0, validate.Range(1, 31))

	assert.Equal(t, "credit_card_settings.closing_day", r.Violations()[0].Field)
}

func TestRules_SafeOnUnexpectedTypes(t *testing.T) {
	r := validate.NewResult()

	assert.NotPanics(t, func() {
		r.Field("a", true, validate.Required)
		r.Field("b", true, validate.MinLen(3))
		r.Field("c", true, validate.MaxLen(10))
		r.Field("d", true, validate.Range(1, 31))
		r.Field("e", true, validate.Positive)
		r.Field("f", true, validate.NonNegative)
		r.Field("g", 123, validate.OneOf("a", "b"))
	})
}

func TestField_Email_AcceptsValidAddresses(t *testing.T) {
	cases := []string{
		"user@example.com",
		"user+tag@example.co.uk",
		"first.last@subdomain.example.com",
	}
	for _, addr := range cases {
		r := validate.NewResult()
		r.Field("email", addr, validate.Email)
		assert.False(t, r.HasViolations(), "expected %q to be valid", addr)
	}
}

func TestField_Email_RejectsInvalidAddresses(t *testing.T) {
	cases := []string{
		"notanemail",
		"missing-at-sign",
		"@nodomain",
		"nolocal@",
		"",
	}
	for _, addr := range cases {
		r := validate.NewResult()
		r.Field("email", addr, validate.Email)
		assert.True(t, r.HasViolations(), "expected %q to be invalid", addr)
	}
}
