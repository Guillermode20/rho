// Package tui provides a Terminal User Interface library with differential rendering.
//
// Key handling supports both legacy terminal sequences and the Kitty keyboard protocol.
package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Global Kitty protocol state.
var (
	kittyProtocolActive bool
	kittyMu             sync.RWMutex
)

// SetKittyProtocolActive sets the global Kitty keyboard protocol state.
func SetKittyProtocolActive(active bool) {
	kittyMu.Lock()
	kittyProtocolActive = active
	kittyMu.Unlock()
}

// IsKittyProtocolActive returns whether Kitty keyboard protocol is currently active.
func IsKittyProtocolActive() bool {
	kittyMu.RLock()
	defer kittyMu.RUnlock()
	return kittyProtocolActive
}

// KeyID is a typed key identifier like "enter", "ctrl+c", "shift+ctrl+d".
type KeyID string

// KeyEventType describes the type of a key event.
type KeyEventType int

const (
	KeyPress  KeyEventType = iota
	KeyRepeat
	KeyRelease
)

// Key is a helper for constructing typed key identifiers.
var Key = keyHelper{}

type keyHelper struct{}

func (keyHelper) Enter() KeyID           { return "enter" }
func (keyHelper) Tab() KeyID             { return "tab" }
func (keyHelper) Backspace() KeyID       { return "backspace" }
func (keyHelper) Escape() KeyID          { return "esc" }
func (keyHelper) Up() KeyID              { return "up" }
func (keyHelper) Down() KeyID            { return "down" }
func (keyHelper) Left() KeyID            { return "left" }
func (keyHelper) Right() KeyID           { return "right" }
func (keyHelper) Home() KeyID            { return "home" }
func (keyHelper) End() KeyID             { return "end" }
func (keyHelper) PageUp() KeyID          { return "pageup" }
func (keyHelper) PageDown() KeyID        { return "pagedown" }
func (keyHelper) Insert() KeyID          { return "insert" }
func (keyHelper) Delete() KeyID          { return "del" }
func (keyHelper) F(n int) KeyID          { return KeyID(fmt.Sprintf("f%d", n)) }
func (keyHelper) Ctrl(key string) KeyID  { return KeyID("ctrl+" + key) }
func (keyHelper) Shift(key string) KeyID { return KeyID("shift+" + key) }
func (keyHelper) Alt(key string) KeyID   { return KeyID("alt+" + key) }

// kittyResponsePattern matches the Kitty protocol response: ESC [? <flags> u
var kittyResponsePattern = regexp.MustCompile(`^\x1b\[\?(\d+)u$`)

// matchesKeySequence checks if the given data matches the specified key identifier.
func matchesKey(data string, keyID KeyID) bool {
	id := string(keyID)
	parsed := parseKey(data)
	return parsed != "" && parsed == id
}

// MatchesKey reports whether the input data matches the given key identifier.
func MatchesKey(data string, keyID KeyID) bool {
	return matchesKey(data, keyID)
}

// IsKeyRelease returns true if the input represents a key release event (Kitty protocol).
func IsKeyRelease(data string) bool {
	// Kitty protocol release events end with a capital letter suffix
	// e.g., CSI <flags>u (press), CSI <flags>U (release)
	if IsKittyProtocolActive() {
		// Release: ESC [ <digits> ; <digits> U
		kittyReleaseRe := regexp.MustCompile(`^\x1b\[\d+(?:;\d+)*[A-Z]$`)
		return kittyReleaseRe.MatchString(data)
	}
	return false
}

// IsKeyRepeat returns true if the input represents a key repeat event (Kitty protocol).
func IsKeyRepeat(data string) bool {
	if IsKittyProtocolActive() {
		// Repeat: ESC [ <digits> ; <digits> ; 2 u
		if strings.HasSuffix(data, "u") && strings.Count(data, ";") >= 2 {
			parts := strings.Split(strings.TrimRight(data, "u"), ";")
			if len(parts) >= 3 && parts[len(parts)-1] == "2" {
				return true
			}
		}
	}
	return false
}

