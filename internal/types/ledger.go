package types

// EntryType represents the direction of a ledger entry.
// Restricted to EntryTypeDebit and EntryTypeCredit — matches the CHECK constraint in the DB.
type EntryType string

const (
	// EntryTypeDebit represents a debit entry (increases asset/expense accounts).
	EntryTypeDebit EntryType = "DEBIT"
	// EntryTypeCredit represents a credit entry (increases liability/income accounts).
	EntryTypeCredit EntryType = "CREDIT"
)
