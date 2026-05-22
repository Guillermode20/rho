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
	AllItems      []AutocompleteItem
	Items         []AutocompleteItem
	Query         []rune
	SelIdx        int
	OnSelect      func(AutocompleteItem)
	OnSelectCancel func()

	// Confirm mode
	Message string
	OnYes   func()
	OnNo    func()
	ConfirmIdx int // 0 = Yes, 1 = No

	// Info mode
	Content    []string
	InfoScroll int
	OnDismiss  func()
}

type ChatModel struct {
	Width         int
	Height        int
	Status        string
	Input         []rune
	Cursor        int
	Scroll        int
	Messages      []agent.AgentMessage
	MdTheme       tui.MarkdownTheme
	undoStack     *tui.UndoStack[editorState]
	redoStack     *tui.UndoStack[editorState]
	killRing      *tui.KillRing
	lastAction    string
	yankLen       int
	// Design system
	Theme *tui.Theme

	// Metadata footer fields
	modelName     string
	providerName  string
	TokenCount    int
	contextWindow int
	TotalCost     float64
	GitBranch     string

	// Cached cost map: rebuilt only when needed (lazy, set to nil to force rebuild)
	costMap      map[string]ai.ModelDefinition
	costMapDirty bool

	OnSubmit          func(string)
	OnMessage         func(tea.Msg)
	OnAutocomplete    func(text string, cursor int) []AutocompleteItem
	OnAction          func(action string) bool
	Autocomplete      []AutocompleteItem
	AutocompleteIdx   int

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

const mouseWheelScrollLines = 3
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
			m.Scroll += mouseWheelScrollLines
			m.clampScroll()
		case msg.Button == tea.MouseButtonWheelDown || msg.Type == tea.MouseWheelDown:
			m.Scroll -= mouseWheelScrollLines
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

	th := m.Theme
	if th == nil {
		th = tui.DefaultTheme
	}

	if m.Modal != nil {
		return m.renderModalView(width)
	}

	status := m.statusStyle.Width(width).Render(tui.SliceByColumn(m.Status, 0, max(1, width-2), true))

	var hint string
	if len(m.Messages) == 0 {
		hint = th.Muted("Type a message and press Enter  Ctrl+L change model  /commands for help")
	} else {
		hint = th.Muted("Enter send  Alt+Enter newline  ↑↓/PgUp/PgDn scroll  Ctrl+L model")
	}

	viewport := m.renderTranscript(width, m.viewportHeight())
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
	if suggestions := m.renderAutocomplete(width); suggestions != "" {
		parts = append(parts, suggestions)
	}

	return strings.Join(parts, "\n")
}

