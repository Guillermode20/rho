package codecui

import (
	"fmt"
	"strings"
	"time"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/agent/extensions"
	"github.com/earendil-works/rho/pkg/tui"
)

// BorderStyle defines the style of a border.
type BorderStyle int

const (
	BorderSingle  BorderStyle = iota // ┌─┐
	BorderDouble                     // ╔═╗
	BorderRounded                    // ╭─╮
	BorderBold                       // ┏━┓
	BorderAnimated                   // animated dots
	BorderHidden                     // no border
)

// DynamicBorder is an animated/configurable border component.
type DynamicBorder struct {
	style    BorderStyle
	title    string
	child    tui.Component
	interval time.Duration
	animIdx  int
	lastTick time.Time
}

// NewDynamicBorder creates a dynamic border around a child component.
func NewDynamicBorder(title string, child tui.Component) *DynamicBorder {
	return &DynamicBorder{
		style:    BorderRounded,
		title:    title,
		child:    child,
		interval: 200 * time.Millisecond,
	}
}

// SetStyle sets the border style.
func (db *DynamicBorder) SetStyle(style BorderStyle) {
	db.style = style
}

func (db *DynamicBorder) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	innerWidth := width - 2
	if innerWidth <= 0 {
		lines := db.child.Render(width)
		return lines
	}

	// Tick animation
	if time.Since(db.lastTick) > db.interval {
		db.animIdx++
		db.lastTick = time.Now()
	}

	// Get border characters
	topLeft, topRight, bottomLeft, bottomRight, h, v := db.getBorderChars()

	childLines := db.child.Render(innerWidth)

	var lines []string

	// Top border
	top := topLeft + h
	if db.title != "" {
		titleMax := innerWidth - 2
		title := db.title
		if len(title) > titleMax {
			title = title[:titleMax]
		}
		pad := innerWidth - len(title) - 2
		if pad < 0 {
			pad = 0
		}
		top += " " + title + " " + strings.Repeat(h, pad)
	} else {
		top += strings.Repeat(h, innerWidth)
	}
	top += topRight
	lines = append(lines, top)

	// Content
	for _, cl := range childLines {
		padded := cl
		if tui.VisibleWidth(padded) < innerWidth {
			padded += strings.Repeat(" ", innerWidth-tui.VisibleWidth(padded))
		}
		lines = append(lines, v+padded+v)
	}

	// Bottom border
	bottom := bottomLeft + strings.Repeat(h, innerWidth) + bottomRight
	lines = append(lines, bottom)

	return lines
}

func (db *DynamicBorder) getBorderChars() (topLeft, topRight, bottomLeft, bottomRight, h, v string) {
	switch db.style {
	case BorderDouble:
		return "╔", "╗", "╚", "╝", "═", "║"
	case BorderRounded:
		return "╭", "╮", "╰", "╯", "─", "│"
	case BorderBold:
		return "┏", "┓", "┗", "┛", "━", "┃"
	case BorderAnimated:
		frames := []string{"·", "●", "○", "◌"}
		frame := frames[db.animIdx%len(frames)]
		return frame, frame, frame, frame, "─", "│"
	case BorderHidden:
		return " ", " ", " ", " ", " ", " "
	default: // BorderSingle
		return "┌", "┐", "└", "┘", "─", "│"
	}
}

func (db *DynamicBorder) HandleInput(data string) {
	db.child.HandleInput(data)
}

func (db *DynamicBorder) Invalidate() {
	db.child.Invalidate()
}

func (db *DynamicBorder) WantsKeyRelease() bool { return false }

// LoginDialog displays an OAuth login flow.
type LoginDialog struct {
	providerName string
	status       string // "waiting", "code_received", "completed", "error"
	authURL      string
	code         string
	errorMessage string
	focused      bool
}

// NewLoginDialog creates a login dialog for OAuth.
func NewLoginDialog(providerName, authURL string) *LoginDialog {
	return &LoginDialog{
		providerName: providerName,
		authURL:      authURL,
		status:       "waiting",
	}
}

// SetCode sets the received authorization code.
func (ld *LoginDialog) SetCode(code string) {
	ld.code = code
	ld.status = "code_received"
}

// SetError sets an error message.
func (ld *LoginDialog) SetError(err string) {
	ld.errorMessage = err
	ld.status = "error"
}

// SetCompleted marks the login as complete.
func (ld *LoginDialog) SetCompleted() {
	ld.status = "completed"
}

func (ld *LoginDialog) SetFocused(focused bool) {
	ld.focused = focused
}

