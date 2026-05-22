package tui

import (
	"testing"
)

func TestText_Render(t *testing.T) {
	text := NewText("hello")
	lines := text.Render(80)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0] != "hello" {
		t.Errorf("expected 'hello', got '%s'", lines[0])
	}

	// Width 0 should return nil
	lines = text.Render(0)
	if lines != nil {
		t.Error("expected nil for width 0")
	}
}

func TestText_SetContent(t *testing.T) {
	text := NewText("hello")
	text.SetContent("world")
	lines := text.Render(80)
	if lines[0] != "world" {
		t.Errorf("expected 'world', got '%s'", lines[0])
	}
}

func TestSpacer_Render(t *testing.T) {
	s := NewSpacer(3)
	lines := s.Render(80)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	for _, line := range lines {
		if line != "" {
			t.Errorf("expected empty line, got '%s'", line)
		}
	}

	// Height 0
	s2 := NewSpacer(0)
	lines = s2.Render(80)
	if lines != nil {
		t.Error("expected nil for height 0")
	}
}

func TestBox_Render(t *testing.T) {
	inner := NewText("content")
	box := NewBox("title", inner, 0)
	lines := box.Render(20)

	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines (top border, content, bottom border), got %d", len(lines))
	}

	// Check top border
	if len(lines[0]) < 2 || string([]rune(lines[0])[0]) != "┌" {
		t.Errorf("first line should start with ┌, got '%s'", lines[0])
	}

	// Check content line
	if len(lines) >= 2 {
		contentLine := lines[1]
		runes := []rune(contentLine)
		if len(runes) > 0 && string(runes[0]) != "│" {
			t.Errorf("content line should start with │, got '%s'", contentLine)
		}
		if len(runes) > 0 && string(runes[len(runes)-1]) != "│" {
			t.Errorf("content line should end with │, got '%s'", contentLine)
		}
	}

	// Check bottom border
	lastIdx := len(lines) - 1
	if len(lines[lastIdx]) > 0 && string([]rune(lines[lastIdx])[0]) != "└" {
		t.Errorf("last line should start with └, got '%s'", lines[lastIdx])
	}
}

func TestBox_NoTitle(t *testing.T) {
	inner := NewText("hi")
	box := NewBox("", inner, 0)
	lines := box.Render(10)

	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}

	// Top border without title: ┌────────┐
	if lines[0] != "┌────────┐" {
		t.Errorf("expected '┌────────┐', got '%s'", lines[0])
	}

	// Bottom border: └────────┘
	last := lines[len(lines)-1]
	if last != "└────────┘" {
		t.Errorf("expected '└────────┘', got '%s'", last)
	}
}

func TestInput_Render(t *testing.T) {
	input := NewInput()
	input.SetValue("test")

	lines := input.Render(10)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	// Input should have spaces around it
	if len(lines[0]) != 10 {
		t.Errorf("expected line length 10, got %d", len(lines[0]))
	}
}

func TestInput_HandleInput(t *testing.T) {
	input := NewInput()
	input.SetFocused(true)

	// Test typing
	input.HandleInput("h")
	if input.Value() != "h" {
		t.Errorf("expected 'h', got '%s'", input.Value())
	}

	input.HandleInput("e")
	input.HandleInput("l")
	input.HandleInput("l")
	input.HandleInput("o")
	if input.Value() != "hello" {
		t.Errorf("expected 'hello', got '%s'", input.Value())
	}

	// Test backspace
	input.HandleInput("\x7f") // backspace
	if input.Value() != "hell" {
		t.Errorf("expected 'hell', got '%s'", input.Value())
	}
}

func TestInput_HandleUnicodeAndPaste(t *testing.T) {
	input := NewInput()
	input.SetFocused(true)

	input.HandleInput("λ")
	input.HandleInput(" pasted")

	if input.Value() != "λ pasted" {
		t.Fatalf("expected unicode and pasted text, got %q", input.Value())
	}
}

func TestInput_IgnoresNamedKeys(t *testing.T) {
	input := NewInput()
	input.SetFocused(true)

	input.HandleInput("tab")
	input.HandleInput("ctrl+x")

	if input.Value() != "" {
		t.Fatalf("expected named keys to be ignored, got %q", input.Value())
	}
}

func TestInput_Submit(t *testing.T) {
	var submitted string
	input := NewInput()
	input.SetFocused(true)
	input.SetOnSubmit(func(value string) {
		submitted = value
	})
	input.SetValue("hello")
	input.HandleInput("\x0a") // enter

	if submitted != "hello" {
		t.Errorf("expected 'hello', got '%s'", submitted)
	}
}
