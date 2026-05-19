package tui

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

// UndoStack manages undo/redo for the editor.
type UndoStack struct {
	mu    sync.Mutex
	stack []undoEntry
	pos   int
	limit int
}

type undoEntry struct {
	text        string
	cursorLine  int
	cursorCol   int
}

// NewUndoStack creates an undo stack.
func NewUndoStack() *UndoStack {
	return &UndoStack{limit: 100, pos: -1}
}

func (u *UndoStack) Push(text string, line, col int) {
	u.mu.Lock()
	if u.pos < len(u.stack)-1 {
		u.stack = u.stack[:u.pos+1]
	}
	u.stack = append(u.stack, undoEntry{text, line, col})
	u.pos = len(u.stack) - 1
	if len(u.stack) > u.limit {
		u.stack = u.stack[1:]
		u.pos--
	}
	u.mu.Unlock()
}

func (u *UndoStack) Undo() (string, int, int, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.pos < 0 {
		return "", 0, 0, false
	}
	entry := u.stack[u.pos]
	u.pos--
	return entry.text, entry.cursorLine, entry.cursorCol, true
}

func (u *UndoStack) Redo() (string, int, int, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.pos >= len(u.stack)-1 {
		return "", 0, 0, false
	}
	u.pos++
	entry := u.stack[u.pos]
	return entry.text, entry.cursorLine, entry.cursorCol, true
}

// KillRing stores killed/deleted text for yanking.
type KillRing struct {
	mu   sync.Mutex
	data []string
	pos  int
	max  int
}

func NewKillRing(max int) *KillRing {
	return &KillRing{max: max, pos: -1}
}

func (k *KillRing) Add(text string) {
	k.mu.Lock()
	k.data = append(k.data, text)
	k.pos = len(k.data) - 1
	if len(k.data) > k.max {
		k.data = k.data[1:]
		k.pos--
	}
	k.mu.Unlock()
}

func (k *KillRing) Yank() string {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.pos < 0 || len(k.data) == 0 {
		return ""
	}
	return k.data[k.pos]
}

func (k *KillRing) YankNext() string {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(k.data) == 0 {
		return ""
	}
	k.pos = (k.pos - 1 + len(k.data)) % len(k.data)
	return k.data[k.pos]
}

// EditorOptions configures the editor.
type EditorOptions struct {
	Theme     EditorTheme
	TabSize   int
	MaxHeight int
}

// EditorTheme defines editor styling.
type EditorTheme struct {
	Text       func(string) string
	CursorLine func(string) string
	LineNumber func(string) string
}

func DefaultEditorTheme() EditorTheme {
	reset := "\x1b[0m"
	dim := "\x1b[2m"
	yellow := "\x1b[33m"
	return EditorTheme{
		Text:       func(s string) string { return s },
		CursorLine: func(s string) string { return yellow + s + reset },
		LineNumber: func(s string) string { return dim + s + reset },
	}
}

// Editor is a multi-line text editor component for the TUI.
type Editor struct {
	mu          sync.Mutex
	lines       []string
	cursorLine  int
	cursorCol   int
	scroll      int
	focused     bool
	undoStack   *UndoStack
	killRing    *KillRing
	onSubmit    func(string)
	onChange    func(string)
	placeholder string
	theme       EditorTheme
	opts        EditorOptions
}

func NewEditor(opts EditorOptions) *Editor {
	if opts.TabSize == 0 {
		opts.TabSize = 4
	}
	if opts.MaxHeight == 0 {
		opts.MaxHeight = 10
	}
	return &Editor{
		lines:     []string{""},
		theme:     opts.Theme,
		opts:      opts,
		undoStack: NewUndoStack(),
		killRing:  NewKillRing(20),
	}
}

func (e *Editor) GetText() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return strings.Join(e.lines, "\n")
}

func (e *Editor) SetText(text string) {
	oldText := e.GetText()
	oldLine := e.cursorLine
	oldCol := e.cursorCol
	if text == "" {
		e.lines = []string{""}
	} else {
		e.lines = strings.Split(text, "\n")
	}
	e.cursorLine = 0
	e.cursorCol = 0
	e.scroll = 0
	e.undoStack.Push(oldText, oldLine, oldCol)
	if e.onChange != nil {
		e.onChange(text)
	}
}

