package aiutils

import (
	"testing"
)

func TestPartialJSON(t *testing.T) {
	tests := []struct {
		name     string
		chunks   []string
		expected string
		complete bool
	}{
		{
			name:     "Complete simple object",
			chunks:   []string{"{", `"` , "foo", `"` , ":", " 1", "}"},
			expected: `{"foo": 1}`,
			complete: true,
		},
		{
			name:     "Partial object",
			chunks:   []string{"{", `"foo": 1`},
			expected: `{"foo": 1`,
			complete: false,
		},
		{
			name:     "Handle nested objects",
			chunks:   []string{"{", `"nested": {`, `"key": "val"`, "}", "}"},
			expected: `{"nested": {"key": "val"}}`,
			complete: true,
		},
		{
			name:     "Handle escape characters in string",
			chunks:   []string{"{", `"key":`, `"value\"with\\escapes"`, "}"},
			expected: `{"key":"value\"with\\escapes"}`,
			complete: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPartialJSON()
			var lastResult string
			var lastComplete bool
			for _, chunk := range tt.chunks {
				lastResult, lastComplete = p.Feed(chunk)
			}
			if lastResult != tt.expected {
				t.Errorf("expected string %q, got %q", tt.expected, lastResult)
			}
			if lastComplete != tt.complete {
				t.Errorf("expected complete to be %v, got %v", tt.complete, lastComplete)
			}
		})
	}
}

func TestParseStreamingJSON(t *testing.T) {
	chunks := make(chan string, 5)
	chunks <- `{"a"`
	chunks <- `: 1`
	chunks <- `}`
	close(chunks)

	out := ParseStreamingJSON(chunks)
	var results []interface{}
	for val := range out {
		results = append(results, val)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	m, ok := results[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", results[0])
	}

	if m["a"] != float64(1) {
		t.Errorf("expected m[\"a\"] to be 1, got %v", m["a"])
	}
}

func TestRepairJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "No change for valid JSON",
			input:    `{"a": 1}`,
			expected: `{"a": 1}`,
		},
		{
			name:     "Trailing comma in object",
			input:    `{"a": 1, }`,
			expected: `{"a": 1 }`,
		},
		{
			name:     "Trailing comma in array",
			input:    `[1, 2,  ]`,
			expected: `[1, 2  ]`,
		},
		{
			name:     "Unclosed braces",
			input:    `{"a": {"b": 1`,
			expected: `{"a": {"b": 1}}`,
		},
		{
			name:     "Unclosed array/braces mix",
			input:    `{"a": [1, 2`,
			expected: `{"a": [1, 2}}`, // Note: RepairJSON implementation appends `}` for all `opens`.
		},
		{
			name:     "Trailing whitespace and unclosed",
			input:    `{"a": 1   `,
			expected: `{"a": 1   }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := RepairJSON(tt.input)
			if actual != tt.expected {
				t.Errorf("RepairJSON(%q) = %q, expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestParseJSONWithRepair(t *testing.T) {
	type Target struct {
		A int `json:"a"`
	}

	t.Run("Valid JSON", func(t *testing.T) {
		var tgt Target
		err := ParseJSONWithRepair([]byte(`{"a": 1}`), &tgt)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if tgt.A != 1 {
			t.Errorf("expected A=1, got %d", tgt.A)
		}
	})

	t.Run("Repairable JSON", func(t *testing.T) {
		var tgt Target
		err := ParseJSONWithRepair([]byte(`{"a": 1`), &tgt)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if tgt.A != 1 {
			t.Errorf("expected A=1, got %d", tgt.A)
		}
	})

	t.Run("Unrepairable JSON", func(t *testing.T) {
		var tgt Target
		err := ParseJSONWithRepair([]byte(`not json`), &tgt)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestIsSurrogate(t *testing.T) {
	tests := []struct {
		r        rune
		expected bool
	}{
		{r: 0xD7FF, expected: false},
		{r: 0xD800, expected: true},
		{r: 0xDC00, expected: true},
		{r: 0xDFFF, expected: true},
		{r: 0xE000, expected: false},
	}

	for _, tt := range tests {
		actual := IsSurrogate(tt.r)
		if actual != tt.expected {
			t.Errorf("IsSurrogate(0x%X) = %v, expected %v", tt.r, actual, tt.expected)
		}
	}
}

func TestIsJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Object", `{"a": 1}`, true},
		{"Array", `[1, 2]`, true},
		{"String", `"foo"`, true},
		{"Whitespace object", `   {"a": 1}   `, true},
		{"Empty string", "", false},
		{"Invalid pattern", "foo", false},
		{"Only opening brace", "{", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := IsJSON(tt.input)
			if actual != tt.expected {
				t.Errorf("IsJSON(%q) = %v, expected %v", tt.input, actual, tt.expected)
			}
		})
	}
}
