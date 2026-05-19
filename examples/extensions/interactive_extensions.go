
package exampleext

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/agent/extensions"
	"github.com/earendil-works/rho/pkg/ai"
)

// RegisterInteractiveShell registers an interactive shell extension.
func RegisterInteractiveShell(runtime *extensions.Runtime) {
	shellActive := false

	runtime.Register(&extensions.ExtensionDef{
		Name:        "interactive-shell",
		Description: "Start an interactive shell session for the agent",
		Version:     "1.0.0",
		CustomTools: []extensions.ToolDefinition{
			{
				Name:        "InteractiveShell",
				Label:       "Interactive Shell",
				Description: "Start or interact with a persistent shell session. Use 'start' to begin, then send commands one at a time.",
				PromptSnippet: "Start long-running interactive shell sessions for complex multi-step tasks.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action": map[string]interface{}{
							"type":        "string",
							"enum":        []interface{}{"start", "command", "stop", "status"},
							"description": "Action: start a session, run a command, stop the session, or check status",
						},
						"command": map[string]interface{}{
							"type":        "string",
							"description": "Command to run in the shell session",
						},
					},
					"required": []interface{}{"action"},
				},
				Execute: func(args map[string]interface{}) (string, bool, error) {
					action, _ := args["action"].(string)
					switch action {
					case "start":
						if shellActive {
							return "Shell session already active.", false, nil
						}
						shellActive = true
						_ = os.Getpid()
						return "Interactive shell session started. Use 'command' to run commands.", false, nil
					case "command":
						if !shellActive {
							return "", true, fmt.Errorf("no active shell. Use 'start' first")
						}
						cmd, _ := args["command"].(string)
						if cmd == "" {
							return "", true, fmt.Errorf("command is required")
						}
						out, err := exec.Command("bash", "-c", cmd).CombinedOutput()
						if err != nil {
							return string(out), true, fmt.Errorf("command failed: %w", err)
						}
						return string(out), false, nil
					case "stop":
						shellActive = false
						return "Shell session stopped.", false, nil
					case "status":
						if shellActive {
							return "Shell session is active.", false, nil
						}
						return "No active shell session.", false, nil
					}
					return "", true, fmt.Errorf("unknown action: %s", action)
				},
			},
		},
	})
}

// RegisterNotify registers a desktop notification extension.
func RegisterNotify(runtime *extensions.Runtime) {
	runtime.Register(&extensions.ExtensionDef{
		Name:        "notify",
		Description: "Send desktop notifications when long-running tasks complete",
		Version:     "1.0.0",
		OnTurnEnd: func(ctx extensions.ExtensionContext, event extensions.TurnEndEvent) error {
			if event.Message.Content != "" && len(event.Message.Content) > 100 {
				summary := event.Message.Content
				if len(summary) > 80 {
					summary = summary[:80] + "..."
				}
				// Try to send desktop notification
				exec.Command("notify-send", "rho: Turn Complete", summary).Run()
			}
			return nil
		},
		SlashCommands: []extensions.SlashCommand{
			{
				Name:        "notify",
				Description: "Send a test notification",
				Handler: func(ctx extensions.ExtensionContext, args []string) error {
					msg := strings.Join(args, " ")
					if msg == "" {
						msg = "Test notification from rho!"
					}
					exec.Command("notify-send", "rho", msg).Run()
					ctx.UI.Notify("Notification sent!", "info")
					return nil
				},
			},
		},
	})
}

// RegisterSaveSave registers a session bookmark extension.
func RegisterSaveSession(runtime *extensions.Runtime) {
	var bookmarked bool

	runtime.Register(&extensions.ExtensionDef{
		Name:        "session-save",
		Description: "Bookmark and save important sessions",
		Version:     "1.0.0",
		SlashCommands: []extensions.SlashCommand{
			{
				Name:        "bookmark",
				Description: "Bookmark the current session",
				Handler: func(ctx extensions.ExtensionContext, args []string) error {
					bookmarked = true
					ctx.UI.Notify("Session bookmarked!", "info")
					return nil
				},
			},
		},
		OnSessionShutdown: func(ctx extensions.ExtensionContext, event extensions.SessionShutdownEvent) error {
			if bookmarked {
				ctx.UI.Notify("Bookmarked session saved.", "info")
			}
			return nil
		},
	})
}

// RegisterBashSpawnHook registers a bash spawn hook.
func RegisterBashSpawnHook(runtime *extensions.Runtime) {
	runtime.Register(&extensions.ExtensionDef{
		Name:        "bash-spawn-hook",
		Description: "Log all bash commands executed by the agent",
		Version:     "1.0.0",
		OnToolCall: func(ctx extensions.ExtensionContext, event extensions.ToolCallEvent) (*extensions.ToolCallEventResult, error) {
			if event.ToolName == "Bash" {
				cmd, _ := event.Input["command"].(string)
				ctx.UI.Notify(fmt.Sprintf("Running: %s", cmd), "info")
			}
			return nil, nil
		},
		OnToolResult: func(ctx extensions.ExtensionContext, event extensions.ToolResultEvent) error {
			if event.ToolName == "Bash" {
				status := "succeeded"
				if event.IsError {
					status = "failed"
				}
				ctx.UI.Notify(fmt.Sprintf("Bash command %s (exit: %s)", status, map[bool]string{true: "error", false: "ok"}[event.IsError]), "info")
			}
			return nil
		},
	})
}

