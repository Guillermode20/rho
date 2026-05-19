// Package exampleext demonstrates how to create a rho extension.
//
// Extensions are Go packages that register with the extension runtime.
// They can provide custom tools, slash commands, AI providers, and
// subscribe to lifecycle events.
//
// Build as a plugin: go build -buildmode=plugin -o example.so .
// Then place in ~/.rho/extensions/ or load via config.
package exampleext

import (
	"fmt"
	"os"
	"strings"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/agent/extensions"
	"github.com/earendil-works/rho/pkg/ai"
)

// RegisterExtension is the entry point called by the extension loader.
func RegisterExtension(runtime *extensions.Runtime) {
	runtime.Register(createExampleExtension())
}

func createExampleExtension() *extensions.ExtensionDef {
	return &extensions.ExtensionDef{
		Name:        "example",
		Description: "Example extension demonstrating the rho extension API",
		Version:     "1.0.0",

		// ====================================================================
		// Custom Tools
		// ====================================================================
		CustomTools: []extensions.ToolDefinition{
			{
				Name:        "Weather",
				Label:       "Weather",
				Description: "Get the current weather for a city (demo tool)",
				PromptSnippet: "Get weather information for any city",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"city": map[string]interface{}{
							"type":        "string",
							"description": "The city name",
						},
					},
					"required": []interface{}{"city"},
				},
				Execute: func(args map[string]interface{}) (string, bool, error) {
					city, _ := args["city"].(string)
					if city == "" {
						return "", true, fmt.Errorf("city is required")
					}
					// Demo: return simulated weather
					return fmt.Sprintf("Weather in %s:\n  Temperature: 22°C\n  Conditions: Partly cloudy\n  Humidity: 45%%\n  Wind: 12 km/h", city), false, nil
				},
			},
			{
				Name:        "ListEnv",
				Label:       "List Environment",
				Description: "List environment variables matching a prefix (demo tool)",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"prefix": map[string]interface{}{
							"type":        "string",
							"description": "Prefix to filter environment variables (e.g., 'PATH', 'HOME')",
						},
					},
				},
				Execute: func(args map[string]interface{}) (string, bool, error) {
					prefix, _ := args["prefix"].(string)
					env := os.Environ()
					var matched []string
					for _, e := range env {
						if prefix == "" || strings.HasPrefix(e, prefix) {
							matched = append(matched, e)
						}
					}
					if len(matched) == 0 {
						return fmt.Sprintf("No environment variables matching prefix %q", prefix), false, nil
					}
					return strings.Join(matched, "\n"), false, nil
				},
			},
		},

		// ====================================================================
		// Slash Commands
		// ====================================================================
		SlashCommands: []extensions.SlashCommand{
			{
				Name:        "example",
				Description: "Show example extension info",
				Handler: func(ctx extensions.ExtensionContext, args []string) error {
					ctx.UI.Notify("Example extension v1.0.0 loaded!", "info")
					fmt.Fprintf(os.Stderr, "Example extension: CWD=%s, tools=2, commands=1\n", ctx.CWD)
					return nil
				},
			},
			{
				Name:        "echo",
				Description: "Echo text back (args: <text>)",
				Args:        []string{"text"},
				Handler: func(ctx extensions.ExtensionContext, args []string) error {
					text := strings.Join(args, " ")
					if text == "" {
						text = "Hello from rho extension!"
					}
					ctx.UI.Notify("Echo: "+text, "info")
					return nil
				},
			},
		},

		// ====================================================================
		// Custom AI Provider (e.g., a local model)
		// ====================================================================
		CustomProviders: []extensions.ProviderConfig{
			{
				Name:    "local-llm",
				API:     ai.APIOpenAICompletions,
				BaseURL: "http://localhost:8080",
				Models: []ai.Model{
					{
						API:      ai.APIOpenAICompletions,
						Provider: ai.Provider("local-llm"),
						Name:     "local-model",
						BaseURL:  "http://localhost:8080",
					},
				},
			},
		},

		// ====================================================================
		// Event Handlers
		// ====================================================================

		// OnAgentStart: Fired when the agent begins processing.
		OnAgentStart: func(ctx extensions.ExtensionContext) error {
			ctx.UI.SetStatus("example", "active")
			return nil
		},

		// OnAgentEnd: Fired when the agent finishes.
		OnAgentEnd: func(ctx extensions.ExtensionContext, event extensions.AgentEndEvent) error {
			ctx.UI.SetStatus("example", "idle")
			return nil
		},

		// OnTurnStart: Fired at the start of each agent turn.
		OnTurnStart: func(ctx extensions.ExtensionContext, event extensions.TurnStartEvent) error {
			_ = event.TurnIndex
			return nil
		},

		// OnTurnEnd: Fired at the end of each agent turn.
		OnTurnEnd: func(ctx extensions.ExtensionContext, event extensions.TurnEndEvent) error {
			_ = event.Message
			_ = event.ToolResults
			return nil
		},

		// OnInput: Intercept user input. Can transform or handle it.
		OnInput: func(ctx extensions.ExtensionContext, event extensions.InputEvent) (*extensions.InputEventResult, error) {
			// Handle /example shortcut
			if event.Text == "/example" {
				return &extensions.InputEventResult{
					Action: "handled",
				}, nil
			}
			return nil, nil
		},

		// OnBeforeAgentStart: Modify the system prompt or user prompt before processing.
		OnBeforeAgentStart: func(ctx extensions.ExtensionContext, event extensions.BeforeAgentStartEvent) error {
			_ = event.Prompt
			_ = event.SystemPrompt
			return nil
		},

		// OnContext: Modify messages before they are sent to the LLM.
		OnContext: func(ctx extensions.ExtensionContext, event extensions.ContextEvent) ([]agent.AgentMessage, error) {
			return event.Messages, nil // Pass through unchanged
		},

		// OnToolCall: Block or modify tool calls before execution.
		OnToolCall: func(ctx extensions.ExtensionContext, event extensions.ToolCallEvent) (*extensions.ToolCallEventResult, error) {
			// Example: Block dangerous bash commands
			if event.ToolName == "Bash" {
				if cmd, ok := event.Input["command"].(string); ok {
					if strings.Contains(cmd, "rm -rf /") || strings.Contains(cmd, "rm -rf /*") {
						return &extensions.ToolCallEventResult{
							Block:  true,
							Reason: "Blocked dangerous command: rm -rf /",
						}, nil
					}
				}
			}
			return nil, nil
		},

		// OnUserBash: Intercept user bash commands.
		OnUserBash: func(ctx extensions.ExtensionContext, event extensions.UserBashEvent) error {
			_ = event.Command
			return nil
		},

		// ====================================================================
		// Keybindings
		// ====================================================================
		Keybindings: []extensions.ExtensionShortcut{
			{
				Key:         "ctrl+e",
				Description: "Example extension: show info",
				Handler: func() error {
					fmt.Fprintf(os.Stderr, "Example extension triggered via Ctrl+E\n")
					return nil
				},
			},
		},

		// ====================================================================
		// CLI Flags
		// ====================================================================
		CLIFlags: []extensions.ExtensionFlag{
			{
				Name:        "example-flag",
				Description: "Example extension flag",
				Handler: func(value string) error {
					fmt.Fprintf(os.Stderr, "Example extension flag set to: %s\n", value)
					return nil
				},
			},
		},

		// ====================================================================
		// Message Renderers
		// ====================================================================
		MessageRenderers: []extensions.MessageRenderer{
			{
				Type: "custom_notification",
				Render: func(msg agent.AgentMessage, width int) []string {
					return []string{
						"┌─ Custom Notification ─────────────────────┐",
						"│  " + msg.Content,
						"└──────────────────────────────────────────┘",
					}
				},
			},
		},
	}
}
