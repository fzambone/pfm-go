package user_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	domainuser "github.com/zambone/pfm-go/internal/domain/user"
)

// TestUser_FullStruct verifies that User carries all required fields including
// display name, version, and audit fields. This is a compile-time correctness test —
// if the fields do not exist, this file will not compile.
func TestUser_FullStruct(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	callerID := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	u := domainuser.User{
		ID:           uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Email:        "user@example.com",
		DisplayName:  "Test User",
		PasswordHash: "hash",
		Version:      1,
		CreatedAt:    now,
		UpdatedAt:    now,
		CreatedBy:    callerID,
		UpdatedBy:    callerID,
	}

	if u.DisplayName != "Test User" {
		t.Errorf("DisplayName = %q, want %q", u.DisplayName, "Test User")
	}
	if u.Version != 1 {
		t.Errorf("Version = %d, want 1", u.Version)
	}
	if u.CreatedAt != now {
		t.Errorf("CreatedAt = %v, want %v", u.CreatedAt, now)
	}
	if u.UpdatedAt != now {
		t.Errorf("UpdatedAt = %v, want %v", u.UpdatedAt, now)
	}
	if u.CreatedBy != callerID {
		t.Errorf("CreatedBy = %v, want %v", u.CreatedBy, callerID)
	}
	if u.UpdatedBy != callerID {
		t.Errorf("UpdatedBy = %v, want %v", u.UpdatedBy, callerID)
	}
}

// TestRegisterInput_IsConstructible verifies the RegisterInput type exists
// with the expected fields.
func TestRegisterInput_IsConstructible(t *testing.T) {
	in := domainuser.RegisterInput{
		Email:       "new@example.com",
		DisplayName: "New User",
		Password:    "correct-horse-battery-staple",
	}

	if in.Email == "" {
		t.Error("Email must be set")
	}
	if in.DisplayName == "" {
		t.Error("DisplayName must be set")
	}
	if in.Password == "" {
		t.Error("Password must be set")
	}
}

// TestUpdateProfileInput_IsConstructible verifies the UpdateProfileInput type.
func TestUpdateProfileInput_IsConstructible(t *testing.T) {
	in := domainuser.UpdateProfileInput{
		DisplayName: "Updated Name",
	}

	if in.DisplayName == "" {
		t.Error("DisplayName must be set")
	}
}

// TestChangePasswordInput_IsConstructible verifies the ChangePasswordInput type.
func TestChangePasswordInput_IsConstructible(t *testing.T) {
	in := domainuser.ChangePasswordInput{
		OldPassword: "old-password",
		NewPassword: "new-password",
	}

	if in.OldPassword == "" {
		t.Error("OldPassword must be set")
	}
	if in.NewPassword == "" {
		t.Error("NewPassword must be set")
	}
}
