package codecui

import (
	"strings"

	"github.com/earendil-works/rho/pkg/tui"
)

// EditorFactory creates a custom editor component.
type EditorFactory func(opts EditorFactoryOptions) tui.Component

// EditorFactoryOptions provides dependencies for custom editor creation.
type EditorFactoryOptions struct {
	Width     int
	Theme     *tui.MarkdownTheme
	OnSubmit  func(text string)
	OnCancel  func()
}

// ExtensionEditor wraps a custom editor implementation from an extension.
// Extensions can set their own editor via SetEditorFactory, replacing the
// default input component with a full multi-line editor, vim-mode editor,
// or any other custom implementation.
type ExtensionEditor struct {
	factory   EditorFactory
	component tui.Component
	opts      EditorFactoryOptions
	text      string
}

// NewExtensionEditor creates an extension editor wrapper.
// If no factory is provided, uses a basic multi-line input fallback.
func NewExtensionEditor(text string, factory EditorFactory) *ExtensionEditor {
	return &ExtensionEditor{
		text:    text,
		factory: factory,
	}
}

// SetOnSubmit sets the submit callback.
func (ee *ExtensionEditor) SetOnSubmit(fn func(text string)) {
	ee.opts.OnSubmit = fn
}

// SetOnCancel sets the cancel callback.
func (ee *ExtensionEditor) SetOnCancel(fn func()) {
	ee.opts.OnCancel = fn
}

// SetText sets the editor text.
func (ee *ExtensionEditor) SetText(text string) {
	ee.text = text
}

// GetText returns the current editor text.
func (ee *ExtensionEditor) GetText() string {
	return ee.text
}

// SetEditorFactory sets the custom editor factory from an extension.
func (ee *ExtensionEditor) SetEditorFactory(factory EditorFactory) {
	ee.factory = factory
	ee.component = nil // Force re-creation
}

func (ee *ExtensionEditor) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	ee.opts.Width = width

	// Use custom factory if available
	if ee.factory != nil {
		if ee.component == nil {
			ee.component = ee.factory(ee.opts)
		}
		return ee.component.Render(width)
	}

	// Fallback: basic multi-line input
	var lines []string
	if ee.text == "" {
		lines = append(lines, "  "+tui.DefaultMarkdownTheme().Code("[empty editor — type your message]"))
	} else {
		textLines := strings.Split(ee.text, "\n")
		for _, tl := range textLines {
			truncated := tl
			if tui.VisibleWidth(truncated) > width-4 {
				truncated = tui.SliceByColumn(truncated, 0, width-4, true)
			}
			lines = append(lines, "  "+truncated)
		}
	}

	// Submit hint
	if len(lines) > 0 {
		lines = append(lines, "")
		lines = append(lines, tui.DefaultMarkdownTheme().Code(" Ctrl+Enter: submit  Esc: cancel"))
	}

	return lines
}

func (ee *ExtensionEditor) HandleInput(data string) {
	if ee.component != nil {
		ee.component.HandleInput(data)
		return
	}

	if tui.MatchesKey(data, "escape") && ee.opts.OnCancel != nil {
		ee.opts.OnCancel()
	}
}

func (ee *ExtensionEditor) Invalidate() {
	if ee.component != nil {
		ee.component.Invalidate()
	}
}

func (ee *ExtensionEditor) WantsKeyRelease() bool { return false }