func (ld *LoginDialog) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	reset := "\x1b[0m"
	bold := "\x1b[1m"
	dim := "\x1b[2m"
	cyan := "\x1b[36m"
	green := "\x1b[32m"
	red := "\x1b[31m"

	var lines []string
	lines = append(lines, bold+cyan+"Login: "+ld.providerName+reset)
	lines = append(lines, "")

	switch ld.status {
	case "waiting":
		lines = append(lines, dim+"  Waiting for authentication..."+reset)
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("  %s%s%s", dim+"Open this URL:", reset, cyan+ld.authURL+reset))
		lines = append(lines, "")
		lines = append(lines, dim+"  Then paste the authorization code below."+reset)

	case "code_received":
		lines = append(lines, green+"  \u2713 Authorization code received"+reset)
		lines = append(lines, dim+"  Exchanging for tokens..."+reset)

	case "completed":
		lines = append(lines, green+"  \u2713 Login complete!"+reset)

	case "error":
		lines = append(lines, red+"  \u2717 Login failed"+reset)
		lines = append(lines, dim+"  "+ld.errorMessage+reset)
	}

	return lines
}

func (ld *LoginDialog) HandleInput(data string) {}
func (ld *LoginDialog) Invalidate()            {}
func (ld *LoginDialog) WantsKeyRelease() bool  { return false }

// OAuthSelector selects from available OAuth providers.
type OAuthSelector struct {
	providers   []string
	selectedIdx int
	focused     bool
	onSelect    func(provider string)
	onCancel    func()
}

// NewOAuthSelector creates an OAuth provider selector.
func NewOAuthSelector(providers []string) *OAuthSelector {
	return &OAuthSelector{
		providers: providers,
	}
}

func (osel *OAuthSelector) SetOnSelect(fn func(provider string)) {
	osel.onSelect = fn
}

func (osel *OAuthSelector) SetOnCancel(fn func()) {
	osel.onCancel = fn
}

func (osel *OAuthSelector) SetFocused(focused bool) {
	osel.focused = focused
}

func (osel *OAuthSelector) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	reset := "\x1b[0m"
	bold := "\x1b[1m"
_ = "\x1b[2m"
	cyan := "\x1b[36m"

	var lines []string
	lines = append(lines, bold+cyan+"Select OAuth Provider"+reset)
	lines = append(lines, "")

	for i, p := range osel.providers {
		prefix := "  "
		if i == osel.selectedIdx && osel.focused {
			prefix = "\u203a " + cyan
		}
		lines = append(lines, fmt.Sprintf("%s%s", prefix, p))
	}

	return lines
}

func (osel *OAuthSelector) HandleInput(data string) {
	switch {
	case tui.MatchesKey(data, "up") || tui.MatchesKey(data, "ctrl+p"):
		if osel.selectedIdx > 0 {
			osel.selectedIdx--
		}
	case tui.MatchesKey(data, "down") || tui.MatchesKey(data, "ctrl+n"):
		if osel.selectedIdx < len(osel.providers)-1 {
			osel.selectedIdx++
		}
	case tui.MatchesKey(data, "enter"):
		if osel.selectedIdx < len(osel.providers) && osel.onSelect != nil {
			osel.onSelect(osel.providers[osel.selectedIdx])
		}
	case tui.MatchesKey(data, "escape"):
		if osel.onCancel != nil {
			osel.onCancel()
		}
	}
}

func (osel *OAuthSelector) Invalidate()           {}
func (osel *OAuthSelector) WantsKeyRelease() bool { return false }

// CountdownTimer shows a timed countdown overlay.
type CountdownTimer struct {
	message   string
	remaining time.Duration
	total     time.Duration
	running   bool
	onExpire  func()
}

// NewCountdownTimer creates a countdown timer display.
func NewCountdownTimer(message string, duration time.Duration) *CountdownTimer {
	return &CountdownTimer{
		message:   message,
		remaining: duration,
		total:     duration,
		running:   true,
	}
}

// Tick decrements the timer by the given duration.
func (ct *CountdownTimer) Tick(d time.Duration) {
	if !ct.running {
		return
	}
	ct.remaining -= d
	if ct.remaining <= 0 {
		ct.remaining = 0
		ct.running = false
		if ct.onExpire != nil {
			ct.onExpire()
		}
	}
}

// SetOnExpire sets the expiration callback.
func (ct *CountdownTimer) SetOnExpire(fn func()) {
	ct.onExpire = fn
}

// Stop stops the timer.
func (ct *CountdownTimer) Stop() {
	ct.running = false
}

func (ct *CountdownTimer) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	reset := "\x1b[0m"
	yellow := "\x1b[33m"
	red := "\x1b[31m"

	var lines []string

	secs := int(ct.remaining.Seconds())
	timeStr := fmt.Sprintf("%02d:%02d", secs/60, secs%60)

	color := yellow
	if secs <= 5 {
		color = red
	}

	barWidth := width - len(ct.message) - 12
	if barWidth < 5 {
		barWidth = 5
	}

	pct := float64(ct.remaining) / float64(ct.total)
	filled := int(float64(barWidth) * pct)
	bar := strings.Repeat("\u2593", filled) + strings.Repeat("\u2591", barWidth-filled)

	lines = append(lines, fmt.Sprintf("%s %s [%s] %s", ct.message, color+timeStr+reset, bar, color))

	return lines
}

