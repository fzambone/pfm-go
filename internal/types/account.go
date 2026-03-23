package types

// AccountType represents the kind of financial account.
// Restricted to four values — matches the CHECK constraint in the DB.
type AccountType string

const (
	// AccountTypeChecking is a standard checking/current account.
	AccountTypeChecking AccountType = "CHECKING"
	// AccountTypeSavings is a savings account.
	AccountTypeSavings AccountType = "SAVINGS"
	// AccountTypeCreditCard is a credit card account.
	AccountTypeCreditCard AccountType = "CREDIT_CARD"
	// AccountTypeInvestment is an investment/brokerage account.
	AccountTypeInvestment AccountType = "INVESTMENT"
)

// CurrencyCode represents a supported currency in ISO 4217 format.
// Restricted to three values — matches the CHECK constraint in the DB.
type CurrencyCode string

const (
	// CurrencyUSD is United States Dollar.
	CurrencyUSD CurrencyCode = "USD"
	// CurrencyBRL is Brazilian Real.
	CurrencyBRL CurrencyCode = "BRL"
	// CurrencyEUR is Euro.
	CurrencyEUR CurrencyCode = "EUR"
)
