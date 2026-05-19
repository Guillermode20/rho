// Package exampleext contains example extension implementations for rho.

package exampleext

import (
	"fmt"
	"math/rand"

	"github.com/earendil-works/rho/pkg/agent/extensions"
)

// RegisterHelloWorld registers a basic hello world extension.
func RegisterHelloWorld(runtime *extensions.Runtime) {
	runtime.Register(&extensions.ExtensionDef{
		Name:        "hello-world",
		Description: "A simple hello world extension",
		Version:     "1.0.0",
		OnAgentStart: func(ctx extensions.ExtensionContext) error {
			ctx.UI.Notify("Hello from hello-world extension!", "info")
			return nil
		},
		SlashCommands: []extensions.SlashCommand{
			{
				Name:        "hello",
				Description: "Say hello",
				Handler: func(ctx extensions.ExtensionContext, args []string) error {
					ctx.UI.Notify("Hello, world!", "info")
					return nil
				},
			},
		},
	})
}

// RegisterWeatherTool registers a weather lookup tool extension.
func RegisterWeatherTool(runtime *extensions.Runtime) {
	runtime.Register(&extensions.ExtensionDef{
		Name:        "weather-tool",
		Description: "Get current weather for any city",
		Version:     "1.0.0",
		CustomTools: []extensions.ToolDefinition{
			{
				Name:        "Weather",
				Label:       "Weather",
				Description: "Get the current weather for a city. Returns temperature, conditions, humidity, and wind speed.",
				PromptSnippet: "Get weather information for any city worldwide.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"city": map[string]interface{}{
							"type":        "string",
							"description": "The city name (e.g., 'London', 'Tokyo', 'New York')",
						},
						"units": map[string]interface{}{
							"type":        "string",
							"enum":        []interface{}{"metric", "imperial"},
							"description": "Temperature units (default: metric)",
						},
					},
					"required": []interface{}{"city"},
				},
				Execute: func(args map[string]interface{}) (string, bool, error) {
					city, _ := args["city"].(string)
					if city == "" {
						return "", true, fmt.Errorf("city is required")
					}
					units, _ := args["units"].(string)
					if units == "" {
						units = "metric"
					}
					temp := 15 + rand.Intn(20)
					conditions := []string{"Sunny", "Partly cloudy", "Cloudy", "Light rain", "Clear", "Overcast"}
					cond := conditions[rand.Intn(len(conditions))]
					humidity := 30 + rand.Intn(50)
					wind := 5 + rand.Intn(25)
					unit := "°C"
					if units == "imperial" {
						temp = temp*9/5 + 32
						unit = "°F"
					}
					return fmt.Sprintf("Weather in %s:\n  Temperature: %d%s\n  Conditions: %s\n  Humidity: %d%%\n  Wind: %d km/h", city, temp, unit, cond, humidity, wind), false, nil
				},
			},
		},
	})
}

// RegisterConfirmDestructive registers an extension that confirms destructive operations.
func RegisterConfirmDestructive(runtime *extensions.Runtime) {
	runtime.Register(&extensions.ExtensionDef{
		Name:        "confirm-destructive",
		Description: "Confirm before executing destructive operations like rm, dd, format",
		Version:     "1.0.0",
		OnToolCall: func(ctx extensions.ExtensionContext, event extensions.ToolCallEvent) (*extensions.ToolCallEventResult, error) {
			if event.ToolName == "Bash" {
				cmd, _ := event.Input["command"].(string)
				if isDestructive(cmd) {
					confirmed, err := ctx.UI.Confirm("Destructive Command", fmt.Sprintf("Are you sure you want to run:\n\n%s", cmd))
					if err != nil {
						return nil, err
					}
					if !confirmed {
						return &extensions.ToolCallEventResult{
							Block:  true,
							Reason: "User cancelled destructive operation",
						}, nil
					}
				}
			}
			return nil, nil
		},
	})
}

func isDestructive(cmd string) bool {
	patterns := []string{
		"rm -rf ", "rm -fr ", "rm -r /", "rm -f /",
		"dd if=", "mkfs.", "format ",
		":(){ :|:& };:",  // fork bomb
		"> /dev/sda", "> /dev/sdb",
		"chmod 000 /", "chown -R ",
	}
	for _, p := range patterns {
		if len(cmd) >= len(p) && cmd[:len(p)] == p {
			return true
		}
	}
	return false
}
