package tui

// EditorComponent defines the interface for pluggable editor implementations.
// Custom editors can replace the built-in input/editor for specialized input modes.
type EditorComponent interface {
	Component
	Focusable

	// Text returns the current editor content.
	Text() string

	// SetText replaces the editor content.
	SetText(text string)

	// Focus requests focus for this component.
	Focus()

	// Blur removes focus.
	Blur()

	// Insert inserts text at the current cursor position.
	Insert(text string)

	// SetOnSubmit is called when the user presses Enter to submit.
	SetOnSubmit(func(text string))

	// SetOnChange is called when the content changes.
	SetOnChange(func(text string))
}

// CustomEditor is a base implementation of EditorComponent that handles
// app keybindings and delegates text editing to a built-in editor.
// Extend this to create custom editors with additional keybindings.
type CustomEditor struct {
	*Editor
	HandleAppKey func(data string) bool // Return true if handled
	onSubmit     func(text string)
	onChange     func(text string)
}

// NewCustomEditor creates a new CustomEditor wrapping the built-in Editor.
func NewCustomEditor(opts EditorOptions) *CustomEditor {
	return &CustomEditor{
		Editor: NewEditor(opts),
	}
}

// Text returns the editor content.
func (ce *CustomEditor) Text() string {
	return ce.GetText()
}

// Focus sets focus.
func (ce *CustomEditor) Focus() {
	ce.SetFocused(true)
}

// Blur removes focus.
func (ce *CustomEditor) Blur() {
	ce.SetFocused(false)
}

// Insert inserts text at cursor.
func (ce *CustomEditor) Insert(text string) {
	// Delegate to HandleInput character by character
	for _, r := range text {
		ce.Editor.HandleInput(string(r))
	}
}

// SetOnSubmit sets the submit callback.
func (ce *CustomEditor) SetOnSubmit(fn func(text string)) {
	ce.onSubmit = fn
	// Override the editor's submit to go through our wrapper
	oldOnSubmit := ce.Editor.onSubmit
	ce.Editor.SetOnSubmit(func(text string) {
		if oldOnSubmit != nil {
			oldOnSubmit(text)
		}
		if ce.onSubmit != nil {
			ce.onSubmit(text)
		}
	})
}

// SetOnChange sets the change callback.
func (ce *CustomEditor) SetOnChange(fn func(text string)) {
	ce.onChange = fn
	oldOnChange := ce.Editor.onChange
	ce.Editor.SetOnChange(func(text string) {
		if oldOnChange != nil {
			oldOnChange(text)
		}
		if ce.onChange != nil {
			ce.onChange(text)
		}
	})
}

// HandleInput handles keyboard input with app keybinding support.
// Override this or set HandleAppKey in custom editors.
func (ce *CustomEditor) HandleInput(data string) {
	if ce.HandleAppKey != nil && ce.HandleAppKey(data) {
		return
	}
	ce.Editor.HandleInput(data)
}

// VimEditor is an example custom editor with Vim-like keybindings.
type VimEditor struct {
	*CustomEditor
	mode string // "normal" or "insert"
}

// NewVimEditor creates a new VimEditor.
func NewVimEditor(opts EditorOptions) *VimEditor {
	ve := &VimEditor{
		CustomEditor: NewCustomEditor(opts),
		mode:         "insert",
	}
	ve.CustomEditor.HandleAppKey = func(data string) bool {
		return ve.handleVimKey(data)
	}
	return ve
}

func (ve *VimEditor) handleVimKey(data string) bool {
	switch ve.mode {
	case "normal":
		switch {
		case MatchesKey(data, "i"):
			ve.mode = "insert"
			return true
		case MatchesKey(data, "x"):
			// Delete forward - just eat the next input
			ve.mode = "insert"
			ve.HandleInput("\x7f") // backspace
			ve.mode = "normal"
			return true
		case MatchesKey(data, "u"):
			// Undo - not directly supported by base editor
			return true
		case MatchesKey(data, "escape"):
			return true
		}
	case "insert":
		if MatchesKey(data, "escape") {
			ve.mode = "normal"
			return true
		}
	}
	return false
}

// Mode returns the current Vim mode.
func (ve *VimEditor) Mode() string {
	return ve.mode
}

// SetMode sets the Vim mode.
func (ve *VimEditor) SetMode(mode string) {
	ve.mode = mode
}
