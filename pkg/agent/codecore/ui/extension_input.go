package codecui

import (
	"fmt"

	"github.com/earendil-works/rho/pkg/tui"
)

// InputFactory creates a custom input component.
type InputFactory func(opts InputFactoryOptions) tui.Component

// InputFactoryOptions provides dependencies for custom input creation.
type InputFactoryOptions struct {
	Width       int
	Placeholder string
	OnSubmit    func(value string)
	OnCancel    func()
	OnChange    func(value string)
}

// ExtensionInput wraps a custom input implementation from an extension.
// Extensions can replace the default single-line input with custom behavior
// such as autocomplete-rich inputs, multi-step inputs, or voice-to-text.
type ExtensionInput struct {
	factory     InputFactory
	component   tui.Component
	opts        InputFactoryOptions
	value       string
	placeholder string
}

// NewExtensionInput creates an extension input wrapper.
func NewExtensionInput(placeholder string, factory InputFactory) *ExtensionInput {
	return &ExtensionInput{
		placeholder: placeholder,
		factory:     factory,
	}
}

// SetOnSubmit sets the submit callback.
func (ei *ExtensionInput) SetOnSubmit(fn func(value string)) {
	ei.opts.OnSubmit = fn
}

// SetOnCancel sets the cancel callback.
func (ei *ExtensionInput) SetOnCancel(fn func()) {
	ei.opts.OnCancel = fn
}

// SetOnChange sets the change callback.
func (ei *ExtensionInput) SetOnChange(fn func(value string)) {
	ei.opts.OnChange = fn
}

// SetValue sets the input value.
func (ei *ExtensionInput) SetValue(value string) {
	ei.value = value
}

// Value returns the current input value.
func (ei *ExtensionInput) Value() string {
	return ei.value
}

// SetInputFactory sets the custom input factory from an extension.
func (ei *ExtensionInput) SetInputFactory(factory InputFactory) {
	ei.factory = factory
	ei.component = nil
}

func (ei *ExtensionInput) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	ei.opts.Width = width
	ei.opts.Placeholder = ei.placeholder

	if ei.factory != nil {
		if ei.component == nil {
			ei.component = ei.factory(ei.opts)
		}
		return ei.component.Render(width)
	}

	// Fallback: basic styled input
	displayText := ei.value
	if displayText == "" {
		displayText = ei.placeholder
	}

	prompt := fmt.Sprintf(" ❯ %s", displayText)
	if tui.VisibleWidth(prompt) > width {
		prompt = tui.SliceByColumn(prompt, 0, width-1, true) + "…"
	}

	var lines []string
	lines = append(lines, tui.DefaultMarkdownTheme().Code(prompt))
	lines = append(lines, tui.DefaultMarkdownTheme().Code(" Enter: submit"))

	return lines
}

func (ei *ExtensionInput) HandleInput(data string) {
	if ei.component != nil {
		ei.component.HandleInput(data)
		return
	}

	if tui.MatchesKey(data, "enter") && ei.opts.OnSubmit != nil {
		ei.opts.OnSubmit(ei.value)
	} else if tui.MatchesKey(data, "escape") && ei.opts.OnCancel != nil {
		ei.opts.OnCancel()
	}
}

func (ei *ExtensionInput) Invalidate() {
	if ei.component != nil {
		ei.component.Invalidate()
	}
}

func (ei *ExtensionInput) WantsKeyRelease() bool { return false }