func (m *ChatModel) renderModalView(width int) string {
	th := m.Theme
	if th == nil {
		th = tui.DefaultTheme
	}

	status := m.statusStyle.Width(width).Render(tui.SliceByColumn(m.Status, 0, max(1, width-2), true))
	viewport := m.renderTranscript(width, m.viewportHeight())
	hint := th.Muted("")
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

	dimmed := make([]string, 0, len(baseLines))
	for _, line := range baseLines {
		if strings.TrimSpace(line) == "" {
			dimmed = append(dimmed, "")
		} else {
			dimmed = append(dimmed, th.Muted(line))
		}
	}

	modalLines := m.renderModal(width)
	if len(modalLines) == 0 {
		return strings.Join(dimmed, "\n")
	}

	viewH := len(dimmed)
	modalH := len(modalLines)
	top := max(1, (viewH-modalH)/2)

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

func buildCostMap() map[string]ai.ModelDefinition {
	m := make(map[string]ai.ModelDefinition)
	for _, def := range ai.DefaultModels() {
		key := string(def.Provider) + "/" + def.Name
		m[key] = def
	}
	return m
}

func (m *ChatModel) getCostMap() map[string]ai.ModelDefinition {
	if m.costMap == nil || m.costMapDirty {
		m.costMap = buildCostMap()
		m.costMapDirty = false
	}
	return m.costMap
}

func (m *ChatModel) invalidateCostMap() {
	m.costMapDirty = true
}

func (m *ChatModel) recalculateStats() {
	var lastTokens int
	var totalCost float64

	costMap := m.getCostMap()

	for _, msg := range m.Messages {
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

	m.TokenCount = lastTokens
	m.TotalCost = totalCost
}

func (m *ChatModel) renderFooter(width int) string {
	th := m.Theme
	if th == nil {
		th = tui.DefaultTheme
	}
	if width <= 0 {
		return ""
	}

	var parts []string

	if m.GitBranch != "" {
		parts = append(parts, th.Colored("⎇  "+m.GitBranch, th.Palette.Success))
	}

	if m.modelName != "" {
		modelStr := m.modelName
		if m.providerName != "" {
			modelStr = m.providerName + "/" + m.modelName
		}
		parts = append(parts, th.Colored(modelStr, th.Palette.Accent))
	}

	if m.contextWindow > 0 {
		tokenStr := fmt.Sprintf("%d tok", m.TokenCount)
		pct := float64(m.TokenCount) / float64(m.contextWindow) * 100
		var pctStr string
		if pct < 1.0 && m.TokenCount > 0 {
			pctStr = "<1%"
		} else {
			pctStr = fmt.Sprintf("%d%%", int(pct))
		}
		tokenStr = fmt.Sprintf("%s / %d ctx (%s)", tokenStr, m.contextWindow, pctStr)
		parts = append(parts, th.Muted(tokenStr))
	} else if m.TokenCount > 0 {
		parts = append(parts, th.Muted(fmt.Sprintf("%d tok", m.TokenCount)))
	}

	if m.TotalCost > 0 {
		parts = append(parts, th.Colored(fmt.Sprintf("$%g", m.TotalCost), th.Palette.Warning))
	}

	if len(parts) == 0 {
		return ""
	}

	sep := th.Muted(" · ")
	line := strings.Join(parts, sep)

	if tui.VisibleWidth(line) > width {
		line = tui.SliceByColumn(line, 0, width, true)
	} else {
		line += strings.Repeat(" ", max(0, width-tui.VisibleWidth(line)))
	}

	return line
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

func (m *ChatModel) OpenModalPrompt(title, placeholder string, onSubmit func(string), onCancel func()) {
	m.CloseModal()
	m.Modal = &ModalState{
		Active:      true,
		Mode:        "prompt",
		Title:       title,
		Placeholder: placeholder,
		OnSubmit:    onSubmit,
		OnCancel:    onCancel,
	}
}

func (m *ChatModel) OpenModalSelector(title string, items []AutocompleteItem, onSelect func(AutocompleteItem), onCancel func()) {
	m.CloseModal()
	m.Modal = &ModalState{
		Active:         true,
		Mode:           "selector",
		Title:          title,
		AllItems:       append([]AutocompleteItem(nil), items...),
		Items:          append([]AutocompleteItem(nil), items...),
		OnSelect:       onSelect,
		OnSelectCancel: onCancel,
	}
}

func (m *ChatModel) OpenModalConfirm(title, message string, onYes func(), onNo func()) {
	m.CloseModal()
	m.Modal = &ModalState{
		Active:     true,
		Mode:       "confirm",
		Title:      title,
		Message:    message,
		OnYes:      onYes,
		OnNo:       onNo,
		ConfirmIdx: 0,
	}
}

func (m *ChatModel) OpenModalInfo(title string, content []string, onDismiss func()) {
	m.CloseModal()
	m.Modal = &ModalState{
		Active:    true,
		Mode:      "info",
		Title:     title,
		Content:   append([]string(nil), content...),
		OnDismiss: onDismiss,
	}
}

func (m *ChatModel) CloseModal() {
	if m.Modal != nil {
		m.Modal = nil
	}
}

func (m *ChatModel) ShowToast(text string) {
	if text == "" {
		return
	}
	m.toast = text
	m.toastUntil = time.Now().Add(4 * time.Second)
}

func (m *ChatModel) renderToast(width int) string {
	if m.toast == "" || time.Now().After(m.toastUntil) || width <= 0 {
		m.toast = ""
		return ""
	}
	th := m.Theme
	if th == nil {
		th = tui.DefaultTheme
	}
	display := m.toast
	if tui.VisibleWidth(display) > width-6 {
		display = tui.SliceByColumn(display, 0, width-6, true)
	}
	box := " " + display + " "
	boxW := tui.VisibleWidth(box)
	padding := max(0, (width-boxW)/2)
	styled := th.Bg(box, th.Palette.Surface)
	return strings.Repeat(" ", padding) + styled
}

func (m *ChatModel) handleModalInput(msg tea.KeyMsg) {
	modal := m.Modal
	if modal == nil {
		return
	}

	switch modal.Mode {
	case "prompt":
		switch msg.String() {
		case "esc":
			onCancel := modal.OnCancel
			m.CloseModal()
			if onCancel != nil {
				onCancel()
			}
		case "enter":
			value := string(modal.Value)
			onSubmit := modal.OnSubmit
			m.CloseModal()
			if onSubmit != nil {
				onSubmit(value)
			}
		case "backspace", "ctrl+h":
			if len(modal.Value) > 0 {
				modal.Value = modal.Value[:len(modal.Value)-1]
			}
		case "ctrl+u":
			modal.Value = nil
		default:
			if msg.Type == tea.KeyRunes {
				modal.Value = append(modal.Value, msg.Runes...)
			} else if msg.Type == tea.KeySpace {
				modal.Value = append(modal.Value, ' ')
			}
		}

	case "selector":
		switch msg.String() {
		case "esc":
			onCancel := modal.OnSelectCancel
			m.CloseModal()
			if onCancel != nil {
				onCancel()
			}
		case "up", "ctrl+p":
			if modal.SelIdx > 0 {
				modal.SelIdx--
			}
		case "down", "ctrl+n":
			if modal.SelIdx < len(modal.Items)-1 {
				modal.SelIdx++
			}
		case "pgup":
			modal.SelIdx -= 8
			if modal.SelIdx < 0 {
				modal.SelIdx = 0
			}
		case "pgdown":
			modal.SelIdx += 8
			modal.SelIdx = max(0, min(modal.SelIdx, len(modal.Items)-1))
		case "home":
			modal.SelIdx = 0
		case "end":
			modal.SelIdx = max(0, len(modal.Items)-1)
		case "enter":
			if len(modal.Items) > 0 && modal.SelIdx >= 0 && modal.SelIdx < len(modal.Items) {
				item := modal.Items[modal.SelIdx]
				onSelect := modal.OnSelect
				m.CloseModal()
				if onSelect != nil {
					onSelect(item)
				}
			}
		case "backspace", "ctrl+h":
			if len(modal.Query) > 0 {
				modal.Query = modal.Query[:len(modal.Query)-1]
				m.updateModalFilter()
			}
		case "ctrl+u":
			modal.Query = nil
			m.updateModalFilter()
		default:
			if msg.Type == tea.KeyRunes {
				modal.Query = append(modal.Query, msg.Runes...)
				m.updateModalFilter()
			} else if msg.Type == tea.KeySpace {
				modal.Query = append(modal.Query, ' ')
				m.updateModalFilter()
			}
		}

	case "confirm":
		switch msg.String() {
		case "esc":
			onNo := modal.OnNo
			m.CloseModal()
			if onNo != nil {
				onNo()
			}
		case "enter":
			if modal.ConfirmIdx == 0 {
				onYes := modal.OnYes
				m.CloseModal()
				if onYes != nil {
					onYes()
				}
			} else {
				onNo := modal.OnNo
				m.CloseModal()
				if onNo != nil {
					onNo()
				}
			}
		case "left", "ctrl+p":
			modal.ConfirmIdx = 0
		case "right", "ctrl+n":
			modal.ConfirmIdx = 1
		}

	case "info":
		maxLines := max(3, min(len(modal.Content), 16))
		maxScroll := max(0, len(modal.Content)-maxLines)

		switch msg.String() {
		case "up", "ctrl+p":
			if modal.InfoScroll > 0 {
				modal.InfoScroll--
			}
		case "down", "ctrl+n":
			if modal.InfoScroll < maxScroll {
				modal.InfoScroll++
			}
		case "pgup":
			modal.InfoScroll -= 10
			if modal.InfoScroll < 0 {
				modal.InfoScroll = 0
			}
		case "pgdown":
			modal.InfoScroll += 10
			if modal.InfoScroll > maxScroll {
				modal.InfoScroll = maxScroll
			}
		case "home":
			modal.InfoScroll = 0
		case "end":
			modal.InfoScroll = maxScroll
		default:
			onDismiss := modal.OnDismiss
			m.CloseModal()
			if onDismiss != nil {
				onDismiss()
			}
		}
	}
}

func (m *ChatModel) updateModalFilter() {
	modal := m.Modal
	if modal == nil || modal.Mode != "selector" {
		return
	}
	query := strings.ToLower(strings.TrimSpace(string(modal.Query)))
	modal.Items = modal.Items[:0]
	for _, item := range modal.AllItems {
		haystack := strings.ToLower(item.Value + " " + item.Label + " " + item.Description)
		if query == "" || strings.Contains(haystack, query) {
			modal.Items = append(modal.Items, item)
		}
	}
	modal.SelIdx = max(0, min(modal.SelIdx, len(modal.Items)-1))
}

func (m *ChatModel) insertRunes(runes ...rune) {
	next := make([]rune, 0, len(m.Input)+len(runes))
	next = append(next, m.Input[:m.Cursor]...)
	next = append(next, runes...)
	next = append(next, m.Input[m.Cursor:]...)
	m.Input = next
	m.Cursor += len(runes)
	m.updateAutocomplete()
}

func (m *ChatModel) pushUndo() {
	inputCopy := make([]rune, len(m.Input))
	copy(inputCopy, m.Input)
	m.undoStack.Push(editorState{
		input:  inputCopy,
		cursor: m.Cursor,
	})
	m.redoStack.Clear()
}

func (m *ChatModel) undo() {
	state, ok := m.undoStack.Pop()
	if !ok {
		return
	}
	inputCopy := make([]rune, len(m.Input))
	copy(inputCopy, m.Input)
	m.redoStack.Push(editorState{
		input:  inputCopy,
		cursor: m.Cursor,
	})
	m.Input = state.input
	m.Cursor = state.cursor
	m.lastAction = "undo"
	m.updateAutocomplete()
}

func (m *ChatModel) redo() {
	state, ok := m.redoStack.Pop()
	if !ok {
		return
	}
	inputCopy := make([]rune, len(m.Input))
	copy(inputCopy, m.Input)
	m.undoStack.Push(editorState{
		input:  inputCopy,
		cursor: m.Cursor,
	})
	m.Input = state.input
	m.Cursor = state.cursor
	m.lastAction = "redo"
	m.updateAutocomplete()
}

func (m *ChatModel) yank() {
	text := m.killRing.Peek()
	if text == "" {
		return
	}
	m.pushUndo()
	runes := []rune(text)
	m.insertRunes(runes...)
	m.lastAction = "yank"
	m.yankLen = len(runes)
}

func (m *ChatModel) yankPop() {
	if m.lastAction != "yank" || m.killRing.Length() <= 1 {
		return
	}
	m.killRing.Rotate()
	text := m.killRing.Peek()
	if text == "" {
		return
	}
	if m.Cursor >= m.yankLen {
		m.Input = append(m.Input[:m.Cursor-m.yankLen], m.Input[m.Cursor:]...)
		m.Cursor -= m.yankLen
	}
	runes := []rune(text)
	m.insertRunes(runes...)
	m.lastAction = "yank"
	m.yankLen = len(runes)
}

func (m *ChatModel) cursorUpLine() {
	if m.Cursor <= 0 {
		return
	}
	lineStart := m.Cursor
	for lineStart > 0 && m.Input[lineStart-1] != '\n' {
		lineStart--
	}
	if lineStart == 0 {
		m.Cursor = 0
		return
	}
	col := m.Cursor - lineStart
	prevLineEnd := lineStart - 1
	prevLineStart := prevLineEnd
	for prevLineStart > 0 && m.Input[prevLineStart-1] != '\n' {
		prevLineStart--
	}
	newCursor := prevLineStart + col
	if newCursor > prevLineEnd {
		newCursor = prevLineEnd
	}
	m.Cursor = newCursor
}

func (m *ChatModel) cursorDownLine() {
	if m.Cursor >= len(m.Input) {
		return
	}
	lineEnd := m.Cursor
	for lineEnd < len(m.Input) && m.Input[lineEnd] != '\n' {
		lineEnd++
	}
	if lineEnd >= len(m.Input) {
		m.Cursor = len(m.Input)
		return
	}
	lineStart := m.Cursor
	for lineStart > 0 && m.Input[lineStart-1] != '\n' {
		lineStart--
	}
	col := m.Cursor - lineStart
	nextLineStart := lineEnd + 1
	nextLineEnd := nextLineStart
	for nextLineEnd < len(m.Input) && m.Input[nextLineEnd] != '\n' {
		nextLineEnd++
	}
	newCursor := nextLineStart + col
	if newCursor > nextLineEnd {
		newCursor = nextLineEnd
	}
	m.Cursor = newCursor
}

func (m *ChatModel) viewportHeight() int {
	if m.Height <= 0 {
		return 20
	}
	inputLines := len(strings.Split(m.inputTextWithCursor(false), "\n")) + 2
	footerLines := 0
	if m.modelName != "" || m.GitBranch != "" || m.TokenCount > 0 || m.TotalCost > 0 {
		footerLines = 1
	}
	return max(3, m.Height-2-inputLines-m.autocompleteHeight()-footerLines)
}

func (m *ChatModel) renderModal(width int) []string {
	if m.Modal == nil || width <= 0 {
		return nil
	}
	th := m.Theme
	if th == nil {
		th = tui.DefaultTheme
	}

	modal := m.Modal
	boxWidth := min(width-8, 72)
	if boxWidth < 30 {
		boxWidth = width - 2
	}
	innerW := boxWidth - 4

	var contentLines []string

	switch modal.Mode {
	case "prompt":
		value := string(modal.Value)
		displayValue := value
		if displayValue == "" && modal.Placeholder != "" {
			displayValue = th.Muted(modal.Placeholder)
		} else if len(value) > 0 {
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
		query := string(modal.Query)
		if query == "" {
			contentLines = append(contentLines, "  "+th.Muted("search: ")+th.Dim("type to filter"))
		} else {
			contentLines = append(contentLines, "  "+th.Muted("search: ")+th.Accent(query))
		}

		if len(modal.Items) == 0 {
			contentLines = append(contentLines, "  "+th.Muted("No matches"))
		}

		maxVisible := 10
		start := max(0, min(modal.SelIdx-maxVisible/2, len(modal.Items)-maxVisible))
		end := min(start+maxVisible, len(modal.Items))
		for i := start; i < end; i++ {
			item := modal.Items[i]
			var line string
			if i == modal.SelIdx {
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
		if end < len(modal.Items) {
			contentLines = append(contentLines, "  "+th.Muted(fmt.Sprintf("⋯ %d more", len(modal.Items)-end)))
		}
		contentLines = append(contentLines, "")
		contentLines = append(contentLines, "  "+th.Muted("Type to filter  ·  ↑↓ navigate  ·  Enter select  ·  Esc cancel"))

	case "confirm":
		for _, line := range strings.Split(modal.Message, "\n") {
			wrapped := tui.SliceByColumn(line, 0, innerW, true)
			contentLines = append(contentLines, "  "+wrapped)
		}
		contentLines = append(contentLines, "")
		var choiceLine string
		if modal.ConfirmIdx == 0 {
			choiceLine = "  " + th.Accent("▸ Yes") + "    " + th.Muted("No")
		} else {
			choiceLine = "  " + th.Muted("Yes") + "    " + th.Accent("▸ No")
		}
		contentLines = append(contentLines, choiceLine)
		contentLines = append(contentLines, "")
		contentLines = append(contentLines, "  "+th.Muted("← → switch  ·  Enter confirm  ·  Esc cancel"))

	case "info":
		maxLines := max(3, min(len(modal.Content), 16))
		scroll := modal.InfoScroll
		start := max(0, min(scroll, len(modal.Content)-maxLines))
		end := min(start+maxLines, len(modal.Content))
		for i := start; i < end; i++ {
			line := modal.Content[i]
			if tui.VisibleWidth(line) > innerW {
				line = tui.SliceByColumn(line, 0, innerW, true)
			} else {
				line += strings.Repeat(" ", innerW-tui.VisibleWidth(line))
			}
			contentLines = append(contentLines, "  "+line)
		}
		contentLines = append(contentLines, "")
		hint := "any key to close"
		if len(modal.Content) > maxLines {
			hint = "↑↓ scroll  ·  any key to close"
		}
		contentLines = append(contentLines, "  "+th.Muted(hint))
	}

	boxHeight := len(contentLines) + 3
	topBorder := th.Accent("┌── ") + th.BoldAccent(modal.Title) + " " + th.Muted(strings.Repeat("─", max(0, boxWidth-6-len(modal.Title)-3))) + th.Accent("┐")
	bottomBorder := th.Accent("└") + th.Muted(strings.Repeat("─", boxWidth-2)) + th.Accent("┘")

	boxLines := make([]string, 0, boxHeight+2)
	boxLines = append(boxLines, topBorder)
	boxLines = append(boxLines, th.Muted("│")+strings.Repeat(" ", boxWidth-2)+th.Muted("│"))
	for _, line := range contentLines {
		lw := tui.VisibleWidth(line)
		if lw < boxWidth-2 {
			line += strings.Repeat(" ", boxWidth-2-lw)
		}
		boxLines = append(boxLines, th.Muted("│")+line+th.Muted("│"))
	}
	boxLines = append(boxLines, bottomBorder)

	return boxLines
}

func (m *ChatModel) renderTranscript(width, height int) string {
	lines := m.renderMessages(width)
	if height <= 0 {
		return ""
	}
	if len(lines) == 0 {
		lines = m.renderEmptyState(width)
	}

	maxScroll := max(0, len(lines)-height)
	if m.Scroll > maxScroll {
		m.Scroll = maxScroll
	}
	start := max(0, len(lines)-height-m.Scroll)
	end := min(len(lines), start+height)
	view := append([]string(nil), lines[start:end]...)
	for len(view) < height {
		view = append(view, "")
	}
	return strings.Join(view, "\n")
}

func (m *ChatModel) renderEmptyState(width int) []string {
	th := m.Theme
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

func (m *ChatModel) renderMessages(width int) []string {
	th := m.Theme
	if th == nil {
		th = tui.DefaultTheme
	}
	var lines []string
	contentWidth := max(20, width-4)
	for _, msg := range m.Messages {
		if msg.Hide {
			continue
		}
		switch msg.Role {
		case ai.RoleUser:
			lines = append(lines, "")
			lines = append(lines, th.Success("You"))
			if msg.Content != "" {
				lines = append(lines, indentLines(tui.NewMarkdown(msg.Content, m.MdTheme).Render(contentWidth), "  ")...)
			}
		case ai.RoleAssistant:
			name := msg.Model
			if name == "" {
				name = "Assistant"
			}
			lines = append(lines, "")
			if msg.Provider != "" {
				lines = append(lines, th.BoldAccent(name)+"  "+th.Tag(string(msg.Provider)))
			} else {
				lines = append(lines, th.BoldAccent(name))
			}
			if msg.Content != "" {
				lines = append(lines, indentLines(tui.NewMarkdown(msg.Content, m.MdTheme).Render(contentWidth), "  ")...)
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

func (m *ChatModel) renderToolResultSummary(msg agent.AgentMessage, width int) (string, []string) {
	th := m.Theme
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

func (m *ChatModel) renderInput(width int) string {
	th := m.Theme
	if th == nil {
		th = tui.DefaultTheme
	}
	if width <= 0 {
		return ""
	}

	if len(m.Input) == 0 {
		text := th.Muted("> Type a message…")
		return text + strings.Repeat(" ", max(0, width-tui.VisibleWidth(text)))
	}

	rawText := m.inputTextWithCursor(true)
	prompt := th.Accent("▌") + " "

	lines := strings.Split(rawText, "\n")
	var renderedLines []string
	for i, line := range lines {
		var displayLine string
		if i == 0 {
			displayLine = prompt + line
		} else {
			displayLine = "  " + line
		}
		if tui.VisibleWidth(displayLine) > width {
			displayLine = tui.SliceByColumn(displayLine, tui.VisibleWidth(displayLine)-width, tui.VisibleWidth(displayLine), true)
		} else {
			displayLine += strings.Repeat(" ", width-tui.VisibleWidth(displayLine))
		}
		renderedLines = append(renderedLines, displayLine)
	}

	result := strings.Join(renderedLines, "\n")
	return result + "\n" + th.Muted(strings.Repeat("─", width))
}

func (m *ChatModel) renderAutocomplete(width int) string {
	th := m.Theme
	if th == nil {
		th = tui.DefaultTheme
	}
	if len(m.Autocomplete) == 0 || width <= 0 {
		return ""
	}

	maxVisible := min(len(m.Autocomplete), 6)
	start := max(0, min(m.AutocompleteIdx-maxVisible/2, len(m.Autocomplete)-maxVisible))
	end := min(start+maxVisible, len(m.Autocomplete))
	lines := make([]string, 0, end-start+1)
	lines = append(lines, th.Muted(strings.Repeat("─", width)))
	for i := start; i < end; i++ {
		item := m.Autocomplete[i]
		var line string
		if i == m.AutocompleteIdx {
			line = th.Accent("▌") + " " + th.BoldAccent(item.Label)
		} else {
			line = "  " + item.Label
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
	if end < len(m.Autocomplete) {
		lines = append(lines, th.Muted(fmt.Sprintf("  ⋯ %d more", len(m.Autocomplete)-end)))
	}
	return strings.Join(lines, "\n")
}

func (m *ChatModel) autocompleteHeight() int {
	if len(m.Autocomplete) == 0 {
		return 0
	}
	return min(len(m.Autocomplete), 6)
}

func (m *ChatModel) inputTextWithCursor(showCursor bool) string {
	value := append([]rune(nil), m.Input...)
	if showCursor {
		cursor := m.Cursor
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

func (m *ChatModel) clampScroll() {
	maxScroll := max(0, len(m.renderMessages(max(20, m.Width)))-m.viewportHeight())
	if m.Scroll > maxScroll {
		m.Scroll = maxScroll
	}
	if m.Scroll < 0 {
		m.Scroll = 0
	}
}

func (m *ChatModel) updateAutocomplete() {
	if m.OnAutocomplete == nil {
		m.closeAutocomplete()
		return
	}
	items := m.OnAutocomplete(string(m.Input), m.Cursor)
	m.Autocomplete = items
	if len(items) == 0 {
		m.AutocompleteIdx = 0
		return
	}
	if m.AutocompleteIdx >= len(items) {
		m.AutocompleteIdx = len(items) - 1
	}
}

func (m *ChatModel) closeAutocomplete() {
	m.Autocomplete = nil
	m.AutocompleteIdx = 0
}

func (m *ChatModel) applyAutocomplete() {
	if len(m.Autocomplete) == 0 || m.AutocompleteIdx >= len(m.Autocomplete) {
		return
	}
	m.pushUndo()
	item := m.Autocomplete[m.AutocompleteIdx]
	text := string(m.Input)
	replacement := item.Value
	if strings.HasPrefix(text, "/") && !strings.Contains(strings.TrimSpace(text), " ") {
		replacement += " "
	}
	m.Input = []rune(replacement)
	m.Cursor = len(m.Input)
	m.closeAutocomplete()
	m.lastAction = "autocomplete"
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
