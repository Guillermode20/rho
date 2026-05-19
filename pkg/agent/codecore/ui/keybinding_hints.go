package codecui

import (
	"fmt"
	"strings"

	"github.com/earendil-works/rho/pkg/tui"
)

// KeybindingHints renders a bar of available keybinding hints.
type KeybindingHints struct {
	hints   []KeybindingHintItem
	visible bool
}

// NewKeybindingHints creates a keybinding hints component.
func NewKeybindingHints() *KeybindingHints {
	return &KeybindingHints{
		hints:   DefaultKeybindingHints(),
		visible: true,
	}
}

// DefaultKeybindingHints returns the default set of hints.
func DefaultKeybindingHints() []KeybindingHintItem {
	return []KeybindingHintItem{
		{Key: "Enter", Description: "Send"},
		{Key: "Esc", Description: "Cancel"},
		{Key: "Tab", Description: "Focus"},
		{Key: "↑↓", Description: "Navigate"},
		{Key: "Ctrl+D", Description: "Debug"},
		{Key: "Ctrl+L", Description: "Clear"},
		{Key: "Ctrl+O", Description: "Sessions"},
		{Key: "Ctrl+E", Description: "Extensions"},
		{Key: "Ctrl+T", Description: "Thinking"},
		{Key: "Ctrl+M", Description: "Model"},
		{Key: "Ctrl+P", Description: "Settings"},
	}
}

// SetHints overrides the displayed hints.
func (kh *KeybindingHints) SetHints(hints []KeybindingHintItem) {
	kh.hints = hints
}

// SetVisible toggles visibility.
func (kh *KeybindingHints) SetVisible(v bool) {
	kh.visible = v
}

// SetKeybindingHints updates hints conditionally.
func SetKeybindingHints(hints []KeybindingHintItem) []KeybindingHintItem {
	return hints
}

func (kh *KeybindingHints) Render(width int) []string {
	if width <= 0 || !kh.visible || len(kh.hints) == 0 {
		return nil
	}

	reset := "\x1b[0m"
	dim := "\x1b[2m"
	bold := "\x1b[1m"
	reverse := "\x1b[7m"

	var parts []string
	for _, hint := range kh.hints {
		keyDisplay := fmt.Sprintf("%s%s%s%s", reverse, bold, hint.Key, reset)
		descDisplay := dim + " " + hint.Description + reset
		part := keyDisplay + descDisplay

		if tui.VisibleWidth(part) > 20 {
			part = tui.SliceByColumn(part, 0, 20, true)
		}
		parts = append(parts, part)
	}

	line := strings.Join(parts, dim + "  " + reset)

	if tui.VisibleWidth(line) > width {
		// Truncate to fit, keeping the important ones
		line = tui.SliceByColumn(line, 0, width, true)
	}

	return []string{dim + line + reset}
}

func (kh *KeybindingHints) HandleInput(data string) {}
func (kh *KeybindingHints) Invalidate()             {}
func (kh *KeybindingHints) WantsKeyRelease() bool   { return false }
