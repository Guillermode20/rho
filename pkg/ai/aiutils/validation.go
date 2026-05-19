package aiutils

import (
	"fmt"
	"math"
	"strings"
)

// ValidateString checks that a string parameter is non-empty, with optional max length.
func ValidateString(value, name string, maxLen int) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	if maxLen > 0 && len(value) > maxLen {
		return fmt.Errorf("%s exceeds maximum length of %d", name, maxLen)
	}
	return nil
}

// ValidateInt checks that an int is within a range.
func ValidateInt(value int, name string, min, max int) error {
	if value < min || value > max {
		return fmt.Errorf("%s must be between %d and %d, got %d", name, min, max, value)
	}
	return nil
}

// ValidateFloat checks that a float is within a range.
func ValidateFloat(value float64, name string, min, max float64) error {
	if value < min || value > max {
		return fmt.Errorf("%s must be between %f and %f, got %f", name, min, max, value)
	}
	return nil
}

// ValidateTemperature checks that temperature is in [0, 2].
func ValidateTemperature(temp float64) error {
	if temp < 0 || temp > 2 {
		return fmt.Errorf("temperature must be between 0 and 2, got %f", temp)
	}
	return nil
}

// ValidateMaxTokens checks that max_tokens is reasonable.
func ValidateMaxTokens(maxTokens int) error {
	if maxTokens < 1 {
		return fmt.Errorf("max_tokens must be positive, got %d", maxTokens)
	}
	if maxTokens > 200000 {
		return fmt.Errorf("max_tokens %d exceeds maximum of 200000", maxTokens)
	}
	return nil
}

// ValidateTopP checks that top_p is in [0, 1].
func ValidateTopP(topP float64) error {
	if topP < 0 || topP > 1 {
		return fmt.Errorf("top_p must be between 0 and 1, got %f", topP)
	}
	return nil
}

// ValidateAPIKey checks that an API key looks reasonable.
func ValidateAPIKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("API key is required")
	}
	if len(key) < 8 {
		return fmt.Errorf("API key is too short (%d chars, expected at least 8)", len(key))
	}
	return nil
}

// ValidateModelName checks that a model name is non-empty and reasonable length.
func ValidateModelName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("model name is required")
	}
	if len(name) > 200 {
		return fmt.Errorf("model name is too long (%d chars, max 200)", len(name))
	}
	return nil
}

// ValidateToolName checks tool name constraints.
func ValidateToolName(name string) error {
	if err := ValidateString(name, "tool name", 64); err != nil {
		return err
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return fmt.Errorf("tool name %q contains invalid character %q (only alphanumeric, _, - allowed)", name, string(r))
		}
	}
	return nil
}

// ClampTemperature clamps and returns a valid temperature.
func ClampTemperature(temp float64) float64 {
	return ClampFloat64(temp, 0, 2)
}

// ClampMaxTokens clamps and returns a valid max_tokens.
func ClampMaxTokens(tokens int) int {
	if tokens < 1 {
		return 1
	}
	if tokens > 200000 {
		return 200000
	}
	return tokens
}

// ClampTopP clamps and returns a valid top_p.
func ClampTopP(topP float64) float64 {
	return ClampFloat64(topP, 0, 1)
}

// MinInt returns the minimum of two integers.
func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MaxInt returns the maximum of two integers.
func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// EnsureSlice ensures a slice has at least the given capacity.
func EnsureSlice[T any](s []T, capacity int) []T {
	if cap(s) < capacity {
		newSlice := make([]T, len(s), capacity)
		copy(newSlice, s)
		return newSlice
	}
	return s
}

// SafeDivide performs division, returning 0 for zero denominator.
func SafeDivide(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

// RoundTo rounds a float64 to the given number of decimal places.
func RoundTo(value float64, places int) float64 {
	shift := math.Pow(10, float64(places))
	return math.Round(value*shift) / shift
}
