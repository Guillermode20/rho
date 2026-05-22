package tui

import (
	"testing"
)

func TestVisibleWidth(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"hello", 5},
		{"héllo", 5},
		{"你好", 4},                 // CJK characters are 2 wide each
		{"hello 世界", 10},          // "hello " (6) + "世界" (4) = 10
		{"\x1b[31mred\x1b[0m", 3}, // ANSI sequences have zero width
		{"a\x1b[1mb", 2},
	}

	for _, tt := range tests {
		got := VisibleWidth(tt.input)
		if got != tt.want {
			t.Errorf("VisibleWidth(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestTruncateToWidth(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"hello", 5, "hello"},
		{"hello", 3, "hel"},
		{"hello", 0, ""},
		{"hello world", 5, "hello"},
		{"你好世界", 2, "你"},
		{"你好世界", 4, "你好"},
	}

	for _, tt := range tests {
		got := TruncateToWidth(tt.input, tt.max)
		if got != tt.want {
			t.Errorf("TruncateToWidth(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
		}
	}
}

func TestSliceByColumn(t *testing.T) {
	tests := []struct {
		input  string
		start  int
		end    int
		strict bool
		want   string
	}{
		{"hello", 0, 5, true, "hello"},
		{"hello", 1, 4, true, "ell"},
		{"hello", 0, 3, true, "hel"},
		{"你好世界", 0, 2, true, "你"},
		{"你好世界", 2, 4, true, "好"},
		{"你好世界", 0, 1, true, ""}, // Strict: wide char at boundary excluded
	}

	for _, tt := range tests {
		got := SliceByColumn(tt.input, tt.start, tt.end, tt.strict)
		if got != tt.want {
			t.Errorf("SliceByColumn(%q, %d, %d, %v) = %q, want %q", tt.input, tt.start, tt.end, tt.strict, got, tt.want)
		}
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"\x1b[31mhello\x1b[0m", "hello"},
		{"\x1b[1;31mbold red\x1b[0m", "bold red"},
		{"", ""},
	}

	for _, tt := range tests {
		got := StripANSI(tt.input)
		if got != tt.want {
			t.Errorf("StripANSI(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsImageLine(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"hello", false},
		{"\x1b_Ga=d,i=1;hello", true},
		{"\x1b_Ga=T,f=100;data", true},
	}

	for _, tt := range tests {
		got := IsImageLine(tt.input)
		if got != tt.want {
			t.Errorf("IsImageLine(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestWrapTextWithAnsi(t *testing.T) {
	tests := []struct {
		text     string
		maxWidth int
		wantLen  int
	}{
		{"hello world", 5, 3}, // "hello", " worl", "d" (character-level)
		{"short", 10, 1},
		{"", 10, 0},
		{"a", 5, 1},
	}

	for _, tt := range tests {
		got := WrapTextWithAnsi(tt.text, tt.maxWidth)
		if len(got) != tt.wantLen {
			t.Errorf("WrapTextWithAnsi(%q, %d) returned %d lines, want %d", tt.text, tt.maxWidth, len(got), tt.wantLen)
		}
	}
}

func TestExtractSegments(t *testing.T) {
	before, at, after := ExtractSegments("hello world", 3, 8, 5, true)
	if before != "hel" {
		t.Errorf("ExtractSegments before = %q, want %q", before, "hel")
	}
	if at != "lo wo" {
		t.Errorf("ExtractSegments at = %q, want %q", at, "lo wo")
	}
	if after != "rld" {
		t.Errorf("ExtractSegments after = %q, want %q", after, "rld")
	}
}

func TestRuneWidth(t *testing.T) {
	tests := []struct {
		r    rune
		want int
	}{
		{'a', 1},
		{' ', 1},
		{'你', 2},
		{'世', 2},
		{'\n', 0},
		{0x7f, 0},
	}

	for _, tt := range tests {
		got := runeWidth(tt.r)
		if got != tt.want {
			t.Errorf("runeWidth(%c / U+%04X) = %d, want %d", tt.r, tt.r, got, tt.want)
		}
	}
}

func TestNormalizeTerminalOutput(t *testing.T) {
	// Test cursor marker removal
	input := "hello\x1b_pi:c\x07world"
	want := "helloworld"
	got := NormalizeTerminalOutput(input)
	if got != want {
		t.Errorf("NormalizeTerminalOutput = %q, want %q", got, want)
	}
}
