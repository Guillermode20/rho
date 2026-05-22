// Inline Bash Extension
//
// Provides a tool for running inline bash commands with output capture.
// Unlike the regular bash tool, this is designed for quick, non-interactive commands
// where you just need the result.
//
// Build:  go build -o inline-bash ./examples/extensions/inline-bash/
// Deploy: cp inline-bash ~/.rho/extensions/inline-bash/
//         cp examples/extensions/inline-bash/rho.toml ~/.rho/extensions/inline-bash/
package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/earendil-works/rho/pkg/sdk"
)

func main() {
	ext := sdk.New("rho.inline-bash")

	ext.Tool("run", "Run a quick inline command and capture its output. "+
		"Useful for simple operations like listing files, checking versions, or reading short outputs. "+
		"For complex or interactive commands, use the Bash tool instead.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cmd": map[string]interface{}{
					"type":        "string",
					"description": "Command to run",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Brief description of what this command does (for documentation)",
				},
				"timeout": map[string]interface{}{
					"type":        "number",
					"description": "Timeout in seconds (default: 30)",
				},
			},
			"required": []interface{}{"cmd"},
		},
		func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			cmdStr, _ := args["cmd"].(string)
			description, _ := args["description"].(string)
			timeoutSec := 30
			if t, ok := args["timeout"].(float64); ok && t > 0 {
				timeoutSec = int(t)
			}

			if cmdStr == "" {
				return "", true, fmt.Errorf("cmd is required")
			}

			ctx.SetStatus("run", fmt.Sprintf("⚡ running: %s", truncate(cmdStr, 60)))

			ctxGo, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctxGo, "bash", "-c", cmdStr)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()

			ctx.SetStatus("run", "")

			var result strings.Builder
			if description != "" {
				result.WriteString(fmt.Sprintf("$ %s  # %s\n", cmdStr, description))
			} else {
				result.WriteString(fmt.Sprintf("$ %s\n", cmdStr))
			}

			if stdout.Len() > 0 {
				result.WriteString(stdout.String())
			}
			if stderr.Len() > 0 {
				if stdout.Len() > 0 {
					result.WriteString("\n")
				}
				result.WriteString(stderr.String())
			}

			if err != nil {
				if ctxGo.Err() == context.DeadlineExceeded {
					return result.String() + fmt.Sprintf("\n⏱️ Command timed out after %d seconds", timeoutSec), false, nil
				}
				return result.String(), true, fmt.Errorf("exit: %w", err)
			}

			return result.String(), false, nil
		},
	)

	ext.Run()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
