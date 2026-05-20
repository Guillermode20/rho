package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/agent/auth"
	"github.com/earendil-works/rho/pkg/agent/codecore"
	"github.com/earendil-works/rho/pkg/agent/extensions"
	"github.com/earendil-works/rho/pkg/agent/systemprompt"
	agenttheme "github.com/earendil-works/rho/pkg/agent/theme"
	"github.com/earendil-works/rho/pkg/agent/tools"
	agentutils "github.com/earendil-works/rho/pkg/agent/utils"
	"github.com/earendil-works/rho/pkg/ai"
	"github.com/earendil-works/rho/pkg/ai/providers"
	"github.com/earendil-works/rho/pkg/tui"
)

// RuntimeConfig holds the runtime configuration.
type RuntimeConfig struct {
	Model          ai.Model
	SystemPrompt   string
	APIKey         string
	Provider       ai.Provider
	CWD            string
	ExtDirs        []string
	AuthStorage    *auth.AuthStorage
	OAuthStore     *auth.OAuthStore
	Settings       *codecore.SettingsManager
	ThemeManager   *agenttheme.ThemeManager
	ClipboardWrite func(text string) error
	OpenURL        func(rawURL string) error
	OAuthExchange  func(provider ai.OAuthProviderID, code string, pkce *ai.PKCE) (*ai.OAuthCredentials, error)
}

