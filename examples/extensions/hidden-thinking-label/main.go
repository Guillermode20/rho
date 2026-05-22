// Hidden Thinking Label Extension
//
// When the model is thinking (reasoning), this extension shows a visible
// "Thinking..." indicator in the UI so the user knows the agent is still working.
//
// Build:  go build -o hidden-thinking-label ./examples/extensions/hidden-thinking-label/
// Deploy: cp hidden-thinking-label ~/.rho/extensions/hidden-thinking-label/
//
//	cp examples/extensions/hidden-thinking-label/rho.toml ~/.rho/extensions/hidden-thinking-label/
package main

import (
	"fmt"
	"time"

	"github.com/earendil-works/rho/pkg/sdk"
)

func main() {
	ext := sdk.New("rho.hidden-thinking-label")

	ext.Command("thinking-status", func(ctx sdk.Context, args []string) error {
		if len(args) == 0 {
			ctx.SetStatus("thinking", "🧠")
			ctx.Notify("Thinking status indicator toggled", "info")
			return nil
		}
		status := args[0]
		switch status {
		case "on", "show", "visible":
			ctx.SetStatus("thinking", "🧠 Thinking...")
			ctx.Notify("Thinking indicator shown", "info")
		case "off", "hide":
			ctx.SetStatus("thinking", "")
			ctx.Notify("Thinking indicator hidden", "info")
		default:
			ctx.Notify(fmt.Sprintf("Usage: /thinking-status [on|off]"), "info")
		}
		return nil
	})

	ext.Tool("show_thinking", "Display a visible thinking/running indicator in the UI status. "+
		"Useful when the model has internal reasoning that takes time and you want to reassure the user.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"label": map[string]interface{}{
					"type":        "string",
					"description": "Custom label to show (default: 'Thinking...')",
				},
				"showSpinner": map[string]interface{}{
					"type":        "boolean",
					"description": "Show an animated spinner (default: true)",
				},
			},
		},
		func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			label, _ := args["label"].(string)
			showSpinner := true
			if s, ok := args["showSpinner"].(bool); ok {
				showSpinner = s
			}

			if label == "" {
				label = "Thinking..."
			}

			prefix := ""
			if showSpinner {
				prefix = "🧠 "
			}

			ctx.SetStatus("thinking", prefix+label)

			// Auto-clear after 30 seconds if not explicitly cleared
			go func() {
				time.Sleep(30 * time.Second)
				ctx.SetStatus("thinking", "")
			}()

			return fmt.Sprintf("Thinking indicator shown: %s%s", prefix, label), false, nil
		},
	)

	ext.Tool("clear_thinking", "Clear the thinking/running indicator from the UI status.",
		map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			ctx.SetStatus("thinking", "")
			return "Thinking indicator cleared.", false, nil
		},
	)

	ext.Run()
}
