package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/earendil-works/rho/pkg/agent"
	agenttheme "github.com/earendil-works/rho/pkg/agent/theme"
	"github.com/earendil-works/rho/pkg/ai"
	"github.com/earendil-works/rho/pkg/tui"
)

type editorState struct {
	input  []rune
	cursor int
}

type AutocompleteItem struct {
	Value       string
	Label       string
	Description string
}

// ModalState drives all centered overlay dialogs.
type ModalState struct {
	Active bool
	Title  string
	Mode   string // "prompt" | "selector" | "confirm" | "info"

	// Prompt mode
	Placeholder string
	Value       []rune
	OnSubmit    func(string)
	OnCancel    func()

	// Selector mode
	AllItems       []AutocompleteItem
	Items          []AutocompleteItem
	Query          []rune
	SelIdx         int
	OnSelect       func(AutocompleteItem)
	OnSelectCancel func()

	// Confirm mode
	Message    string
	OnYes      func()
	OnNo       func()
	ConfirmIdx int // 0 = Yes, 1 = No

	// Info mode
	Content    []string
	InfoScroll int
	OnDismiss  func()
}

type ChatModel struct {
	Width      int
	Height     int
	Status     string
	Input      []rune
	Cursor     int
	Scroll     int
	Messages   []agent.AgentMessage
	MdTheme    tui.MarkdownTheme
	undoStack  *tui.UndoStack[editorState]
	redoStack  *tui.UndoStack[editorState]
	killRing   *tui.KillRing
	lastAction string
	yankLen    int
	// Design system
	Theme *tui.Theme

	// Metadata footer fields
	modelName     string
	providerName  string
	TokenCount    int
	contextWindow int
	TotalCost     float64
	GitBranch     string
	SessionName   string
	ThinkingLevel string

	// Cached cost map: built once on first access
	costMap map[string]ai.ModelDefinition

	OnSubmit        func(string)
	OnMessage       func(tea.Msg)
	OnAutocomplete  func(text string, cursor int) []AutocompleteItem
	OnAction        func(action string) bool
	Autocomplete    []AutocompleteItem
	AutocompleteIdx int

	// Modal state — replaces old inline prompt/selector/confirm
	Modal *ModalState

	// Toast for brief system notifications
	toast      string
	toastUntil time.Time

	// Legacy styles – kept for transition, but theme is primary.
	statusStyle    lipgloss.Style
	helpStyle      lipgloss.Style
	panelStyle     lipgloss.Style
	inputStyle     lipgloss.Style
	userStyle      lipgloss.Style
	assistantStyle lipgloss.Style
	mutedStyle     lipgloss.Style
}

const MouseWheelScrollLines = 3
const toolPreviewLines = 3

func NewChatModel(status string) *ChatModel {
	th := tui.DefaultTheme
	return &ChatModel{
		Status:    status,
		Theme:     th,
		MdTheme:   tui.DefaultMarkdownTheme(),
		undoStack: tui.NewUndoStack[editorState](),
		redoStack: tui.NewUndoStack[editorState](),
		killRing:  tui.NewKillRing(),
		statusStyle: lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("255")).
			Padding(0, 1),
		helpStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Padding(0, 1),
		panelStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true, false, false, false).
			BorderForeground(lipgloss.Color("238")).
			Padding(1, 2, 0, 2),
		inputStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1),
		userStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("75")).
			Bold(true),
		assistantStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("120")).
			Bold(true),
		mutedStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("242")),
	}
}