type chatModel struct {
	width    int
	height   int
	status   string
	input    []rune
	cursor   int
	scroll   int
	messages []agent.AgentMessage
	mdTheme  tui.MarkdownTheme
	// Design system
	theme *tui.Theme

	// Metadata footer fields
	modelName     string
	providerName  string
	tokenCount    int
	contextWindow int
	totalCost     float64
	gitBranch     string

	// Cached cost map: rebuilt only when needed (lazy, set to nil to force rebuild)
	costMap      map[string]ai.ModelDefinition
	costMapDirty bool

	onSubmit          func(string)
	onMessage         func(tea.Msg)
	onAutocomplete    func(text string, cursor int) []autocompleteItem
	onAction          func(action string) bool
	autocomplete      []autocompleteItem
	autocompleteIdx   int

	// Modal state — replaces old inline prompt/selector/confirm
	modal *modalState

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

const mouseWheelScrollLines = 3
const toolPreviewLines = 3

type autocompleteItem struct {
	Value       string
	Label       string
	Description string
}

// modalState drives all centered overlay dialogs.
type modalState struct {
	active bool
	title  string
	mode   string // "prompt" | "selector" | "confirm" | "info"

	// Prompt mode
	placeholder string
	value       []rune
	onSubmit    func(string)
	onCancel    func()

	// Selector mode
	allItems      []autocompleteItem
	items         []autocompleteItem
	query         []rune
	selIdx        int
	onSelect      func(autocompleteItem)
	onSelectCancel func()

	// Confirm mode
	message string
	onYes   func()
	onNo    func()
	confirmIdx int // 0 = Yes, 1 = No

	// Info mode
	content   []string
	infoScroll int
	onDismiss  func()
}

type agentLoopEventMsg struct {
	Event agent.AgentEvent
}

type agentLoopFinalMsg struct {
	Message *agent.AgentMessage
}

type uiSelectRequestMsg struct {
	Title   string
	Options []string
	Resp    chan uiStringResponse
}

type uiConfirmRequestMsg struct {
	Title   string
	Message string
	Resp    chan uiBoolResponse
}

type uiInputRequestMsg struct {
	Title       string
	Placeholder string
	Resp        chan uiStringResponse
}

type uiStringResponse struct {
	Value string
	Err   error
}

type uiBoolResponse struct {
	Value bool
	Err   error
}

type AgentExtensionStatusMsg struct {
	Key  string
	Text string
}

func newChatModel(status string) *chatModel {
	th := tui.DefaultTheme
	return &chatModel{
		status: status,
		theme:  th,
		mdTheme: tui.DefaultMarkdownTheme(),
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

func (m *chatModel) ApplyTheme(agtTheme agenttheme.Theme) {
	bg := lipgloss.Color(themeColor(agtTheme, "bg", "235"))
	fg := lipgloss.Color(themeColor(agtTheme, "fg", "255"))
	accent := lipgloss.Color(themeColor(agtTheme, "accent", "63"))
	success := lipgloss.Color(themeColor(agtTheme, "success", "120"))
	subtle := lipgloss.Color(themeColor(agtTheme, "subtle", "242"))

	// Build new design-system theme from agent theme
	pal := tui.DefaultPalette()
	pal.Bg = 235
	pal.Fg = 255
	pal.Accent = 63
	pal.Success = 120
	pal.Dim = 242
	pal.Border = 238
	m.theme = tui.NewTheme(pal)

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
	m.mdTheme = markdownThemeFromAgentTheme(agtTheme)
}

func themeColor(theme agenttheme.Theme, key, fallback string) string {
	color, ok := theme.Colors[key]
	if !ok {
		return fallback
	}
	if color.Hex != "" {
		return color.Hex
	}
	if color.ANSI != 0 {
		return fmt.Sprintf("%d", color.ANSI)
	}
	return fallback
}

func markdownThemeFromAgentTheme(theme agenttheme.Theme) tui.MarkdownTheme {
	reset := agenttheme.Reset()
	style := func(name, fallback string) string {
		if theme.Styles != nil {
			if value := theme.Styles[name]; value != "" {
				return value
			}
		}
		return fallback
	}
	title := style("title", "\x1b[1;36m")
	bold := style("bold", "\x1b[1m")
	italic := style("italic", "\x1b[2m")
	code := style("code", "\x1b[33m")
	codeBlock := style("codeblock", code)
	link := style("link", "\x1b[34m")
	dim := style("dim", "\x1b[2m")
	separator := style("separator", dim)

	return tui.MarkdownTheme{
		H1:         func(text string) string { return title + text + reset },
		H2:         func(text string) string { return title + text + reset },
		H3:         func(text string) string { return title + text + reset },
		Bold:       func(text string) string { return bold + text + reset },
		Italic:     func(text string) string { return italic + text + reset },
		Code:       func(text string) string { return code + text + reset },
		CodeBlock:  func(text string) string { return codeBlock + text + reset },
		Link:       func(text, url string) string { return link + text + reset + dim + " (" + url + ")" + reset },
		Blockquote: func(text string) string { return dim + "|" + text + reset },
		ListBullet: func(text string) string { return "- " + text },
		ListNumber: func(text string) string { return text },
		Horizontal: func() string { return separator + strings.Repeat("-", 40) + reset },
	}
}

func (m *chatModel) Init() tea.Cmd {
	return tea.WindowSize()
}

func (m *chatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.onMessage != nil {
		m.onMessage(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampScroll()
	case tea.MouseMsg:
		switch {
		case msg.Button == tea.MouseButtonWheelUp || msg.Type == tea.MouseWheelUp:
			m.scroll += mouseWheelScrollLines
			m.clampScroll()
		case msg.Button == tea.MouseButtonWheelDown || msg.Type == tea.MouseWheelDown:
			m.scroll -= mouseWheelScrollLines
			m.clampScroll()
		}
	case tea.KeyMsg:
		// Dismiss toast on any keypress
		if m.toast != "" {
			m.toast = ""
		}

		// Modal takes priority over everything
		if m.modal != nil {
			m.handleModalInput(msg)
			return m, nil
		}

		if len(m.autocomplete) > 0 {
			switch msg.String() {
			case "esc":
				m.closeAutocomplete()
				return m, nil
			case "up", "ctrl+p":
				if m.autocompleteIdx > 0 {
					m.autocompleteIdx--
				}
				return m, nil
			case "down", "ctrl+n":
				if m.autocompleteIdx < len(m.autocomplete)-1 {
					m.autocompleteIdx++
				}
				return m, nil
			case "pgup":
				m.autocompleteIdx -= 6
				if m.autocompleteIdx < 0 {
					m.autocompleteIdx = 0
				}
				return m, nil
			case "pgdown":
				m.autocompleteIdx += 6
				if m.autocompleteIdx >= len(m.autocomplete) {
					m.autocompleteIdx = len(m.autocomplete) - 1
				}
				return m, nil
			case "home":
				m.autocompleteIdx = 0
				return m, nil
			case "end":
				m.autocompleteIdx = len(m.autocomplete) - 1
				return m, nil
			case "tab", "enter":
				m.applyAutocomplete()
				return m, nil
			}
		}

		if m.onAction != nil {
			switch msg.String() {
			case "ctrl+l":
				if m.onAction("model.select") {
					return m, nil
				}
			case "ctrl+p":
				if m.onAction("settings.open") {
					return m, nil
				}
			case "ctrl+r":
				if m.onAction("session.resume") {
					return m, nil
				}
			case "ctrl+t":
				if m.onAction("thinking.cycle") {
					return m, nil
				}
			}
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "pgup":
			m.scroll += max(1, m.viewportHeight()-1)
			m.clampScroll()
		case "pgdown":
			m.scroll -= max(1, m.viewportHeight()-1)
			m.clampScroll()
		case "ctrl+a", "home":
			m.cursor = 0
			m.updateAutocomplete()
		case "ctrl+e", "end":
			m.cursor = len(m.input)
			m.updateAutocomplete()
		case "ctrl+u":
			m.input = nil
			m.cursor = 0
			m.updateAutocomplete()
		case "ctrl+k":
			if m.cursor < len(m.input) {
				m.input = m.input[:m.cursor]
			}
			m.updateAutocomplete()
		case "left":
			if m.cursor > 0 {
				m.cursor--
			}
			m.updateAutocomplete()
		case "right":
			if m.cursor < len(m.input) {
				m.cursor++
			}
			m.updateAutocomplete()
		case "backspace", "ctrl+h":
			if m.cursor > 0 {
				m.input = append(m.input[:m.cursor-1], m.input[m.cursor:]...)
				m.cursor--
			}
			m.updateAutocomplete()
		case "delete":
			if m.cursor < len(m.input) {
				m.input = append(m.input[:m.cursor], m.input[m.cursor+1:]...)
			}
			m.updateAutocomplete()
		case "alt+enter", "ctrl+j":
			m.insertRunes('\n')
		case "enter":
			value := strings.TrimSpace(string(m.input))
			if value != "" && m.onSubmit != nil {
				m.onSubmit(value)
			}
		case "esc":
			if m.onAction != nil && m.onAction("agent.abort") {
				return m, nil
			}
		default:
			if msg.Type == tea.KeyRunes {
				m.insertRunes(msg.Runes...)
			} else if msg.Type == tea.KeySpace {
				m.insertRunes(' ')
			}
		}
	}

	return m, nil
}

func (m *chatModel) View() string {
	width := m.width
	if width <= 0 {
		width = 80
	}

	th := m.theme
	if th == nil {
		th = tui.DefaultTheme
	}

	// If a modal is active, render the normal view dimmed with the modal overlaid
	if m.modal != nil {
		return m.renderModalView(width)
	}

	// ── Status bar (minimal) ──
	status := m.statusStyle.Width(width).Render(tui.SliceByColumn(m.status, 0, max(1, width-2), true))

	// ── Contextual hint (very subtle) ──
	var hint string
	if len(m.messages) == 0 {
		hint = th.Muted("Type a message and press Enter  Ctrl+L change model  /commands for help")
	} else {
		hint = th.Muted("Enter send  Alt+Enter newline  ↑↓/PgUp/PgDn scroll  Ctrl+L model")
	}

	// ── Main content ──
	viewport := m.renderTranscript(width, m.viewportHeight())
	input := m.renderInput(width)

	// ── Assemble ──
	parts := make([]string, 0, 7)
	parts = append(parts, status)
	if toast := m.renderToast(width); toast != "" {
		parts = append(parts, "")
		parts = append(parts, toast)
	}
	parts = append(parts, viewport)
	parts = append(parts, hint)
	parts = append(parts, input)
	if footer := m.renderFooter(width); footer != "" {
		parts = append(parts, footer)
	}
	if suggestions := m.renderAutocomplete(width); suggestions != "" {
		parts = append(parts, suggestions)
	}

	return strings.Join(parts, "\n")
}

// renderModalView renders the full normal view dimmed, then overlays the modal box centered.
func (m *chatModel) renderModalView(width int) string {
	th := m.theme
	if th == nil {
		th = tui.DefaultTheme
	}

	// Build the normal view lines (status + transcript + hint + input + footer)
	status := m.statusStyle.Width(width).Render(tui.SliceByColumn(m.status, 0, max(1, width-2), true))
	viewport := m.renderTranscript(width, m.viewportHeight())
	hint := th.Muted("")
	if len(m.messages) == 0 {
		hint = th.Muted("")
	}
	input := m.renderInput(width)

	parts := make([]string, 0, 7)
	parts = append(parts, status)
	if toast := m.renderToast(width); toast != "" {
		parts = append(parts, "")
		parts = append(parts, toast)
	}
	parts = append(parts, viewport)
	parts = append(parts, hint)
	parts = append(parts, input)
	if footer := m.renderFooter(width); footer != "" {
		parts = append(parts, footer)
	}

	baseStr := strings.Join(parts, "\n")
	baseLines := strings.Split(baseStr, "\n")

	// Dim all lines
	dimmed := make([]string, 0, len(baseLines))
	for _, line := range baseLines {
		if strings.TrimSpace(line) == "" {
			dimmed = append(dimmed, "")
		} else {
			dimmed = append(dimmed, th.Muted(line))
		}
	}

	// Render modal box
	modalLines := m.renderModal(width)
	if len(modalLines) == 0 {
		return strings.Join(dimmed, "\n")
	}

	// Center the modal vertically
	viewH := len(dimmed)
	modalH := len(modalLines)
	top := max(1, (viewH-modalH)/2)

	// Overlay modal onto dimmed view, preserving and dimming background on left/right sides
	result := make([]string, len(dimmed))
	copy(result, dimmed)
	for i := 0; i < modalH && top+i < len(result); i++ {
		bgLine := baseLines[top+i]
		modalLine := modalLines[i]
		modalW := tui.VisibleWidth(modalLine)
		padding := max(0, (width-modalW)/2)

		leftBg := tui.SliceByColumn(bgLine, 0, padding, true)
		if lw := tui.VisibleWidth(leftBg); lw < padding {
			leftBg += strings.Repeat(" ", padding-lw)
		}

		rightBg := tui.SliceByColumn(bgLine, padding+modalW, 1000, true)
		rightW := max(0, width-padding-modalW)
		if rw := tui.VisibleWidth(rightBg); rw < rightW {
			rightBg += strings.Repeat(" ", rightW-rw)
		}

		result[top+i] = th.Muted(leftBg) + modalLine + th.Muted(rightBg)
	}

	return strings.Join(result, "\n")
}

func (m *chatModel) AddMessage(msg agent.AgentMessage) {
	m.messages = append(m.messages, msg)
	m.scroll = 0
	m.recalculateStats()
}

func (m *chatModel) UpdateMessage(index int, msg agent.AgentMessage) {
	if index < 0 || index >= len(m.messages) {
		return
	}
	m.messages[index] = msg
	m.scroll = 0
	m.recalculateStats()
}

func (m *chatModel) ClearMessages() {
	m.messages = nil
	m.scroll = 0
	m.recalculateStats()
}

func (m *chatModel) SetModel(model, provider string) {
	m.modelName = model
	m.providerName = provider
}

func (m *chatModel) SetTokenCount(count, window int) {
	m.tokenCount = count
	m.contextWindow = window
}

func (m *chatModel) SetTotalCost(cost float64) {
	m.totalCost = cost
}

func (m *chatModel) SetGitBranch(branch string) {
	m.gitBranch = branch
}

// buildCostMap builds a lookup map from provider/model key to ModelDefinition for fast cost calculation.
func buildCostMap() map[string]ai.ModelDefinition {
	m := make(map[string]ai.ModelDefinition)
	for _, def := range ai.DefaultModels() {
		key := string(def.Provider) + "/" + def.Name
		m[key] = def
	}
	return m
}

func (m *chatModel) getCostMap() map[string]ai.ModelDefinition {
	if m.costMap == nil || m.costMapDirty {
		m.costMap = buildCostMap()
		m.costMapDirty = false
	}
	return m.costMap
}

func (m *chatModel) invalidateCostMap() {
	m.costMapDirty = true
}

func (m *chatModel) recalculateStats() {
	var lastTokens int
	var totalCost float64

	costMap := m.getCostMap()

	for _, msg := range m.messages {
		if msg.Usage != nil {
			lastTokens = msg.Usage.TotalTokens
			if msg.Usage.Cost.Total > 0 {
				totalCost += msg.Usage.Cost.Total
			} else if msg.Usage.Input > 0 || msg.Usage.Output > 0 {
				key := string(msg.Provider) + "/" + msg.Model
				if def, ok := costMap[key]; ok {
					c := ai.CalculateCost(def, *msg.Usage)
					totalCost += c.Total
				}
			}
		}
	}

	m.tokenCount = lastTokens
	m.totalCost = totalCost
}

func (m *chatModel) renderFooter(width int) string {
	th := m.theme
	if th == nil {
		th = tui.DefaultTheme
	}
	if width <= 0 {
		return ""
	}

	var parts []string

	// Git branch
	if m.gitBranch != "" {
		parts = append(parts, th.Colored("⎇  "+m.gitBranch, th.Palette.Success))
	}

	// Model/provider
	if m.modelName != "" {
		modelStr := m.modelName
		if m.providerName != "" {
			modelStr = m.providerName + "/" + m.modelName
		}
		parts = append(parts, th.Colored(modelStr, th.Palette.Accent))
	}

	// Token usage & context window
	if m.contextWindow > 0 {
		tokenStr := fmt.Sprintf("%d tok", m.tokenCount)
		pct := float64(m.tokenCount) / float64(m.contextWindow) * 100
		var pctStr string
		if pct < 1.0 && m.tokenCount > 0 {
			pctStr = "<1%"
		} else {
			pctStr = fmt.Sprintf("%d%%", int(pct))
		}
		tokenStr = fmt.Sprintf("%s / %d ctx (%s)", tokenStr, m.contextWindow, pctStr)
		parts = append(parts, th.Muted(tokenStr))
	} else if m.tokenCount > 0 {
		parts = append(parts, th.Muted(fmt.Sprintf("%d tok", m.tokenCount)))
	}

	// Cost — use %g to avoid trailing zeros (0.0045 not 0.00450000)
	if m.totalCost > 0 {
		parts = append(parts, th.Colored(fmt.Sprintf("$%g", m.totalCost), th.Palette.Warning))
	}

	if len(parts) == 0 {
		return ""
	}

	// Build the line
	sep := th.Muted(" · ")
	line := strings.Join(parts, sep)

	// Pad to full width
	if tui.VisibleWidth(line) > width {
		line = tui.SliceByColumn(line, 0, width, true)
	} else {
		line += strings.Repeat(" ", max(0, width-tui.VisibleWidth(line)))
	}

	return line
}

func (m *chatModel) Message(index int) (agent.AgentMessage, bool) {
	if index < 0 || index >= len(m.messages) {
		return agent.AgentMessage{}, false
	}
	return m.messages[index], true
}

func (m *chatModel) LastMessageIndex() int {
	return len(m.messages) - 1
}

func (m *chatModel) Snapshot() []agent.AgentMessage {
	return append([]agent.AgentMessage(nil), m.messages...)
}

func (m *chatModel) SetStatus(status string) {
	m.status = status
}

func (m *chatModel) ClearInput() {
	m.input = nil
	m.cursor = 0
	m.closeAutocomplete()
}

// ─── Modal methods ────────────────────────────────────────────────────────────

func (m *chatModel) OpenModalPrompt(title, placeholder string, onSubmit func(string), onCancel func()) {
	m.closeModal()
	m.modal = &modalState{
		active:      true,
		mode:        "prompt",
		title:       title,
		placeholder: placeholder,
		onSubmit:    onSubmit,
		onCancel:    onCancel,
	}
}

func (m *chatModel) OpenModalSelector(title string, items []autocompleteItem, onSelect func(autocompleteItem), onCancel func()) {
	m.closeModal()
	m.modal = &modalState{
		active:         true,
		mode:           "selector",
		title:          title,
		allItems:       append([]autocompleteItem(nil), items...),
		items:          append([]autocompleteItem(nil), items...),
		onSelect:       onSelect,
		onSelectCancel: onCancel,
	}
}

func (m *chatModel) OpenModalConfirm(title, message string, onYes func(), onNo func()) {
	m.closeModal()
	m.modal = &modalState{
		active:     true,
		mode:       "confirm",
		title:      title,
		message:    message,
		onYes:      onYes,
		onNo:       onNo,
		confirmIdx: 0,
	}
}

func (m *chatModel) OpenModalInfo(title string, content []string, onDismiss func()) {
	m.closeModal()
	m.modal = &modalState{
		active:    true,
		mode:      "info",
		title:     title,
		content:   append([]string(nil), content...),
		onDismiss: onDismiss,
	}
}

func (m *chatModel) closeModal() {
	if m.modal != nil {
		m.modal = nil
	}
}

// showToast displays a brief centred notification that auto-dismisses.
func (m *chatModel) showToast(text string) {
	if text == "" {
		return
	}
	m.toast = text
	m.toastUntil = time.Now().Add(4 * time.Second)
}

// renderToast returns a centred toast banner line, or empty string if no toast is active.
func (m *chatModel) renderToast(width int) string {
	// Check expiry
	if m.toast == "" || time.Now().After(m.toastUntil) || width <= 0 {
		m.toast = ""
		return ""
	}
	th := m.theme
	if th == nil {
		th = tui.DefaultTheme
	}
	// Style the toast with a subtle background box
	display := m.toast
	if tui.VisibleWidth(display) > width-6 {
		display = tui.SliceByColumn(display, 0, width-6, true)
	}
	// Centred with styled background
	box := " " + display + " "
	boxW := tui.VisibleWidth(box)
	padding := max(0, (width-boxW)/2)
	// Use a subtle background colour to make it pop
	styled := th.Bg(box, th.Palette.Surface)
	return strings.Repeat(" ", padding) + styled
}

// handleModalInput processes keyboard input when a modal is active.
func (m *chatModel) handleModalInput(msg tea.KeyMsg) {
	modal := m.modal
	if modal == nil {
		return
	}

	switch modal.mode {
	case "prompt":
		switch msg.String() {
		case "esc":
			onCancel := modal.onCancel
			m.closeModal()
			if onCancel != nil {
				onCancel()
			}
		case "enter":
			value := string(modal.value)
			onSubmit := modal.onSubmit
			m.closeModal()
			if onSubmit != nil {
				onSubmit(value)
			}
		case "backspace", "ctrl+h":
			if len(modal.value) > 0 {
				modal.value = modal.value[:len(modal.value)-1]
			}
		case "ctrl+u":
			modal.value = nil
		default:
			if msg.Type == tea.KeyRunes {
				modal.value = append(modal.value, msg.Runes...)
			} else if msg.Type == tea.KeySpace {
				modal.value = append(modal.value, ' ')
			}
		}

	case "selector":
		switch msg.String() {
		case "esc":
			onCancel := modal.onSelectCancel
			m.closeModal()
			if onCancel != nil {
				onCancel()
			}
		case "up", "ctrl+p":
			if modal.selIdx > 0 {
				modal.selIdx--
			}
		case "down", "ctrl+n":
			if modal.selIdx < len(modal.items)-1 {
				modal.selIdx++
			}
		case "pgup":
			modal.selIdx -= 8
			if modal.selIdx < 0 {
				modal.selIdx = 0
			}
		case "pgdown":
			modal.selIdx += 8
			modal.selIdx = max(0, min(modal.selIdx, len(modal.items)-1))
		case "home":
			modal.selIdx = 0
		case "end":
			modal.selIdx = max(0, len(modal.items)-1)
		case "enter":
			if len(modal.items) > 0 && modal.selIdx >= 0 && modal.selIdx < len(modal.items) {
				item := modal.items[modal.selIdx]
				onSelect := modal.onSelect
				m.closeModal()
				if onSelect != nil {
					onSelect(item)
				}
			}
		case "backspace", "ctrl+h":
			if len(modal.query) > 0 {
				modal.query = modal.query[:len(modal.query)-1]
				m.updateModalFilter()
			}
		case "ctrl+u":
			modal.query = nil
			m.updateModalFilter()
		default:
			if msg.Type == tea.KeyRunes {
				modal.query = append(modal.query, msg.Runes...)
				m.updateModalFilter()
			} else if msg.Type == tea.KeySpace {
				modal.query = append(modal.query, ' ')
				m.updateModalFilter()
			}
		}

	case "confirm":
		switch msg.String() {
		case "esc":
			onNo := modal.onNo
			m.closeModal()
			if onNo != nil {
				onNo()
			}
		case "enter":
			if modal.confirmIdx == 0 {
				onYes := modal.onYes
				m.closeModal()
				if onYes != nil {
					onYes()
				}
			} else {
				onNo := modal.onNo
				m.closeModal()
				if onNo != nil {
					onNo()
				}
			}
		case "left", "ctrl+p":
			modal.confirmIdx = 0
		case "right", "ctrl+n":
			modal.confirmIdx = 1
		}

	case "info":
		maxLines := max(3, min(len(modal.content), 16))
		maxScroll := max(0, len(modal.content)-maxLines)

		switch msg.String() {
		case "up", "ctrl+p":
			if modal.infoScroll > 0 {
				modal.infoScroll--
			}
		case "down", "ctrl+n":
			if modal.infoScroll < maxScroll {
				modal.infoScroll++
			}
		case "pgup":
			modal.infoScroll -= 10
			if modal.infoScroll < 0 {
				modal.infoScroll = 0
			}
		case "pgdown":
			modal.infoScroll += 10
			if modal.infoScroll > maxScroll {
				modal.infoScroll = maxScroll
			}
		case "home":
			modal.infoScroll = 0
		case "end":
			modal.infoScroll = maxScroll
		default:
			// Any key dismisses the info modal
			onDismiss := modal.onDismiss
			m.closeModal()
			if onDismiss != nil {
				onDismiss()
			}
		}
	}
}

func (m *chatModel) updateModalFilter() {
	modal := m.modal
	if modal == nil || modal.mode != "selector" {
		return
	}
	query := strings.ToLower(strings.TrimSpace(string(modal.query)))
	modal.items = modal.items[:0]
	for _, item := range modal.allItems {
		haystack := strings.ToLower(item.Value + " " + item.Label + " " + item.Description)
		if query == "" || strings.Contains(haystack, query) {
			modal.items = append(modal.items, item)
		}
	}
	modal.selIdx = max(0, min(modal.selIdx, len(modal.items)-1))
}

func (m *chatModel) insertRunes(runes ...rune) {
	next := make([]rune, 0, len(m.input)+len(runes))
	next = append(next, m.input[:m.cursor]...)
	next = append(next, runes...)
	next = append(next, m.input[m.cursor:]...)
	m.input = next
	m.cursor += len(runes)
	m.updateAutocomplete()
}

func (m *chatModel) viewportHeight() int {
	if m.height <= 0 {
		return 20
	}
	inputLines := len(strings.Split(m.inputTextWithCursor(false), "\n")) + 2
	footerLines := 0
	if m.modelName != "" || m.gitBranch != "" || m.tokenCount > 0 || m.totalCost > 0 {
		footerLines = 1
	}
	return max(3, m.height-2-inputLines-m.autocompleteHeight()-footerLines)
}

// renderModal returns the centered modal box lines.
// Returns nil if no modal is active.
func (m *chatModel) renderModal(width int) []string {
	if m.modal == nil || width <= 0 {
		return nil
	}
	th := m.theme
	if th == nil {
		th = tui.DefaultTheme
	}

	modal := m.modal

	// Calculate modal dimensions
	boxWidth := min(width-8, 72)
	if boxWidth < 30 {
		boxWidth = width - 2
	}
	innerW := boxWidth - 4 // padding inside box

	var contentLines []string

	switch modal.mode {
	case "prompt":
		// Input field
		value := string(modal.value)
		displayValue := value
		if displayValue == "" && modal.placeholder != "" {
			displayValue = th.Muted(modal.placeholder)
		} else if len(value) > 0 {
			// Mask the API key / sensitive input — show last 4 chars
			if len(value) > 4 {
				displayValue = strings.Repeat("█", len(value)-4) + value[len(value)-4:]
			} else {
				displayValue = strings.Repeat("█", len(value))
			}
		}
		promptLine := " " + th.Accent("▌") + " " + displayValue
		if tui.VisibleWidth(promptLine) > innerW {
			promptLine = tui.SliceByColumn(promptLine, tui.VisibleWidth(promptLine)-innerW, tui.VisibleWidth(promptLine), true)
		} else {
			promptLine += strings.Repeat(" ", innerW-tui.VisibleWidth(promptLine))
		}
		contentLines = append(contentLines, "  "+th.Muted("┌"+strings.Repeat("─", innerW)+"┐"))
		contentLines = append(contentLines, "  "+th.Muted("│")+promptLine+th.Muted("│"))
		contentLines = append(contentLines, "  "+th.Muted("└"+strings.Repeat("─", innerW)+"┘"))
		contentLines = append(contentLines, "")
		contentLines = append(contentLines, "  "+th.Muted("Enter submit  ·  Esc cancel"))

	case "selector":
		// Filter line
		query := string(modal.query)
		if query == "" {
			contentLines = append(contentLines, "  "+th.Muted("search: ")+th.Dim("type to filter"))
		} else {
			contentLines = append(contentLines, "  "+th.Muted("search: ")+th.Accent(query))
		}

		if len(modal.items) == 0 {
			contentLines = append(contentLines, "  "+th.Muted("No matches"))
		}

		// Items — limit to visible height
		maxVisible := 10
		start := max(0, min(modal.selIdx-maxVisible/2, len(modal.items)-maxVisible))
		end := min(start+maxVisible, len(modal.items))
		for i := start; i < end; i++ {
			item := modal.items[i]
			var line string
			if i == modal.selIdx {
				line = "  " + th.Accent("▸") + " " + th.BoldAccent(item.Label)
			} else {
				line = "    " + item.Label
			}
			if item.Description != "" && innerW > 40 {
				descMax := innerW - tui.VisibleWidth(line) - 2
				if descMax > 8 {
					desc := tui.SliceByColumn(strings.ReplaceAll(item.Description, "\n", " "), 0, descMax, true)
					padding := max(1, innerW-tui.VisibleWidth(line)-tui.VisibleWidth(desc)-1)
					line += strings.Repeat(" ", padding) + th.Muted(desc)
				}
			}
			if tui.VisibleWidth(line) > innerW+2 {
				line = tui.SliceByColumn(line, 0, innerW+2, true)
			} else {
				line += strings.Repeat(" ", innerW+2-tui.VisibleWidth(line))
			}
			contentLines = append(contentLines, line)
		}
		if end < len(modal.items) {
			contentLines = append(contentLines, "  "+th.Muted(fmt.Sprintf("⋯ %d more", len(modal.items)-end)))
		}
		contentLines = append(contentLines, "")
		contentLines = append(contentLines, "  "+th.Muted("Type to filter  ·  ↑↓ navigate  ·  Enter select  ·  Esc cancel"))

	case "confirm":
		// Message
		for _, line := range strings.Split(modal.message, "\n") {
			wrapped := tui.SliceByColumn(line, 0, innerW, true)
			contentLines = append(contentLines, "  "+wrapped)
		}
		contentLines = append(contentLines, "")
		// Yes / No toggle
		var choiceLine string
		if modal.confirmIdx == 0 {
			choiceLine = "  " + th.Accent("▸ Yes") + "    " + th.Muted("No")
		} else {
			choiceLine = "  " + th.Muted("Yes") + "    " + th.Accent("▸ No")
		}
		contentLines = append(contentLines, choiceLine)
		contentLines = append(contentLines, "")
		contentLines = append(contentLines, "  "+th.Muted("← → switch  ·  Enter confirm  ·  Esc cancel"))

	case "info":
		// Scrollable content
		maxLines := max(3, min(len(modal.content), 16))
		scroll := modal.infoScroll
		start := max(0, min(scroll, len(modal.content)-maxLines))
		end := min(start+maxLines, len(modal.content))
		for i := start; i < end; i++ {
			line := modal.content[i]
			if tui.VisibleWidth(line) > innerW {
				line = tui.SliceByColumn(line, 0, innerW, true)
			} else {
				line += strings.Repeat(" ", innerW-tui.VisibleWidth(line))
			}
			contentLines = append(contentLines, "  "+line)
		}
		contentLines = append(contentLines, "")
		hint := "any key to close"
		if len(modal.content) > maxLines {
			hint = "↑↓ scroll  ·  any key to close"
		}
		contentLines = append(contentLines, "  "+th.Muted(hint))
	}

	// Build the modal box
	boxHeight := len(contentLines) + 3 // top/bottom borders + title
	topBorder := th.Accent("┌── ") + th.BoldAccent(modal.title) + " " + th.Muted(strings.Repeat("─", max(0, boxWidth-6-len(modal.title)-3))) + th.Accent("┐")
	bottomBorder := th.Accent("└") + th.Muted(strings.Repeat("─", boxWidth-2)) + th.Accent("┘")

	boxLines := make([]string, 0, boxHeight+2)
	boxLines = append(boxLines, topBorder)
	boxLines = append(boxLines, th.Muted("│")+strings.Repeat(" ", boxWidth-2)+th.Muted("│")) // spacer
	for _, line := range contentLines {
		// Pad line to fill box width
		lw := tui.VisibleWidth(line)
		if lw < boxWidth-2 {
			line += strings.Repeat(" ", boxWidth-2-lw)
		}
		boxLines = append(boxLines, th.Muted("│")+line+th.Muted("│"))
	}
	boxLines = append(boxLines, bottomBorder)

	return boxLines
}

func (m *chatModel) renderTranscript(width, height int) string {
	lines := m.renderMessages(width)
	if height <= 0 {
		return ""
	}
	if len(lines) == 0 {
		lines = m.renderEmptyState(width)
	}

	maxScroll := max(0, len(lines)-height)
	if m.scroll > maxScroll {
		m.scroll = maxScroll
	}
	start := max(0, len(lines)-height-m.scroll)
	end := min(len(lines), start+height)
	view := append([]string(nil), lines[start:end]...)
	for len(view) < height {
		view = append(view, "")
	}
	return strings.Join(view, "\n")
}

func (m *chatModel) renderEmptyState(width int) []string {
	th := m.theme
	if th == nil {
		th = tui.DefaultTheme
	}
	pad := strings.Repeat(" ", 2)
	lines := []string{
		"",
		pad + th.BoldAccentAlt("rho") + th.Muted(" — your local coding agent"),
		"",
		pad + th.Muted("Start a task, ask a question, or run a slash command."),
		"",
		pad + th.Muted("Ctrl+L  select model   Ctrl+P  settings   /commands  help"),
		"",
	}
	return lines
}

func (m *chatModel) renderMessages(width int) []string {
	th := m.theme
	if th == nil {
		th = tui.DefaultTheme
	}
	var lines []string
	contentWidth := max(20, width-4)
	for _, msg := range m.messages {
		if msg.Hide {
			continue
		}
		switch msg.Role {
		case ai.RoleUser:
			lines = append(lines, "")
			lines = append(lines, th.Success("You"))
			if msg.Content != "" {
				lines = append(lines, indentLines(tui.NewMarkdown(msg.Content, m.mdTheme).Render(contentWidth), "  ")...)
			}
		case ai.RoleAssistant:
			name := msg.Model
			if name == "" {
				name = "Assistant"
			}
			lines = append(lines, "")
			// Short model tag on the same line as the name
			if msg.Provider != "" {
				lines = append(lines, th.BoldAccent(name)+"  "+th.Tag(string(msg.Provider)))
			} else {
				lines = append(lines, th.BoldAccent(name))
			}
			if msg.Content != "" {
				lines = append(lines, indentLines(tui.NewMarkdown(msg.Content, m.mdTheme).Render(contentWidth), "  ")...)
			} else if len(msg.ToolCalls) == 0 && msg.ErrorMessage == "" {
				lines = append(lines, "  "+th.Muted("Thinking...")+th.Dim(" ⋯"))
			}
			if msg.ErrorMessage != "" {
				lines = append(lines, "  "+th.Error("✖ Error: "+msg.ErrorMessage))
			}
		case ai.RoleToolResult:
			title, preview := m.renderToolResultSummary(msg, contentWidth)
			if title != "" {
				lines = append(lines, "", title)
			}
			if preview != nil {
				lines = append(lines, preview...)
			}
		}
	}
	return lines
}

func (m *chatModel) renderToolResultSummary(msg agent.AgentMessage, width int) (string, []string) {
	th := m.theme
	if th == nil {
		th = tui.DefaultTheme
	}
	name := msg.ToolName
	if name == "" {
		name = "tool"
	}

	content := strings.TrimSpace(msg.Content)
	status := "done"
	if content == "" || content == "Running..." {
		status = "running"
	} else if strings.HasPrefix(content, "Preparing ") {
		status = "preparing"
	} else if strings.HasPrefix(content, "Queued ") {
		status = "queued"
	}
	if msg.IsError {
		status = "failed"
	}

	lineCount := 0
	if (status == "done" || msg.IsError) && content != "" {
		lineCount = len(strings.Split(content, "\n"))
	}

	// Compact status badge
	statusIcon := "●"
	switch status {
	case "running":
		statusIcon = th.Accent("●")
	case "failed":
		statusIcon = th.Error("●")
	case "preparing", "queued":
		statusIcon = th.Muted("○")
	default:
		statusIcon = th.Success("●")
	}

	title := fmt.Sprintf("%s %s", statusIcon, th.Muted(name))
	if lineCount > toolPreviewLines {
		title += th.Muted(fmt.Sprintf("  %d lines", lineCount))
	}

	if status != "done" && !msg.IsError {
		return title, nil
	}
	if content == "" {
		return title, nil
	}

	contentLines := strings.Split(content, "\n")
	if len(contentLines) > toolPreviewLines {
		contentLines = append(contentLines[:toolPreviewLines], fmt.Sprintf("... %d more lines", len(contentLines)-toolPreviewLines))
	}
	preview := make([]string, 0, len(contentLines))
	for _, line := range contentLines {
		rendered := tui.SliceByColumn(line, 0, width, true)
		if msg.IsError {
			preview = append(preview, "  "+th.Error(rendered))
		} else {
			preview = append(preview, "  "+th.Muted(rendered))
		}
	}
	return title, preview
}

func (m *chatModel) renderInput(width int) string {
	th := m.theme
	if th == nil {
		th = tui.DefaultTheme
	}
	if width <= 0 {
		return ""
	}
	text := m.inputTextWithCursor(true)
	if len(m.input) == 0 {
		// Show placeholder with a subtle prompt icon
		text = th.Muted("> Type a message…")
		return text + strings.Repeat(" ", max(0, width-tui.VisibleWidth(text)))
	}
	prompt := th.Accent("▌") + " "
	text = prompt + text
	if tui.VisibleWidth(text) > width {
		text = tui.SliceByColumn(text, tui.VisibleWidth(text)-width, tui.VisibleWidth(text), true)
	} else {
		text += strings.Repeat(" ", width-tui.VisibleWidth(text))
	}
	// Subtle bottom border
	return text + "\n" + th.Muted(strings.Repeat("─", width))
}

func (m *chatModel) renderAutocomplete(width int) string {
	th := m.theme
	if th == nil {
		th = tui.DefaultTheme
	}
	if len(m.autocomplete) == 0 || width <= 0 {
		return ""
	}

	maxVisible := min(len(m.autocomplete), 6)
	start := max(0, min(m.autocompleteIdx-maxVisible/2, len(m.autocomplete)-maxVisible))
	end := min(start+maxVisible, len(m.autocomplete))
	lines := make([]string, 0, end-start+1)
	// Header
	lines = append(lines, th.Muted(strings.Repeat("─", width)))
	for i := start; i < end; i++ {
		item := m.autocomplete[i]
		var line string
		if i == m.autocompleteIdx {
			// Selected item with accent bar
			line = th.Accent("▌") + " " + th.BoldAccent(item.Label)
		} else {
			line = " " + " " + item.Label
		}
		if item.Description != "" && width > 44 {
			descMax := width - tui.VisibleWidth(line) - 3
			if descMax > 8 {
				desc := tui.SliceByColumn(strings.ReplaceAll(item.Description, "\n", " "), 0, descMax, true)
				line += strings.Repeat(" ", max(1, width-tui.VisibleWidth(line)-tui.VisibleWidth(desc)-1)) + th.Muted(desc)
			}
		}
		lines = append(lines, tui.SliceByColumn(line, 0, width, true))
	}
	if end < len(m.autocomplete) {
		lines = append(lines, th.Muted(fmt.Sprintf("  ⋯ %d more", len(m.autocomplete)-end)))
	}
	return strings.Join(lines, "\n")
}

func (m *chatModel) autocompleteHeight() int {
	if len(m.autocomplete) == 0 {
		return 0
	}
	return min(len(m.autocomplete), 6)
}

func (m *chatModel) inputTextWithCursor(showCursor bool) string {
	value := append([]rune(nil), m.input...)
	if showCursor {
		cursor := m.cursor
		if cursor < 0 {
			cursor = 0
		}
		if cursor > len(value) {
			cursor = len(value)
		}
		value = append(value[:cursor], append([]rune{'▏'}, value[cursor:]...)...)
	}
	return string(value)
}

func (m *chatModel) clampScroll() {
	maxScroll := max(0, len(m.renderMessages(max(20, m.width)))-m.viewportHeight())
	if m.scroll > maxScroll {
		m.scroll = maxScroll
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}

func (m *chatModel) updateAutocomplete() {
	if m.onAutocomplete == nil {
		m.closeAutocomplete()
		return
	}
	items := m.onAutocomplete(string(m.input), m.cursor)
	m.autocomplete = items
	if len(items) == 0 {
		m.autocompleteIdx = 0
		return
	}
	if m.autocompleteIdx >= len(items) {
		m.autocompleteIdx = len(items) - 1
	}
}

func (m *chatModel) closeAutocomplete() {
	m.autocomplete = nil
	m.autocompleteIdx = 0
}

func (m *chatModel) applyAutocomplete() {
	if len(m.autocomplete) == 0 || m.autocompleteIdx >= len(m.autocomplete) {
		return
	}
	item := m.autocomplete[m.autocompleteIdx]
	text := string(m.input)
	replacement := item.Value
	if strings.HasPrefix(text, "/") && !strings.Contains(strings.TrimSpace(text), " ") {
		replacement += " "
	}
	m.input = []rune(replacement)
	m.cursor = len(m.input)
	m.closeAutocomplete()
}

func indentLines(lines []string, prefix string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			out = append(out, "")
			continue
		}
		out = append(out, prefix+line)
	}
	return out
}

// InteractiveMode is the full TUI agent interface.
type InteractiveMode struct {
	program              *tea.Program
	programRunning       bool
	ui                   *chatModel
	agent                *agent.AgentLoop
	config               *RuntimeConfig
	extRuntime           *extensions.Runtime
	extCtx               extensions.ExtensionContext
	coordinator          *codecore.RuntimeCoordinator
	sessionManager       *agent.SessionManager
	slashCommands        *codecore.SlashCommandManager
	sessionID            string
	pendingLoginProvider string
	extensionStatuses    map[string]string
	streamingMessageIdx  int
	pendingToolMessages  map[string]int
	agentCancel          context.CancelFunc
}

// NewInteractiveMode creates a new interactive mode.
func NewInteractiveMode(cfg *RuntimeConfig) *InteractiveMode {
	// Create extension runtime
	extRuntime := extensions.NewRuntime()

	// Build extension context
	extCtx := extensions.ExtensionContext{
		HasUI:            true,
		CWD:              cfg.CWD,
		ExtensionRuntime: extRuntime,
		Abort:            nil, // set when agent loop is created
		Shutdown: func() {
			os.Exit(0)
		},
	}

	// Load extensions from configured directories
	if len(cfg.ExtDirs) > 0 {
		result := extensions.LoadExtensions(cfg.ExtDirs, extRuntime)
		for _, name := range result.Loaded {
			fmt.Fprintf(os.Stderr, "Loaded extension: %s\n", name)
		}
		for _, err := range result.Errors {
			fmt.Fprintf(os.Stderr, "Extension error: %s\n", err)
		}
	}

	// Create session manager
	rhoDir := filepath.Join(os.Getenv("HOME"), ".rho")
	sessionMgr := agent.NewSessionManager(filepath.Join(rhoDir, "sessions"))
	settingsMgr := cfg.Settings
	if settingsMgr == nil {
		settingsMgr = codecore.NewSettingsManager(filepath.Join(rhoDir, "settings"), cfg.CWD)
		cfg.Settings = settingsMgr
	}
	themeMgr := cfg.ThemeManager
	if themeMgr == nil {
		themeMgr = agenttheme.NewThemeManager(filepath.Join(rhoDir, "themes"))
		if err := themeMgr.LoadThemes(); err != nil {
			fmt.Fprintf(os.Stderr, "Theme load error: %s\n", err)
		}
		cfg.ThemeManager = themeMgr
	}
	if selectedTheme := settingsMgr.GetString("theme"); selectedTheme != "" {
		_ = themeMgr.SetActive(selectedTheme)
	}

	services, err := codecore.NewAgentSessionServices(codecore.CreateAgentSessionServicesOptions{
		ExtDirs: cfg.ExtDirs,
	})
	var coordinator *codecore.RuntimeCoordinator
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not initialize agent services: %v\n", err)
	} else {
		services.AuthStorage = cfg.AuthStorage
		services.OAuthStore = cfg.OAuthStore
		services.Settings = cfg.Settings
		services.ModelReg.SetAuthProvider(cfg.AuthStorage)

		coordinator = codecore.NewRuntimeCoordinator(
			services,
			cfg.OAuthStore,
			cfg.AuthStorage,
			themeMgr,
			cfg.Settings,
			sessionMgr,
			cfg.Model,
			cfg.Provider,
			cfg.APIKey,
			cfg.CWD,
			agent.CurrentSessionID(),
		)
	}

	im := &InteractiveMode{
		config:              cfg,
		extRuntime:          extRuntime,
		extCtx:              extCtx,
		sessionManager:      sessionMgr,
		slashCommands:       codecore.NewSlashCommandManager(),
		sessionID:           agent.CurrentSessionID(),
		extensionStatuses:   make(map[string]string),
		streamingMessageIdx: -1,
		pendingToolMessages: make(map[string]int),
		coordinator:         coordinator,
	}

	// Set up UI on the BTModel
	im.setupUI()
	im.extCtx.UI = im.extensionUIContext()

	return im
}

// Run starts the interactive mode.
func (im *InteractiveMode) Run() error {
	defer func() {
		im.extCtx.ExtensionRuntime.FireSessionShutdown(im.extCtx, extensions.SessionShutdownEvent{
			Reason: extensions.SessionQuit,
		})
		im.extRuntime.StopAllProcesses()
	}()

	// Fire session start
	im.extCtx.ExtensionRuntime.FireSessionStart(im.extCtx, extensions.SessionStartEvent{
		Type: extensions.SessionStartup,
	})

	// Start Bubble Tea program (blocks until quit)
	_, err := im.program.Run()
	return err
}

func (im *InteractiveMode) setupUI() {
	status := im.statusText("")
	im.ui = newChatModel(status)
	im.applyActiveTheme()

	// Initialize metadata footer details
	im.ui.SetModel(im.config.Model.Name, string(im.config.Provider))
	im.ui.SetGitBranch(agentutils.GetGitBranch(im.config.CWD))
	def := providers.GuessModelDefinition(im.config.Provider, im.config.Model.Name)
	im.ui.SetTokenCount(0, def.ContextWindow)

	im.ui.onMessage = func(msg tea.Msg) {
		im.handleCustomMessage(msg)
	}
	im.ui.onSubmit = func(value string) {
		im.handleSubmit(value)
	}
	im.ui.onAutocomplete = func(text string, cursor int) []autocompleteItem {
		return im.autocomplete(text, cursor)
	}
	im.ui.onAction = func(action string) bool {
		return im.handleAppAction(action)
	}
	im.program = tea.NewProgram(im.ui, tea.WithAltScreen(), tea.WithMouseCellMotion())

	im.addWelcomeMessage()
}

func (im *InteractiveMode) handleAppAction(action string) bool {
	switch action {
	case "model.select":
		im.ui.ClearInput()
		im.showModelSelector("")
		return true
	case "settings.open":
		im.ui.ClearInput()
		im.showSettingsSelector()
		return true
	case "session.resume":
		im.ui.ClearInput()
		im.showSessionSelector("Resume session")
		return true
	case "thinking.cycle":
		current := im.settingString("thinkingLevel", "off")
		next := nextThinkingLevel(current)
		im.setUserSetting("thinkingLevel", next)
		im.addSystemMessage(fmt.Sprintf("Thinking level: %s", next))
		return true
	case "agent.abort":
		return im.abortAgent()
	}
	return false
}

func (im *InteractiveMode) abortAgent() bool {
	if im.agentCancel == nil {
		return false
	}
	im.agentCancel()
	im.agentCancel = nil
	im.ui.SetStatus(im.statusText("Aborting..."))
	im.addSystemMessage("Operation aborted.")
	return true
}

func (im *InteractiveMode) hasOAuth() bool {
	if im.config.OAuthStore != nil {
		return im.config.OAuthStore.HasProvider(string(im.config.Provider))
	}
	return false
}

func (im *InteractiveMode) addWelcomeMessage() {
	if im.config.Model.Name == "" {
		welcome := "rho v" + version + " — Your local coding agent\n\nNo API keys configured. Use /login to set up a provider, or Ctrl+L to select a model."
		im.ui.AddMessage(agent.AgentMessage{
			Role:    ai.RoleAssistant,
			Content: welcome,
			Model:   "rho",
		})
		return
	}
	welcome := "rho v" + version + " — Your local coding agent\nType a message and press Enter to start. Ctrl+C to quit."
	im.ui.AddMessage(agent.AgentMessage{
		Role:    ai.RoleAssistant,
		Content: welcome,
		Model:   "rho",
	})
}

func (im *InteractiveMode) extensionUIContext() extensions.ExtensionUIContext {
	return extensions.ExtensionUIContext{
		Select:    im.extensionSelect,
		Confirm:   im.extensionConfirm,
		Input:     im.extensionInput,
		Notify:    im.extensionNotify,
		SetStatus: im.extensionSetStatus,
	}
}

func (im *InteractiveMode) extensionSelect(title string, options []string) (string, error) {
	if im.program == nil {
		return "", errors.New("extension UI is not running")
	}
	resp := make(chan uiStringResponse, 1)
	im.program.Send(uiSelectRequestMsg{Title: title, Options: options, Resp: resp})
	result := <-resp
	return result.Value, result.Err
}

func (im *InteractiveMode) extensionConfirm(title, message string) (bool, error) {
	if im.program == nil {
		return false, errors.New("extension UI is not running")
	}
	resp := make(chan uiBoolResponse, 1)
	im.program.Send(uiConfirmRequestMsg{Title: title, Message: message, Resp: resp})
	result := <-resp
	return result.Value, result.Err
}

func (im *InteractiveMode) extensionInput(title, placeholder string) (string, error) {
	if im.program == nil {
		return "", errors.New("extension UI is not running")
	}
	resp := make(chan uiStringResponse, 1)
	im.program.Send(uiInputRequestMsg{Title: title, Placeholder: placeholder, Resp: resp})
	result := <-resp
	return result.Value, result.Err
}

func (im *InteractiveMode) extensionNotify(message string, msgType string) {
	if strings.TrimSpace(message) == "" {
		return
	}
	if im.program == nil {
		im.addSystemMessage(message)
		return
	}
	im.program.Send(tui.AddMessageMsg{
		Role:    string(ai.RoleAssistant),
		Content: message,
		Model:   "rho",
	})
}

func (im *InteractiveMode) extensionSetStatus(key, text string) {
	if key == "" {
		return
	}
	if im.program == nil {
		if im.extensionStatuses == nil {
			im.extensionStatuses = make(map[string]string)
		}
		im.extensionStatuses[key] = text
		return
	}
	im.program.Send(AgentExtensionStatusMsg{Key: key, Text: text})
}

func (im *InteractiveMode) handleSubmit(value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}

	if im.pendingLoginProvider != "" {
		im.handlePendingLogin(value)
		return
	}

	// Check for slash commands.
	if strings.HasPrefix(value, "/") {
		if im.handleSlashCommand(value) {
			return
		}
	}

	// If no model is configured, show a helpful message instead of failing
	if im.config.Model.Name == "" {
		im.ui.AddMessage(agent.AgentMessage{
			Role:    ai.RoleAssistant,
			Content: "No model is configured. Use /login to set up an API key, or Ctrl+L to select a model.",
			Model:   "rho",
		})
		im.ui.ClearInput()
		return
	}

	// If no API key is resolved, show a helpful message
	if im.config.APIKey == "" && !im.hasOAuth() {
		im.ui.AddMessage(agent.AgentMessage{
			Role:    ai.RoleAssistant,
			Content: fmt.Sprintf("No API key configured for %s. Use /login %s to set one up, or Ctrl+L to select a different provider.", im.config.Provider, im.config.Provider),
			Model:   "rho",
		})
		im.ui.ClearInput()
		return
	}

	// Fire input event to extensions
	inputResult, err := im.extRuntime.FireInput(im.extCtx, extensions.InputEvent{
		Text:   value,
		Source: "interactive",
	})
	if err != nil {
		im.ui.AddMessage(agent.AgentMessage{
			Role:    ai.RoleAssistant,
			Content: fmt.Sprintf("Extension error: %v", err),
			Model:   im.config.Model.Name,
		})
		return
	}
	if inputResult != nil && inputResult.Action == "handled" {
		return
	}
	if inputResult != nil && inputResult.Action == "transform" {
		value = inputResult.Text
	}

	userMsg := agent.AgentMessage{
		Role:      ai.RoleUser,
		Content:   value,
		Timestamp: time.Now().UnixMilli(),
	}
	im.ui.AddMessage(userMsg)
	turnMessages := im.ui.Snapshot()
	im.ui.ClearInput()

	im.ui.SetStatus(im.statusText("Thinking..."))

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		im.agentCancel = cancel
		defer func() {
			im.agentCancel = nil
		}()
		// Fire agent start
		im.extRuntime.FireAgentStart(im.extCtx)

		agentMsg, newMessages, err := im.runAgent(ctx, value, turnMessages)
		if err != nil {
			im.program.Send(tui.AddMessageMsg{
				Role:    string(ai.RoleAssistant),
				Content: fmt.Sprintf("Error: %v", err),
				Model:   im.config.Model.Name,
			})
			newMessages = append(newMessages, agent.AgentMessage{
				Role:         ai.RoleAssistant,
				Content:      fmt.Sprintf("Error: %v", err),
				Model:        im.config.Model.Name,
				ErrorMessage: err.Error(),
				Timestamp:    time.Now().UnixMilli(),
			})
		} else if agentMsg != nil {
			im.program.Send(agentLoopFinalMsg{Message: agentMsg})
		}

		// Save session
		priorMessages := priorConversation(turnMessages)
		sessionMessages := append(priorMessages, newMessages...)
		header := agent.SessionHeader{
			ID:        im.sessionID,
			Timestamp: time.Now().Format(time.RFC3339),
			CWD:       im.config.CWD,
		}
		if err := im.sessionManager.Save(im.sessionID, header, sessionMessages); err != nil {
			im.program.Send(tui.AddMessageMsg{
				Role:    string(ai.RoleAssistant),
				Content: fmt.Sprintf("Session save error: %v", err),
				Model:   im.config.Model.Name,
			})
		}

		// Fire agent end
		im.extRuntime.FireAgentEnd(im.extCtx, extensions.AgentEndEvent{
			Messages: sessionMessages,
		})

		im.program.Send(tui.AgentStatusMsg{
			Text: im.statusText(""),
		})
	}()
}

