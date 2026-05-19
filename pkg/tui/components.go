package tui

import (
	"strings"
)

// Text is a simple component that renders static text.
type Text struct {
	content string
}

// NewText creates a new Text component.
func NewText(content string) *Text {
	return &Text{content: content}
}

// SetContent updates the text content.
func (t *Text) SetContent(content string) {
	t.content = content
}

func (t *Text) Render(width int) []string {
	if width <= 0 {
		return nil
	}
	return []string{t.content}
}

func (t *Text) HandleInput(data string) {}

func (t *Text) Invalidate() {}

func (t *Text) WantsKeyRelease() bool { return false }

// Spacer is a component that renders empty space.
type Spacer struct {
	height int
}

// NewSpacer creates a new Spacer with the given height.
func NewSpacer(height int) *Spacer {
	return &Spacer{height: height}
}

func (s *Spacer) Render(width int) []string {
	if s.height <= 0 || width <= 0 {
		return nil
	}
	return make([]string, s.height)
}

func (s *Spacer) HandleInput(data string) {}
func (s *Spacer) Invalidate()            {}
func (s *Spacer) WantsKeyRelease() bool  { return false }

// Box is a component that renders with a border around its child content.
type Box struct {
	title   string
	child   Component
	padding int
}

// NewBox creates a new Box component.
func NewBox(title string, child Component, padding int) *Box {
	return &Box{title: title, child: child, padding: padding}
}

func (b *Box) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	innerWidth := width - 2 - 2*b.padding
	if innerWidth <= 0 {
		return nil
	}

	childLines := b.child.Render(innerWidth)

	// Top border
	var lines []string
	if b.title != "" {
		titleMax := width - 4
		displayTitle := b.title
		if len(displayTitle) > titleMax {
			displayTitle = displayTitle[:titleMax]
		}
		lines = append(lines, "┌─ "+displayTitle+" "+strings.Repeat("─", max(0, width-4-len(displayTitle)-2))+"┐")
	} else {
		lines = append(lines, "┌"+strings.Repeat("─", width-2)+"┐")
	}

	// Padding top
	padLine := "│" + strings.Repeat(" ", width-2) + "│"
	for i := 0; i < b.padding; i++ {
		lines = append(lines, padLine)
	}

	// Content with padding
	for _, childLine := range childLines {
		content := "│" + strings.Repeat(" ", b.padding)
		// Pad/truncate inner content
		innerContent := childLine
		innerContentW := VisibleWidth(innerContent)
		if innerContentW < innerWidth {
			innerContent += strings.Repeat(" ", innerWidth-innerContentW)
		} else if innerContentW > innerWidth {
			innerContent = SliceByColumn(innerContent, 0, innerWidth, true)
		}
		content += innerContent + strings.Repeat(" ", b.padding) + "│"
		lines = append(lines, content)
	}

	// Padding bottom
	for i := 0; i < b.padding; i++ {
		lines = append(lines, padLine)
	}

	// Bottom border
	lines = append(lines, "└"+strings.Repeat("─", width-2)+"┘")

	return lines
}

func (b *Box) HandleInput(data string) {
	b.child.HandleInput(data)
}

func (b *Box) Invalidate() {
	b.child.Invalidate()
}

func (b *Box) WantsKeyRelease() bool { return false }

// Input is a single-line text input component.
type Input struct {
	value      []rune
	cursorPos  int
	focused    bool
	onSubmit   func(value string)
	placeholder string
}

// NewInput creates a new Input component.
func NewInput() *Input {
	return &Input{
		placeholder: "",
	}
}

func (in *Input) SetPlaceholder(placeholder string) {
	in.placeholder = placeholder
}

func (in *Input) SetValue(value string) {
	in.value = []rune(value)
	if in.cursorPos > len(in.value) {
		in.cursorPos = len(in.value)
	}
}

func (in *Input) Value() string {
	return string(in.value)
}

func (in *Input) SetOnSubmit(fn func(value string)) {
	in.onSubmit = fn
}

func (in *Input) SetFocused(focused bool) {
	in.focused = focused
}

func (in *Input) Focused() bool {
	return in.focused
}

func (in *Input) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	contentWidth := width - 2 // account for border
	if contentWidth <= 0 {
		return nil
	}

	var displayContent string
	if len(in.value) == 0 && !in.focused && in.placeholder != "" {
		displayContent = in.placeholder
	} else {
		displayContent = string(in.value)
	}

	// Truncate if needed
	contentW := VisibleWidth(displayContent)
	if contentW > contentWidth {
		displayContent = SliceByColumn(displayContent, contentW-contentWidth, contentW, true)
	} else if contentW < contentWidth {
		displayContent += strings.Repeat(" ", contentWidth-contentW)
	}

	// Add cursor marker if focused
	if in.focused {
		// Calculate cursor visual position relative to visible text
		visCursor := VisibleWidth(string(in.value[:in.cursorPos]))
		visContent := VisibleWidth(string(in.value))
		offset := max(0, visContent-contentWidth)
		cursorOffset := visCursor - offset
		if cursorOffset < 0 {
			cursorOffset = 0
		}
		if cursorOffset >= len(displayContent) {
			cursorOffset = len(displayContent) - 1
		}
		// Insert cursor marker
		runes := []rune(displayContent)
		if cursorOffset < len(runes) {
			displayContent = string(runes[:cursorOffset]) + CursorMarker + string(runes[cursorOffset:])
		} else {
			displayContent = displayContent + CursorMarker
		}
	}

	return []string{" " + displayContent + " "}
}

func (in *Input) HandleInput(data string) {
	if !in.focused {
		return
	}

	switch {
	case MatchesKey(data, "enter"):
		if in.onSubmit != nil {
			in.onSubmit(string(in.value))
		}
	case MatchesKey(data, "backspace") || MatchesKey(data, "ctrl+h"):
		if in.cursorPos > 0 {
			in.cursorPos--
			in.value = append(in.value[:in.cursorPos], in.value[in.cursorPos+1:]...)
		}
	case MatchesKey(data, "del"):
		if in.cursorPos < len(in.value) {
			in.value = append(in.value[:in.cursorPos], in.value[in.cursorPos+1:]...)
		}
	case MatchesKey(data, "left"):
		if in.cursorPos > 0 {
			in.cursorPos--
		}
	case MatchesKey(data, "right"):
		if in.cursorPos < len(in.value) {
			in.cursorPos++
		}
	case MatchesKey(data, "home"):
		in.cursorPos = 0
	case MatchesKey(data, "end"):
		in.cursorPos = len(in.value)
	default:
		// Printable characters
		if len(data) == 1 && data[0] >= 0x20 && data[0] <= 0x7e {
			r := rune(data[0])
			newVal := make([]rune, 0, len(in.value)+1)
			newVal = append(newVal, in.value[:in.cursorPos]...)
			newVal = append(newVal, r)
			newVal = append(newVal, in.value[in.cursorPos:]...)
			in.value = newVal
			in.cursorPos++
		}
	}
}

func (in *Input) Invalidate() {}

func (in *Input) WantsKeyRelease() bool { return false }
