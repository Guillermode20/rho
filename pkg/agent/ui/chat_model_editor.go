package ui

import (
	"strings"
)

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
