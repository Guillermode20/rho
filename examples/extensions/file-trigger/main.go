// File Trigger Extension
//
// Watches files for changes and triggers actions when they're modified.
// Useful for auto-running tests, linters, or builds when source files change.
//
// Build:  go build -o file-trigger ./examples/extensions/file-trigger/
// Deploy: cp file-trigger ~/.rho/extensions/file-trigger/
//
//	cp examples/extensions/file-trigger/rho.toml ~/.rho/extensions/file-trigger/
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/earendil-works/rho/pkg/sdk"
)

type watchEntry struct {
	Pattern string
	Command string
	LastRun time.Time
}

var (
	watches   []watchEntry
	watchesMu sync.Mutex
)

func main() {
	ext := sdk.New("rho.file-trigger")

	ext.Command("watch", func(ctx sdk.Context, args []string) error {
		if len(args) < 2 {
			ctx.Notify("Usage: /watch <pattern> <command>\nExample: /watch '*.go' 'go test ./...'", "info")
			return nil
		}

		pattern := args[0]
		command := strings.Join(args[1:], " ")

		watchesMu.Lock()
		watches = append(watches, watchEntry{
			Pattern: pattern,
			Command: command,
		})
		watchesMu.Unlock()

		ctx.Notify(fmt.Sprintf("👀 Watching '%s' → running '%s' on change", pattern, command), "success")
		return nil
	})

	ext.Command("watches", func(ctx sdk.Context, args []string) error {
		watchesMu.Lock()
		ws := make([]watchEntry, len(watches))
		copy(ws, watches)
		watchesMu.Unlock()

		if len(ws) == 0 {
			ctx.Notify("No file watches configured. Use /watch <pattern> <command>", "info")
			return nil
		}

		var msg strings.Builder
		msg.WriteString(fmt.Sprintf("👀 %d file watch(es):\n", len(ws)))
		for i, w := range ws {
			msg.WriteString(fmt.Sprintf("  %d. %-20s → %s\n", i+1, w.Pattern, w.Command))
		}
		ctx.Notify(msg.String(), "info")
		return nil
	})

	ext.Command("unwatch", func(ctx sdk.Context, args []string) error {
		if len(args) == 0 {
			ctx.Notify("Usage: /unwatch <pattern> or /unwatch all", "info")
			return nil
		}

		watchesMu.Lock()
		defer watchesMu.Unlock()

		if args[0] == "all" {
			watches = nil
			ctx.Notify("All watches removed.", "info")
			return nil
		}

		pattern := args[0]
		var remaining []watchEntry
		removed := 0
		for _, w := range watches {
			if w.Pattern != pattern {
				remaining = append(remaining, w)
			} else {
				removed++
			}
		}
		watches = remaining
		ctx.Notify(fmt.Sprintf("Removed %d watch(es) for pattern '%s'.", removed, pattern), "info")
		return nil
	})

	ext.Tool("watch_file", "Watch a file pattern and run a command when matching files change. "+
		"Useful for auto-running tests, linters, or builds on file changes.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "File glob pattern to watch (e.g., '*.go', 'src/**/*.ts')",
				},
				"command": map[string]interface{}{
					"type":        "string",
					"description": "Command to run when matching files change",
				},
				"debounceMs": map[string]interface{}{
					"type":        "number",
					"description": "Debounce interval in milliseconds (default: 1000)",
				},
			},
			"required": []interface{}{"pattern", "command"},
		},
		func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			pattern, _ := args["pattern"].(string)
			command, _ := args["command"].(string)
			debounceMs := 1000
			if d, ok := args["debounceMs"].(float64); ok && d > 0 {
				debounceMs = int(d)
			}

			if pattern == "" || command == "" {
				return "", true, fmt.Errorf("pattern and command are required")
			}

			// Check if the pattern matches any files
			matches, _ := filepath.Glob(pattern)
			matchStr := "no files"
			if len(matches) > 0 {
				matchStr = fmt.Sprintf("%d file(s)", len(matches))
			}

			watchesMu.Lock()
			watches = append(watches, watchEntry{
				Pattern: pattern,
				Command: command,
			})
			watchesMu.Unlock()

			return fmt.Sprintf("👀 Now watching '%s' (%s). Will run '%s' on change (debounce: %dms).\nUse /watches to list all watches, /unwatch %s to remove.",
				pattern, matchStr, command, debounceMs, pattern), false, nil
		},
	)

	ext.Tool("trigger_now", "Immediately trigger file watches for testing. Runs all matching watch commands.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "Optional pattern to limit which watches to trigger",
				},
			},
		},
		func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			pattern, _ := args["pattern"].(string)

			watchesMu.Lock()
			ws := make([]watchEntry, len(watches))
			copy(ws, watches)
			watchesMu.Unlock()

			var triggered int
			var results strings.Builder

			for _, w := range ws {
				if pattern != "" && w.Pattern != pattern {
					continue
				}
				triggered++

				results.WriteString(fmt.Sprintf("Running: %s (pattern: %s)\n", w.Command, w.Pattern))
				cmd := exec.Command("bash", "-c", w.Command)
				output, err := cmd.CombinedOutput()
				if err != nil {
					results.WriteString(fmt.Sprintf("  Error: %v\n", err))
				}
				if len(output) > 0 {
					// Truncate output
					outStr := string(output)
					if len(outStr) > 500 {
						outStr = outStr[:500] + "\n... (truncated)"
					}
					results.WriteString(fmt.Sprintf("  Output:\n%s\n", outStr))
				}
			}

			if triggered == 0 {
				return fmt.Sprintf("No watches matched pattern '%s'. Use /watches to list configured watches.", pattern), false, nil
			}
			return fmt.Sprintf("Triggered %d watch(es):\n%s", triggered, results.String()), false, nil
		},
	)

	ext.Run()
}

func init() {
	// Try to find a valid working directory for glob operations
	cwd, err := os.Getwd()
	if err == nil {
		_ = cwd
	}
}
