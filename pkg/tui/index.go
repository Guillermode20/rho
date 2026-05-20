// Package tui provides a Terminal User Interface built on Bubble Tea.
//
// It uses github.com/charmbracelet/bubbletea as the underlying TUI framework
// while keeping existing legacy components (Text, Spacer, Box, Input, SelectList,
// Markdown) working through the Component interface.
//
// The main entry point is BTCtx which wraps a tea.Program. The BTModel
// implements tea.Model and bridges Bubble Tea's message-passing model with
// the existing Component interface.
//
// Features:
// - Bubble Tea framework for terminal management and rendering
// - lipgloss styling for status bars and UI chrome
// - Legacy Component interface for backward compatibility
// - Kitty keyboard protocol support
// - Markdown rendering
// - Image display (Kitty/iTerm2 protocols)
// - Wide character (CJK, emoji) support
package tui
