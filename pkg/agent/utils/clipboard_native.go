package agentutils

import (
	"fmt"
	"os"
	"runtime"
)

// NativeClipboard provides native clipboard operations.
type NativeClipboard struct{}

// WriteOSC52 writes text to the clipboard using the OSC 52 terminal escape sequence.
// Supported by: iTerm2, kitty, tmux (with clipboard setting), Windows Terminal, etc.
func (nc *NativeClipboard) WriteOSC52(text string) error {
	// OSC 52 format: ESC ] 52 ; c ; <base64> ST (ESC \) or BEL
	encoded := encodeBase64([]byte(text))
	osc52 := fmt.Sprintf("\x1b]52;c;%s\x07", encoded)
	if _, err := os.Stdout.WriteString(osc52); err != nil {
		return fmt.Errorf("OSC 52 write failed: %w", err)
	}
	return nil
}

// WriteOSC52WithSelection writes to a specific clipboard selection.
// Selection: "c" = clipboard, "p" = primary (X11), "s" = secondary (X11)
func (nc *NativeClipboard) WriteOSC52WithSelection(text, selection string) error {
	encoded := encodeBase64([]byte(text))
	osc52 := fmt.Sprintf("\x1b]52;%s;%s\x07", selection, encoded)
	if _, err := os.Stdout.WriteString(osc52); err != nil {
		return fmt.Errorf("OSC 52 write failed: %w", err)
	}
	return nil
}

// SupportsOSC52 reports whether the current terminal likely supports OSC 52.
func (nc *NativeClipboard) SupportsOSC52() bool {
	term := os.Getenv("TERM")
	switch {
	case term == "xterm-kitty":
		return true
	case os.Getenv("TMUX") != "":
		return true
	case os.Getenv("ITERM_PROFILE") != "":
		return true
	case os.Getenv("WT_SESSION") != "":
		return true
	case os.Getenv("TERMINAL_EMULATOR") == "JetBrains-JediTerm":
		return true
	case runtime.GOOS == "darwin":
		return true
	}
	return false
}

// ClipboardFile provides file-based clipboard fallback using a temp file.
type ClipboardFile struct {
	Path string
}

// NewClipboardFile creates a file-based clipboard.
func NewClipboardFile(path string) *ClipboardFile {
	if path == "" {
		path = os.TempDir() + "/rho-clipboard"
	}
	return &ClipboardFile{Path: path}
}

// Write writes text to the clipboard file.
func (cf *ClipboardFile) Write(text string) error {
	return os.WriteFile(cf.Path, []byte(text), 0644)
}

// Read reads text from the clipboard file.
func (cf *ClipboardFile) Read() (string, error) {
	data, err := os.ReadFile(cf.Path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
