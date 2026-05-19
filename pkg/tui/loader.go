package tui

import (
	"strings"
	"sync"
	"time"
)

// Loader is an animated spinner/loader component.
type Loader struct {
	message       string
	frames        []string
	currentFrame  int
	interval      time.Duration
	active        bool
	stopChan      chan struct{}
	mu            sync.Mutex
	ticker        *time.Ticker
	onRender      func()
}

// DefaultSpinnerFrames are the animation frames for the loader.
var DefaultSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// NewLoader creates a new Loader component.
func NewLoader(message string) *Loader {
	return &Loader{
		message:  message,
		frames:   DefaultSpinnerFrames,
		interval: 80 * time.Millisecond,
		stopChan: make(chan struct{}),
	}
}

// SetMessage updates the loader message.
func (l *Loader) SetMessage(message string) {
	l.mu.Lock()
	l.message = message
	l.mu.Unlock()
}

// Start begins the animation.
func (l *Loader) Start() {
	l.mu.Lock()
	if l.active {
		l.mu.Unlock()
		return
	}
	l.active = true
	l.ticker = time.NewTicker(l.interval)
	l.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Don't re-panic in a background goroutine
			}
		}()
		for {
			select {
			case <-l.ticker.C:
				l.mu.Lock()
				l.currentFrame = (l.currentFrame + 1) % len(l.frames)
				cb := l.onRender
				l.mu.Unlock()
				if cb != nil {
					cb()
				}
			case <-l.stopChan:
				return
			}
		}
	}()
}

// Stop stops the animation.
func (l *Loader) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.active {
		l.active = false
		if l.ticker != nil {
			l.ticker.Stop()
		}
		close(l.stopChan)
		l.stopChan = make(chan struct{})
	}
}

// SetOnRender sets a callback that is called when the animation frame changes.
func (l *Loader) SetOnRender(cb func()) {
	l.mu.Lock()
	l.onRender = cb
	l.mu.Unlock()
}

func (l *Loader) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	l.mu.Lock()
	frame := l.frames[l.currentFrame]
	msg := l.message
	l.mu.Unlock()

	line := frame + " " + msg
	if VisibleWidth(line) > width {
		line = SliceByColumn(line, 0, width, true)
	}

	return []string{line}
}

func (l *Loader) HandleInput(data string) {}
func (l *Loader) Invalidate()            {}
func (l *Loader) WantsKeyRelease() bool  { return false }

// CancellableLoader is a loader that can be cancelled by the user.
type CancellableLoader struct {
	*Loader
	cancelled bool
	onCancel  func()
}

// NewCancellableLoader creates a new cancellable loader.
func NewCancellableLoader(message string) *CancellableLoader {
	return &CancellableLoader{
		Loader: NewLoader(message),
	}
}

// SetOnCancel sets the cancel callback.
func (cl *CancellableLoader) SetOnCancel(fn func()) {
	cl.onCancel = fn
}

// IsCancelled returns whether the loader was cancelled.
func (cl *CancellableLoader) IsCancelled() bool {
	return cl.cancelled
}

func (cl *CancellableLoader) HandleInput(data string) {
	if MatchesKey(data, "ctrl+c") || MatchesKey(data, "escape") {
		cl.cancelled = true
		if cl.onCancel != nil {
			cl.onCancel()
		}
	}
}

func (cl *CancellableLoader) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	cl.Loader.mu.Lock()
	frame := cl.frames[cl.currentFrame]
	msg := cl.message
	cl.Loader.mu.Unlock()

	cancelHint := ""
	if !cl.cancelled {
		cancelHint = " [Esc to cancel]"
	}

	line := frame + " " + msg + cancelHint
	if VisibleWidth(line) > width {
		line = SliceByColumn(line, 0, width, true)
	} else if VisibleWidth(line) < width {
		line += strings.Repeat(" ", width-VisibleWidth(line))
	}

	return []string{line}
}

// TruncatedText displays text truncated to a maximum width with an ellipsis.
type TruncatedText struct {
	content string
	maxLen  int
}

// NewTruncatedText creates a new TruncatedText.
func NewTruncatedText(content string, maxLen int) *TruncatedText {
	return &TruncatedText{content: content, maxLen: maxLen}
}

func (t *TruncatedText) Render(width int) []string {
	if width <= 0 {
		return nil
	}
	maxW := width
	if t.maxLen > 0 && t.maxLen < maxW {
		maxW = t.maxLen
	}
	text := t.content
	if VisibleWidth(text) > maxW {
		text = SliceByColumn(text, 0, maxW-1, true) + "…"
	}
	return []string{text}
}

func (t *TruncatedText) HandleInput(data string) {}
func (t *TruncatedText) Invalidate()             {}
func (t *TruncatedText) WantsKeyRelease() bool   { return false }