func (im *InteractiveMode) handlePendingLogin(value string) {
	provider := im.pendingLoginProvider
	im.pendingLoginProvider = ""
	im.ui.ClearInput()

	if value == "/cancel" {
		im.addSystemMessage(fmt.Sprintf("Login cancelled for %s.", provider))
		return
	}
	if im.config.AuthStorage == nil {
		im.addSystemMessage("No auth storage is configured.")
		return
	}
	key := strings.TrimSpace(value)
	if key == "" {
		im.addSystemMessage(fmt.Sprintf("No API key saved for %s.", provider))
		return
	}
	if err := im.config.AuthStorage.SetAPIKey(provider, key); err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not save API key for %s: %v", provider, err))
		return
	}
	if string(im.config.Provider) == provider {
		im.config.APIKey = key
	}
	im.addSystemMessage(fmt.Sprintf("Saved API key for %s in %s.", provider, shortenPath(defaultAuthKeysPath())))
	if im.coordinator != nil && im.coordinator.Services != nil && im.coordinator.Services.ModelReg != nil {
		im.coordinator.Services.ModelReg.FetchModelsAsync(ai.Provider(provider))
	}
}

func (im *InteractiveMode) handleSlashCommand(value string) bool {
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return true
	}
	cmdName := strings.ToLower(strings.TrimPrefix(parts[0], "/"))
	args := parts[1:]

	switch cmdName {
	case "login":
		im.handleLoginCommand(args)
		return true
	case "logout":
		im.handleLogoutCommand(args)
		return true
	case "model", "models":
		im.handleModelCommand(args)
		return true
	case "settings", "config":
		im.handleSettingsCommand(args)
		return true
	case "theme", "themes":
		im.handleThemeCommand(args)
		return true
	case "extensions", "ext":
		im.showExtensionsSelector()
		return true
	case "copy":
		im.copyLastAssistantMessage()
		return true
	case "sessions", "session", "resume":
		im.handleSessionsCommand(args, "Resume session")
		return true
	case "name":
		im.handleNameCommand(args)
		return true
	case "tree":
		im.handleSessionsCommand(args, "Session tree")
		return true
	case "fork":
		im.forkSession()
		return true
	case "new":
		im.startNewSession()
		return true
	case "commands":
		im.showCommandList()
		im.ui.ClearInput()
		return true
	case "tools":
		im.showAvailableTools()
		im.ui.ClearInput()
		return true
	case "reload":
		im.handleReloadCommand()
		return true
	}

	if im.hasExtensionCommand(cmdName) {
		im.ui.ClearInput()
		go func() {
			if err := im.extRuntime.HandleSlashCommand(im.extCtx, cmdName, args); err != nil {
				im.program.Send(tui.AddMessageMsg{
					Role:    string(ai.RoleAssistant),
					Content: fmt.Sprintf("Extension command error: %v", err),
					Model:   "rho",
				})
			}
		}()
		return true
	}

	if im.slashCommands != nil {
		ctx := codecore.SlashCommandContext{
			CWD:            im.config.CWD,
			SessionManager: im.sessionManager,
			Model:          &im.config.Model,
			SystemPrompt:   im.config.SystemPrompt,
			Notify: func(message string, msgType string) {
				im.addSystemMessage(message)
			},
		}
		if err := im.slashCommands.Execute(ctx, value); err == nil {
			im.ui.ClearInput()
			return true
		}
	}

	im.addSystemMessage(fmt.Sprintf("Unknown command: /%s. Type /help to see available commands.", cmdName))
	im.ui.ClearInput()
	return true
}