// parseKey converts a raw terminal input string into a KeyID string.
func parseKey(data string) string {
	if len(data) == 0 {
		return ""
	}

	// Single byte (ASCII)
	if len(data) == 1 {
		b := data[0]
		switch {
		case b == 0x1b:
			return "esc"
		case b == 0x09:
			return "tab"
		case b == 0x0a:
			return "enter"
		case b == 0x7f || b == 0x08:
			return "backspace"
		case b >= 0x20 && b <= 0x7e:
			// Regular printable character
			// Ctrl+letter mappings (0x01-0x1a)
			return string(b)
		case b >= 0x01 && b <= 0x1a:
			// Ctrl+letter: Ctrl+A = 0x01, Ctrl+Z = 0x1a
			return "ctrl+" + string(rune('a'+b-1))
		case b == 0x1c:
			return "ctrl+\\"
		case b == 0x1d:
			return "ctrl+]"
		case b == 0x1e:
			return "ctrl+^"
		case b == 0x1f:
			return "ctrl+/"
		default:
			return fmt.Sprintf("unknown:0x%02x", b)
		}
	}

	// CSI sequences: ESC [ ...
	if data[0] == 0x1b && len(data) > 1 && data[1] == '[' {
		return parseCSISequence(data[2:])
	}

	// SS3 sequences: ESC O ...
	if data[0] == 0x1b && len(data) > 1 && data[1] == 'O' {
		return parseSS3Sequence(data[2:])
	}

	// Kitty protocol sequences
	if IsKittyProtocolActive() && data[0] == 0x1b && len(data) > 1 && data[1] == '[' {
		return parseKittySequence(data[2:])
	}

	return ""
}

// parseCSISequence parses CSI sequences (ESC [ ...).
func parseCSISequence(params string) string {
	if len(params) == 0 {
		return ""
	}

	// Find the terminator byte (last character that is a letter or ~)
	termPos := len(params) - 1
	for ; termPos >= 0; termPos-- {
		c := params[termPos]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '~' {
			break
		}
	}
	if termPos < 0 {
		return ""
	}

	terminator := params[termPos:]
	paramStr := params[:termPos]

	// Parse parameters
	parts := strings.Split(paramStr, ";")
	ints := make([]int, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		if n, err := strconv.Atoi(p); err == nil {
			ints = append(ints, n)
		}
	}

	switch terminator {
	case "A":
		return applyModifiers("up", ints)
	case "B":
		return applyModifiers("down", ints)
	case "C":
		return applyModifiers("right", ints)
	case "D":
		return applyModifiers("left", ints)
	case "H":
		return applyModifiers("home", ints)
	case "F":
		return applyModifiers("end", ints)
	case "Z":
		return "shift+tab"
	case "~":
		return parseTildeCode(ints)
	case "u":
		// Legacy modified key or Kitty protocol
		return parseModifiedKey(ints)
	}

	return ""
}

// parseSS3Sequence parses SS3 sequences (ESC O ...).
func parseSS3Sequence(params string) string {
	if len(params) == 0 {
		return ""
	}

	switch params[0] {
	case 'P':
		return "f1"
	case 'Q':
		return "f2"
	case 'R':
		return "f3"
	case 'S':
		return "f4"
	}

	return ""
}

// parseKittySequence parses Kitty keyboard protocol sequences.
func parseKittySequence(params string) string {
	if len(params) == 0 {
		return ""
	}

	// Kitty format: ESC [ <base> ; <modifiers> ; <event_type> u
	// event_type: 1=press, 2=repeat, 3=release
	// modifiers: 1=shift, 2=alt, 4=ctrl, 8=meta, 16=capslock, etc.
	partStr := params
	if last := len(partStr) - 1; last >= 0 && partStr[last] == 'u' {
		partStr = partStr[:last]
	} else if last >= 0 && partStr[last] == 'U' {
		partStr = partStr[:last]
	}

	parts := strings.Split(partStr, ";")
	ints := make([]int, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		if n, err := strconv.Atoi(p); err == nil {
			ints = append(ints, n)
		}
	}

	if len(ints) == 0 {
		return ""
	}

	baseCode := ints[0]
	modifiers := 0
	if len(ints) >= 2 {
		modifiers = ints[1]
	}

	return kittyCodeToKey(baseCode, modifiers)
}

// kittyCodeToKey converts a Kitty key code and modifier flag to a key identifier.
func kittyCodeToKey(code, modifiers int) string {
	key := kittyBaseKey(code)
	if key == "" {
		return ""
	}
	return applyKittyModifiers(key, modifiers)
}

