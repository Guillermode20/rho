package tui

import (
	"strings"
	"sync"
	"time"
)

// StdinBufferEvent represents events emitted by the StdinBuffer.
type StdinBufferEvent interface {
	isStdinBufferEvent()
}

// DataEvent is emitted when a complete input sequence is parsed.
type DataEvent struct {
	Data string
}

func (e DataEvent) isStdinBufferEvent() {}

// PasteEvent is emitted when pasted content is detected.
type PasteEvent struct {
	Content string
}

func (e PasteEvent) isStdinBufferEvent() {}

// StdinBuffer buffers raw stdin input and splits it into individual sequences.
// It handles batched input and paste detection via bracketed paste mode markers.
type StdinBuffer struct {
	mu             sync.Mutex
	buffer         string
	timeout        time.Duration
	inPaste        bool
	pasteBuffer    strings.Builder
	onData         func(string)
	onPaste        func(string)
	timer          *time.Timer
	done           chan struct{}
}

// NewStdinBuffer creates a new stdin buffer with the given timeout for sequence assembly.
func NewStdinBuffer(timeout time.Duration) *StdinBuffer {
	return &StdinBuffer{
		timeout: timeout,
		done:    make(chan struct{}),
	}
}

// OnData registers a callback for individual input sequences.
func (sb *StdinBuffer) OnData(handler func(data string)) {
	sb.mu.Lock()
	sb.onData = handler
	sb.mu.Unlock()
}

// OnPaste registers a callback for pasted content.
func (sb *StdinBuffer) OnPaste(handler func(content string)) {
	sb.mu.Lock()
	sb.onPaste = handler
	sb.mu.Unlock()
}

// Process feeds raw data into the buffer for parsing.
func (sb *StdinBuffer) Process(data string) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	// Handle paste start/end markers
	if strings.Contains(data, "\x1b[200~") {
		// Paste start - begin collecting paste content
		parts := strings.SplitN(data, "\x1b[200~", 2)
		if len(parts) > 0 && parts[0] != "" {
			sb.flushPending(parts[0])
		}
		sb.inPaste = true
		sb.pasteBuffer.Reset()
		if len(parts) > 1 && parts[1] != "" {
			sb.handlePasteContent(parts[1])
		}
		return
	}

	if sb.inPaste {
		if strings.Contains(data, "\x1b[201~") {
			// Paste end
			parts := strings.SplitN(data, "\x1b[201~", 2)
			if len(parts) > 0 && parts[0] != "" {
				sb.pasteBuffer.WriteString(parts[0])
			}
			content := sb.pasteBuffer.String()
			sb.inPaste = false
			if sb.onPaste != nil {
				sb.onPaste(content)
			}
			if len(parts) > 1 && parts[1] != "" {
				sb.flushPending(parts[1])
			}
		} else {
			sb.handlePasteContent(data)
		}
		return
	}

	// Buffer the data and try to parse sequences
	sb.buffer += data
	sb.tryFlushSequences()
}

func (sb *StdinBuffer) handlePasteContent(data string) {
	sb.pasteBuffer.WriteString(data)
}

// tryFlushSequences attempts to extract complete sequences from the buffer.
func (sb *StdinBuffer) tryFlushSequences() {
	// If we have a complete escape sequence, flush it
	if sb.buffer == "" {
		return
	}

	// Check for complete CSI sequences: ESC [ <params> <final byte>
	if strings.HasPrefix(sb.buffer, "\x1b") {
		// Need at least ESC + one more byte
		if len(sb.buffer) >= 2 {
			if sb.buffer[1] == '[' {
				// CSI sequence: find the terminator (letter or ~)
				for i := 2; i < len(sb.buffer); i++ {
					c := sb.buffer[i]
					if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '~' {
						// Found complete CSI sequence
						seq := sb.buffer[:i+1]
						sb.buffer = sb.buffer[i+1:]
						if sb.onData != nil {
							sb.onData(seq)
						}
						// Continue parsing remaining buffer
						sb.tryFlushSequences()
						return
					}
				}
				// Incomplete CSI sequence - wait for more data
				sb.startTimer()
				return
			} else if sb.buffer[1] == 'O' {
				// SS3 sequence: ESC O <byte>
				if len(sb.buffer) >= 3 {
					seq := sb.buffer[:3]
					sb.buffer = sb.buffer[3:]
					if sb.onData != nil {
						sb.onData(seq)
					}
					sb.tryFlushSequences()
					return
				}
				// Incomplete SS3
				sb.startTimer()
				return
			}
			// Unknown escape sequence, flush single byte
			seq := sb.buffer[:1]
			sb.buffer = sb.buffer[1:]
			if sb.onData != nil {
				sb.onData(seq)
			}
			sb.tryFlushSequences()
			return
		}
		// Just ESC, wait for more
		sb.startTimer()
		return
	}

	// Printable ASCII - flush character by character (common for typing)
	for len(sb.buffer) > 0 {
		// Check if we have an escape sequence starting
		if sb.buffer[0] == '\x1b' {
			sb.tryFlushSequences()
			return
		}
		ch := sb.buffer[:1]
		sb.buffer = sb.buffer[1:]
		if sb.onData != nil {
			sb.onData(ch)
		}
	}
}

func (sb *StdinBuffer) flushPending(data string) {
	if sb.onData != nil {
		sb.onData(data)
	}
}

func (sb *StdinBuffer) startTimer() {
	if sb.timer != nil {
		sb.timer.Stop()
	}
	sb.timer = time.AfterFunc(sb.timeout, func() {
		sb.mu.Lock()
		if sb.buffer != "" {
			data := sb.buffer
			sb.buffer = ""
			if sb.onData != nil {
				sb.onData(data)
			}
		}
		sb.mu.Unlock()
	})
}

// Destroy cleans up the buffer's resources.
func (sb *StdinBuffer) Destroy() {
	sb.mu.Lock()
	if sb.timer != nil {
		sb.timer.Stop()
	}
	sb.onData = nil
	sb.onPaste = nil
	sb.mu.Unlock()
}
