package codecui

import (
	"fmt"
	"strings"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/tui"
)

// Footer displays git branch, model, token count, thinking level, cost,
// session name, auth status, and extension statuses.
type Footer struct {
	gitBranch     string
	sessionName   string
	modelName     string
	providerName  string
	thinkingLevel string
	tokenCount    int
	contextWindow int
	totalCost     float64
	authStatus    string
	oauthActive   bool
	statuses      map[string]string // key -> text from extensions
	showHelp      bool
}

// NewFooter creates a new footer component.
func NewFooter() *Footer {
	return &Footer{
		statuses: make(map[string]string),
	}
}

// SetGitBranch sets the git branch display.
func (f *Footer) SetGitBranch(branch string) {
	f.gitBranch = branch
}

// SetSessionName sets the session display name.
func (f *Footer) SetSessionName(name string) {
	f.sessionName = name
}

// SetModel sets the model and provider display.
func (f *Footer) SetModel(model, provider string) {
	f.modelName = model
	f.providerName = provider
}

// SetThinkingLevel sets the thinking level indicator.
func (f *Footer) SetThinkingLevel(level string) {
	f.thinkingLevel = level
}

// SetTokenCount sets the token count and context window.
func (f *Footer) SetTokenCount(count, window int) {
	f.tokenCount = count
	f.contextWindow = window
}

// SetTotalCost sets the running total cost display.
func (f *Footer) SetTotalCost(cost float64) {
	f.totalCost = cost
}

// SetAuthStatus sets the auth status text (e.g. "OAuth" or "API key").
func (f *Footer) SetAuthStatus(status string, oauth bool) {
	f.authStatus = status
	f.oauthActive = oauth
}

// SetStatus sets an extension status by key.
func (f *Footer) SetStatus(key, text string) {
	if text == "" {
		delete(f.statuses, key)
	} else {
		f.statuses[key] = text
	}
}

// SetShowHelp toggles help hints.
func (f *Footer) SetShowHelp(show bool) {
	f.showHelp = show
}

func (f *Footer) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	th := tui.DefaultTheme

	var parts []string

	// Git branch
	if f.gitBranch != "" {
		parts = append(parts, th.Colored("\u2302 "+f.gitBranch, th.Palette.Success))
	}

	// Session name (nickname)
	if f.sessionName != "" {
		parts = append(parts, th.Muted("@ "+f.sessionName))
	}

	// Model/provider
	if f.modelName != "" {
		modelStr := f.modelName
		if f.providerName != "" {
			modelStr = f.providerName + "/" + f.modelName
		}
		parts = append(parts, th.Colored(modelStr, th.Palette.Accent))
	}

	// Thinking level
	if f.thinkingLevel != "" && f.thinkingLevel != "off" {
		parts = append(parts, th.Muted("\u2606 "+f.thinkingLevel))
	}

	// Token count + context %
	if f.tokenCount > 0 {
		tokenStr := fmt.Sprintf("%d tok", f.tokenCount)
		if f.contextWindow > 0 {
			pct := int(float64(f.tokenCount) / float64(f.contextWindow) * 100)
			tokenStr = fmt.Sprintf("%d/%d (%d%%)", f.tokenCount, f.contextWindow, pct)
		}
		parts = append(parts, th.Muted(tokenStr))
	}

	// Cost
	if f.totalCost > 0 {
		parts = append(parts, th.Muted(fmt.Sprintf("$%.4f", f.totalCost)))
	}

	// Auth status
	if f.authStatus != "" {
		authIcon := "\u2663" // club = key
		if f.oauthActive {
			authIcon = "\u2666" // diamond = OAuth
		}
		parts = append(parts, th.Muted(authIcon+f.authStatus))
	}

	// Extension statuses
	for _, status := range f.statuses {
		parts = append(parts, th.Muted(status))
	}

	// Help hint
	if f.showHelp {
		parts = append(parts, th.Muted("? for help"))
	}

	// Use subtle middle dot as separator instead of pipe
	sep := th.Muted(" \u00b7 ")
	line := strings.Join(parts, sep)

	// Pad to full width so background fills
	if tui.VisibleWidth(line) > width {
		line = tui.SliceByColumn(line, 0, width, true)
	} else {
		line += strings.Repeat(" ", max(0, width-tui.VisibleWidth(line)))
	}

	return []string{th.Muted(line)}
}

func (f *Footer) HandleInput(data string) {}
func (f *Footer) Invalidate()            {}
func (f *Footer) WantsKeyRelease() bool  { return false }

// KeybindingHint displays keybinding hints at the bottom.
type KeybindingHint struct {
	hints []KeybindingHintItem
}

// NewKeybindingHint creates a new keybinding hint bar.
func NewKeybindingHint() *KeybindingHint {
	return &KeybindingHint{}
}

// SetHints sets the hint items.
func (kh *KeybindingHint) SetHints(hints []KeybindingHintItem) {
	kh.hints = hints
}

func (kh *KeybindingHint) Render(width int) []string {
	if width <= 0 || len(kh.hints) == 0 {
		return nil
	}

	dim := "\x1b[2m"
	reset := "\x1b[0m"
	bold := "\x1b[1m"

	var parts []string
	for _, hint := range kh.hints {
		if hint.Key == "" {
			continue
		}
		formatted := fmt.Sprintf("%s%s%s %s", bold, hint.Key, reset, hint.Description)
		parts = append(parts, formatted)
	}

	line := strings.Join(parts, dim+" \u2022 "+reset)
	if tui.VisibleWidth(line) > width {
		line = tui.SliceByColumn(line, 0, width, true)
	}

	return []string{dim + line + reset}
}

func (kh *KeybindingHint) HandleInput(data string) {}
func (kh *KeybindingHint) Invalidate()            {}
func (kh *KeybindingHint) WantsKeyRelease() bool  { return false }

// BorderedLoader shows a loading indicator with a border and optional message.
type BorderedLoader struct {
	message   string
	frames    []string
	frameIdx  int
}

// NewBorderedLoader creates a new bordered loader.
func NewBorderedLoader(message string) *BorderedLoader {
	return &BorderedLoader{
		message: message,
		frames:  []string{"\u25D0", "\u25D3", "\u25D1", "\u25D2"},
	}
}

// SetMessage updates the loader message.
func (bl *BorderedLoader) SetMessage(msg string) {
	bl.message = msg
}

func (bl *BorderedLoader) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	dim := "\x1b[2m"
	reset := "\x1b[0m"
	cyan := "\x1b[36m"
	bold := "\x1b[1m"

	spinner := bl.frames[bl.frameIdx%len(bl.frames)]
	bl.frameIdx++

	lines := []string{
		dim + "\u250c" + strings.Repeat("\u2500", width-2) + "\u2510" + reset,
		dim + "\u2502" + reset + "  " + cyan + bold + spinner + reset + " " + bl.message + strings.Repeat(" ", width-tui.VisibleWidth(bl.message)-6) + dim + "\u2502" + reset,
		dim + "\u2514" + strings.Repeat("\u2500", width-2) + "\u2518" + reset,
	}
	return lines
}

func (bl *BorderedLoader) HandleInput(data string) {}
func (bl *BorderedLoader) Invalidate()            {}
func (bl *BorderedLoader) WantsKeyRelease() bool  { return false }



// Ensure types are used.
var _ = agent.AgentMessage{}
