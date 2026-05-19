package tui

import (
	"testing"
)

func TestParseKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"a", "a"},
		{"A", "A"},
		{"\x1b", "esc"},
		{"\x09", "tab"},
		{"\x0a", "enter"},
		{"\x7f", "backspace"},
		{"\x08", "backspace"},
		// Ctrl+letter
		{"\x01", "ctrl+a"},
		{"\x1a", "ctrl+z"},
		// CSI cursor keys
		{"\x1b[A", "up"},
		{"\x1b[B", "down"},
		{"\x1b[C", "right"},
		{"\x1b[D", "left"},
		{"\x1b[H", "home"},
		{"\x1b[F", "end"},
		// CSI tilde codes
		{"\x1b[1~", "home"},
		{"\x1b[2~", "insert"},
		{"\x1b[3~", "del"},
		{"\x1b[4~", "end"},
		{"\x1b[5~", "pageup"},
		{"\x1b[6~", "pagedown"},
		// Function keys (SS3)
		{"\x1bOP", "f1"},
		{"\x1bOQ", "f2"},
		{"\x1bOR", "f3"},
		{"\x1bOS", "f4"},
		// Shift+Tab
		{"\x1b[Z", "shift+tab"},
		// Empty
		{"", ""},
	}

	for _, tt := range tests {
		got := ParseKey(tt.input)
		if got != tt.want {
			t.Errorf("ParseKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMatchesKey(t *testing.T) {
	tests := []struct {
		input string
		key   KeyID
		want  bool
	}{
		{"a", "a", true},
		{"b", "a", false},
		{"\x1b", "esc", true},
		{"\x1b[A", "up", true},
		{"\x1b[A", "down", false},
		{"", "enter", false},
	}

	for _, tt := range tests {
		got := MatchesKey(tt.input, tt.key)
		if got != tt.want {
			t.Errorf("MatchesKey(%q, %q) = %v, want %v", tt.input, tt.key, got, tt.want)
		}
	}
}

func TestKeyHelper(t *testing.T) {
	if Key.Enter() != "enter" {
		t.Errorf("Key.Enter() = %q, want %q", Key.Enter(), "enter")
	}
	if Key.Ctrl("c") != "ctrl+c" {
		t.Errorf("Key.Ctrl(\"c\") = %q, want %q", Key.Ctrl("c"), "ctrl+c")
	}
	if Key.Shift("tab") != "shift+tab" {
		t.Errorf("Key.Shift(\"tab\") = %q, want %q", Key.Shift("tab"), "shift+tab")
	}
	if Key.Alt("enter") != "alt+enter" {
		t.Errorf("Key.Alt(\"enter\") = %q, want %q", Key.Alt("enter"), "alt+enter")
	}
	if Key.Up() != "up" {
		t.Errorf("Key.Up() = %q, want %q", Key.Up(), "up")
	}
	if Key.Down() != "down" {
		t.Errorf("Key.Down() = %q, want %q", Key.Down(), "down")
	}
	if Key.F(1) != "f1" {
		t.Errorf("Key.F(1) = %q, want %q", Key.F(1), "f1")
	}
	if Key.F(12) != "f12" {
		t.Errorf("Key.F(12) = %q, want %q", Key.F(12), "f12")
	}
}

func TestIsKeyRelease(t *testing.T) {
	// Without Kitty protocol active, nothing should be a release
	if IsKeyRelease("a") {
		t.Error("IsKeyRelease('a') should be false when Kitty protocol is inactive")
	}

	// With Kitty protocol active
	SetKittyProtocolActive(true)
	defer SetKittyProtocolActive(false)

	// Simple test: strings ending with capital letter suffix
	// This is a simplified check
	if IsKeyRelease("hello") {
		t.Error("IsKeyRelease('hello') should be false")
	}
}

func TestKittyProtocolState(t *testing.T) {
	// Should start as false (or whatever state it's in)
	SetKittyProtocolActive(false)
	if IsKittyProtocolActive() {
		t.Error("Kitty protocol should be inactive")
	}

	SetKittyProtocolActive(true)
	if !IsKittyProtocolActive() {
		t.Error("Kitty protocol should be active")
	}

	SetKittyProtocolActive(false)
}