// kittyBaseKey returns the base key name for a Kitty key code.
func kittyBaseKey(code int) string {
	switch {
	case code >= 32 && code <= 126:
		return string(rune(code))
	case code == 0x09:
		return "tab"
	case code == 0x0a:
		return "enter"
	case code == 0x1b:
		return "esc"
	case code == 0x7f:
		return "backspace"
	case code >= 0x01 && code <= 0x1a:
		return "ctrl+" + string(rune('a'+code-1))

	// Function keys
	case code >= 57377 && code <= 57388:
		return fmt.Sprintf("f%d", code-57376)
	case code == 57399:
		return "insert"
	case code == 57400:
		return "home"
	case code == 57401:
		return "pageup"
	case code == 57402:
		return "del"
	case code == 57403:
		return "end"
	case code == 57404:
		return "pagedown"

	// Cursor keys
	case code == 57405:
		return "up"
	case code == 57406:
		return "down"
	case code == 57407:
		return "right"
	case code == 57408:
		return "left"

	default:
		return ""
	}
}

// applyKittyModifiers applies Kitty modifier flags to a base key name.
func applyKittyModifiers(key string, modifiers int) string {
	hasShift := modifiers&1 != 0
	hasAlt := modifiers&2 != 0
	hasCtrl := modifiers&4 != 0
	hasMeta := modifiers&8 != 0

	prefix := ""
	if hasCtrl {
		prefix = "ctrl+"
	}
	if hasAlt {
		prefix = "alt+" + prefix
	}
	if hasShift && !hasCtrl && !hasAlt {
		// Only shift without other modifiers: shift+<key>
		prefix = "shift+"
	} else if hasShift {
		// Shift with other modifiers
		prefix = "shift+" + prefix
	}
	if hasMeta {
		prefix = "meta+" + prefix
	}

	if prefix != "" {
		return prefix + key
	}
	return key
}

// applyModifiers extracts modifiers from a CSI parameter list and applies them to a key name.
func applyModifiers(key string, params []int) string {
	if len(params) >= 2 {
		mod := params[1]
		var parts []string
		if mod&4 != 0 {
			parts = append(parts, "ctrl")
		}
		if mod&1 != 0 {
			parts = append(parts, "shift")
		}
		if mod&2 != 0 {
			parts = append(parts, "alt")
		}
		if len(parts) > 0 {
			return strings.Join(parts, "+") + "+" + key
		}
	}
	return key
}

// parseTildeCode parses CSI ~ codes (e.g., ESC [ 3 ~ = Delete).
func parseTildeCode(params []int) string {
	if len(params) == 0 {
		return ""
	}

	code := params[0]
	mod := 0
	if len(params) >= 2 {
		mod = params[1]
	}

	key := ""
	switch code {
	case 1:
		key = "home"
	case 2:
		key = "insert"
	case 3:
		key = "del"
	case 4:
		key = "end"
	case 5:
		key = "pageup"
	case 6:
		key = "pagedown"
	case 11:
		key = "f1"
	case 12:
		key = "f2"
	case 13:
		key = "f3"
	case 14:
		key = "f4"
	case 15:
		key = "f5"
	case 17:
		key = "f6"
	case 18:
		key = "f7"
	case 19:
		key = "f8"
	case 20:
		key = "f9"
	case 21:
		key = "f10"
	case 23:
		key = "f11"
	case 24:
		key = "f12"
	default:
		return ""
	}

	return applyModifiers(key, []int{code, mod})
}

// parseModifiedKey parses a CSI u sequence (legacy modified key notation).
// Format: ESC [ <code> ; <modifiers> u
func parseModifiedKey(params []int) string {
	if len(params) == 0 {
		return ""
	}
	code := params[0]
	mod := 0
	if len(params) >= 2 {
		mod = params[1]
	}

	key := ""
	switch {
	case code >= 32 && code <= 126:
		key = string(rune(code))
	case code == 0x7f || code == 0x08:
		key = "backspace"
	case code == 0x09:
		key = "tab"
	case code == 0x0d:
		key = "enter"
	case code == 0x1b:
		key = "esc"
	default:
		return ""
	}

	return applyModifiers(key, []int{code, mod})
}

// ParseKey converts raw terminal input to a key identifier string.
// Returns empty string if the input cannot be parsed.
func ParseKey(data string) string {
	return parseKey(data)
}
