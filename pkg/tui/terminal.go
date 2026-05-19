package tui

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/term"
)

// Terminal defines the interface for terminal I/O operations.
type Terminal interface {
	// Start initializes the terminal with input and resize handlers.
	Start(onInput func(data string), onResize func())

	// Stop restores the terminal to its original state.
	Stop()

	// Write writes data to the terminal.
	Write(data string)

	// Columns returns the terminal width in characters.
	Columns() int

	// Rows returns the terminal height in characters.
	Rows() int

	// KittyProtocolActive returns whether the Kitty keyboard protocol is active.
	KittyProtocolActive() bool

	// MoveBy moves the cursor up (negative) or down (positive) by N lines.
	MoveBy(lines int)

	// HideCursor hides the cursor.
	HideCursor()

	// ShowCursor shows the cursor.
	ShowCursor()

	// ClearLine clears the current line.
	ClearLine()

	// ClearFromCursor clears from cursor to end of screen.
	ClearFromCursor()

	// ClearScreen clears the entire screen and moves cursor to (0,0).
	ClearScreen()

	// SetTitle sets the terminal window title.
	SetTitle(title string)

	// SetProgress displays (true) or clears (false) an indeterminate progress indicator.
	SetProgress(active bool)
}

const (
	terminalProgressKeepaliveMs = 1000
	terminalProgressActiveSeq   = "\x1b]9;4;3\x07"
	terminalProgressClearSeq    = "\x1b]9;4;0;\x07"
	kittyProtocolQueryTimeout   = 150 * time.Millisecond
)

// ProcessTerminal implements Terminal using real stdin/stdout.
type ProcessTerminal struct {
	mu               sync.Mutex
	wasRaw           bool
	inputHandler     func(string)
	resizeHandler    func()
	kittyActive      bool
	modifyOtherKeys  bool
	progressInterval *time.Timer
	progressDone     chan struct{}
	stopChan         chan struct{}
	oldState         *term.State
	writeLogPath     string
	writeLogFile     io.WriteCloser
}

// NewProcessTerminal creates a new ProcessTerminal.
func NewProcessTerminal() *ProcessTerminal {
	return &ProcessTerminal{
		stopChan: make(chan struct{}),
	}
}

func (pt *ProcessTerminal) KittyProtocolActive() bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	return pt.kittyActive
}

func (pt *ProcessTerminal) Start(onInput func(string), onResize func()) {
	pt.mu.Lock()
	pt.inputHandler = onInput
	pt.resizeHandler = onResize
	pt.mu.Unlock()

	// Save terminal state and enable raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err == nil {
		pt.mu.Lock()
		pt.wasRaw = true
		pt.oldState = oldState
		pt.mu.Unlock()
	}

	// Enable bracketed paste mode
	os.Stdout.WriteString("\x1b[?2004h")

	// Set up resize signal handling
	sigwinch := make(chan os.Signal, 1)
	signal.Notify(sigwinch, syscall.SIGWINCH)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Don't re-panic in a background goroutine
			}
		}()
		for {
			select {
			case <-sigwinch:
				pt.mu.Lock()
				rh := pt.resizeHandler
				pt.mu.Unlock()
				if rh != nil {
					rh()
				}
			case <-pt.stopChan:
				signal.Stop(sigwinch)
				return
			}
		}
	}()

	// Trigger initial resize to refresh dimensions
	syscall.Kill(syscall.Getpid(), syscall.SIGWINCH)

	// Query and enable Kitty keyboard protocol
	go pt.queryAndEnableKittyProtocol()

	// Start reading stdin
	go pt.readStdin()
}

func (pt *ProcessTerminal) queryAndEnableKittyProtocol() {
	defer func() {
		if r := recover(); r != nil {
			// Don't re-panic in a background goroutine
		}
	}()
	// Send Kitty protocol query
	os.Stdout.WriteString("\x1b[?u")
	SetKittyProtocolActive(false)

	// Give the terminal time to respond, then fall back to modifyOtherKeys
	time.Sleep(kittyProtocolQueryTimeout)

	pt.mu.Lock()
	if !pt.kittyActive && !pt.modifyOtherKeys {
		pt.modifyOtherKeys = true
		os.Stdout.WriteString("\x1b[>4;2m")
	}
	pt.mu.Unlock()
}

