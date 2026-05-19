// Package harnessutils provides truncation utilities for the agent harness.
package harnessutils

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// TruncationResult describes how content was truncated.
type TruncationResult struct {
	Truncated    bool   `json:"truncated"`
	OriginalSize int    `json:"originalSize"`
	FinalSize    int    `json:"finalSize"`
	Message      string `json:"message,omitempty"`
}

// TruncationOptions configures truncation behavior.
type TruncationOptions struct {
	MaxBytes    int  `json:"maxBytes,omitempty"`
	MaxLines    int  `json:"maxLines,omitempty"`
	AddEllipsis bool `json:"addEllipsis,omitempty"`
}

// DefaultTruncationOptions returns sensible defaults.
var DefaultTruncationOptions = TruncationOptions{
	MaxBytes:    50000,
	MaxLines:    2000,
	AddEllipsis: true,
}

// TruncateHead truncates the beginning of content, keeping the tail.
func TruncateHead(content string, opts TruncationOptions) (string, TruncationResult) {
	if opts.MaxBytes <= 0 && opts.MaxLines <= 0 {
		opts = DefaultTruncationOptions
	}

	originalSize := len(content)

	// Check by bytes first
	if opts.MaxBytes > 0 && len(content) > opts.MaxBytes {
		cut := len(content) - opts.MaxBytes
		content = content[cut:]
		// Ensure we don't split a UTF-8 rune
		for len(content) > 0 && !utf8.Valid([]byte{content[0]}) {
			content = content[1:]
		}
		if opts.AddEllipsis {
			content = "..." + content
		}
		return content, TruncationResult{
			Truncated: true, OriginalSize: originalSize,
			FinalSize: len(content),
			Message:   fmt.Sprintf("truncated head (%d -> %d bytes)", originalSize, len(content)),
		}
	}

	// Check by lines
	if opts.MaxLines > 0 {
		lines := strings.Split(content, "\n")
		if len(lines) > opts.MaxLines {
			keepLines := lines[len(lines)-opts.MaxLines:]
			content = strings.Join(keepLines, "\n")
			if opts.AddEllipsis {
				content = "...\n" + content
			}
			return content, TruncationResult{
				Truncated: true, OriginalSize: originalSize,
				FinalSize: len(content),
				Message:   fmt.Sprintf("truncated head (%d -> %d lines)", len(lines), len(keepLines)),
			}
		}
	}

	return content, TruncationResult{Truncated: false, OriginalSize: originalSize, FinalSize: len(content)}
}

// TruncateTail truncates the end of content, keeping the head.
func TruncateTail(content string, opts TruncationOptions) (string, TruncationResult) {
	if opts.MaxBytes <= 0 && opts.MaxLines <= 0 {
		opts = DefaultTruncationOptions
	}

	originalSize := len(content)

	if opts.MaxBytes > 0 && len(content) > opts.MaxBytes {
		content = content[:opts.MaxBytes]
		// Ensure we don't split a UTF-8 rune
		for len(content) > 0 && !utf8.Valid([]byte{content[len(content)-1]}) {
			content = content[:len(content)-1]
		}
		if opts.AddEllipsis {
			content += "..."
		}
		return content, TruncationResult{
			Truncated: true, OriginalSize: originalSize,
			FinalSize: len(content),
			Message:   fmt.Sprintf("truncated tail (%d -> %d bytes)", originalSize, len(content)),
		}
	}

	if opts.MaxLines > 0 {
		lines := strings.Split(content, "\n")
		if len(lines) > opts.MaxLines {
			keepLines := lines[:opts.MaxLines]
			content = strings.Join(keepLines, "\n")
			if opts.AddEllipsis {
				content += "\n..."
			}
			return content, TruncationResult{
				Truncated: true, OriginalSize: originalSize,
				FinalSize: len(content),
				Message:   fmt.Sprintf("truncated tail (%d -> %d lines)", len(lines), len(keepLines)),
			}
		}
	}

	return content, TruncationResult{Truncated: false, OriginalSize: originalSize, FinalSize: len(content)}
}

// TruncateLine truncates a single line to a maximum visual width.
func TruncateLine(line string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	runes := []rune(line)
	if len(runes) > maxWidth {
		return string(runes[:maxWidth]) + "..."
	}
	return line
}

// FormatSize formats a byte count as a human-readable string.
func FormatSize(bytes int) string {
	switch {
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// FormatLineCount formats a line count with truncation info.
func FormatLineCount(lines int, truncated bool) string {
	if truncated {
		return fmt.Sprintf("%d+ lines", lines)
	}
	if lines == 1 {
		return "1 line"
	}
	return fmt.Sprintf("%d lines", lines)
}
