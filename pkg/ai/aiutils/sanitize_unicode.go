package aiutils

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// SanitizeSurrogates removes or replaces surrogate pairs and other invalid
// UTF-8 sequences from the input string. LLM outputs can occasionally contain
// unpaired surrogates that break JSON serialization.
func SanitizeSurrogates(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError {
			if size == 1 {
				// Invalid byte sequence
				b.WriteByte('?')
				i++
				continue
			}
			// Valid but rune error
			i += size
			continue
		}
		// Replace surrogates (U+D800 to U+DFFF)
		if r >= 0xD800 && r <= 0xDFFF {
			b.WriteRune('\uFFFD') // Replacement character
			i += size
			continue
		}
		b.WriteRune(r)
		i += size
	}
	return b.String()
}

// NormalizeUnicode normalizes Unicode to NFC form and sanitizes surrogates.
func NormalizeUnicode(s string) string {
	s = SanitizeSurrogates(s)
	return s
}

// StripControlCharacters removes control characters (except common whitespace).
// Preserves \t, \n, \r.
func StripControlCharacters(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for _, r := range s {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			continue
		}
		if r == 0x7f { // DEL
			continue
		}
		// Keep other characters including Unicode control chars in the 0x80-0x9f range
		b.WriteRune(r)
	}
	return b.String()
}

// ReplaceControlCharacters replaces control characters with their visible representation.
func ReplaceControlCharacters(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteString("\\t")
		case r == '\n':
			b.WriteString("\\n")
		case r == '\r':
			b.WriteString("\\r")
		case r < 0x20:
			b.WriteString(strings.ToLower(fmt.Sprintf("\\x%02x", r)))
		case r == 0x7f:
			b.WriteString("\\x7f")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// IsValidUTF8 checks if the string is valid UTF-8.
func IsValidUTF8(s string) bool {
	return utf8.ValidString(s)
}

// TruncateUTF8 truncates a string to at most maxBytes bytes at a valid UTF-8 boundary.
func TruncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	// Find the last valid rune start within maxBytes
	b := []byte(s)
	for maxBytes > 0 && !utf8.RuneStart(b[maxBytes]) {
		maxBytes--
	}
	return string(b[:maxBytes])
}

// SafeString ensures a string is safe for JSON/LLM output by sanitizing surrogates
// and stripping control characters.
func SafeString(s string) string {
	s = SanitizeSurrogates(s)
	s = StripControlCharacters(s)
	return s
}
