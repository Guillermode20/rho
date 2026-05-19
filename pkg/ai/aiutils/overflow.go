package aiutils

import (
	"fmt"
	"math"
)

// SafeAdd adds two int64 values with overflow detection.
// Returns the sum and a boolean indicating whether overflow occurred.
func SafeAdd(a, b int64) (int64, bool) {
	if b > 0 && a > math.MaxInt64-b {
		return math.MaxInt64, true
	}
	if b < 0 && a < math.MinInt64-b {
		return math.MinInt64, true
	}
	return a + b, false
}

// SafeMul multiplies two int64 values with overflow detection.
func SafeMul(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, false
	}
	result := a * b
	if result/b != a {
		return math.MaxInt64, true
	}
	return result, false
}

// SafeAddInt adds two int values with overflow detection.
func SafeAddInt(a, b int) (int, bool) {
	if b > 0 && a > math.MaxInt-b {
		return math.MaxInt, true
	}
	if b < 0 && a < math.MinInt-b {
		return math.MinInt, true
	}
	return a + b, false
}

// ClampInt clamps an integer to the range [min, max].
func ClampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ClampInt64 clamps an int64 to the range [min, max].
func ClampInt64(value, min, max int64) int64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ClampFloat64 clamps a float64 to the range [min, max].
func ClampFloat64(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// SafeTokenSum adds token counts with overflow detection, returning clamped value.
func SafeTokenSum(a, b int) int {
	result, overflow := SafeAddInt(a, b)
	if overflow {
		return math.MaxInt
	}
	return result
}

// SafeTokenCost adds cost values, capping at MaxFloat64.
func SafeTokenCost(a, b float64) float64 {
	result := a + b
	if math.IsInf(result, 1) || math.IsNaN(result) {
		return math.MaxFloat64
	}
	if result > math.MaxFloat64 {
		return math.MaxFloat64
	}
	return result
}

// TokenPercentage calculates what percentage `part` is of `total`, with overflow safety.
func TokenPercentage(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return (float64(part) / float64(total)) * 100.0
}

// IsValidTokenCount checks if a token count is reasonable (non-negative, not absurdly large).
func IsValidTokenCount(count int) bool {
	return count >= 0 && count < 100_000_000_000 // 100B token sanity limit
}

// IsValidCost checks if a cost value is reasonable.
func IsValidCost(cost float64) bool {
	return cost >= 0 && !math.IsInf(cost, 0) && !math.IsNaN(cost) && cost < 1_000_000_000
}

// FormatTokens formats a token count for display.
func FormatTokens(count int) string {
	switch {
	case count >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(count)/1_000_000)
	case count >= 1_000:
		return fmt.Sprintf("%.1fK", float64(count)/1_000)
	default:
		return fmt.Sprintf("%d", count)
	}
}

// FormatCost formats a monetary cost for display.
func FormatCost(cost float64) string {
	switch {
	case cost >= 1.0:
		return fmt.Sprintf("$%.2f", cost)
	case cost >= 0.01:
		return fmt.Sprintf("%.2f¢", cost*100)
	case cost >= 0.0001:
		return fmt.Sprintf("%.2f¢", cost*100)
	default:
		return "<0.01¢"
	}
}
