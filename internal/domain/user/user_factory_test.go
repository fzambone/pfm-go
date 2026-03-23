package user_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	domainuser "github.com/zambone/pfm-go/internal/domain/user"
)

// userFactory returns a User with all required fields set to non-zero defaults.
// Individual tests override only the fields relevant to their scenario:
//
//	u := userFactory(func(u *domainuser.User) { u.Email = "custom@example.com" })
func userFactory(overrides ...func(*domainuser.User)) domainuser.User {
	u := domainuser.User{
		ID:           testUserID, // 00000000-0000-0000-0000-000000000001
		Email:        "user@example.com",
		DisplayName:  "Test User",
		PasswordHash: "hashed:correct-password",
		Version:      1,
		CreatedAt:    fixedTime, // 2026-01-01T00:00:00Z
		UpdatedAt:    fixedTime,
		CreatedBy:    uuid.Nil,
		UpdatedBy:    uuid.Nil,
	}
	for _, o := range overrides {
		o(&u)
	}
	return u
}

// registerInputFactory returns a valid RegisterInput with sensible defaults.
func registerInputFactory(overrides ...func(*domainuser.RegisterInput)) domainuser.RegisterInput {
	in := domainuser.RegisterInput{
		Email:       "user@example.com",
		DisplayName: "Test User",
		Password:    "correct-horse-battery-staple",
	}
	for _, o := range overrides {
		o(&in)
	}
	return in
}

// changePasswordInputFactory returns a valid ChangePasswordInput with sensible defaults.
func changePasswordInputFactory(overrides ...func(*domainuser.ChangePasswordInput)) domainuser.ChangePasswordInput {
	in := domainuser.ChangePasswordInput{
		OldPassword: "correct-horse-battery-staple",
		NewPassword: "new-correct-horse-staple",
	}
	for _, o := range overrides {
		o(&in)
	}
	return in
}

// updateProfileInputFactory returns a valid UpdateProfileInput with sensible defaults.
func updateProfileInputFactory(overrides ...func(*domainuser.UpdateProfileInput)) domainuser.UpdateProfileInput {
	in := domainuser.UpdateProfileInput{
		DisplayName: "Updated Name",
	}
	for _, o := range overrides {
		o(&in)
	}
	return in
}

// TestFactories_ProduceValidDefaults verifies that each factory returns fully
// populated values when called with no overrides.
func TestFactories_ProduceValidDefaults(t *testing.T) {
	t.Run("userFactory has non-zero fields", func(t *testing.T) {
		u := userFactory()

		assert.NotEqual(t, uuid.Nil, u.ID)
		assert.NotEmpty(t, u.Email)
		assert.NotEmpty(t, u.DisplayName)
		assert.NotEmpty(t, u.PasswordHash)
		assert.Equal(t, 1, u.Version)
		assert.False(t, u.CreatedAt.IsZero())
		assert.False(t, u.UpdatedAt.IsZero())
	})

	t.Run("registerInputFactory has non-empty fields", func(t *testing.T) {
		in := registerInputFactory()

		assert.NotEmpty(t, in.Email)
		assert.NotEmpty(t, in.DisplayName)
		assert.NotEmpty(t, in.Password)
	})

	t.Run("updateProfileInputFactory has non-empty fields", func(t *testing.T) {
		in := updateProfileInputFactory()

		assert.NotEmpty(t, in.DisplayName)
	})

	t.Run("userFactory override applies", func(t *testing.T) {
		u := userFactory(func(u *domainuser.User) { u.Email = "custom@example.com" })

		assert.Equal(t, "custom@example.com", u.Email)
	})

	t.Run("userFactory fixedTime is 2026-01-01", func(t *testing.T) {
		u := userFactory()

		assert.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), u.CreatedAt)
	})
}