func (e *Editor) SetPlaceholder(p string) {
	e.mu.Lock()
	e.placeholder = p
	e.mu.Unlock()
}
func (e *Editor) SetOnSubmit(fn func(string)) {
	e.mu.Lock()
	e.onSubmit = fn
	e.mu.Unlock()
}
func (e *Editor) SetOnChange(fn func(string)) {
	e.mu.Lock()
	e.onChange = fn
	e.mu.Unlock()
}
func (e *Editor) SetFocused(f bool) {
	e.mu.Lock()
	e.focused = f
	e.mu.Unlock()
}
func (e *Editor) Focused() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.focused
}

func (e *Editor) Render(width int) []string {
	e.mu.Lock()
	defer e.mu.Unlock()

	contentW := width - 6
	if contentW < 10 {
		contentW = 10
	}

	maxH := e.opts.MaxHeight

	// Clamp scroll
	if e.cursorLine < e.scroll {
		e.scroll = e.cursorLine
	}
	if e.cursorLine >= e.scroll+maxH {
		e.scroll = e.cursorLine - maxH + 1
	}

	endLine := e.scroll + maxH
	if endLine > len(e.lines) {
		endLine = len(e.lines)
	}

	var result []string
	for i := e.scroll; i < endLine; i++ {
		ln := fmt.Sprintf("%*d", len(fmt.Sprintf("%d", endLine)), i+1)
		content := e.lines[i]
		if VisibleWidth(content) > contentW {
			content = SliceByColumn(content, 0, contentW, true)
		}

		if i == e.cursorLine && e.focused {
			runes := []rune(content)
			col := e.cursorCol
			if col >= len(runes) {
				content = content + CursorMarker
			} else {
				content = string(runes[:col]) + CursorMarker + string(runes[col:])
			}
		}

		result = append(result, e.theme.LineNumber(ln)+"│ "+content)
	}

	// Fill
	for i := endLine - e.scroll; i < maxH; i++ {
		result = append(result, e.theme.LineNumber(" ")+"│ ")
	}

	// Status
	insertMode := "INS"
	status := fmt.Sprintf(" Ln %d, Col %d  %s", e.cursorLine+1, e.cursorCol+1, insertMode)
	result = append(result, "──"+strings.Repeat("─", max(0, width-2)))
	result = append(result, status)

	return result
}

func (e *Editor) pushState() (string, int, int) {
	return e.GetText(), e.cursorLine, e.cursorCol
}

func (e *Editor) saveState(oldText string, oldLine, oldCol int) {
	newText := e.GetText()
	if newText != oldText {
		e.undoStack.Push(oldText, oldLine, oldCol)
		if e.onChange != nil {
			e.onChange(newText)
		}
	}
}

