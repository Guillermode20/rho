package aiutils

import (
	"testing"
)

func TestSanitizeSurrogates(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Normal string",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "String with emojis",
			input:    "hello 👋 world",
			expected: "hello 👋 world",
		},
		{
			name:     "String with surrogate pair",
			input:    "hello \xed\xa0\xbd\xed\xb1\x8b world", // surrogate representation
			expected: "hello ?????? world", // each byte of invalid surrogate sequence replaced by '?'
		},
		{
			name:     "Invalid UTF-8 byte sequence",
			input:    "hello \xff world",
			expected: "hello ? world", // invalid byte replaced by '?'
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := SanitizeSurrogates(tt.input)
			if actual != tt.expected {
				t.Errorf("SanitizeSurrogates(%q) = %q, expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestNormalizeUnicode(t *testing.T) {
	input := "hello \xed\xa0\xbd\xed\xb1\x8b"
	expected := "hello ??????"
	actual := NormalizeUnicode(input)
	if actual != expected {
		t.Errorf("NormalizeUnicode(%q) = %q, expected %q", input, actual, expected)
	}
}

func TestStripControlCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Preserve normal tabs, newlines, carriage returns",
			input:    "line 1\nline 2\r\n\tindented",
			expected: "line 1\nline 2\r\n\tindented",
		},
		{
			name:     "Strip other control characters",
			input:    "hello \x00world\x1f!",
			expected: "hello world!",
		},
		{
			name:     "Strip DEL character",
			input:    "hello \x7fworld",
			expected: "hello world",
		},
		{
			name:     "Keep high Unicode control characters",
			input:    "hello \u0080world",
			expected: "hello \u0080world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := StripControlCharacters(tt.input)
			if actual != tt.expected {
				t.Errorf("StripControlCharacters(%q) = %q, expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestReplaceControlCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Tabs, newlines, CR",
			input:    "a\tb\nc\rd",
			expected: `a\tb\nc\rd`,
		},
		{
			name:     "Low control characters",
			input:    "a\x00b\x1fc",
			expected: `a\x00b\x1fc`,
		},
		{
			name:     "DEL character",
			input:    "a\x7fb",
			expected: `a\x7fb`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ReplaceControlCharacters(tt.input)
			if actual != tt.expected {
				t.Errorf("ReplaceControlCharacters(%q) = %q, expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestIsValidUTF8(t *testing.T) {
	if !IsValidUTF8("valid string") {
		t.Error("expected true for valid UTF-8 string")
	}
	if IsValidUTF8("invalid \xff string") {
		t.Error("expected false for invalid UTF-8 string")
	}
}

func TestTruncateUTF8(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxBytes int
		expected string
	}{
		{
			name:     "Short string fits",
			input:    "hello",
			maxBytes: 10,
			expected: "hello",
		},
		{
			name:     "Truncate at ASCII boundary",
			input:    "hello world",
			maxBytes: 5,
			expected: "hello",
		},
		{
			name:     "Truncate at safe emoji boundary",
			input:    "hello 👋!", // 👋 is 4 bytes
			maxBytes: 9,          // length of "hello " is 6, space + emoji is 10. Max 9 should truncate before 👋
			expected: "hello ",
		},
		{
			name:     "Truncate exactly after emoji",
			input:    "hello 👋!",
			maxBytes: 10, // hello (5) + space (1) + 👋 (4) = 10
			expected: "hello 👋",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := TruncateUTF8(tt.input, tt.maxBytes)
			if actual != tt.expected {
				t.Errorf("TruncateUTF8(%q, %d) = %q, expected %q", tt.input, tt.maxBytes, actual, tt.expected)
			}
		})
	}
}

func TestSafeString(t *testing.T) {
	input := "hello \xed\xa0\xbd\xed\xb1\x8b \x00world\x7f!"
	expected := "hello ?????? world!"
	actual := SafeString(input)
	if actual != expected {
		t.Errorf("SafeString(%q) = %q, expected %q", input, actual, expected)
	}
}
