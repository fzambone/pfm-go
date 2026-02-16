package money

// Split divides an amount into n equal parts.
// Any remainder is added to the first share to ensure the sum is exact.
// Example: Split(10000, 3) returns [3334, 3333, 3333].
func Split(amount int64, n int) []int64 {
	base := amount / int64(n)
	remainder := amount % int64(n)

	parts := make([]int64, n)
	for i := range parts {
		parts[i] = base
	}

	for i := int64(0); i < remainder; i++ {
		parts[i]++
	}

	return parts
}