func (m *ChatModel) ApplyTheme(agtTheme agenttheme.Theme) {
	bgStr := themeColor(agtTheme, "bg", "235")
	fgStr := themeColor(agtTheme, "fg", "255")
	accentStr := themeColor(agtTheme, "accent", "63")
	successStr := themeColor(agtTheme, "success", "120")
	subtleStr := themeColor(agtTheme, "subtle", "242")
	borderStr := themeColor(agtTheme, "border", "238")
	warningStr := themeColor(agtTheme, "warning", "214")
	errorStr := themeColor(agtTheme, "error", "196")
	accentAltStr := themeColor(agtTheme, "accentAlt", "120")
	highlightStr := themeColor(agtTheme, "highlight", "235")
	surfaceStr := themeColor(agtTheme, "surface", "234")
	surfaceAltStr := themeColor(agtTheme, "surfaceAlt", "235")

	toColor := func(val string, fallback int) tui.Color {
		if strings.HasPrefix(val, "#") {
			return tui.Color{Hex: val}
		}
		var i int
		if _, err := fmt.Sscanf(val, "%d", &i); err == nil {
			return tui.Color{ANSI: i}
		}
		if len(val) == 6 || len(val) == 3 {
			if _, _, _, err := tui.ParseHexColor("#" + val); err == nil {
				return tui.Color{Hex: "#" + val}
			}
		}
		return tui.Color{ANSI: fallback}
	}

	pal := tui.Palette{
		Bg:         toColor(bgStr, 235),
		Fg:         toColor(fgStr, 255),
		Accent:     toColor(accentStr, 63),
		AccentAlt:  toColor(accentAltStr, 120),
		Success:    toColor(successStr, 120),
		Warning:    toColor(warningStr, 214),
		Error:      toColor(errorStr, 196),
		Dim:        toColor(subtleStr, 242),
		Border:     toColor(borderStr, 238),
		Highlight:  toColor(highlightStr, 235),
		Surface:    toColor(surfaceStr, 234),
		SurfaceAlt: toColor(surfaceAltStr, 235),
	}

	m.Theme = tui.NewTheme(pal)

	toLipglossColor := func(tc tui.Color, fallback string) lipgloss.Color {
		if tc.Hex != "" {
			return lipgloss.Color(tc.Hex)
		}
		return lipgloss.Color(fmt.Sprintf("%d", tc.ANSI))
	}

	bg := toLipglossColor(pal.Bg, "235")
	fg := toLipglossColor(pal.Fg, "255")
	accent := toLipglossColor(pal.Accent, "63")
	success := toLipglossColor(pal.Success, "120")
	subtle := toLipglossColor(pal.Dim, "242")

	m.statusStyle = lipgloss.NewStyle().
		Background(bg).
		Foreground(fg).
		Padding(0, 1)
	m.helpStyle = lipgloss.NewStyle().
		Foreground(subtle).
		Padding(0, 1)
	m.panelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true, false, false, false).
		BorderForeground(subtle).
		Padding(1, 2, 0, 2)
	m.inputStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(0, 1)
	m.userStyle = lipgloss.NewStyle().
		Foreground(success).
		Bold(true)
	m.assistantStyle = lipgloss.NewStyle().
		Foreground(accent).
		Bold(true)
	m.mutedStyle = lipgloss.NewStyle().Foreground(subtle)
	m.MdTheme = markdownThemeFromAgentTheme(agtTheme)
}

func (m *ChatModel) Init() tea.Cmd {
	return nil
}