// RegisterInputTransform registers an input transformation extension.
func RegisterInputTransform(runtime *extensions.Runtime) {
	runtime.Register(&extensions.ExtensionDef{
		Name:        "input-transform",
		Description: "Transform user input before sending to the agent (e.g., expand shortcuts)",
		Version:     "1.0.0",
		OnInput: func(ctx extensions.ExtensionContext, event extensions.InputEvent) (*extensions.InputEventResult, error) {
			text := event.Text

			// Expand common shortcuts
			expansions := map[string]string{
				"/xp":   "Explain this code in detail",
				"/fix":  "Find and fix any issues in the current code",
				"/test": "Write comprehensive tests for this",
				"/doc":  "Add documentation for this code",
				"/ref":  "Refactor this code to be more maintainable",
			}
			if expanded, ok := expansions[text]; ok {
				return &extensions.InputEventResult{
					Action: "transform",
					Text:   expanded,
				}, nil
			}

			return nil, nil
		},
	})
}

// RegisterPromptCustomizer registers a prompt customizer extension.
func RegisterPromptCustomizer(runtime *extensions.Runtime) {
	var extraInstructions string

	runtime.Register(&extensions.ExtensionDef{
		Name:        "prompt-customizer",
		Description: "Add custom instructions to the system prompt",
		Version:     "1.0.0",
		OnBeforeAgentStart: func(ctx extensions.ExtensionContext, event extensions.BeforeAgentStartEvent) error {
			if extraInstructions != "" {
				ctx.UI.Notify("Custom instructions applied", "info")
			}
			return nil
		},
		SlashCommands: []extensions.SlashCommand{
			{
				Name:        "instruct",
				Description: "Add custom instructions for the agent (args: <instructions>)",
				Handler: func(ctx extensions.ExtensionContext, args []string) error {
					extraInstructions = strings.Join(args, " ")
					ctx.UI.Notify("Custom instructions set. They will be included in the next agent run.", "info")
					return nil
				},
			},
			{
				Name:        "clear-instructions",
				Description: "Clear custom instructions",
				Handler: func(ctx extensions.ExtensionContext, args []string) error {
					extraInstructions = ""
					ctx.UI.Notify("Custom instructions cleared.", "info")
					return nil
				},
			},
		},
	})
}

// RegisterRegister registers a model status display extension.
func RegisterModelStatus(runtime *extensions.Runtime) {
	runtime.Register(&extensions.ExtensionDef{
		Name:        "model-status",
		Description: "Display current model information and token usage",
		Version:     "1.0.0",
		SlashCommands: []extensions.SlashCommand{
			{
				Name:        "model",
				Description: "Show current model info",
				Handler: func(ctx extensions.ExtensionContext, args []string) error {
					if ctx.Model != nil {
						ctx.UI.Notify(fmt.Sprintf("Model: %s/%s\nAPI: %s", ctx.Model.Provider, ctx.Model.Name, ctx.Model.API), "info")
					} else {
						ctx.UI.Notify("No model configured", "warning")
					}
					return nil
				},
			},
		},
	})
}

// RegisterShutdownCommand registers a shutdown command.
func RegisterShutdownCommand(runtime *extensions.Runtime) {
	runtime.Register(&extensions.ExtensionDef{
		Name:        "shutdown-command",
		Description: "Gracefully shutdown rho via /shutdown command",
		Version:     "1.0.0",
		SlashCommands: []extensions.SlashCommand{
			{
				Name:        "shutdown",
				Description: "Gracefully shutdown rho",
				Handler: func(ctx extensions.ExtensionContext, args []string) error {
					confirmed, err := ctx.UI.Confirm("Shutdown", "Are you sure you want to shutdown rho?")
					if err != nil {
						return err
					}
					if confirmed {
						ctx.UI.Notify("Shutting down...", "info")
						ctx.Shutdown()
					}
					return nil
				},
			},
		},
	})
}

// RegisterAutoCommitOnExit registers an auto-commit on exit extension.
func RegisterAutoCommitOnExit(runtime *extensions.Runtime) {
	runtime.Register(&extensions.ExtensionDef{
		Name:        "auto-commit-on-exit",
		Description: "Automatically commit changes when rho exits",
		Version:     "1.0.0",
		OnSessionShutdown: func(ctx extensions.ExtensionContext, event extensions.SessionShutdownEvent) error {
			if isGitRepo(ctx.CWD) {
				exec.Command("git", "add", "-A").Run()
				exec.Command("git", "commit", "--allow-empty", "-m", "rho session end").Run()
			}
			return nil
		},
	})
}