func (im *InteractiveMode) hasExtensionCommand(name string) bool {
	for _, cmd := range im.extRuntime.GetSlashCommands() {
		if cmd.Name == name {
			return true
		}
	}
	return false
}

func (im *InteractiveMode) handleSettingsCommand(args []string) {
	im.ui.ClearInput()
	if len(args) > 0 {
		im.addSystemMessage("Interactive settings editing is available from /settings. Select an item to change related state.")
		return
	}
	im.showSettingsSelector()
}

func (im *InteractiveMode) handleThemeCommand(args []string) {
	im.ui.ClearInput()
	if len(args) == 0 {
		im.showThemeSelector()
		return
	}
	im.selectTheme(strings.Join(args, " "))
}

func (im *InteractiveMode) showAvailableTools() {
	allTools := extensions.MergeTools(tools.AllTools(im.config.CWD), im.extRuntime.GetCustomTools())
	var lines []string
	for _, tool := range allTools {
		lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.theme.Accent(tool.Name), im.ui.theme.Muted(tool.Description)))
	}
	if len(lines) == 0 {
		lines = []string{"  No tools available."}
	}
	im.ui.OpenModalInfo("📋 Available tools", lines, nil)
}

func (im *InteractiveMode) copyLastAssistantMessage() {
	im.ui.ClearInput()
	msg, ok := im.lastAssistantMessage()
	if !ok {
		im.addSystemMessage("No assistant message to copy.")
		return
	}
	write := im.config.ClipboardWrite
	if write == nil {
		write = agentutils.DefaultClipboard().Write
	}
	if err := write(msg.Content); err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not copy assistant message: %v", err))
		return
	}
	im.addSystemMessage("Copied last assistant message to clipboard.")
}

