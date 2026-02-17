package validate

// Rule is a function that validates a value and returns an error message if invalid.
// An empty string means the value is valid.
type Rule func(value any) string

// Field validates a value against one or more rules, collecting all violations.
func (r *Result) Field(field string, value any, rules ...Rule) {
	for _, rule := range rules {
		if msg := rule(value); msg != "" {
			r.violations = append(r.violations, Violation{
				Field:   field,
				Message: msg,
			})
		}
	}
}
