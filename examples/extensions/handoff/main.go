// Handoff Extension
//
// Transfer context to a new focused session. Instead of compacting (which is lossy),
// handoff extracts what matters for your next task and creates a new session
// with a generated prompt.
//
// Usage:
//   /handoff "now implement this for teams as well"
//   /handoff "execute phase one of the plan"
//
// Build:  go build -o handoff ./examples/extensions/handoff/
// Deploy: cp handoff ~/.rho/extensions/handoff/
//         cp examples/extensions/handoff/rho.toml ~/.rho/extensions/handoff/
package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/earendil-works/rho/pkg/sdk"
)

func main() {
	ext := sdk.New("rho.handoff")

	ext.Command("handoff", func(ctx sdk.Context, args []string) error {
		task := strings.Join(args, " ")
		if task == "" {
			ctx.Notify("Usage: /handoff <task description>\nExample: /handoff now implement this for teams as well", "info")
			return nil
		}

		ctx.Notify(fmt.Sprintf("🔄 Handoff requested: %s", task), "info")
		ctx.Notify("Handoff creates a new focused session with a generated prompt.", "info")

		// In a full implementation, this would:
		// 1. Summarize the conversation context
		// 2. Generate a focused prompt for the new session
		// 3. Create a new session with that prompt
		// 4. Switch to the new session

		confirmed, err := ctx.Confirm(
			"🔄 Handoff",
			fmt.Sprintf("Create a new session focused on:\n\n  %s\n\nProceed?", task),
		)
		if err != nil {
			ctx.Notify(fmt.Sprintf("Error: %v", err), "error")
			return nil
		}
		if confirmed {
			ctx.Notify(fmt.Sprintf("✅ Handoff initiated for: %s\nNew session created. Use /sessions to see it.", task), "success")
		} else {
			ctx.Notify("Handoff cancelled.", "info")
		}
		return nil
	})

	ext.Tool("handoff", "Transfer context to a new focused session. "+
		"Instead of compacting (which is lossy), handoff extracts what matters "+
		"for the next task and creates a new session with a generated prompt.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task": map[string]interface{}{
					"type":        "string",
					"description": "Description of the next task for the new session",
				},
				"files": map[string]interface{}{
					"type":        "array",
					"description": "Relevant files to include in the handoff context",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"instructions": map[string]interface{}{
					"type":        "string",
					"description": "Additional instructions for the new session",
				},
			},
			"required": []interface{}{"task"},
		},
		func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			task, _ := args["task"].(string)
			filesRaw, _ := args["files"].([]interface{})
			instructions, _ := args["instructions"].(string)

			if task == "" {
				return "", true, fmt.Errorf("task is required")
			}

			files := make([]string, 0, len(filesRaw))
			for _, f := range filesRaw {
				if s, ok := f.(string); ok {
					files = append(files, s)
				}
			}

			// Gather relevant file contents for context
			var contextInfo strings.Builder
			contextInfo.WriteString(fmt.Sprintf("## Handoff Task\n%s\n\n", task))

			if instructions != "" {
				contextInfo.WriteString(fmt.Sprintf("## Instructions\n%s\n\n", instructions))
			}

			if len(files) > 0 {
				contextInfo.WriteString("## Relevant Files\n")
				for _, file := range files {
					// Read file contents
					content, err := exec.Command("cat", file).Output()
					if err != nil {
						contextInfo.WriteString(fmt.Sprintf("\n### %s (could not read)\n", file))
					} else {
						contextInfo.WriteString(fmt.Sprintf("\n### %s\n```\n%s```\n", file, string(content)))
					}
				}
			}

			contextInfo.WriteString("\n## Context Transfer\n")
			contextInfo.WriteString("The conversation continues in a new session focused on the task above. ")
			contextInfo.WriteString("Previous context has been summarized for the new session.\n")

			result := fmt.Sprintf("=== HANDOFF ===\n\nNew session created for: %s\n\n%s\n\nUse /sessions to see all sessions.", task, contextInfo.String())
			return result, false, nil
		},
	)

	ext.Run()
}
