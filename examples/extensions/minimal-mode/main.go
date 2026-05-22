// Minimal Mode Extension
//
// Switches the UI to a minimal display mode that only shows essential information.
// Hides tool execution details and focuses on the conversation flow.
//
// Build:  go build -o minimal-mode ./examples/extensions/minimal-mode/
// Deploy: cp minimal-mode ~/.rho/extensions/minimal-mode/
//         cp examples/extensions/minimal-mode/rho.toml ~/.rho/extensions/minimal-mode/
package main

import (
	"fmt"
	"strings"

	"github.com/earendil-works/rho/pkg/sdk"
)

var (
	minimalModeEnabled bool
	showToolOutput     = true
	showThinking       = true
)

func main() {
	ext := sdk.New("rho.minimal-mode")

	ext.Command("minimal", func(ctx sdk.Context, args []string) error {
		enable := !minimalModeEnabled
		if len(args) > 0 {
			switch args[0] {
			case "on", "enable":
				enable = true
			case "off", "disable":
				enable = false
			}
		}

		minimalModeEnabled = enable

		if enable {
			showToolOutput = false
			showThinking = false
			ctx.SetStatus("mode", "🧘 minimal")
			ctx.Notify("🧘 Minimal mode enabled. Tool output and thinking indicators hidden.", "info")
		} else {
			showToolOutput = true
			showThinking = true
			ctx.SetStatus("mode", "")
			ctx.Notify("Standard mode restored.", "info")
		}
		return nil
	})

	ext.Command("focus", func(ctx sdk.Context, args []string) error {
		if len(args) == 0 {
			ctx.Notify("Usage: /focus <on|off>", "info")
			return nil
		}
		switch args[0] {
		case "on":
			minimalModeEnabled = true
			showToolOutput = false
			showThinking = false
			ctx.SetStatus("mode", "🎯 focus")
			ctx.Notify("🎯 Focus mode: only essential information shown.", "info")
		case "off":
			minimalModeEnabled = false
			showToolOutput = true
			showThinking = true
			ctx.SetStatus("mode", "")
			ctx.Notify("Focus mode off. Full display restored.", "info")
		default:
			ctx.Notify("Usage: /focus <on|off>", "info")
		}
		return nil
	})

	ext.Tool("set_display_mode", "Control the UI display mode. "+
		"'minimal' hides tool execution details and thinking indicators for a cleaner view. "+
		"'normal' shows all details.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"mode": map[string]interface{}{
					"type":        "string",
					"description": "Display mode: 'normal' or 'minimal'",
					"enum":        []interface{}{"normal", "minimal"},
				},
				"showToolOutput": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to show tool execution output",
				},
				"showThinking": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to show thinking indicators",
				},
			},
		},
		func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			mode, _ := args["mode"].(string)
			showTools, hasTools := args["showToolOutput"].(bool)
			showThink, hasThink := args["showThinking"].(bool)

			if mode != "" {
				switch mode {
				case "normal":
					minimalModeEnabled = false
					showToolOutput = true
					showThinking = true
					ctx.SetStatus("mode", "")
				case "minimal":
					minimalModeEnabled = true
					showToolOutput = false
					showThinking = false
					ctx.SetStatus("mode", "🧘 minimal")
				}
			}

			if hasTools {
				showToolOutput = showTools
			}
			if hasThink {
				showThinking = showThink
			}

			var status strings.Builder
			status.WriteString("Display mode:\n")
			if minimalModeEnabled {
				status.WriteString("  Mode: 🧘 Minimal\n")
			} else {
				status.WriteString("  Mode: Normal\n")
			}
			status.WriteString(fmt.Sprintf("  Tool output: %v\n", showToolOutput))
			status.WriteString(fmt.Sprintf("  Thinking indicators: %v\n", showThinking))

			return status.String(), false, nil
		},
	)

	ext.Run()
}
