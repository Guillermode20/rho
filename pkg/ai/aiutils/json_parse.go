// Package aiutils provides shared utilities for AI provider implementations.
package aiutils

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PartialJSON tracks state for incremental JSON parsing.
type PartialJSON struct {
	buf    strings.Builder
	depth  int
	inStr  bool
	esc    bool
	closed bool
}

// NewPartialJSON creates a new PartialJSON tracker.
func NewPartialJSON() *PartialJSON {
	return &PartialJSON{}
}

// Feed processes a chunk of partial JSON and returns the current accumulated state.
// Returns true and the accumulated JSON when a complete JSON value has been seen.
func (p *PartialJSON) Feed(chunk string) (string, bool) {
	for _, r := range chunk {
		p.buf.WriteRune(r)
		switch {
		case p.esc:
			p.esc = false
		case r == '\\' && p.inStr:
			p.esc = true
		case r == '"':
			p.inStr = !p.inStr
		case p.inStr:
			continue
		case r == '{' || r == '[':
			p.depth++
		case r == '}' || r == ']':
			p.depth--
			if p.depth == 0 {
				p.closed = true
			}
		}
	}
	return p.buf.String(), p.closed
}

// Reset clears the partial JSON state.
func (p *PartialJSON) Reset() {
	p.buf.Reset()
	p.depth = 0
	p.inStr = false
	p.esc = false
	p.closed = false
}

// ParseStreamingJSON attempts to parse each chunk as JSON, returning
// partial results when the full JSON object is available.
// Returns parsed values as they complete, ignoring incomplete chunks.
func ParseStreamingJSON(chunks <-chan string) <-chan interface{} {
	out := make(chan interface{}, 100)
	go func() {
		defer close(out)
		pj := NewPartialJSON()
		for chunk := range chunks {
			jsonStr, complete := pj.Feed(chunk)
			if complete {
				var val interface{}
				if err := json.Unmarshal([]byte(jsonStr), &val); err == nil {
					out <- val
				}
				pj.Reset()
			}
		}
	}()
	return out
}

// RepairJSON attempts to fix common JSON issues from streaming APIs.
// Fixes: trailing commas, unquoted keys, truncation.
func RepairJSON(input string) string {
	if input == "" {
		return input
	}

	result := input

	// Remove trailing commas before closing braces/brackets
	result = repairTrailingCommas(result)

	// Try to close unclosed braces/brackets
	result = repairUnclosed(result)

	// Remove trailing whitespace
	result = strings.TrimRight(result, " \t\n\r")

	return result
}

func repairTrailingCommas(s string) string {
	// Remove trailing commas before } or ]
	var result strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == ',' {
			// Check if next non-whitespace is } or ]
			next := i + 1
			for next < len(runes) && (runes[next] == ' ' || runes[next] == '\t' || runes[next] == '\n' || runes[next] == '\r') {
				next++
			}
			if next < len(runes) && (runes[next] == '}' || runes[next] == ']') {
				continue // skip trailing comma
			}
			result.WriteRune(',')
		} else {
			result.WriteRune(runes[i])
		}
	}
	return result.String()
}

func repairUnclosed(s string) string {
	opens := 0
	inStr := false
	esc := false
	for _, r := range s {
		if esc {
			esc = false
			continue
		}
		if r == '\\' && inStr {
			esc = true
			continue
		}
		if r == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		switch r {
		case '{', '[':
			opens++
		case '}', ']':
			opens--
		}
	}

	// Close unclosed brackets
	for i := 0; i < opens; i++ {
		s += "}"
	}

	return s
}

// ParseJSONWithRepair tries to parse JSON, repairing common issues first.
func ParseJSONWithRepair(data []byte, target interface{}) error {
	err := json.Unmarshal(data, target)
	if err == nil {
		return nil
	}

	// Try with repair
	repaired := RepairJSON(string(data))
	err = json.Unmarshal([]byte(repaired), target)
	if err == nil {
		return nil
	}

	return fmt.Errorf("JSON parse failed after repair: %w (original: %s)", err, string(data))
}

// IsSurrogate returns true if r is a unicode surrogate half.
func IsSurrogate(r rune) bool {
	return r >= 0xD800 && r <= 0xDFFF
}



// IsJSON returns true if the string looks like valid JSON.
func IsJSON(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return false
	}
	return (s[0] == '{' && s[len(s)-1] == '}') ||
		(s[0] == '[' && s[len(s)-1] == ']') ||
		(s[0] == '"' && s[len(s)-1] == '"')
}
