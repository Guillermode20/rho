package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/earendil-works/rho/pkg/tui"
)

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