func (pt *ProcessTerminal) readStdin() {
	defer func() {
		if r := recover(); r != nil {
			// Don't re-panic in a background goroutine
		}
	}()
	buf := make([]byte, 4096)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			select {
			case <-pt.stopChan:
				return
			default:
				continue
			}
		}
		if n == 0 {
			continue
		}

		data := string(buf[:n])

		// Check for Kitty protocol response
		if !pt.kittyActive && kittyResponsePattern.MatchString(data) {
			pt.mu.Lock()
			pt.kittyActive = true
			pt.modifyOtherKeys = false
			pt.mu.Unlock()
			SetKittyProtocolActive(true)

			// Enable Kitty keyboard protocol (push flags)
			os.Stdout.WriteString("\x1b[>7u")
			continue
		}

		pt.mu.Lock()
		ih := pt.inputHandler
		pt.mu.Unlock()
		if ih != nil {
			ih(data)
		}
	}
}

func (pt *ProcessTerminal) Stop() {
	close(pt.stopChan)

	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Clear progress indicator
	if pt.progressInterval != nil {
		os.Stdout.WriteString(terminalProgressClearSeq)
		pt.progressInterval.Stop()
		pt.progressInterval = nil
	}

	// Disable bracketed paste mode
	os.Stdout.WriteString("\x1b[?2004l")

	// Disable Kitty protocol
	if pt.kittyActive {
		os.Stdout.WriteString("\x1b[<u")
		pt.kittyActive = false
		SetKittyProtocolActive(false)
	}
	if pt.modifyOtherKeys {
		os.Stdout.WriteString("\x1b[>4;0m")
		pt.modifyOtherKeys = false
	}

	// Restore terminal state
	if pt.oldState != nil {
		term.Restore(int(os.Stdin.Fd()), pt.oldState)
		pt.oldState = nil
	}

	pt.inputHandler = nil
	pt.resizeHandler = nil

	// Close write log
	if pt.writeLogFile != nil {
		pt.writeLogFile.Close()
		pt.writeLogFile = nil
	}
}

func (pt *ProcessTerminal) Write(data string) {
	pt.mu.Lock()
	wlf := pt.writeLogFile
	pt.mu.Unlock()

	os.Stdout.WriteString(data)
	if wlf != nil {
		wlf.Write([]byte(data))
	}
}

func (pt *ProcessTerminal) Columns() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 80
	}
	return width
}

func (pt *ProcessTerminal) Rows() int {
	_, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 24
	}
	return height
}

func (pt *ProcessTerminal) MoveBy(lines int) {
	if lines > 0 {
		os.Stdout.WriteString(fmt.Sprintf("\x1b[%dB", lines))
	} else if lines < 0 {
		os.Stdout.WriteString(fmt.Sprintf("\x1b[%dA", -lines))
	}
}

func (pt *ProcessTerminal) HideCursor() {
	os.Stdout.WriteString("\x1b[?25l")
}

func (pt *ProcessTerminal) ShowCursor() {
	os.Stdout.WriteString("\x1b[?25h")
}

func (pt *ProcessTerminal) ClearLine() {
	os.Stdout.WriteString("\x1b[K")
}

func (pt *ProcessTerminal) ClearFromCursor() {
	os.Stdout.WriteString("\x1b[J")
}

func (pt *ProcessTerminal) ClearScreen() {
	os.Stdout.WriteString("\x1b[2J\x1b[H")
}

func (pt *ProcessTerminal) SetTitle(title string) {
	os.Stdout.WriteString(fmt.Sprintf("\x1b]0;%s\x07", title))
}

func (pt *ProcessTerminal) SetProgress(active bool) {
	if active {
		os.Stdout.WriteString(terminalProgressActiveSeq)
		pt.mu.Lock()
		if pt.progressInterval == nil {
			pt.progressDone = make(chan struct{})
			t := time.NewTimer(terminalProgressKeepaliveMs)
			pt.progressInterval = t
			go func() {
			defer func() {
				if r := recover(); r != nil {
					// Don't re-panic in a background goroutine
				}
			}()
			for {
				select {
				case <-t.C:
					os.Stdout.WriteString(terminalProgressActiveSeq)
					t.Reset(terminalProgressKeepaliveMs)
				case <-pt.progressDone:
					t.Stop()
					return
				}
			}
		}()
		}
		pt.mu.Unlock()
	} else {
		pt.mu.Lock()
		if pt.progressInterval != nil {
			pt.progressInterval.Stop()
			close(pt.progressDone)
			pt.progressInterval = nil
			pt.progressDone = nil
		}
		pt.mu.Unlock()
		os.Stdout.WriteString(terminalProgressClearSeq)
	}
}
