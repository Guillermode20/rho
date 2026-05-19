// Package codecui provides TUI components for the interactive coding agent mode.
//
// This package implements all the visual components used in the interactive
// TUI mode: message display, selectors, dialogs, tool execution displays,
// and utility components.
package codecui

// KeybindingHintItem represents a single keybinding hint.
type KeybindingHintItem struct {
	Key         string
	Description string
}
