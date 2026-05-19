package codecui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/earendil-works/rho/pkg/tui"
)

// LoginDialogState tracks the login flow.
type LoginDialogState string

const (
	LoginIdle      LoginDialogState = "idle"
	LoginWaiting   LoginDialogState = "waiting"
	LoginSuccess   LoginDialogState = "success"
	LoginFailed    LoginDialogState = "failed"
	LoginExpired   LoginDialogState = "expired"
)

// LoginDialogComponent renders an OAuth/provider login overlay.
type LoginDialogComponent struct {
	mu             sync.Mutex
	state          LoginDialogState
	providerName   string
	loginURL       string
	clientID       string
	message        string
	startTime      time.Time
	timeout        time.Duration
	spinnerFrames  []string
	spinnerIdx     int
	onCancel       func()
	onRetry        func()
}

// NewLoginDialogComponent creates a new login dialog.
func NewLoginDialogComponent(providerName, loginURL, clientID string) *LoginDialogComponent {
	return &LoginDialogComponent{
		state:         LoginIdle,
		providerName:  providerName,
		loginURL:      loginURL,
		clientID:      clientID,
		startTime:     time.Now(),
		timeout:       5 * time.Minute,
		spinnerFrames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	}
}

// SetState updates the dialog state.
func (d *LoginDialogComponent) SetState(state LoginDialogState, message string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.state = state
	d.message = message
}

// SetOnCancel sets the cancel callback.
func (d *LoginDialogComponent) SetOnCancel(fn func()) {
	d.onCancel = fn
}

// SetOnRetry sets the retry callback.
func (d *LoginDialogComponent) SetOnRetry(fn func()) {
	d.onRetry = fn
}

// Tick advances the spinner.
func (d *LoginDialogComponent) Tick() {
	d.mu.Lock()
	d.spinnerIdx = (d.spinnerIdx + 1) % len(d.spinnerFrames)
	d.mu.Unlock()
}

func (d *LoginDialogComponent) Render(width int) []string {
	d.mu.Lock()
	defer d.mu.Unlock()

	if width <= 0 {
		return nil
	}

	// Dialog width
	dialogWidth := 60
	if width < dialogWidth {
		dialogWidth = width - 4
	}
	if dialogWidth < 40 {
		dialogWidth = 40
	}

	innerWidth := dialogWidth - 4
	var lines []string

	// Top border
	lines = append(lines, "┌"+strings.Repeat("─", dialogWidth-2)+"┐")

	// Title
	title := fmt.Sprintf("  Login to %s", d.providerName)
	lines = append(lines, fmt.Sprintf("│%s%s│", title, strings.Repeat(" ", dialogWidth-2-len(title))))

	// Separator
	lines = append(lines, "│"+strings.Repeat("─", dialogWidth-2)+"│")

	switch d.state {
	case LoginIdle, LoginWaiting:
		spinner := d.spinnerFrames[d.spinnerIdx%len(d.spinnerFrames)]

		lines = append(lines, fmt.Sprintf("│  %s Waiting for browser login...   │", spinner))
		lines = append(lines, "│" + strings.Repeat(" ", dialogWidth-2) + "│")

		// Login URL
		url := d.loginURL
		if len(url) > innerWidth {
			url = url[:innerWidth-3] + "..."
		}
		lines = append(lines, fmt.Sprintf("│  \x1b[36m%s\x1b[0m%s│", url, strings.Repeat(" ", dialogWidth-4-len(url))))

		// Client ID
		if d.clientID != "" {
			cid := fmt.Sprintf("Client ID: %s", d.clientID)
			if len(cid) > innerWidth {
				cid = cid[:innerWidth-3] + "..."
			}
			lines = append(lines, fmt.Sprintf("│  %s%s│", cid, strings.Repeat(" ", dialogWidth-4-len(cid))))
		}

		lines = append(lines, "│" + strings.Repeat(" ", dialogWidth-2) + "│")

		// Timeout info
		remaining := d.timeout - time.Since(d.startTime)
		if remaining < 0 {
			remaining = 0
		}
		timeoutStr := fmt.Sprintf("Auto-cancels in %s", formatDuration(remaining))
		lines = append(lines, fmt.Sprintf("│  \x1b[2m%s\x1b[0m%s│", timeoutStr, strings.Repeat(" ", dialogWidth-4-len(timeoutStr))))

		lines = append(lines, fmt.Sprintf("│  \x1b[2m[Esc to cancel]\x1b[0m%s│", strings.Repeat(" ", dialogWidth-20)))

	case LoginSuccess:
		lines = append(lines, "│  \x1b[32m✓ Login successful!\x1b[0m" + strings.Repeat(" ", dialogWidth-24) + "│")
		if d.message != "" {
			lines = append(lines, fmt.Sprintf("│  %s%s│", d.message, strings.Repeat(" ", dialogWidth-4-len(d.message))))
		}

	case LoginFailed:
		lines = append(lines, "│  \x1b[31m✗ Login failed\x1b[0m" + strings.Repeat(" ", dialogWidth-18) + "│")
		if d.message != "" {
			msg := d.message
			if len(msg) > innerWidth {
				msg = msg[:innerWidth-3] + "..."
			}
			lines = append(lines, fmt.Sprintf("│  %s%s│", msg, strings.Repeat(" ", dialogWidth-4-len(msg))))
		}
		lines = append(lines, "│" + strings.Repeat(" ", dialogWidth-2) + "│")
		lines = append(lines, "│  \x1b[33m[Enter to retry]\x1b[0m" + strings.Repeat(" ", dialogWidth-20) + "│")

	case LoginExpired:
		lines = append(lines, "│  \x1b[33m⚠ Login timed out\x1b[0m" + strings.Repeat(" ", dialogWidth-22) + "│")
		lines = append(lines, "│" + strings.Repeat(" ", dialogWidth-2) + "│")
		lines = append(lines, "│  \x1b[33m[Enter to retry]\x1b[0m" + strings.Repeat(" ", dialogWidth-20) + "│")
	}

	// Bottom border
	lines = append(lines, "└"+strings.Repeat("─", dialogWidth-2)+"┘")

	return lines
}

func (d *LoginDialogComponent) HandleInput(data string) {
	if tui.MatchesKey(data, "escape") {
		if d.onCancel != nil {
			d.onCancel()
		}
	}
	if tui.MatchesKey(data, "enter") {
		if d.state == LoginFailed || d.state == LoginExpired {
			if d.onRetry != nil {
				d.mu.Lock()
				d.state = LoginWaiting
				d.startTime = time.Now()
				d.mu.Unlock()
				d.onRetry()
			}
		}
	}
}

func (d *LoginDialogComponent) Invalidate()          {}
func (d *LoginDialogComponent) WantsKeyRelease() bool { return false }