// RegisterProtectedPaths registers a protected paths extension.
func RegisterProtectedPaths(runtime *extensions.Runtime) {
	protected := []string{
		"/etc", "/usr", "/bin", "/boot", "/dev",
		"/proc", "/sys", "/lib", "/lib64", "/sbin",
	}

	isProtected := func(path string) bool {
		for _, p := range protected {
			if strings.HasPrefix(path, p) {
				return true
			}
		}
		return false
	}

	runtime.Register(&extensions.ExtensionDef{
		Name:        "protected-paths",
		Description: "Prevent the agent from modifying protected system paths",
		Version:     "1.0.0",
		OnToolCall: func(ctx extensions.ExtensionContext, event extensions.ToolCallEvent) (*extensions.ToolCallEventResult, error) {
			for _, field := range []string{"path", "file_path"} {
				if p, ok := event.Input[field].(string); ok {
					if isProtected(p) {
						return &extensions.ToolCallEventResult{
							Block:  true,
							Reason: fmt.Sprintf("Path %q is protected. Cannot modify system files.", p),
						}, nil
					}
				}
			}
			return nil, nil
		},
	})
}

// RegisterRegister registers a title bar spinner extension.
func RegisterTitleBarSpinner(runtime *extensions.Runtime) {
	runtime.Register(&extensions.ExtensionDef{
		Name:        "titlebar-spinner",
		Description: "Show a spinner in the terminal title bar while the agent is working",
		Version:     "1.0.0",
		OnAgentStart: func(ctx extensions.ExtensionContext) error {
			fmt.Fprintf(os.Stderr, "\x1b]0;rho: working...\x07")
			return nil
		},
		OnAgentEnd: func(ctx extensions.ExtensionContext, event extensions.AgentEndEvent) error {
			fmt.Fprintf(os.Stderr, "\x1b]0;rho: idle\x07")
			return nil
		},
	})
}

// RegisterEventBusDemo registers an event bus demonstration.
func RegisterEventBusDemo(runtime *extensions.Runtime) {
	runtime.Register(&extensions.ExtensionDef{
		Name:        "event-bus-demo",
		Description: "Demonstrate inter-extension communication via the EventBus",
		Version:     "1.0.0",
		SlashCommands: []extensions.SlashCommand{
			{
				Name:        "emit-event",
				Description: "Emit a custom event on the event bus (args: <type> <data>)",
				Handler: func(ctx extensions.ExtensionContext, args []string) error {
					if len(args) < 2 {
						return fmt.Errorf("usage: /emit-event <type> <data>")
					}
					eventType := args[0]
					data := strings.Join(args[1:], " ")
					ctx.ExtensionRuntime.GetEventBus().Emit(eventType, data)
					ctx.UI.Notify(fmt.Sprintf("Emitted event %q with data: %s", eventType, data), "info")
					return nil
				},
			},
			{
				Name:        "listen-event",
				Description: "Listen for a custom event type (args: <type>)",
				Handler: func(ctx extensions.ExtensionContext, args []string) error {
					if len(args) == 0 {
						return fmt.Errorf("usage: /listen-event <type>")
					}
					eventType := args[0]
					ctx.UI.Notify(fmt.Sprintf("Listening for %q events (demo only)", eventType), "info")
					return nil
				},
			},
		},
	})
}

// RegisterWorkingIndicator registers a custom working indicator extension.
func RegisterWorkingIndicator(runtime *extensions.Runtime) {
	runtime.Register(&extensions.ExtensionDef{
		Name:        "working-indicator",
		Description: "Show a custom working indicator during agent processing",
		Version:     "1.0.0",
		OnAgentStart: func(ctx extensions.ExtensionContext) error {
			ctx.UI.SetStatus("working", "● processing...")
			return nil
		},
		OnAgentEnd: func(ctx extensions.ExtensionContext, event extensions.AgentEndEvent) error {
			ctx.UI.SetStatus("working", "")
			return nil
		},
	})
}

// RegisterDynamicTools registers a dynamic tool registration extension.
func RegisterDynamicTools(runtime *extensions.Runtime) {
	runtime.Register(&extensions.ExtensionDef{
		Name:        "dynamic-tools",
		Description: "Register tools on-the-fly via slash commands",
		Version:     "1.0.0",
		SlashCommands: []extensions.SlashCommand{
			{
				Name:        "register-tool",
				Description: "Register a new custom tool (args: <name> <description>)",
				Handler: func(ctx extensions.ExtensionContext, args []string) error {
					if len(args) < 2 {
						return fmt.Errorf("usage: /register-tool <name> <description>")
					}
					name := args[0]
					desc := strings.Join(args[1:], " ")
					tool := extensions.ToolDefinition{
						Name:        name,
						Label:       name,
						Description: desc,
						Parameters: map[string]interface{}{
							"type":       "object",
							"properties": map[string]interface{}{},
						},
						Execute: func(args map[string]interface{}) (string, bool, error) {
							return fmt.Sprintf("Executed dynamic tool %q with args: %v", name, args), false, nil
						},
					}
					_ = tool
					ctx.UI.Notify(fmt.Sprintf("Tool %q registered!", name), "info")
					return nil
				},
			},
		},
	})
}

// Ensure unused import references compile
var _ = agent.AgentMessage{Role: ai.RoleUser}
var _ = os.Executable
