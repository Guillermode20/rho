package main

import (
	"reflect"
	"testing"
)

func TestParseArgs(t *testing.T) {
	testCases := []struct {
		name     string
		input    []string
		expected Args
	}{
		{
			name:  "Standard flags and positional message",
			input: []string{"--provider", "anthropic", "--model", "claude-3-5-sonnet", "Hello World"},
			expected: Args{
				Provider:     "anthropic",
				Model:        "claude-3-5-sonnet",
				Messages:     []string{"Hello World"},
				UnknownFlags: map[string]interface{}{},
			},
		},
		{
			name:  "Short and print flags",
			input: []string{"-p", "Tell me a joke", "-c"},
			expected: Args{
				Print:        true,
				Messages:     []string{"Tell me a joke"},
				Continue:     true,
				UnknownFlags: map[string]interface{}{},
			},
		},
		{
			name:  "File args",
			input: []string{"@foo.txt", "@bar.png", "Analysis"},
			expected: Args{
				FileArgs:     []string{"foo.txt", "bar.png"},
				Messages:     []string{"Analysis"},
				UnknownFlags: map[string]interface{}{},
			},
		},
		{
			name:  "Unknown custom flags",
			input: []string{"--custom-flag", "--foo=bar", "--baz-flag", "value"},
			expected: Args{
				UnknownFlags: map[string]interface{}{
					"custom-flag": true,
					"foo":         "bar",
					"baz-flag":    "value",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := ParseArgs(tc.input)
			if actual.Provider != tc.expected.Provider {
				t.Errorf("expected Provider %q, got %q", tc.expected.Provider, actual.Provider)
			}
			if actual.Model != tc.expected.Model {
				t.Errorf("expected Model %q, got %q", tc.expected.Model, actual.Model)
			}
			if actual.Print != tc.expected.Print {
				t.Errorf("expected Print %v, got %v", tc.expected.Print, actual.Print)
			}
			if actual.Continue != tc.expected.Continue {
				t.Errorf("expected Continue %v, got %v", tc.expected.Continue, actual.Continue)
			}
			if !reflect.DeepEqual(actual.Messages, tc.expected.Messages) {
				t.Errorf("expected Messages %v, got %v", tc.expected.Messages, actual.Messages)
			}
			if !reflect.DeepEqual(actual.FileArgs, tc.expected.FileArgs) {
				t.Errorf("expected FileArgs %v, got %v", tc.expected.FileArgs, actual.FileArgs)
			}
			if !reflect.DeepEqual(actual.UnknownFlags, tc.expected.UnknownFlags) {
				t.Errorf("expected UnknownFlags %v, got %v", tc.expected.UnknownFlags, actual.UnknownFlags)
			}
		})
	}
}
