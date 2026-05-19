package codecui

import (
	"strings"
	"sync"
	"time"
)

// ArminState represents Armin's emotional/activity state.
type ArminState int

const (
	ArminIdle    ArminState = iota
	ArminThinking
	ArminHappy
	ArminWorking
	ArminError
)

// Armin is a cute ASCII robot character that provides animated visual feedback.
type Armin struct {
	state       ArminState
	frame       int
	mu          sync.Mutex
	active      bool
	stopChan    chan struct{}
	message     string
}

// NewArmin creates a new Armin character.
func NewArmin() *Armin {
	return &Armin{
		state:    ArminIdle,
		stopChan: make(chan struct{}),
	}
}

// SetState changes Armin's visual state.
func (a *Armin) SetState(state ArminState) {
	a.mu.Lock()
	a.state = state
	a.mu.Unlock()
}

// SetMessage sets the message displayed next to Armin.
func (a *Armin) SetMessage(msg string) {
	a.mu.Lock()
	a.message = msg
	a.mu.Unlock()
}

// Start begins the animation loop.
func (a *Armin) Start() {
	a.mu.Lock()
	if a.active {
		a.mu.Unlock()
		return
	}
	a.active = true
	a.mu.Unlock()

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				a.mu.Lock()
				a.frame = (a.frame + 1) % 4
				a.mu.Unlock()
			case <-a.stopChan:
				return
			}
		}
	}()
}

// Stop stops the animation.
func (a *Armin) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.active {
		a.active = false
		close(a.stopChan)
		a.stopChan = make(chan struct{})
	}
}

// Render returns the current Armin frame as lines.
func (a *Armin) Render(width int) []string {
	a.mu.Lock()
	state := a.state
	frame := a.frame
	msg := a.message
	a.mu.Unlock()

	faces := arminFaces(state)
	face := faces[frame%len(faces)]

	var lines []string
	for _, l := range face {
		lines = append(lines, l)
	}

	if msg != "" {
		prefix := "  "
		wrapped := wrapLine(msg, width-len(prefix))
		for _, wl := range wrapped {
			lines = append(lines, prefix+wl)
		}
	}

	return lines
}

func arminFaces(state ArminState) [][]string {
	switch state {
	case ArminThinking:
		return [][]string{
			{
				" ╭─────────────────╮",
				" │  🤔  thinking   │",
				" │  ( . _ .)       │",
				" │   /|  |\\        │",
				" │    /  \\         │",
				" ╰─────────────────╯",
			},
			{
				" ╭─────────────────╮",
				" │  🤔  thinking   │",
				" │  ( . _ .)       │",
				" │   /|  |\\        │",
				" │    /  \\         │",
				" ╰─────────────────╯",
			},
			{
				" ╭─────────────────╮",
				" │  🤔  thinking   │",
				" │  ( o _ o)       │",
				" │   /|  |\\        │",
				" │    /  \\         │",
				" ╰─────────────────╯",
			},
			{
				" ╭─────────────────╮",
				" │  🤔  thinking   │",
				" │  ( . _ .)       │",
				" │   /|  |\\        │",
				" │    /  \\         │",
				" ╰─────────────────╯",
			},
		}

	case ArminHappy:
		return [][]string{
			{
				" ╭─────────────────╮",
				" │  😊  done!      │",
				" │  ( ^ _ ^)       │",
				" │   /|  |\\        │",
				" │    /  \\         │",
				" ╰─────────────────╯",
			},
			{
				" ╭─────────────────╮",
				" │  😊  done!      │",
				" │  ( ^ _ ^)>      │",
				" │   /|  |\\        │",
				" │    /  \\         │",
				" ╰─────────────────╯",
			},
		}

	case ArminWorking:
		return [][]string{
			{
				" ╭─────────────────╮",
				" │  🔧  working    │",
				" │  ( ⚆ _ ⚆)      │",
				" │   /|  |\\        │",
				" │    /  \\  🔧    │",
				" ╰─────────────────╯",
			},
			{
				" ╭─────────────────╮",
				" │  🔧  working    │",
				" │  ( ⚆ _ ⚆)      │",
				" │   /|  |\\   🔧  │",
				" │    /  \\         │",
				" ╰─────────────────╯",
			},
			{
				" ╭─────────────────╮",
				" │  🔧  working    │",
				" │  ( ⚆ _ ⚆)      │",
				" │   /|  |\\        │",
				" │  🔧 /  \\       │",
				" ╰─────────────────╯",
			},
		}

	case ArminError:
		return [][]string{
			{
				" ╭─────────────────╮",
				" │  😞  error      │",
				" │  ( > _ <)       │",
				" │   /|  |\\        │",
				" │    /  \\         │",
				" ╰─────────────────╯",
			},
			{
				" ╭─────────────────╮",
				" │  😞  error      │",
				" │  ( > _ <)       │",
				" │   /|  |\\  !!    │",
				" │    /  \\         │",
				" ╰─────────────────╯",
			},
		}

	default: // ArminIdle
		return [][]string{
			{
				" ╭─────────────────╮",
				" │  🌀  idle       │",
				" │  ( . _ .)       │",
				" │   /|  |\\        │",
				" │    /  \\         │",
				" ╰─────────────────╯",
			},
			{
				" ╭─────────────────╮",
				" │  🌀  idle       │",
				" │  ( . _ .)       │",
				" │   /|  |\\        │",
				" │    /  \\         │",
				" ╰─────────────────╯",
			},
			{
				" ╭─────────────────╮",
				" │  🌀  idle       │",
				" │  ( - _ -)       │",
				" │   /|  |\\        │",
				" │   _/  \\_        │",
				" ╰─────────────────╯",
			},
			{
				" ╭─────────────────╮",
				" │  🌀  idle       │",
				" │  ( . _ .)       │",
				" │   /|  |\\        │",
				" │    /  \\         │",
				" ╰─────────────────╯",
			},
		}
	}
}

func wrapLine(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}
	var lines []string
	runes := []rune(text)
	for len(runes) > 0 {
		if len(runes) <= maxWidth {
			lines = append(lines, string(runes))
			break
		}
		lines = append(lines, string(runes[:maxWidth]))
		runes = runes[maxWidth:]
	}
	return lines
}

var _ = strings.Repeat
