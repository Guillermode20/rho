// Package tui provides a Terminal User Interface built on Bubble Tea.
package tui

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Messages ───────────────────────────────────────────────────────────────

// RenderRequest signals that the UI should re-render.
type RenderRequest struct{}

// AgentResultMsg carries the result of an async agent execution.
type AgentResultMsg struct {
	Message string
	Error   error
}

// AgentStatusMsg updates the status bar during agent execution.
type AgentStatusMsg struct {
	Text string
}

// AddMessageMsg adds a message to the conversation.
// Role uses a string to avoid circular imports with the ai package.
type AddMessageMsg struct {
	Role     string
	Content  string
	Model    string
	ToolCall string
}

// ClearScreenMsg signals a full screen clear.
type ClearScreenMsg struct{}

// FocusChangedMsg is sent when focus moves between components.
type FocusChangedMsg struct {
	Previous, Current Component
}

// ─── Styles ────────────────────────────────────────────────────────────────

var (
	StyleDim       = lipgloss.NewStyle().Faint(true)
	StyleBold      = lipgloss.NewStyle().Bold(true)
	StyleStatusBar = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)
	StyleError   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	StyleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	StylePrompt  = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	StyleMuted   = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
)

// ─── BTModel ───────────────────────────────────────────────────────────────

// BTModel is the top-level Bubble Tea model for the rho TUI.
// It replaces the old TUI+ProcessTerminal pairing with a single tea.Model.
type BTModel struct {
	// Children are the top-level components rendered in order.
	Children []Component

	// FocusedComponent is the component currently receiving keyboard input.
	FocusedComponent Component

	// Width and Height are the current terminal dimensions (set by tea.WindowSizeMsg).
	Width  int
	Height int

	// ShowCursor controls hardware cursor visibility.
	ShowCursor bool

	onDebug func()

	// OnMessage is called for every message that reaches Update.
	// Set this to intercept custom messages like AddMessageMsg, AgentStatusMsg.
	OnMessage func(tea.Msg)
}

// NewBTModel creates a new top-level Bubble Tea model.
func NewBTModel() *BTModel {
	return &BTModel{
		ShowCursor: false,
	}
}

// ─── tea.Model implementation ──────────────────────────────────────────────

// Init initializes the model.
func (m *BTModel) Init() tea.Cmd {
	return tea.WindowSize()
}

// Update handles messages and returns the updated model.
func (m *BTModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Call OnMessage for custom message handling
	if m.OnMessage != nil {
		m.OnMessage(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		// Invalidate children to recalculate layouts
		for _, child := range m.Children {
			child.Invalidate()
		}

	case tea.KeyMsg:
		if m.FocusedComponent != nil {
			// Handle global debug key (Shift+Ctrl+D)
			if msg.String() == "ctrl+d" && m.onDebug != nil {
				m.onDebug()
				return m, nil
			}

			// Ctrl+C to quit
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}

			// Delegate to focused component
			m.FocusedComponent.HandleInput(keyInput(msg))
		}

	case RenderRequest:
		// no-op: View() will re-render

	case ClearScreenMsg:
		// Full redraw: invalidate all children
		for _, child := range m.Children {
			child.Invalidate()
		}
		// Also send clear command for any Kitty images
		cmds = append(cmds, tea.Printf("\x1b_Ga=d,d=A,q=2\x1b\\"))
	}

	return m, tea.Batch(cmds...)
}

func keyInput(msg tea.KeyMsg) string {
	switch msg.Type {
	case tea.KeyRunes:
		return string(msg.Runes)
	case tea.KeySpace:
		return " "
	default:
		return msg.String()
	}
}

// View renders the UI using the old Component interface.
func (m *BTModel) View() string {
	width := m.Width
	if width <= 0 {
		width = 80
	}

	var sections []string

	for _, child := range m.Children {
		lines := child.Render(width)
		for _, line := range lines {
			sections = append(sections, line)
		}
	}

	out := ""
	for i, s := range sections {
		if i > 0 {
			out += "\r\n"
		}
		out += s
	}

	return out
}

// ─── Focus helpers ─────────────────────────────────────────────────────────

// SetFocus sets the focused component, unfocusing the previous one.
func (m *BTModel) SetFocus(comp Component) {
	if prev, ok := m.FocusedComponent.(Focusable); ok {
		prev.SetFocused(false)
	}
	m.FocusedComponent = comp
	if next, ok := comp.(Focusable); ok {
		next.SetFocused(true)
	}
}

// AddChild adds a child component to the layout.
func (m *BTModel) AddChild(child Component) {
	m.Children = append(m.Children, child)
}

// RemoveChild removes a child component.
func (m *BTModel) RemoveChild(child Component) {
	for i, c := range m.Children {
		if c == child {
			m.Children = append(m.Children[:i], m.Children[i+1:]...)
			return
		}
	}
}

// SetOnDebug sets the debug callback (triggered by Ctrl+D).
func (m *BTModel) SetOnDebug(fn func()) {
	m.onDebug = fn
}

// ─── BTStatusBar ───────────────────────────────────────────────────────────

// BTStatusBar is a styled status bar component using lipgloss.
type BTStatusBar struct {
	content string
	style   lipgloss.Style
}

// NewBTStatusBar creates a new styled status bar.
func NewBTStatusBar(content string) *BTStatusBar {
	return &BTStatusBar{
		content: content,
		style:   StyleStatusBar,
	}
}

// SetContent updates the status bar text.
func (sb *BTStatusBar) SetContent(content string) {
	sb.content = content
}

func (sb *BTStatusBar) Render(width int) []string {
	if width <= 0 {
		return nil
	}
	rendered := sb.style.Width(width).Render(sb.content)
	return []string{rendered}
}

func (sb *BTStatusBar) HandleInput(data string) {}
func (sb *BTStatusBar) Invalidate()             {}
func (sb *BTStatusBar) WantsKeyRelease() bool   { return false }