func (e *Editor) HandleInput(data string) {
	oldT, oldL, oldC := e.pushState()

	switch {
	case MatchesKey(data, "enter") || MatchesKey(data, "ctrl+m"):
		line := []rune(e.lines[e.cursorLine])
		before := string(line[:e.cursorCol])
		after := string(line[e.cursorCol:])
		newLines := make([]string, 0, len(e.lines)+1)
		newLines = append(newLines, e.lines[:e.cursorLine]...)
		newLines = append(newLines, before, after)
		newLines = append(newLines, e.lines[e.cursorLine+1:]...)
		e.lines = newLines
		e.cursorLine++
		e.cursorCol = 0

	case MatchesKey(data, "backspace") || MatchesKey(data, "ctrl+h"):
		if e.cursorCol > 0 {
			line := []rune(e.lines[e.cursorLine])
			e.lines[e.cursorLine] = string(append(line[:e.cursorCol-1], line[e.cursorCol:]...))
			e.cursorCol--
		} else if e.cursorLine > 0 {
			prevLen := len([]rune(e.lines[e.cursorLine-1]))
			e.lines[e.cursorLine-1] += e.lines[e.cursorLine]
			e.lines = append(e.lines[:e.cursorLine], e.lines[e.cursorLine+1:]...)
			e.cursorLine--
			e.cursorCol = prevLen
		}

	case MatchesKey(data, "del"):
		line := []rune(e.lines[e.cursorLine])
		if e.cursorCol < len(line) {
			e.lines[e.cursorLine] = string(append(line[:e.cursorCol], line[e.cursorCol+1:]...))
		} else if e.cursorLine < len(e.lines)-1 {
			e.lines[e.cursorLine] += e.lines[e.cursorLine+1]
			e.lines = append(e.lines[:e.cursorLine+1], e.lines[e.cursorLine+2:]...)
		}

	case MatchesKey(data, "tab"):
		e.lines[e.cursorLine] = e.lines[e.cursorLine] + strings.Repeat(" ", e.opts.TabSize)
		e.cursorCol += e.opts.TabSize

	case MatchesKey(data, "up") || MatchesKey(data, "ctrl+p"):
		if e.cursorLine > 0 {
			e.cursorLine--
			lineLen := len([]rune(e.lines[e.cursorLine]))
			if e.cursorCol > lineLen {
				e.cursorCol = lineLen
			}
		}

	case MatchesKey(data, "down") || MatchesKey(data, "ctrl+n"):
		if e.cursorLine < len(e.lines)-1 {
			e.cursorLine++
			lineLen := len([]rune(e.lines[e.cursorLine]))
			if e.cursorCol > lineLen {
				e.cursorCol = lineLen
			}
		}

	case MatchesKey(data, "left") || MatchesKey(data, "ctrl+b"):
		if e.cursorCol > 0 {
			e.cursorCol--
		} else if e.cursorLine > 0 {
			e.cursorLine--
			e.cursorCol = len([]rune(e.lines[e.cursorLine]))
		}

	case MatchesKey(data, "right") || MatchesKey(data, "ctrl+f"):
		lineLen := len([]rune(e.lines[e.cursorLine]))
		if e.cursorCol < lineLen {
			e.cursorCol++
		} else if e.cursorLine < len(e.lines)-1 {
			e.cursorLine++
			e.cursorCol = 0
		}

	case MatchesKey(data, "home") || MatchesKey(data, "ctrl+a"):
		e.cursorCol = 0

	case MatchesKey(data, "end") || MatchesKey(data, "ctrl+e"):
		e.cursorCol = len([]rune(e.lines[e.cursorLine]))

	case MatchesKey(data, "ctrl+k"):
		line := []rune(e.lines[e.cursorLine])
		if e.cursorCol < len(line) {
			killed := string(line[e.cursorCol:])
			e.killRing.Add(killed)
			e.lines[e.cursorLine] = string(line[:e.cursorCol])
		} else if e.cursorLine < len(e.lines)-1 {
			e.killRing.Add("\n")
			e.lines = append(e.lines[:e.cursorLine+1], e.lines[e.cursorLine+2:]...)
		}

	case MatchesKey(data, "ctrl+y"):
		killed := e.killRing.Yank()
		if killed != "" {
			line := []rune(e.lines[e.cursorLine])
			e.lines[e.cursorLine] = string(append(line[:e.cursorCol], append([]rune(killed), line[e.cursorCol:]...)...))
			e.cursorCol += len([]rune(killed))
		}

	case MatchesKey(data, "ctrl+z"):
		if text, line, col, ok := e.undoStack.Undo(); ok {
			if text == "" {
				e.lines = []string{""}
			} else {
				e.lines = strings.Split(text, "\n")
			}
			e.cursorLine = line
			e.cursorCol = col
		}

	case MatchesKey(data, "ctrl+y"):
		if text, line, col, ok := e.undoStack.Redo(); ok {
			if text == "" {
				e.lines = []string{""}
			} else {
				e.lines = strings.Split(text, "\n")
			}
			e.cursorLine = line
			e.cursorCol = col
		}

	case MatchesKey(data, "pageup"):
		e.cursorLine -= e.opts.MaxHeight
		if e.cursorLine < 0 {
			e.cursorLine = 0
		}

	case MatchesKey(data, "pagedown"):
		e.cursorLine += e.opts.MaxHeight
		if e.cursorLine >= len(e.lines) {
			e.cursorLine = len(e.lines) - 1
		}

	default:
		if len(data) == 1 && data[0] >= 0x20 && data[0] <= 0x7e {
			line := []rune(e.lines[e.cursorLine])
			e.lines[e.cursorLine] = string(append(line[:e.cursorCol], append([]rune(data), line[e.cursorCol:]...)...))
			e.cursorCol++
		}
	}

	e.saveState(oldT, oldL, oldC)
}

func (e *Editor) Invalidate() {}
func (e *Editor) WantsKeyRelease() bool { return false }

var _ = utf8.ValidString
var _ = sort.Slice