func (im *InteractiveMode) lastAssistantMessage() (agent.AgentMessage, bool) {
	for i := len(im.ui.messages) - 1; i >= 0; i-- {
		msg := im.ui.messages[i]
		if msg.Hide || msg.Model == "rho" {
			continue
		}
		if msg.Role == ai.RoleAssistant && strings.TrimSpace(msg.Content) != "" {
			return msg, true
		}
	}
	return agent.AgentMessage{}, false
}

func (im *InteractiveMode) handleSessionsCommand(args []string, title string) {
	im.ui.ClearInput()
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "new":
			im.startNewSession()
			return
		case "list", "switch", "resume":
			im.showSessionSelector(title)
			return
		}
	}
	im.showSessionSelector(title)
}

func (im *InteractiveMode) handleNameCommand(args []string) {
	im.ui.ClearInput()
	if len(args) > 0 {
		im.setSessionName(strings.Join(args, " "))
		return
	}
	current := im.sessionName(im.sessionID)
	im.ui.OpenModalPrompt("Session name", current, func(value string) {
		im.setSessionName(value)
	}, func() {
		im.addSystemMessage("Session naming cancelled.")
	})
}

func (im *InteractiveMode) setSessionName(name string) {
	name = strings.TrimSpace(name)
	names := im.sessionNames()
	if name == "" {
		delete(names, im.sessionID)
		im.addSystemMessage("Session name cleared.")
	} else {
		names[im.sessionID] = name
		im.addSystemMessage(fmt.Sprintf("Session name set to: %s", name))
	}
	im.setUserSetting("sessionNames", names)
	im.ui.SetStatus(im.statusText(""))
}

func (im *InteractiveMode) sessionName(sessionID string) string {
	return im.sessionNames()[sessionID]
}

func (im *InteractiveMode) sessionNames() map[string]string {
	out := make(map[string]string)
	if im.config.Settings == nil {
		return out
	}
	raw := im.config.Settings.Get("sessionNames")
	switch vals := raw.(type) {
	case map[string]string:
		for k, v := range vals {
			out[k] = v
		}
	case map[string]interface{}:
		for k, v := range vals {
			if s, ok := v.(string); ok {
				out[k] = s
			}
		}
	}
	return out
}

func (im *InteractiveMode) showSessionSelector(title string) {
	if im.sessionManager == nil {
		im.addSystemMessage("No session manager is configured.")
		return
	}
	sessions, err := im.sessionManager.List()
	if err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not list sessions: %v", err))
		return
	}
	if len(sessions) == 0 {
		im.addSystemMessage("No saved sessions.")
		return
	}
	items := make([]autocompleteItem, 0, len(sessions))
	for _, session := range sessions {
		label := session.ID
		if session.ID == im.sessionID {
			label += " (current)"
		}
		desc := strings.TrimSpace(session.Preview)
		if desc == "" {
			desc = fmt.Sprintf("%d messages", session.MessageCount)
		}
		if session.CWD != "" {
			desc = shortenPath(session.CWD) + " - " + desc
		}
		items = append(items, autocompleteItem{
			Value:       session.ID,
			Label:       label,
			Description: desc,
		})
	}
	im.ui.OpenModalSelector(title, items, func(item autocompleteItem) {
		im.resumeSession(item.Value)
	}, func() {
		im.addSystemMessage("Session selection cancelled.")
	})
}

func (im *InteractiveMode) resumeSession(sessionID string) {
	header, messages, err := im.sessionManager.Load(sessionID)
	if err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not resume session %s: %v", sessionID, err))
		return
	}
	im.sessionID = sessionID
	if header.CWD != "" {
		im.config.CWD = header.CWD
	}
	im.ui.ClearMessages()
	im.addWelcomeMessage()
	for _, msg := range messages {
		im.ui.AddMessage(msg)
	}
	im.ui.SetStatus(im.statusText(""))
	im.addSystemMessage(fmt.Sprintf("Resumed session %s.", sessionID))
}

func (im *InteractiveMode) startNewSession() {
	im.sessionID = agent.CurrentSessionID()
	im.ui.ClearMessages()
	im.ui.ClearInput()
	im.addWelcomeMessage()
	im.ui.SetStatus(im.statusText(""))
	im.addSystemMessage(fmt.Sprintf("Started new session %s.", im.sessionID))
}

func (im *InteractiveMode) forkSession() {
	if im.sessionManager == nil {
		im.addSystemMessage("No session manager is configured.")
		return
	}
	parentID := im.sessionID
	newID := agent.CurrentSessionID()
	messages := conversationMessages(im.ui.Snapshot())
	header := agent.SessionHeader{
		ID:            newID,
		Timestamp:     time.Now().Format(time.RFC3339),
		CWD:           im.config.CWD,
		ParentSession: parentID,
	}
	if err := im.sessionManager.Save(newID, header, messages); err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not fork session: %v", err))
		return
	}
	im.sessionID = newID
	im.ui.SetStatus(im.statusText(""))
	im.addSystemMessage(fmt.Sprintf("Forked session %s from %s.", newID, parentID))
}

func (im *InteractiveMode) showSettingsSelector() {
	apiKeyStatus := "not configured"
	if strings.TrimSpace(im.config.APIKey) != "" {
		apiKeyStatus = "configured"
	}
	showImages := im.settingBool("showImages", true)
	thinkingLevel := im.settingString("thinkingLevel", "off")
	activeTheme := im.activeThemeName()
	items := []autocompleteItem{
		{Value: "model", Label: "Model", Description: fmt.Sprintf("%s/%s", im.config.Provider, im.config.Model.Name)},
		{Value: "auth", Label: "Authentication", Description: fmt.Sprintf("%s API key %s", im.config.Provider, apiKeyStatus)},
		{Value: "theme", Label: "Theme", Description: activeTheme},
		{Value: "extensions", Label: "Extensions", Description: "Manage installed extensions"},
		{Value: "showImages", Label: "Show images", Description: boolLabel(showImages)},
		{Value: "thinkingLevel", Label: "Thinking level", Description: thinkingLevel},
		{Value: "cwd", Label: "Working directory", Description: im.config.CWD},
		{Value: "commands", Label: "Slash commands", Description: "Show all registered commands"},
	}
	im.ui.OpenModalSelector("Settings", items, func(item autocompleteItem) {
		switch item.Value {
		case "model":
			im.showModelSelector("")
		case "auth":
			im.showLoginAuthTypeSelector()
		case "theme":
			im.showThemeSelector()
		case "extensions":
			im.showExtensionsSelector()
		case "commands":
			im.showCommandList()
		case "showImages":
			next := !showImages
			im.setUserSetting("showImages", next)
			im.addSystemMessage(fmt.Sprintf("Show images: %s", boolLabel(next)))
			im.showSettingsSelector()
		case "thinkingLevel":
			next := nextThinkingLevel(thinkingLevel)
			im.setUserSetting("thinkingLevel", next)
			im.addSystemMessage(fmt.Sprintf("Thinking level: %s", next))
			im.showSettingsSelector()
		default:
			im.addSystemMessage(fmt.Sprintf("%s: %s", item.Label, item.Description))
		}
	}, func() {
		im.addSystemMessage("Settings closed.")
	})
}

func (im *InteractiveMode) showThemeSelector() {
	if im.config.ThemeManager == nil {
		im.addSystemMessage("No theme manager is configured.")
		return
	}
	names := im.config.ThemeManager.ListThemes()
	if len(names) == 0 {
		im.addSystemMessage("No themes are available.")
		return
	}
	active := im.activeThemeName()
	items := make([]autocompleteItem, 0, len(names))
	for _, name := range names {
		item := autocompleteItem{
			Value:       name,
			Label:       name,
			Description: "theme",
		}
		if t, ok := im.config.ThemeManager.GetTheme(name); ok {
			if t.Description != "" {
				item.Description = t.Description
			}
			if t.Dark {
				if item.Description == "" || item.Description == "theme" {
					item.Description = "dark"
				} else {
					item.Description += " | dark"
				}
			}
		}
		if name == active {
			if item.Description == "" || item.Description == "theme" {
				item.Description = "current"
			} else {
				item.Description += " | current"
			}
		}
		items = append(items, item)
	}
	im.ui.OpenModalSelector("Select theme", items, func(item autocompleteItem) {
		im.selectTheme(item.Value)
	}, func() {
		im.addSystemMessage("Theme selection cancelled.")
	})
}

