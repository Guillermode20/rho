package agentutils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Clipboard provides read/write access to system clipboard.
type Clipboard struct {
	Primary  func() (string, error)
	Write    func(text string) error
	fallback string
}

// DefaultClipboard returns the platform-appropriate clipboard handler.
func DefaultClipboard() *Clipboard {
	return &Clipboard{
		Primary:  readClipboard,
		Write:    writeClipboard,
		fallback: filepath.Join(os.TempDir(), "rho-clipboard"),
	}
}

// readClipboard reads text from the system clipboard.
func readClipboard() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return execClipboard("pbpaste")
	case "linux":
		// Try xclip first, then wl-paste (Wayland)
		if hasBinary("xclip") {
			return execClipboard("xclip", "-o", "-selection", "clipboard")
		}
		if hasBinary("wl-paste") {
			return execClipboard("wl-paste")
		}
		// Try OSC 52 escape sequence read
		return readClipboardOSC52()
	default:
		// Windows
		if hasBinary("powershell") {
			return execClipboard("powershell", "-command", "Get-Clipboard")
		}
	}
	return "", fmt.Errorf("no clipboard tool available")
}

// writeClipboard writes text to the system clipboard.
func writeClipboard(text string) error {
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	case "linux":
		if hasBinary("xclip") {
			cmd := exec.Command("xclip", "-i", "-selection", "clipboard")
			cmd.Stdin = strings.NewReader(text)
			return cmd.Run()
		}
		if hasBinary("wl-copy") {
			cmd := exec.Command("wl-copy")
			cmd.Stdin = strings.NewReader(text)
			return cmd.Run()
		}
		// Fallback to OSC 52
		return writeClipboardOSC52(text)
	default:
		if hasBinary("powershell") {
			cmd := exec.Command("powershell", "-command", "Set-Clipboard")
			cmd.Stdin = strings.NewReader(text)
			return cmd.Run()
		}
	}
	return fmt.Errorf("no clipboard tool available")
}

func execClipboard(args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n\r"), nil
}

func hasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// readClipboardOSC52 reads clipboard via OSC 52 escape (terminal-dependent).
func readClipboardOSC52() (string, error) {
	return "", fmt.Errorf("OSC 52 clipboard read not supported")
}

// writeClipboardOSC52 writes clipboard via OSC 52 escape sequence (terminal-dependent).
func writeClipboardOSC52(text string) error {
	// OSC 52 escape: ESC ] 52 ; [selection] ; <base64> BEL
	// Not all terminals support writing via OSC 52
	_, err := os.Stdout.WriteString(fmt.Sprintf("\x1b]52;c;%s\x07", encodeBase64([]byte(text))))
	return err
}

func encodeBase64(data []byte) string {
	const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var result strings.Builder
	for i := 0; i < len(data); i += 3 {
		var buf [3]byte
		var n int
		for j := 0; j < 3 && i+j < len(data); j++ {
			buf[j] = data[i+j]
			n++
		}
		// Encode to base64
		result.WriteByte(base64Chars[buf[0]>>2])
		result.WriteByte(base64Chars[((buf[0]&0x03)<<4)|(buf[1]>>4)])
		if n > 1 {
			result.WriteByte(base64Chars[((buf[1]&0x0f)<<2)|(buf[2]>>6)])
		} else {
			result.WriteByte('=')
		}
		if n > 2 {
			result.WriteByte(base64Chars[buf[2]&0x3f])
		} else {
			result.WriteByte('=')
		}
	}
	return result.String()
}