func (ct *CountdownTimer) HandleInput(data string) {
	if tui.MatchesKey(data, "escape") || tui.MatchesKey(data, "ctrl+c") {
		ct.Stop()
		if ct.onExpire != nil {
			ct.onExpire()
		}
	}
}

func (ct *CountdownTimer) Invalidate()            {}
func (ct *CountdownTimer) WantsKeyRelease() bool  { return false }

// CustomMessage renders a custom message type using extension renderers.
type CustomMessage struct {
	msg       agent.AgentMessage
	renderers []extensions.MessageRenderer
}

// NewCustomMessage creates a custom message display.
func NewCustomMessage(msg agent.AgentMessage, renderers []extensions.MessageRenderer) *CustomMessage {
	return &CustomMessage{
		msg:       msg,
		renderers: renderers,
	}
}

func (cm *CustomMessage) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	// Find matching renderer
	for _, r := range cm.renderers {
		rendered := r.Render(cm.msg, width)
		if rendered != nil {
			return rendered
		}
	}

	// Fallback: show raw content
	return []string{cm.msg.Content}
}

func (cm *CustomMessage) HandleInput(data string) {}
func (cm *CustomMessage) Invalidate()            {}
func (cm *CustomMessage) WantsKeyRelease() bool  { return false }

// ExtensionSelector shows loaded extensions with enable/disable toggle.
type ExtensionSelector struct {
	extensions  []extensions.ExtensionDef
	selectedIdx int
	enabled     map[string]bool
	focused     bool
	onToggle    func(name string, enabled bool)
	onCancel    func()
}

// NewExtensionSelector creates an extension selector.
func NewExtensionSelector(exts []extensions.ExtensionDef) *ExtensionSelector {
	enabled := make(map[string]bool)
	for _, e := range exts {
		enabled[e.Name] = true
	}
	return &ExtensionSelector{
		extensions: exts,
		enabled:    enabled,
	}
}

func (es *ExtensionSelector) SetOnToggle(fn func(name string, enabled bool)) {
	es.onToggle = fn
}

func (es *ExtensionSelector) SetOnCancel(fn func()) {
	es.onCancel = fn
}

func (es *ExtensionSelector) SetFocused(focused bool) {
	es.focused = focused
}

func (es *ExtensionSelector) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	reset := "\x1b[0m"
	bold := "\x1b[1m"
	dim := "\x1b[2m"
	cyan := "\x1b[36m"
	green := "\x1b[32m"
	red := "\x1b[31m"

	var lines []string
	lines = append(lines, bold+cyan+"Extensions"+reset)
	lines = append(lines, "")

	if len(es.extensions) == 0 {
		lines = append(lines, dim+"  No extensions loaded"+reset)
		return lines
	}

	for i, ext := range es.extensions {
		prefix := "  "
		if i == es.selectedIdx && es.focused {
			prefix = "\u203a "
		}

		enabled := es.enabled[ext.Name]
		status := green + "ON" + reset
		if !enabled {
			status = red + "OFF" + reset
		}

		ver := ""
		if ext.Version != "" {
			ver = dim + " v" + ext.Version + reset
		}

		line := fmt.Sprintf("%s%s  %s%s  %s", prefix, bold+ext.Name+reset, status, ver, dim+ext.Description+reset)
		if tui.VisibleWidth(line) > width {
			line = tui.SliceByColumn(line, 0, width, true)
		}
		lines = append(lines, line)
	}

	lines = append(lines, "")
	lines = append(lines, dim+"  Enter to toggle, Esc to close"+reset)

	return lines
}

func (es *ExtensionSelector) HandleInput(data string) {
	switch {
	case tui.MatchesKey(data, "up") || tui.MatchesKey(data, "ctrl+p"):
		if es.selectedIdx > 0 {
			es.selectedIdx--
		}
	case tui.MatchesKey(data, "down") || tui.MatchesKey(data, "ctrl+n"):
		if es.selectedIdx < len(es.extensions)-1 {
			es.selectedIdx++
		}
	case tui.MatchesKey(data, "enter") || tui.MatchesKey(data, " "):
		if es.selectedIdx < len(es.extensions) {
			name := es.extensions[es.selectedIdx].Name
			es.enabled[name] = !es.enabled[name]
			if es.onToggle != nil {
				es.onToggle(name, es.enabled[name])
			}
		}
	case tui.MatchesKey(data, "escape"):
		if es.onCancel != nil {
			es.onCancel()
		}
	}
}

func (es *ExtensionSelector) Invalidate()           {}
func (es *ExtensionSelector) WantsKeyRelease() bool { return false }
