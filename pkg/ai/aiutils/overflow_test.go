package aiutils

import (
	"math"
	"testing"
)

func TestSafeAdd(t *testing.T) {
	tests := []struct {
		name         string
		a, b         int64
		expectedVal  int64
		expectedOver bool
	}{
		{"No overflow normal", 5, 10, 15, false},
		{"No overflow negative", -5, -10, -15, false},
		{"Overflow MaxInt64", math.MaxInt64 - 2, 5, math.MaxInt64, true},
		{"Underflow MinInt64", math.MinInt64 + 2, -5, math.MinInt64, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, over := SafeAdd(tt.a, tt.b)
			if val != tt.expectedVal || over != tt.expectedOver {
				t.Errorf("SafeAdd(%d, %d) = (%d, %v), expected (%d, %v)", tt.a, tt.b, val, over, tt.expectedVal, tt.expectedOver)
			}
		})
	}
}

func TestSafeMul(t *testing.T) {
	tests := []struct {
		name         string
		a, b         int64
		expectedVal  int64
		expectedOver bool
	}{
		{"Zero multiplication", 5, 0, 0, false},
		{"Normal positive", 5, 10, 50, false},
		{"Overflow case", math.MaxInt64 / 2, 3, math.MaxInt64, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, over := SafeMul(tt.a, tt.b)
			if val != tt.expectedVal || over != tt.expectedOver {
				t.Errorf("SafeMul(%d, %d) = (%d, %v), expected (%d, %v)", tt.a, tt.b, val, over, tt.expectedVal, tt.expectedOver)
			}
		})
	}
}

func TestSafeAddInt(t *testing.T) {
	tests := []struct {
		name         string
		a, b         int
		expectedVal  int
		expectedOver bool
	}{
		{"No overflow normal", 5, 10, 15, false},
		{"Overflow MaxInt", math.MaxInt - 2, 5, math.MaxInt, true},
		{"Underflow MinInt", math.MinInt + 2, -5, math.MinInt, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, over := SafeAddInt(tt.a, tt.b)
			if val != tt.expectedVal || over != tt.expectedOver {
				t.Errorf("SafeAddInt(%d, %d) = (%d, %v), expected (%d, %v)", tt.a, tt.b, val, over, tt.expectedVal, tt.expectedOver)
			}
		})
	}
}

func TestClampOverflow(t *testing.T) {
	if val := ClampInt(5, 1, 10); val != 5 {
		t.Errorf("expected 5, got %d", val)
	}
	if val := ClampInt(0, 1, 10); val != 1 {
		t.Errorf("expected 1, got %d", val)
	}
	if val := ClampInt(12, 1, 10); val != 10 {
		t.Errorf("expected 10, got %d", val)
	}

	if val := ClampInt64(5, 1, 10); val != 5 {
		t.Errorf("expected 5, got %d", val)
	}
	if val := ClampInt64(0, 1, 10); val != 1 {
		t.Errorf("expected 1, got %d", val)
	}
	if val := ClampInt64(12, 1, 10); val != 10 {
		t.Errorf("expected 10, got %d", val)
	}

	if val := ClampFloat64(5.5, 1.0, 10.0); val != 5.5 {
		t.Errorf("expected 5.5, got %f", val)
	}
	if val := ClampFloat64(0.5, 1.0, 10.0); val != 1.0 {
		t.Errorf("expected 1.0, got %f", val)
	}
	if val := ClampFloat64(12.5, 1.0, 10.0); val != 10.0 {
		t.Errorf("expected 10.0, got %f", val)
	}
}

func TestSafeTokenSum(t *testing.T) {
	if val := SafeTokenSum(100, 200); val != 300 {
		t.Errorf("expected 300, got %d", val)
	}
	if val := SafeTokenSum(math.MaxInt, 5); val != math.MaxInt {
		t.Errorf("expected MaxInt, got %d", val)
	}
}

func TestSafeTokenCost(t *testing.T) {
	if val := SafeTokenCost(1.5, 2.5); val != 4.0 {
		t.Errorf("expected 4.0, got %f", val)
	}
	if val := SafeTokenCost(math.MaxFloat64, 1e308); val != math.MaxFloat64 {
		t.Errorf("expected MaxFloat64, got %f", val)
	}
	if val := SafeTokenCost(math.Inf(1), 1.0); val != math.MaxFloat64 {
		t.Errorf("expected MaxFloat64, got %f", val)
	}
}

func TestTokenPercentage(t *testing.T) {
	if val := TokenPercentage(25, 100); val != 25.0 {
		t.Errorf("expected 25.0, got %f", val)
	}
	if val := TokenPercentage(25, 0); val != 0.0 {
		t.Errorf("expected 0.0, got %f", val)
	}
}

func TestIsValidTokenCount(t *testing.T) {
	if !IsValidTokenCount(500) {
		t.Error("expected 500 to be a valid token count")
	}
	if IsValidTokenCount(-1) {
		t.Error("expected negative count to be invalid")
	}
	if IsValidTokenCount(100_000_000_000 + 1) {
		t.Error("expected very large count to be invalid")
	}
}

func TestIsValidCost(t *testing.T) {
	if !IsValidCost(0.50) {
		t.Error("expected 0.50 to be valid")
	}
	if IsValidCost(-0.01) {
		t.Error("expected negative cost to be invalid")
	}
	if IsValidCost(1_000_000_000 + 1) {
		t.Error("expected extremely large cost to be invalid")
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		count    int
		expected string
	}{
		{500, "500"},
		{1200, "1.2K"},
		{1250000, "1.2M"},
	}

	for _, tt := range tests {
		actual := FormatTokens(tt.count)
		if actual != tt.expected {
			t.Errorf("FormatTokens(%d) = %q, expected %q", tt.count, actual, tt.expected)
		}
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		cost     float64
		expected string
	}{
		{1.50, "$1.50"},
		{0.05, "5.00¢"},
		{0.0005, "0.05¢"},
		{0.00001, "<0.01¢"},
	}

	for _, tt := range tests {
		actual := FormatCost(tt.cost)
		if actual != tt.expected {
			t.Errorf("FormatCost(%f) = %q, expected %q", tt.cost, actual, tt.expected)
		}
	}
}
