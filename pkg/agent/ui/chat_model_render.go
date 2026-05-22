package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/earendil-works/rho/pkg/agent"
	agenttheme "github.com/earendil-works/rho/pkg/agent/theme"
	"github.com/earendil-works/rho/pkg/ai"
	"github.com/earendil-works/rho/pkg/tui"
)

// baseLayout assembles the common layout parts (status, toast, viewport, hint, input, footer).
// Returns the parts joined by newlines. Excludes autocomplete (only shown in non-modal view).
func (m *ChatModel) baseLayout(width int) []string {
	th := m.Theme
	if th == nil {
		th = tui.DefaultTheme
	}

	status := m.statusStyle.Width(width).Render(tui.SliceByColumn(m.Status, 0, max(1, width-2), true))
	viewport := m.renderTranscript(width, m.viewportHeight())
	var hint string
	if len(m.Messages) == 0 {
		hint = th.Muted("Type a message and press Enter  Ctrl+L change model  /commands for help")
	} else {
		hint = th.Muted("Enter send  Alt+Enter newline  ↑↓/PgUp/PgDn scroll  Ctrl+L model")
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
	return parts
}

func (m *ChatModel) renderModalView(width int) string {
	th := m.Theme
	if th == nil {
		th = tui.DefaultTheme
	}

	parts := m.baseLayout(width)
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

func buildCostMap() map[string]ai.ModelDefinition {
	m := make(map[string]ai.ModelDefinition)
	for _, def := range ai.DefaultModels() {
		key := string(def.Provider) + "/" + def.Name
		m[key] = def
	}
	return m
}

func (m *ChatModel) getCostMap() map[string]ai.ModelDefinition {
	if m.costMap == nil {
		m.costMap = buildCostMap()
	}
	return m.costMap
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

	if m.SessionName != "" {
		parts = append(parts, th.Muted("@"+m.SessionName))
	}

	if m.ThinkingLevel != "" {
		parts = append(parts, th.Muted("🧠 "+m.ThinkingLevel))
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
