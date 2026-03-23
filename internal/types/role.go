// Package types contains shared value types that cross layer boundaries.
// Only leaf types with no business logic belong here — the domain, platform,
// and adapter layers all import this package safely.
package types

// Role represents the permission level of a household member.
// Restricted to RoleAdmin and RoleMember — matches the CHECK constraint in the DB.
type Role string

const (
	// RoleAdmin grants full management capabilities within the household.
	RoleAdmin Role = "ADMIN"
	// RoleMember grants read and limited write access within the household.
	RoleMember Role = "MEMBER"
)

// Status represents the lifecycle state of a household.
// Restricted to StatusActive and StatusInactive — matches the CHECK constraint in the DB.
type Status string

const (
	// StatusActive indicates the household is operational.
	StatusActive Status = "ACTIVE"
	// StatusInactive indicates the household has been deactivated.
	StatusInactive Status = "INACTIVE"
)