func (im *InteractiveMode) selectTheme(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		im.showThemeSelector()
		return
	}
	if im.config.ThemeManager == nil {
		im.addSystemMessage("No theme manager is configured.")
		return
	}
	if err := im.config.ThemeManager.SetActive(name); err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not select theme %s: %v", name, err))
		return
	}
	if im.config.Settings != nil {
		im.setUserSetting("theme", name)
	}
	im.applyActiveTheme()
	im.ui.SetStatus(im.statusText(""))
	im.addSystemMessage(fmt.Sprintf("Selected theme: %s", name))
}

func (im *InteractiveMode) applyActiveTheme() {
	if im.ui == nil || im.config.ThemeManager == nil {
		return
	}
	im.ui.ApplyTheme(im.config.ThemeManager.Active())
}

type installedExtension struct {
	Name        string
	Version     string
	Description string
	Enabled     bool
	Dir         string
}

func (im *InteractiveMode) getInstalledExtensions() []installedExtension {
	var result []installedExtension
	seen := make(map[string]bool)

	for _, dir := range im.config.ExtDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}

			name := entry.Name()
			if seen[name] {
				continue
			}
			seen[name] = true

			extDir := filepath.Join(dir, name)
			enabled := true
			if _, err := os.Stat(filepath.Join(extDir, ".disabled")); err == nil {
				enabled = false
			}

			// Read version and description from manifest
			version := "0.1.0"
			description := ""

			// 1. rho.toml
			if data, err := os.ReadFile(filepath.Join(extDir, "rho.toml")); err == nil {
				manifest, err := extensions.ParseTOML(string(data))
				if err == nil {
					version = manifest.Version
					description = manifest.Description
					if manifest.Name != "" {
						name = manifest.Name
					}
				}
			} else if data, err := os.ReadFile(filepath.Join(extDir, "extension.json")); err == nil {
				var m struct {
					Name    string `json:"name"`
					Version string `json:"version"`
					Desc    string `json:"description"`
				}
				if json.Unmarshal(data, &m) == nil {
					version = m.Version
					description = m.Desc
					if m.Name != "" {
						name = m.Name
					}
				}
			}

			result = append(result, installedExtension{
				Name:        name,
				Version:     version,
				Description: description,
				Enabled:     enabled,
				Dir:         extDir,
			})
		}
	}
	return result
}

func (im *InteractiveMode) showExtensionsSelector() {
	installed := im.getInstalledExtensions()

	items := make([]autocompleteItem, 0, len(installed))
	for _, ext := range installed {
		status := "ON"
		if !ext.Enabled {
			status = "OFF"
		}

		desc := fmt.Sprintf("[%s] v%s — %s", status, ext.Version, ext.Description)
		items = append(items, autocompleteItem{
			Value:       ext.Name,
			Label:       ext.Name,
			Description: desc,
		})
	}

	im.ui.OpenModalSelector("Manage Extensions", items, func(item autocompleteItem) {
		var selected *installedExtension
		for i := range installed {
			if installed[i].Name == item.Value {
				selected = &installed[i]
				break
			}
		}
		if selected != nil {
			im.showExtensionActions(*selected)
		}
	}, func() {
		im.addSystemMessage("Extensions closed.")
	})
}

func (im *InteractiveMode) showExtensionActions(ext installedExtension) {
	toggleLabel := "Enable"
	if ext.Enabled {
		toggleLabel = "Disable"
	}

	items := []autocompleteItem{
		{Value: "toggle", Label: toggleLabel, Description: "Toggle enabled/disabled status"},
		{Value: "reload", Label: "Reload", Description: "Hot reload this extension"},
		{Value: "uninstall", Label: "Uninstall", Description: "Remove extension files and uninstall"},
		{Value: "back", Label: "Back", Description: "Return to extensions list"},
	}

	im.ui.OpenModalSelector(fmt.Sprintf("Extension: %s", ext.Name), items, func(item autocompleteItem) {
		switch item.Value {
		case "toggle":
			disabledPath := filepath.Join(ext.Dir, ".disabled")
			if ext.Enabled {
				err := os.WriteFile(disabledPath, []byte("disabled"), 0644)
				if err != nil {
					im.addSystemMessage(fmt.Sprintf("Failed to disable extension: %v", err))
				} else {
					im.addSystemMessage(fmt.Sprintf("Disabled extension: %s. Restart session to apply.", ext.Name))
				}
			} else {
				err := os.Remove(disabledPath)
				if err != nil && !os.IsNotExist(err) {
					im.addSystemMessage(fmt.Sprintf("Failed to enable extension: %v", err))
				} else {
					im.addSystemMessage(fmt.Sprintf("Enabled extension: %s. Restart session to apply.", ext.Name))
				}
			}
			im.showExtensionsSelector()
		case "reload":
			if ext.Enabled {
				err := im.extRuntime.ReloadExtensionFromDir(ext.Name, ext.Dir, im.extensionUIContext())
				if err != nil {
					im.addSystemMessage(fmt.Sprintf("Failed to reload %s: %v", ext.Name, err))
				} else {
					im.addSystemMessage(fmt.Sprintf("Reloaded extension: %s", ext.Name))
				}
			} else {
				im.addSystemMessage(fmt.Sprintf("Cannot reload disabled extension: %s", ext.Name))
			}
			im.showExtensionsSelector()
		case "uninstall":
			im.ui.OpenModalConfirm("Uninstall Extension", fmt.Sprintf("Are you sure you want to uninstall %s?", ext.Name), func() {
				err := os.RemoveAll(ext.Dir)
				if err != nil {
					im.addSystemMessage(fmt.Sprintf("Failed to uninstall %s: %v", ext.Name, err))
				} else {
					im.extRuntime.Unregister(ext.Name)
					im.addSystemMessage(fmt.Sprintf("Uninstalled extension: %s", ext.Name))
				}
				im.showExtensionsSelector()
			}, func() {
				im.showExtensionActions(ext)
			})
		case "back":
			im.showExtensionsSelector()
		}
	}, func() {
		im.showExtensionsSelector()
	})
}

func (im *InteractiveMode) activeThemeName() string {
	if im.config.ThemeManager == nil {
		return "default"
	}
	return im.config.ThemeManager.ActiveName()
}

func (im *InteractiveMode) settingBool(key string, fallback bool) bool {
	if im.config.Settings == nil {
		return fallback
	}
	value := im.config.Settings.Get(key)
	if value == nil {
		return fallback
	}
	if b, ok := value.(bool); ok {
		return b
	}
	return fallback
}

func (im *InteractiveMode) settingString(key, fallback string) string {
	if im.config.Settings == nil {
		return fallback
	}
	if s := im.config.Settings.GetString(key); s != "" {
		return s
	}
	return fallback
}

func (im *InteractiveMode) setUserSetting(key string, value interface{}) {
	if im.config.Settings == nil {
		im.addSystemMessage("No settings manager is configured.")
		return
	}
	if err := im.config.Settings.SetUser(key, value); err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not save setting %s: %v", key, err))
	}
}

func boolLabel(value bool) string {
	if value {
		return "on"
	}
	return "off"
}

func nextThinkingLevel(current string) string {
	levels := []string{"off", "low", "medium", "high"}
	for i, level := range levels {
		if current == level {
			return levels[(i+1)%len(levels)]
		}
	}
	return "off"
}

func (im *InteractiveMode) handleLoginCommand(args []string) {
	im.ui.ClearInput()
	if im.config.AuthStorage == nil {
		im.addSystemMessage("No auth storage is configured.")
		return
	}

	if len(args) == 0 {
		im.showLoginAuthTypeSelector()
		return
	}

	provider := strings.ToLower(args[0])
	if !im.isKnownProvider(provider) {
		im.addSystemMessage(fmt.Sprintf("Unknown provider: %s\nAvailable providers: %s", provider, strings.Join(im.availableProviderNames(), ", ")))
		return
	}

	if len(args) > 1 {
		key := strings.TrimSpace(strings.Join(args[1:], " "))
		if err := im.config.AuthStorage.SetAPIKey(provider, key); err != nil {
			im.addSystemMessage(fmt.Sprintf("Could not save API key for %s: %v", provider, err))
			return
		}
		if string(im.config.Provider) == provider {
			im.config.APIKey = key
		}
		im.addSystemMessage(fmt.Sprintf("Saved API key for %s in %s.", provider, shortenPath(defaultAuthKeysPath())))
		if im.coordinator != nil && im.coordinator.Services != nil && im.coordinator.Services.ModelReg != nil {
			im.coordinator.Services.ModelReg.FetchModelsAsync(ai.Provider(provider))
		}
		return
	}

	im.pendingLoginProvider = provider
	im.addSystemMessage(fmt.Sprintf("Paste API key for %s. It will be stored in %s. Enter /cancel to abort.", provider, shortenPath(defaultAuthKeysPath())))
}

func (im *InteractiveMode) showLoginAuthTypeSelector() {
	items := []autocompleteItem{
		{Value: "api-key", Label: "API key", Description: "Paste and store a provider API key"},
		{Value: "oauth", Label: "OAuth", Description: "Choose an OAuth-capable provider"},
	}
	im.ui.OpenModalSelector("Login method", items, func(item autocompleteItem) {
		switch item.Value {
		case "api-key":
			im.showProviderSelector("Login provider", "login")
		case "oauth":
			im.showOAuthProviderSelector()
		}
	}, func() {
		im.addSystemMessage("Login cancelled.")
	})
}

func (im *InteractiveMode) showOAuthProviderSelector() {
	options := ai.GetOAuthProviders()
	if len(options) == 0 {
		im.addSystemMessage("No OAuth providers are registered.")
		return
	}
	items := make([]autocompleteItem, 0, len(options))
	for _, option := range options {
		items = append(items, autocompleteItem{
			Value:       string(option.ProviderID),
			Label:       option.Name,
			Description: option.Description,
		})
	}
	im.ui.OpenModalSelector("OAuth provider", items, func(item autocompleteItem) {
		provider := ai.OAuthProviderFactory(ai.OAuthProviderID(item.Value))
		if provider == nil || provider.AuthInfo() == nil {
			im.addSystemMessage(fmt.Sprintf("OAuth provider %s is not available.", item.Value))
			return
		}
		info := provider.AuthInfo()
		im.addSystemMessage(fmt.Sprintf("OAuth login for %s is not fully automated in this TUI yet.\nAuthorization URL: %s\nUse /login %s <api-key> for API-key auth.", item.Value, info.AuthURL, item.Value))
	}, func() {
		im.addSystemMessage("OAuth login cancelled.")
	})
}

func (im *InteractiveMode) handleLogoutCommand(args []string) {
	im.ui.ClearInput()
	if im.config.AuthStorage == nil {
		im.addSystemMessage("No auth storage is configured.")
		return
	}
	if len(args) == 0 {
		im.showProviderSelector("Logout provider", "logout")
		return
	}
	provider := strings.ToLower(args[0])
	if err := im.config.AuthStorage.DeleteAPIKey(provider); err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not remove API key for %s: %v", provider, err))
		return
	}
	if string(im.config.Provider) == provider {
		im.config.APIKey = resolveAPIKey(im.config.Model, im.config.AuthStorage)
	}
	im.addSystemMessage(fmt.Sprintf("Removed saved API key for %s.", provider))
	if im.coordinator != nil && im.coordinator.Services != nil && im.coordinator.Services.ModelReg != nil {
		im.coordinator.Services.ModelReg.ResetProviderModels(ai.Provider(provider))
	}
}

func (im *InteractiveMode) handleReloadCommand() {
	im.ui.ClearInput()
	im.addSystemMessage("Reloading extensions, skills, prompts, and themes...")

	// Track results
	var summary []string

	// 1. Reload themes
	if im.config.ThemeManager != nil {
		if err := im.config.ThemeManager.LoadThemes(); err != nil {
			summary = append(summary, fmt.Sprintf("Themes: error — %v", err))
		} else {
			// Re-apply the active theme
			active := im.config.ThemeManager.ActiveName()
			if active != "" {
				_ = im.config.ThemeManager.SetActive(active)
			}
			im.applyActiveTheme()
			summary = append(summary, fmt.Sprintf("Themes: reloaded (%d available)", len(im.config.ThemeManager.ListThemes())))
		}
	} else {
		summary = append(summary, "Themes: skipped (no theme manager)")
	}

	// 2. Reload extensions — collect names first
	oldExts := im.extRuntime.GetAllExtensions()
	extNames := make([]string, 0, len(oldExts))
	for _, ext := range oldExts {
		extNames = append(extNames, ext.Name)
	}

	// Stop all processes and watchers
	im.extRuntime.StopAllProcesses()

	// Unregister all extensions
	for _, name := range extNames {
		im.extRuntime.Unregister(name)
	}

	// Reload extensions from configured directories
	result := extensions.LoadExtensions(im.config.ExtDirs, im.extRuntime)

	loadedCount := len(result.Loaded)
	if loadedCount > 0 {
		summary = append(summary, fmt.Sprintf("Extensions: %d loaded", loadedCount))
	}
	if len(result.Skipped) > 0 {
		summary = append(summary, fmt.Sprintf("Extensions: %d skipped", len(result.Skipped)))
	}
	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			summary = append(summary, fmt.Sprintf("Extension error: %s", err))
		}
	}
	if loadedCount == 0 && len(result.Errors) == 0 {
		summary = append(summary, "Extensions: none found")
	}

	// 3. Update extension statuses on the UI
	im.ui.SetStatus(im.statusText(""))

	// Show summary
	for _, line := range summary {
		im.addSystemMessage(line)
	}
	im.addSystemMessage("Reload complete.")
}

func (im *InteractiveMode) handleModelCommand(args []string) {
	im.ui.ClearInput()
	if len(args) == 0 {
		im.showModelSelector("")
		return
	}
	query := strings.ToLower(strings.Join(args, " "))
	var bestModel *ai.ModelDefinition
	bestPriority := 9999
	for _, model := range ai.DefaultModels() {
		if strings.ToLower(model.Name) == query || strings.ToLower(string(model.Provider)+"/"+model.Name) == query {
			prio := ai.ProviderPriority(model.Provider)
			if prio < bestPriority {
				m := model
				bestModel = &m
				bestPriority = prio
			}
		}
	}
	if bestModel != nil {
		im.selectModel(*bestModel)
		return
	}
	im.showModelSelector(query)
}

