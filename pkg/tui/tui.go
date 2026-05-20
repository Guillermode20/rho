// Package tui provides a Terminal User Interface library with differential rendering.
//
// It supports:
// - Differential rendering for efficient terminal updates
// - Kitty keyboard protocol for extended key reporting
// - Overlay/modal system with compositing
// - Components: Text, Spacer, Box, Input, SelectList
// - ANSI escape sequence handling
// - Wide character (CJK, emoji) support
package tui

// Component is the interface all UI components must implement.
type Component interface {
	// Render renders the component to lines for the given viewport width.
	Render(width int) []string

	// HandleInput handles keyboard input when the component has focus.
	HandleInput(data string)

	// Invalidate clears any cached rendering state.
	Invalidate()

	// WantsKeyRelease returns true if the component wants key release events.
	WantsKeyRelease() bool
}

// Focusable is an interface for components that can receive focus and display a hardware cursor.
type Focusable interface {
	// SetFocused sets whether this component is focused.
	SetFocused(focused bool)
	// Focused returns whether this component is focused.
	Focused() bool
}

// IsFocusable checks if a component implements Focusable.
func IsFocusable(c Component) bool {
	_, ok := c.(Focusable)
	return ok
}
