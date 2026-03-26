package creditcard_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/zambone/pfm-go/internal/domain/creditcard"
)

var (
	fixedTime     = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	testSettingsID = uuid.MustParse("00000000-0000-0000-0000-000000000030")
	testAccountID  = uuid.MustParse("00000000-0000-0000-0000-000000000020")
	testCallerID   = uuid.MustParse("00000000-0000-0000-0000-000000000001")
)

// settingsFactory returns a Settings with all required fields set to non-zero defaults.
func settingsFactory(overrides ...func(*creditcard.Settings)) creditcard.Settings {
	s := creditcard.Settings{
		ID:          testSettingsID,
		AccountID:   testAccountID,
		ClosingDay:  25,
		DueDay:      10,
		LimitAmount: 500000, // $5,000.00 in cents
		Version:     1,
		CreatedAt:   fixedTime,
		UpdatedAt:   fixedTime,
		CreatedBy:   testCallerID,
		UpdatedBy:   testCallerID,
	}
	for _, o := range overrides {
		o(&s)
	}
	return s
}

// createInputFactory returns a valid CreateInput with sensible defaults.
func createInputFactory(overrides ...func(*creditcard.CreateInput)) creditcard.CreateInput {
	in := creditcard.CreateInput{
		ClosingDay:  25,
		DueDay:      10,
		LimitAmount: 500000,
	}
	for _, o := range overrides {
		o(&in)
	}
	return in
}

// updateClosingDayInputFactory returns a valid UpdateClosingDayInput with sensible defaults.
func updateClosingDayInputFactory(overrides ...func(*creditcard.UpdateClosingDayInput)) creditcard.UpdateClosingDayInput {
	in := creditcard.UpdateClosingDayInput{
		ClosingDay: 15,
	}
	for _, o := range overrides {
		o(&in)
	}
	return in
}

// updateDueDayInputFactory returns a valid UpdateDueDayInput with sensible defaults.
func updateDueDayInputFactory(overrides ...func(*creditcard.UpdateDueDayInput)) creditcard.UpdateDueDayInput {
	in := creditcard.UpdateDueDayInput{
		DueDay: 5,
	}
	for _, o := range overrides {
		o(&in)
	}
	return in
}

// updateLimitInputFactory returns a valid UpdateLimitInput with sensible defaults.
func updateLimitInputFactory(overrides ...func(*creditcard.UpdateLimitInput)) creditcard.UpdateLimitInput {
	in := creditcard.UpdateLimitInput{
		LimitAmount: 1000000, // $10,000.00 in cents
	}
	for _, o := range overrides {
		o(&in)
	}
	return in
}

// TestFactories_ProduceValidDefaults verifies that each factory returns fully
// populated values when called with no overrides.
func TestFactories_ProduceValidDefaults(t *testing.T) {
	t.Run("settingsFactory has non-zero fields", func(t *testing.T) {
		s := settingsFactory()

		assert.NotEqual(t, uuid.Nil, s.ID)
		assert.NotEqual(t, uuid.Nil, s.AccountID)
		assert.Equal(t, 25, s.ClosingDay)
		assert.Equal(t, 10, s.DueDay)
		assert.Equal(t, int64(500000), s.LimitAmount)
		assert.Equal(t, 1, s.Version)
		assert.False(t, s.CreatedAt.IsZero())
		assert.False(t, s.UpdatedAt.IsZero())
		assert.NotEqual(t, uuid.Nil, s.CreatedBy)
	})

	t.Run("settingsFactory override applies", func(t *testing.T) {
		s := settingsFactory(func(s *creditcard.Settings) { s.ClosingDay = 1 })

		assert.Equal(t, 1, s.ClosingDay)
	})

	t.Run("createInputFactory has valid defaults", func(t *testing.T) {
		in := createInputFactory()

		assert.Equal(t, 25, in.ClosingDay)
		assert.Equal(t, 10, in.DueDay)
		assert.Equal(t, int64(500000), in.LimitAmount)
	})

	t.Run("updateClosingDayInputFactory has valid default", func(t *testing.T) {
		in := updateClosingDayInputFactory()

		assert.Equal(t, 15, in.ClosingDay)
	})

	t.Run("updateDueDayInputFactory has valid default", func(t *testing.T) {
		in := updateDueDayInputFactory()

		assert.Equal(t, 5, in.DueDay)
	})

	t.Run("updateLimitInputFactory has valid default", func(t *testing.T) {
		in := updateLimitInputFactory()

		assert.Equal(t, int64(1000000), in.LimitAmount)
	})

	t.Run("settingsFactory fixedTime is 2026-01-01", func(t *testing.T) {
		s := settingsFactory()

		assert.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), s.CreatedAt)
	})
}