func (m *ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.OnMessage != nil {
		m.OnMessage(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.clampScroll()
	case tea.MouseMsg:
		switch {
		case msg.Button == tea.MouseButtonWheelUp || msg.Type == tea.MouseWheelUp:
			m.Scroll += MouseWheelScrollLines
			m.clampScroll()
		case msg.Button == tea.MouseButtonWheelDown || msg.Type == tea.MouseWheelDown:
			m.Scroll -= MouseWheelScrollLines
			m.clampScroll()
		}
	case tea.KeyMsg:
		if m.toast != "" {
			m.toast = ""
		}

		if m.Modal != nil {
			m.handleModalInput(msg)
			return m, nil
		}

		if len(m.Autocomplete) > 0 {
			switch msg.String() {
			case "esc":
				m.closeAutocomplete()
				return m, nil
			case "up", "ctrl+p":
				if m.AutocompleteIdx > 0 {
					m.AutocompleteIdx--
				}
				return m, nil
			case "down", "ctrl+n":
				if m.AutocompleteIdx < len(m.Autocomplete)-1 {
					m.AutocompleteIdx++
				}
				return m, nil
			case "pgup":
				m.AutocompleteIdx -= 6
				if m.AutocompleteIdx < 0 {
					m.AutocompleteIdx = 0
				}
				return m, nil
			case "pgdown":
				m.AutocompleteIdx += 6
				if m.AutocompleteIdx >= len(m.Autocomplete) {
					m.AutocompleteIdx = len(m.Autocomplete) - 1
				}
				return m, nil
			case "home":
				m.AutocompleteIdx = 0
				return m, nil
			case "end":
				m.AutocompleteIdx = len(m.Autocomplete) - 1
				return m, nil
			case "tab", "enter":
				m.applyAutocomplete()
				return m, nil
			}
		}

		if m.OnAction != nil {
			switch msg.String() {
			case "ctrl+l":
				if m.OnAction("model.select") {
					return m, nil
				}
			case "ctrl+p":
				if m.OnAction("settings.open") {
					return m, nil
				}
			case "ctrl+r":
				if m.OnAction("session.resume") {
					return m, nil
				}
			case "ctrl+t":
				if m.OnAction("thinking.cycle") {
					return m, nil
				}
			}
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "pgup":
			m.Scroll += max(1, m.viewportHeight()-1)
			m.clampScroll()
		case "pgdown":
			m.Scroll -= max(1, m.viewportHeight()-1)
			m.clampScroll()
		case "ctrl+a", "home":
			m.Cursor = 0
			m.updateAutocomplete()
		case "ctrl+e", "end":
			m.Cursor = len(m.Input)
			m.updateAutocomplete()
		case "ctrl+u":
			if len(m.Input) > 0 {
				m.pushUndo()
				m.killRing.Push(string(m.Input), false, false)
				m.Input = nil
				m.Cursor = 0
				m.lastAction = "kill"
			}
			m.updateAutocomplete()
		case "ctrl+k":
			if m.Cursor < len(m.Input) {
				m.pushUndo()
				killedText := string(m.Input[m.Cursor:])
				accumulate := m.lastAction == "kill"
				m.killRing.Push(killedText, false, accumulate)
				m.Input = m.Input[:m.Cursor]
				m.lastAction = "kill"
			}
			m.updateAutocomplete()
		case "ctrl+z", "ctrl+_":
			m.undo()
		case "ctrl+shift+z":
			m.redo()
		case "ctrl+y":
			m.yank()
		case "alt+y":
			m.yankPop()
		case "up":
			m.cursorUpLine()
			m.updateAutocomplete()
		case "down":
			m.cursorDownLine()
			m.updateAutocomplete()
		case "left":
			if m.Cursor > 0 {
				m.Cursor--
			}
			m.updateAutocomplete()
		case "right":
			if m.Cursor < len(m.Input) {
				m.Cursor++
			}
			m.updateAutocomplete()
		case "backspace", "ctrl+h":
			if m.Cursor > 0 {
				if m.lastAction != "deleting" {
					m.pushUndo()
					m.lastAction = "deleting"
				}
				m.Input = append(m.Input[:m.Cursor-1], m.Input[m.Cursor:]...)
				m.Cursor--
			}
			m.updateAutocomplete()
		case "delete":
			if m.Cursor < len(m.Input) {
				if m.lastAction != "deleting" {
					m.pushUndo()
					m.lastAction = "deleting"
				}
				m.Input = append(m.Input[:m.Cursor], m.Input[m.Cursor+1:]...)
			}
			m.updateAutocomplete()
		case "alt+enter", "ctrl+j":
			m.pushUndo()
			m.insertRunes('\n')
			m.lastAction = "typing"
		case "enter":
			value := strings.TrimSpace(string(m.Input))
			if value != "" && m.OnSubmit != nil {
				m.OnSubmit(value)
			}
		case "esc":
			if m.OnAction != nil && m.OnAction("agent.abort") {
				return m, nil
			}
		default:
			if msg.Type == tea.KeyRunes {
				if m.lastAction != "typing" {
					m.pushUndo()
					m.lastAction = "typing"
				}
				m.insertRunes(msg.Runes...)
			} else if msg.Type == tea.KeySpace {
				m.pushUndo()
				m.lastAction = "typing"
				m.insertRunes(' ')
			}
		}
	}

	return m, nil
}

func (m *ChatModel) View() string {
	width := m.Width
	if width <= 0 {
		width = 80
	}

	if m.Modal != nil {
		return m.renderModalView(width)
	}

	parts := m.baseLayout(width)
	if suggestions := m.renderAutocomplete(width); suggestions != "" {
		parts = append(parts, suggestions)
	}

	return strings.Join(parts, "\n")
}

func (m *ChatModel) AddMessage(msg agent.AgentMessage) {
	m.Messages = append(m.Messages, msg)
	m.Scroll = 0
	m.recalculateStats()
}

func (m *ChatModel) UpdateMessage(index int, msg agent.AgentMessage) {
	if index < 0 || index >= len(m.Messages) {
		return
	}
	m.Messages[index] = msg
	m.Scroll = 0
	m.recalculateStats()
}

func (m *ChatModel) ClearMessages() {
	m.Messages = nil
	m.Scroll = 0
	m.recalculateStats()
}

func (m *ChatModel) SetModel(model, provider string) {
	m.modelName = model
	m.providerName = provider
}

func (m *ChatModel) SetTokenCount(count, window int) {
	m.TokenCount = count
	m.contextWindow = window
}

func (m *ChatModel) SetTotalCost(cost float64) {
	m.TotalCost = cost
}

func (m *ChatModel) SetGitBranch(branch string) {
	m.GitBranch = branch
}

func (m *ChatModel) SetSessionName(name string) {
	m.SessionName = name
}

func (m *ChatModel) SetThinkingLevel(level string) {
	m.ThinkingLevel = level
}

func (m *ChatModel) Message(index int) (agent.AgentMessage, bool) {
	if index < 0 || index >= len(m.Messages) {
		return agent.AgentMessage{}, false
	}
	return m.Messages[index], true
}

func (m *ChatModel) LastMessageIndex() int {
	return len(m.Messages) - 1
}

func (m *ChatModel) Snapshot() []agent.AgentMessage {
	return append([]agent.AgentMessage(nil), m.Messages...)
}

func (m *ChatModel) SetStatus(status string) {
	m.Status = status
}

func (m *ChatModel) ClearInput() {
	m.Input = nil
	m.Cursor = 0
	m.undoStack.Clear()
	m.redoStack.Clear()
	m.lastAction = ""
	m.closeAutocomplete()
}
