package validate

// Violation represents a single validation failure.
type Violation struct {
	Field   string
	Message string
}

// Result collects validation violations.
type Result struct {
	violations []Violation
}

// NewResult creates an empty validation result.
func NewResult() *Result {
	return &Result{}
}

// HasViolations returns true if any violations are collected.
func (r *Result) HasViolations() bool {
	return len(r.violations) > 0
}

// Violations returns all collected violations.
func (r *Result) Violations() []Violation {
	return r.violations
}

// ValidationError carries all validation as a structured error.
type ValidationError struct {
	Violations []Violation
}

// Error returns a human-readable summary of all violations.
func (e *ValidationError) Error() string {
	msg := "validation failed:"
	for _, v := range e.Violations {
		msg += " " + v.Field + " " + v.Message + ";"
	}
	return msg
}

// Error returns nil if no violations, or a ValidationError if there are violations.
func (r *Result) Error() error {
	if !r.HasViolations() {
		return nil
	}
	return &ValidationError{Violations: r.violations}
}