func (im *InteractiveMode) showProviderSelector(title, mode string) {
	items := make([]autocompleteItem, 0)
	for _, provider := range im.availableProviderNames() {
		items = append(items, autocompleteItem{
			Value:       provider,
			Label:       provider,
			Description: "provider",
		})
	}
	if len(items) == 0 {
		im.addSystemMessage("No providers are available.")
		return
	}
	im.ui.OpenModalSelector(title, items, func(item autocompleteItem) {
		switch mode {
		case "login":
			im.handleLoginCommand([]string{item.Value})
		case "logout":
			im.handleLogoutCommand([]string{item.Value})
		}
	}, func() {
		im.addSystemMessage(title + " cancelled.")
	})
}

func (im *InteractiveMode) showModelSelector(query string) {
	query = strings.ToLower(query)
	items := make([]autocompleteItem, 0)
	noAuthItems := make([]autocompleteItem, 0)
	for _, model := range ai.DefaultModels() {
		value := string(model.Provider) + "/" + model.Name
		haystack := strings.ToLower(value)
		if query != "" && !strings.Contains(haystack, query) {
			continue
		}
		hasAuth := providerHasAuth(model.Provider, im.config.AuthStorage, im.config.OAuthStore)
		item := autocompleteItem{
			Value:       value,
			Label:       model.Name,
			Description: string(model.Provider),
		}
		if hasAuth {
			items = append(items, item)
		} else {
			noAuthItems = append(noAuthItems, item)
		}
	}
	// Append unavailable models at the end with a muted indicator
	if len(noAuthItems) > 0 {
		for i := range noAuthItems {
			noAuthItems[i].Description += " (no API key)"
		}
		items = append(items, noAuthItems...)
	}
	if len(items) == 0 {
		im.addSystemMessage("No models matched " + query + ".")
		return
	}
	// Check if any models actually have auth configured
	noAvailable := true
	for _, model := range ai.DefaultModels() {
		value := string(model.Provider) + "/" + model.Name
		haystack := strings.ToLower(value)
		if query != "" && !strings.Contains(haystack, query) {
			continue
		}
		if providerHasAuth(model.Provider, im.config.AuthStorage, im.config.OAuthStore) {
			noAvailable = false
			break
		}
	}

	im.ui.OpenModalSelector("Select model", items, func(item autocompleteItem) {
		for _, model := range ai.DefaultModels() {
			if item.Value == string(model.Provider)+"/"+model.Name {
				im.selectModel(model)
				return
			}
		}
	}, func() {
		im.addSystemMessage("Model selection cancelled.")
	})

	if noAvailable && query == "" {
		im.addSystemMessage("No API keys configured. Configure one with /login to use that provider's models.")
	}
}

func (im *InteractiveMode) selectModel(model ai.ModelDefinition) {
	im.config.Model = ai.Model{
		API:      model.API,
		Provider: model.Provider,
		Name:     model.Name,
		BaseURL:  model.BaseURL,
	}
	im.config.Provider = model.Provider
	im.config.APIKey = resolveAPIKey(im.config.Model, im.config.AuthStorage)
	im.ui.SetModel(model.Name, string(model.Provider))
	im.ui.SetTokenCount(im.ui.tokenCount, model.ContextWindow)
	im.ui.SetStatus(im.statusText(""))
	im.addSystemMessage(fmt.Sprintf("Selected model: %s/%s", model.Provider, model.Name))
}

func (im *InteractiveMode) showCommandList() {
	var lines []string
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.theme.Accent("/login"), im.ui.theme.Muted("Set up a provider or API key")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.theme.Accent("/logout"), im.ui.theme.Muted("Remove stored credentials")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.theme.Accent("/model"), im.ui.theme.Muted("Select a model")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.theme.Accent("/settings"), im.ui.theme.Muted("Open settings")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.theme.Accent("/theme"), im.ui.theme.Muted("Select a theme")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.theme.Accent("/new"), im.ui.theme.Muted("Start a new session")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.theme.Accent("/resume"), im.ui.theme.Muted("Resume a previous session")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.theme.Accent("/fork"), im.ui.theme.Muted("Fork the current session")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.theme.Accent("/name"), im.ui.theme.Muted("Name the current session")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.theme.Accent("/copy"), im.ui.theme.Muted("Copy last assistant message")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.theme.Accent("/commands"), im.ui.theme.Muted("Show all commands")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.theme.Accent("/tools"), im.ui.theme.Muted("Show available tools")))

	// Add extension slash commands
	for _, cmd := range im.extRuntime.GetSlashCommands() {
		lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.theme.Accent("/"+cmd.Name), im.ui.theme.Muted(cmd.Description)))
	}

	im.ui.OpenModalInfo("📋 Slash Commands", lines, nil)
}

