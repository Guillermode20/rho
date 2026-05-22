package aiutils

import (
	"testing"
)

func TestValidateString(t *testing.T) {
	t.Run("Valid string", func(t *testing.T) {
		err := ValidateString("hello", "field", 10)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Empty string error", func(t *testing.T) {
		err := ValidateString("   ", "field", 10)
		if err == nil || err.Error() != "field is required" {
			t.Errorf("expected error 'field is required', got %v", err)
		}
	})

	t.Run("Exceeds max length error", func(t *testing.T) {
		err := ValidateString("too long string", "field", 5)
		if err == nil || err.Error() != "field exceeds maximum length of 5" {
			t.Errorf("expected error 'field exceeds maximum length of 5', got %v", err)
		}
	})
}

func TestValidateInt(t *testing.T) {
	err := ValidateInt(5, "num", 1, 10)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateInt(0, "num", 1, 10)
	if err == nil {
		t.Error("expected error for below min")
	}

	err = ValidateInt(11, "num", 1, 10)
	if err == nil {
		t.Error("expected error for above max")
	}
}

func TestValidateFloat(t *testing.T) {
	err := ValidateFloat(5.5, "num", 1.0, 10.0)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateFloat(0.5, "num", 1.0, 10.0)
	if err == nil {
		t.Error("expected error for below min")
	}
}

func TestValidateTemperature(t *testing.T) {
	if err := ValidateTemperature(1.0); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateTemperature(-0.1); err == nil {
		t.Error("expected error for temp < 0")
	}
	if err := ValidateTemperature(2.1); err == nil {
		t.Error("expected error for temp > 2")
	}
}

func TestValidateMaxTokens(t *testing.T) {
	if err := ValidateMaxTokens(100); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateMaxTokens(0); err == nil {
		t.Error("expected error for max_tokens < 1")
	}
	if err := ValidateMaxTokens(250000); err == nil {
		t.Error("expected error for max_tokens > 200000")
	}
}

func TestValidateTopP(t *testing.T) {
	if err := ValidateTopP(0.5); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateTopP(-0.1); err == nil {
		t.Error("expected error for top_p < 0")
	}
	if err := ValidateTopP(1.1); err == nil {
		t.Error("expected error for top_p > 1")
	}
}

func TestValidateAPIKey(t *testing.T) {
	if err := ValidateAPIKey("sk-abc12345"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateAPIKey("  "); err == nil {
		t.Error("expected error for empty key")
	}
	if err := ValidateAPIKey("short"); err == nil {
		t.Error("expected error for short key")
	}
}

func TestValidateModelName(t *testing.T) {
	if err := ValidateModelName("gpt-4"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateModelName(""); err == nil {
		t.Error("expected error for empty model")
	}
	longName := make([]byte, 201)
	if err := ValidateModelName(string(longName)); err == nil {
		t.Error("expected error for too long model name")
	}
}

func TestValidateToolName(t *testing.T) {
	if err := ValidateToolName("read_file"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateToolName("read-file123"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateToolName("read file"); err == nil {
		t.Error("expected error for spaces in tool name")
	}
	if err := ValidateToolName("read$file"); err == nil {
		t.Error("expected error for invalid characters")
	}
}

func TestClamping(t *testing.T) {
	if val := ClampTemperature(0.5); val != 0.5 {
		t.Errorf("expected 0.5, got %v", val)
	}
	if val := ClampTemperature(-0.5); val != 0.0 {
		t.Errorf("expected 0.0, got %v", val)
	}
	if val := ClampTemperature(2.5); val != 2.0 {
		t.Errorf("expected 2.0, got %v", val)
	}

	if val := ClampMaxTokens(100); val != 100 {
		t.Errorf("expected 100, got %v", val)
	}
	if val := ClampMaxTokens(0); val != 1 {
		t.Errorf("expected 1, got %v", val)
	}
	if val := ClampMaxTokens(250000); val != 200000 {
		t.Errorf("expected 200000, got %v", val)
	}

	if val := ClampTopP(0.5); val != 0.5 {
		t.Errorf("expected 0.5, got %v", val)
	}
	if val := ClampTopP(-0.5); val != 0.0 {
		t.Errorf("expected 0.0, got %v", val)
	}
	if val := ClampTopP(1.5); val != 1.0 {
		t.Errorf("expected 1.0, got %v", val)
	}
}

func TestMinMaxInt(t *testing.T) {
	if val := MinInt(2, 5); val != 2 {
		t.Errorf("expected MinInt(2, 5) = 2, got %d", val)
	}
	if val := MinInt(5, 2); val != 2 {
		t.Errorf("expected MinInt(5, 2) = 2, got %d", val)
	}

	if val := MaxInt(2, 5); val != 5 {
		t.Errorf("expected MaxInt(2, 5) = 5, got %d", val)
	}
	if val := MaxInt(5, 2); val != 5 {
		t.Errorf("expected MaxInt(5, 2) = 5, got %d", val)
	}
}

func TestEnsureSlice(t *testing.T) {
	slice := []int{1, 2}
	newSlice := EnsureSlice(slice, 5)
	if cap(newSlice) < 5 {
		t.Errorf("expected capacity >= 5, got %d", cap(newSlice))
	}
	if len(newSlice) != 2 || newSlice[0] != 1 || newSlice[1] != 2 {
		t.Errorf("expected slice elements [1, 2], got %v", newSlice)
	}

	sameSlice := EnsureSlice(slice, 2)
	if &sameSlice[0] != &slice[0] {
		t.Error("expected EnsureSlice to return the same slice if capacity is sufficient")
	}
}

func TestSafeDivide(t *testing.T) {
	if val := SafeDivide(6.0, 3.0); val != 2.0 {
		t.Errorf("expected 2.0, got %f", val)
	}
	if val := SafeDivide(5.0, 0.0); val != 0.0 {
		t.Errorf("expected 0.0, got %f", val)
	}
}

func TestRoundTo(t *testing.T) {
	tests := []struct {
		val      float64
		places   int
		expected float64
	}{
		{3.14159, 2, 3.14},
		{3.14159, 4, 3.1416},
		{2.5, 0, 3.0},
	}

	for _, tt := range tests {
		actual := RoundTo(tt.val, tt.places)
		if actual != tt.expected {
			t.Errorf("RoundTo(%f, %d) = %f, expected %f", tt.val, tt.places, actual, tt.expected)
		}
	}
}