func (im *InteractiveMode) availableProviderNames() []string {
	seen := make(map[string]bool)
	var names []string
	for _, model := range ai.DefaultModels() {
		name := string(model.Provider)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

func (im *InteractiveMode) isKnownProvider(provider string) bool {
	for _, name := range im.availableProviderNames() {
		if name == provider {
			return true
		}
	}
	return false
}

func (im *InteractiveMode) autocomplete(text string, cursor int) []autocompleteItem {
	if cursor < 0 {
		cursor = 0
	}
	runes := []rune(text)
	if cursor > len(runes) {
		cursor = len(runes)
	}
	prefixText := string(runes[:cursor])
	if !strings.HasPrefix(prefixText, "/") || strings.Contains(prefixText, "\n") {
		return nil
	}

	fields := strings.Fields(prefixText)
	if len(fields) == 0 {
		return im.commandAutocomplete("")
	}

	if len(fields) == 1 && !strings.HasSuffix(prefixText, " ") {
		return im.commandAutocomplete(strings.TrimPrefix(fields[0], "/"))
	}

	cmd := strings.TrimPrefix(fields[0], "/")
	argPrefix := ""
	if len(fields) > 1 {
		argPrefix = fields[len(fields)-1]
	}
	if strings.HasSuffix(prefixText, " ") {
		argPrefix = ""
	}

	switch cmd {
	case "model", "models":
		return im.modelAutocomplete(cmd, argPrefix)
	case "login", "logout":
		return im.providerAutocomplete(cmd, argPrefix)
	default:
		return nil
	}
}

func (im *InteractiveMode) commandAutocomplete(prefix string) []autocompleteItem {
	prefix = strings.ToLower(prefix)
	var items []autocompleteItem
	seen := make(map[string]bool)
	if im.slashCommands != nil {
		for _, cmd := range im.slashCommands.List() {
			if cmd.Hidden {
				continue
			}
			if prefix != "" && !strings.Contains(strings.ToLower(cmd.Name), prefix) {
				continue
			}
			seen[cmd.Name] = true
			items = append(items, autocompleteItem{
				Value:       "/" + cmd.Name,
				Label:       "/" + cmd.Name,
				Description: cmd.Description,
			})
		}
	}
	for _, cmd := range []autocompleteItem{
		{Value: "/tools", Label: "/tools", Description: "Show available agent tools"},
		{Value: "/commands", Label: "/commands", Description: "Show all registered commands"},
	} {
		name := strings.TrimPrefix(cmd.Label, "/")
		if seen[name] {
			continue
		}
		if prefix != "" && !strings.Contains(strings.ToLower(name), prefix) {
			continue
		}
		seen[name] = true
		items = append(items, cmd)
	}
	for _, cmd := range im.extRuntime.GetSlashCommands() {
		if seen[cmd.Name] {
			continue
		}
		if prefix != "" && !strings.Contains(strings.ToLower(cmd.Name), prefix) {
			continue
		}
		seen[cmd.Name] = true
		items = append(items, autocompleteItem{
			Value:       "/" + cmd.Name,
			Label:       "/" + cmd.Name,
			Description: cmd.Description,
		})
	}
	return items
}

func (im *InteractiveMode) modelAutocomplete(cmd, prefix string) []autocompleteItem {
	prefix = strings.ToLower(prefix)
	var items []autocompleteItem
	for _, model := range ai.DefaultModels() {
		value := "/" + cmd + " " + model.Name
		haystack := strings.ToLower(string(model.Provider) + " " + model.Name)
		if prefix != "" && !strings.Contains(haystack, prefix) {
			continue
		}
		desc := string(model.Provider)
		if !providerHasAuth(model.Provider, im.config.AuthStorage, im.config.OAuthStore) {
			desc += " (no API key)"
		}
		items = append(items, autocompleteItem{
			Value:       value,
			Label:       model.Name,
			Description: desc,
		})
	}
	return items
}

func (im *InteractiveMode) providerAutocomplete(cmd, prefix string) []autocompleteItem {
	prefix = strings.ToLower(prefix)
	var items []autocompleteItem
	for _, provider := range im.availableProviderNames() {
		if prefix != "" && !strings.Contains(strings.ToLower(provider), prefix) {
			continue
		}
		items = append(items, autocompleteItem{
			Value:       "/" + cmd + " " + provider,
			Label:       provider,
			Description: "provider",
		})
	}
	return items
}

func (im *InteractiveMode) addSystemMessage(content string) {
	im.ui.AddMessage(agent.AgentMessage{
		Role:      ai.RoleAssistant,
		Content:   content,
		Model:     "rho",
		Timestamp: time.Now().UnixMilli(),
	})
	// Show brief messages as a toast overlay
	if len(content) < 80 && !strings.Contains(content, "\n") {
		im.ui.showToast(content)
	}
}

func (im *InteractiveMode) statusText(activity string) string {
	th := tui.DefaultTheme
	modelLabel := "no model"
	if im.config.Model.Name != "" {
		modelLabel = fmt.Sprintf("%s/%s", im.config.Provider, im.config.Model.Name)
	}
	var parts []string
	// Left: app name + model
	parts = append(parts, th.BoldAccent("ρ"))
	if im.config.Model.Name != "" {
		parts = append(parts, th.Tag(modelLabel))
	}
	// Center: activity or cwd
	if activity != "" {
		parts = append(parts, th.Accent(activity))
	} else {
		parts = append(parts, th.Muted(shortenPath(im.config.CWD)))
	}
	// Right: session name
	if name := im.sessionName(im.sessionID); name != "" {
		parts = append(parts, th.Muted("@"+name))
	}
	// Extension statuses
	for _, text := range im.extensionStatuses {
		if strings.TrimSpace(text) != "" {
			parts = append(parts, th.Muted(text))
		}
	}
	sep := th.Muted(" · ")
	return strings.Join(parts, sep)
}

func (im *InteractiveMode) runAgent(ctx context.Context, prompt string, turnMessages []agent.AgentMessage) (*agent.AgentMessage, []agent.AgentMessage, error) {
	loop := agent.NewAgentLoop(agent.AgentLoopConfig{
		Model:             im.config.Model,
		SystemPrompt:      im.config.SystemPrompt,
		APIKey:            im.config.APIKey,
		ToolExecutionMode: agent.ToolExecutionSequential,
		Signal:            ctx,
	})

	// Apply extension hooks to the agent loop
	extensions.InstallHooks(im.extRuntime, im.extCtx, loop)

	// Merge built-in tools with extension custom tools
	builtinTools := tools.AllTools(im.config.CWD)
	extTools := im.extRuntime.GetCustomTools()
	allTools := extensions.MergeTools(builtinTools, extTools)
	systemPromptText := systemprompt.Build(systemprompt.BuildOptions{
		BaseTemplate:          im.config.SystemPrompt,
		Tools:                 allTools,
		CWD:                   im.config.CWD,
		ModelName:             im.config.Model.Name,
		ProviderName:          string(im.config.Provider),
		ExtensionInstructions: im.extRuntime.GetPromptPatches(),
		Skills:                im.extRuntime.GetCustomSkills(),
	})

	context := agent.AgentContext{
		SystemPrompt: systemPromptText,
		Model:        im.config.Model,
		Messages:     priorConversation(turnMessages),
		Tools:        allTools,
	}

	prompts := []agent.AgentMessage{
		{Role: ai.RoleUser, Content: prompt, Timestamp: time.Now().UnixMilli()},
	}

	emit := func(event agent.AgentEvent) error {
		im.program.Send(agentLoopEventMsg{Event: event})
		return nil
	}

	results, err := loop.Run(prompts, context, emit)
	if err != nil {
		return nil, nil, err
	}

	for i := len(results) - 1; i >= 0; i-- {
		if results[i].Role == ai.RoleAssistant {
			return &results[i], results, nil
		}
	}

	msg := &agent.AgentMessage{
		Role:    ai.RoleAssistant,
		Content: "No response generated.",
		Model:   im.config.Model.Name,
	}
	return msg, append(results, *msg), nil
}

func priorConversation(messages []agent.AgentMessage) []agent.AgentMessage {
	if len(messages) == 0 {
		return nil
	}

	end := len(messages)
	if messages[end-1].Role == ai.RoleUser {
		end--
	}

	prior := make([]agent.AgentMessage, 0, end)
	for _, msg := range messages[:end] {
		if msg.Hide || msg.Model == "rho" {
			continue
		}
		prior = append(prior, msg)
	}
	return prior
}

func conversationMessages(messages []agent.AgentMessage) []agent.AgentMessage {
	if len(messages) == 0 {
		return nil
	}
	out := make([]agent.AgentMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.Hide || msg.Model == "rho" {
			continue
		}
		out = append(out, msg)
	}
	return out
}

func shortenPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

// ─── Handle custom messages ────────────────────────────────────────────────

// This function is called from the Bubble Tea update loop to process
// asynchronous agent messages.
func (im *InteractiveMode) handleCustomMessage(msg tea.Msg) {
	switch m := msg.(type) {
	case tui.AddMessageMsg:
		amsg := agent.AgentMessage{
			Role:    ai.Role(m.Role),
			Content: m.Content,
			Model:   m.Model,
		}
		if m.ToolCall != "" {
			amsg.ToolName = m.ToolCall
		}
		im.ui.AddMessage(amsg)

	case tui.AgentStatusMsg:
		im.ui.SetStatus(m.Text)

	case agentLoopEventMsg:
		im.applyAgentLoopEvent(m.Event)

	case agentLoopFinalMsg:
		im.applyAgentLoopFinal(m.Message)

	case uiSelectRequestMsg:
		items := make([]autocompleteItem, 0, len(m.Options))
		for _, option := range m.Options {
			items = append(items, autocompleteItem{Value: option, Label: option})
		}
		if len(items) == 0 {
			m.Resp <- uiStringResponse{Err: errors.New("no options available")}
			return
		}
		im.ui.OpenModalSelector(m.Title, items, func(item autocompleteItem) {
			m.Resp <- uiStringResponse{Value: item.Value}
		}, func() {
			m.Resp <- uiStringResponse{Err: errors.New("selection cancelled")}
		})

	case uiConfirmRequestMsg:
		im.ui.OpenModalConfirm(m.Title, m.Message, func() {
			m.Resp <- uiBoolResponse{Value: true}
		}, func() {
			m.Resp <- uiBoolResponse{Value: false}
		})

	case uiInputRequestMsg:
		im.ui.OpenModalPrompt(m.Title, m.Placeholder, func(value string) {
			m.Resp <- uiStringResponse{Value: value}
		}, func() {
			m.Resp <- uiStringResponse{Err: errors.New("input cancelled")}
		})

	case AgentExtensionStatusMsg:
		if im.extensionStatuses == nil {
			im.extensionStatuses = make(map[string]string)
		}
		if strings.TrimSpace(m.Text) == "" {
			delete(im.extensionStatuses, m.Key)
		} else {
			im.extensionStatuses[m.Key] = m.Text
		}
		im.ui.SetStatus(im.statusText(""))
	}
}

func (im *InteractiveMode) applyAgentLoopEvent(event agent.AgentEvent) {
	switch event.Type {
	case "agent_start":
		im.streamingMessageIdx = -1
		im.pendingToolMessages = make(map[string]int)
		im.ensureStreamingAssistantMessage()
		im.ui.SetStatus(im.statusText("Thinking..."))
	case "text_delta":
		msg := im.ensureStreamingAssistantMessage()
		msg.Content += event.Delta
		im.ui.UpdateMessage(im.streamingMessageIdx, msg)
		im.ui.SetStatus(im.statusText("Receiving..."))
	case "toolcall_start", "toolcall_delta":
		if event.ToolCall == nil || event.ToolCall.Name == "" {
			return
		}
		msg := im.ensureStreamingAssistantMessage()
		if event.ToolCall.ID != "" && !messageHasToolCall(msg, event.ToolCall.ID) {
			msg.ToolCalls = append(msg.ToolCalls, *event.ToolCall)
			im.ui.UpdateMessage(im.streamingMessageIdx, msg)
		}
		im.ensureToolMessage(event.ToolCall, "Preparing "+formatToolArgs(event.ToolCall.Arguments), false)
	case "toolcall_end":
		if event.ToolCall == nil {
			return
		}
		msg := im.ensureStreamingAssistantMessage()
		if !messageHasToolCall(msg, event.ToolCall.ID) {
			msg.ToolCalls = append(msg.ToolCalls, *event.ToolCall)
			im.ui.UpdateMessage(im.streamingMessageIdx, msg)
		}
		im.ensureToolMessage(event.ToolCall, "Queued "+formatToolArgs(event.ToolCall.Arguments), false)
	case "message_end":
		if event.Message == nil {
			return
		}
		if event.Message.StopReason == ai.StopReasonToolUse {
			if im.streamingMessageIdx >= 0 {
				im.ui.UpdateMessage(im.streamingMessageIdx, *event.Message)
			}
			im.streamingMessageIdx = -1
		}
		if event.Message.StopReason == ai.StopReasonAborted {
			msg := im.ensureStreamingAssistantMessage()
			msg.StopReason = ai.StopReasonAborted
			msg.ErrorMessage = "Operation aborted"
			im.ui.UpdateMessage(im.streamingMessageIdx, msg)
			im.streamingMessageIdx = -1
		}
	case "tool_execution_start":
		if event.ToolCall == nil {
			return
		}
		im.ensureToolMessage(event.ToolCall, "Running...", false)
		im.ui.SetStatus(im.statusText("Running " + event.ToolCall.Name + "..."))
	case "tool_execution_end":
		if event.ToolCall == nil {
			return
		}
		content := event.Content
		if strings.TrimSpace(content) == "" {
			content = "Done."
		}
		im.ensureToolMessage(event.ToolCall, content, event.IsError)
		im.ui.SetStatus(im.statusText("Thinking..."))
	case "error":
		msg := im.ensureStreamingAssistantMessage()
		msg.ErrorMessage = event.Error
		if msg.Content == "" {
			msg.Content = event.Error
		}
		im.ui.UpdateMessage(im.streamingMessageIdx, msg)
		im.ui.SetStatus(im.statusText("Error"))
	case "agent_end":
		im.ui.SetStatus(im.statusText(""))
	}
}

func (im *InteractiveMode) applyAgentLoopFinal(msg *agent.AgentMessage) {
	if msg == nil {
		im.streamingMessageIdx = -1
		return
	}
	if msg.Model == "" {
		msg.Model = im.config.Model.Name
	}
	if im.streamingMessageIdx >= 0 {
		im.ui.UpdateMessage(im.streamingMessageIdx, *msg)
	} else {
		im.ui.AddMessage(*msg)
	}
	im.streamingMessageIdx = -1
	im.ui.SetStatus(im.statusText(""))
}

func (im *InteractiveMode) ensureStreamingAssistantMessage() agent.AgentMessage {
	if msg, ok := im.ui.Message(im.streamingMessageIdx); ok {
		return msg
	}
	msg := agent.AgentMessage{
		Role:      ai.RoleAssistant,
		Model:     im.config.Model.Name,
		Provider:  im.config.Provider,
		Timestamp: time.Now().UnixMilli(),
	}
	im.ui.AddMessage(msg)
	im.streamingMessageIdx = im.ui.LastMessageIndex()
	return msg
}

func (im *InteractiveMode) ensureToolMessage(tc *ai.ToolCall, content string, isError bool) {
	if tc == nil {
		return
	}
	msg := agent.AgentMessage{
		Role:       ai.RoleToolResult,
		ToolCallID: tc.ID,
		ToolName:   tc.Name,
		Content:    content,
		IsError:    isError,
		Model:      im.config.Model.Name,
		Timestamp:  time.Now().UnixMilli(),
	}
	if im.pendingToolMessages == nil {
		im.pendingToolMessages = make(map[string]int)
	}
	if idx, ok := im.pendingToolMessages[tc.ID]; ok {
		im.ui.UpdateMessage(idx, msg)
		return
	}
	im.ui.AddMessage(msg)
	im.pendingToolMessages[tc.ID] = im.ui.LastMessageIndex()
}

func messageHasToolCall(msg agent.AgentMessage, id string) bool {
	for _, tc := range msg.ToolCalls {
		if tc.ID == id {
			return true
		}
	}
	return false
}

func formatToolArgs(args map[string]interface{}) string {
	if len(args) == 0 {
		return "{}"
	}
	var parts []string
	for key, value := range args {
		parts = append(parts, fmt.Sprintf("%s=%v", key, value))
	}
	return strings.Join(parts, ", ")
}

func (im *InteractiveMode) startOAuthLogin(providerID ai.OAuthProviderID) {
	if im.config.OAuthStore == nil {
		im.addSystemMessage("No OAuth storage is configured.")
		return
	}
	provider, ok := ai.OAuthProviderFactory(providerID).(*ai.OAuthProvider)
	if !ok || provider == nil || provider.AuthInfo() == nil {
		im.addSystemMessage(fmt.Sprintf("OAuth provider %s is not available.", providerID))
		return
	}
	authURL, pkce, err := provider.NewAuthorizationURL()
	if err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not start OAuth login for %s: %v", providerID, err))
		return
	}

	callbacks, stop, err := startOAuthCallbackServer(provider.AuthInfo().RedirectURI)
	if err != nil {
		callbacks = nil
		im.addSystemMessage(fmt.Sprintf("Could not listen for OAuth callback: %v\nPaste the redirected URL or authorization code manually.", err))
	}

	var once sync.Once
	finish := func(code string) {
		once.Do(func() {
			if stop != nil {
				stop()
			}
			if im.canSendUI() {
				im.program.Send(uiClosePromptMsg{})
			} else {
				im.ui.closeModal()
			}
			go im.exchangeAndStoreOAuthCode(providerID, code, pkce)
		})
	}
	if callbacks != nil {
		go func() {
			select {
			case code := <-callbacks:
				if im.canSendUI() {
					im.program.Send(tui.AgentStatusMsg{Text: im.statusText("OAuth callback received")})
				}
				finish(code)
			case <-time.After(5 * time.Minute):
				if stop != nil {
					stop()
				}
				if im.canSendUI() {
					im.program.Send(tui.AddMessageMsg{
						Role:    string(ai.RoleAssistant),
						Content: fmt.Sprintf("OAuth login for %s timed out.", providerID),
						Model:   "rho",
					})
				}
			}
		}()
	}

	openErr := im.openOAuthURL(authURL)
	message := fmt.Sprintf("OAuth login for %s started.\nAuthorization URL: %s\nPaste the redirected URL or authorization code below if the browser callback is not captured.", provider.AuthInfo().Name, authURL)
	if openErr != nil {
		message += fmt.Sprintf("\nCould not open browser automatically: %v", openErr)
	}
	im.addSystemMessage(message)
	im.ui.OpenModalPrompt("OAuth callback or code", "Paste redirect URL or authorization code", func(value string) {
		code, err := extractOAuthCode(value)
		if err != nil {
			im.addSystemMessage(fmt.Sprintf("OAuth login cancelled: %v", err))
			if stop != nil {
				stop()
			}
			return
		}
		finish(code)
	}, func() {
		if stop != nil {
			stop()
		}
		im.addSystemMessage("OAuth login cancelled.")
	})
}

func (im *InteractiveMode) exchangeAndStoreOAuthCode(providerID ai.OAuthProviderID, code string, pkce *ai.PKCE) {
	if strings.TrimSpace(code) == "" {
		im.postSystemMessage("OAuth login cancelled: no authorization code provided.")
		return
	}
	if im.canSendUI() {
		im.program.Send(tui.AgentStatusMsg{Text: im.statusText("Completing OAuth login...")})
	}
	exchange := im.config.OAuthExchange
	if exchange == nil {
		exchange = func(providerID ai.OAuthProviderID, code string, pkce *ai.PKCE) (*ai.OAuthCredentials, error) {
			provider, ok := ai.OAuthProviderFactory(providerID).(*ai.OAuthProvider)
			if !ok || provider == nil {
				return nil, fmt.Errorf("OAuth provider %s is not available", providerID)
			}
			return provider.ExchangeCode(code, pkce)
		}
	}
	creds, err := exchange(providerID, code, pkce)
	if err != nil {
		im.postSystemMessage(fmt.Sprintf("OAuth login failed for %s: %v", providerID, err))
		if im.canSendUI() {
			im.program.Send(tui.AgentStatusMsg{Text: im.statusText("")})
		}
		return
	}
	if creds == nil || strings.TrimSpace(creds.AccessToken) == "" {
		im.postSystemMessage(fmt.Sprintf("OAuth login failed for %s: no access token returned.", providerID))
		return
	}
	if err := im.config.OAuthStore.Save(&auth.OAuthCredential{
		Provider:     string(providerID),
		AccessToken:  creds.AccessToken,
		RefreshToken: creds.RefreshToken,
		ExpiresAt:    creds.ExpiresAt,
		Scopes:       creds.Scopes,
		TokenType:    creds.TokenType,
	}); err != nil {
		im.postSystemMessage(fmt.Sprintf("Could not save OAuth credentials for %s: %v", providerID, err))
		return
	}
	if string(im.config.Provider) == string(providerID) {
		im.config.APIKey = creds.AccessToken
	}
	if im.canSendUI() {
		im.program.Send(tui.AgentStatusMsg{Text: im.statusText("")})
	}
	im.postSystemMessage(fmt.Sprintf("Saved OAuth credentials for %s in %s.", providerID, shortenPath(defaultOAuthPath())))
}

func (im *InteractiveMode) openOAuthURL(rawURL string) error {
	if im.config.OpenURL != nil {
		return im.config.OpenURL(rawURL)
	}
	return openURLInBrowser(rawURL)
}

func extractOAuthCode(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("empty OAuth response")
	}
	if strings.Contains(value, "://") {
		u, err := url.Parse(value)
		if err != nil {
			return "", fmt.Errorf("could not parse URL: %w", err)
		}
		if code := u.Query().Get("code"); code != "" {
			return code, nil
		}
		return "", fmt.Errorf("no code parameter in URL")
	}
	return value, nil
}

func startOAuthCallbackServer(redirectURI string) (<-chan string, func(), error) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		return nil, nil, err
	}
	listener, err := net.Listen("tcp", u.Host)
	if err != nil {
		return nil, nil, err
	}
	codeCh := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc(u.Path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			http.Error(w, "missing OAuth code", http.StatusBadRequest)
			return
		}
		select {
		case codeCh <- code:
		default:
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><title>rho login complete</title><p>rho login complete. You can close this tab.</p>"))
	})
	server := &http.Server{Handler: mux}
	go func() {
		_ = server.Serve(listener)
	}()
	stop := func() {
		_ = server.Close()
	}
	return codeCh, stop, nil
}

func (im *InteractiveMode) canSendUI() bool {
	return im.program != nil && im.programRunning
}

func (im *InteractiveMode) postSystemMessage(message string) {
	if im.canSendUI() {
		im.program.Send(tui.AddMessageMsg{
			Role:    string(ai.RoleAssistant),
			Content: message,
			Model:   "rho",
		})
	} else {
		im.addSystemMessage(message)
	}
}

type uiClosePromptMsg struct{}

func openURLInBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}
