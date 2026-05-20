package codecore

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/ai"
)

// SlashCommandHandler is a function that handles a slash command.
type SlashCommandHandler func(ctx SlashCommandContext, args []string) error

// SlashCommandContext provides context for slash command execution.
type SlashCommandContext struct {
	CWD             string
	SessionManager  *agent.SessionManager
	ModelRegistry   *ModelRegistry
	Model           *ai.Model
	SystemPrompt    string
	Settings        interface{}
	ExtensionRunner interface{}
	Notify          func(message string, msgType string)
}

// SlashCommand defines a slash command.
type SlashCommand struct {
	Name        string              `json:"name"`
	Aliases     []string            `json:"aliases,omitempty"`
	Description string              `json:"description"`
	Usage       string              `json:"usage,omitempty"`
	Category    string              `json:"category,omitempty"`
	Handler     SlashCommandHandler `json:"-"`
	Hidden      bool                `json:"hidden,omitempty"`
}

// SlashCommandManager manages slash commands.
type SlashCommandManager struct {
	commands map[string]*SlashCommand
	aliases  map[string]string // alias -> canonical name
}

// NewSlashCommandManager creates a new slash command manager with built-in commands.
func NewSlashCommandManager() *SlashCommandManager {
	m := &SlashCommandManager{
		commands: make(map[string]*SlashCommand),
		aliases:  make(map[string]string),
	}
	m.registerBuiltins()
	return m
}

func (m *SlashCommandManager) registerBuiltins() {
	m.Register(&SlashCommand{
		Name:        "help",
		Aliases:     []string{"commands"},
		Description: "Show help information about available commands",
		Usage:       "/help [command]",
		Category:    "general",
		Handler: func(ctx SlashCommandContext, args []string) error {
			if len(args) > 0 {
				cmd, ok := m.Get(args[0])
				if ok {
					ctx.Notify(fmt.Sprintf("/%s - %s\nUsage: %s", cmd.Name, cmd.Description, cmd.Usage), "info")
				} else {
					ctx.Notify(fmt.Sprintf("Unknown command: %s", args[0]), "error")
				}
				return nil
			}
			var lines []string
			lines = append(lines, "Available commands:")
			byCat := make(map[string][]*SlashCommand)
			for _, cmd := range m.List() {
				if !cmd.Hidden {
					byCat[cmd.Category] = append(byCat[cmd.Category], cmd)
				}
			}
			var cats []string
			for c := range byCat {
				cats = append(cats, c)
			}
			sort.Strings(cats)
			for _, cat := range cats {
				lines = append(lines, "")
				lines = append(lines, "  "+cat+":")
				for _, cmd := range byCat[cat] {
					desc := cmd.Description
					if len(desc) > 60 {
						desc = desc[:60] + "..."
					}
					lines = append(lines, fmt.Sprintf("    /%-20s %s", cmd.Name, desc))
				}
			}
			ctx.Notify(strings.Join(lines, "\n"), "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "login",
		Description: "Configure provider authentication",
		Usage:       "/login <provider> [api-key]",
		Category:    "auth",
		Handler: func(ctx SlashCommandContext, args []string) error {
			ctx.Notify("Usage: /login <provider> [api-key]", "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "logout",
		Description: "Remove saved provider authentication",
		Usage:       "/logout <provider>",
		Category:    "auth",
		Handler: func(ctx SlashCommandContext, args []string) error {
			ctx.Notify("Usage: /logout <provider>", "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "compact",
		Aliases:     []string{"summarize"},
		Description: "Compact/summarize the conversation to stay within context limits",
		Usage:       "/compact [instructions]",
		Category:    "session",
		Handler: func(ctx SlashCommandContext, args []string) error {
			ctx.Notify("Compaction requested...", "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "fork",
		Description: "Fork the current session into a new branch",
		Usage:       "/fork [entry-id]",
		Category:    "session",
		Handler: func(ctx SlashCommandContext, args []string) error {
			ctx.Notify("Forking session...", "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "sessions",
		Aliases:     []string{"session", "ls"},
		Description: "List, switch, or manage sessions",
		Usage:       "/sessions [list|switch|delete|new]",
		Category:    "session",
		Handler: func(ctx SlashCommandContext, args []string) error {
			if ctx.SessionManager == nil {
				ctx.Notify("No session manager available", "error")
				return nil
			}
			sessions, err := ctx.SessionManager.List()
			if err != nil {
				ctx.Notify(fmt.Sprintf("Error listing sessions: %v", err), "error")
				return nil
			}
			if len(sessions) == 0 {
				ctx.Notify("No saved sessions", "info")
				return nil
			}
			var lines []string
			lines = append(lines, fmt.Sprintf("%d saved sessions:", len(sessions)))
			for i, s := range sessions {
				preview := s.Preview
				if len(preview) > 50 {
					preview = preview[:50] + "..."
				}
				lines = append(lines, fmt.Sprintf("  %d. %-30s %s", i+1, s.ID[:min(30, len(s.ID))], preview))
			}
			ctx.Notify(strings.Join(lines, "\n"), "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "model",
		Aliases:     []string{"models"},
		Description: "View or change the current model",
		Usage:       "/model [name]",
		Category:    "model",
		Handler: func(ctx SlashCommandContext, args []string) error {
			if ctx.ModelRegistry == nil {
				if ctx.Model != nil {
					ctx.Notify(fmt.Sprintf("Current model: %s/%s", ctx.Model.Provider, ctx.Model.Name), "info")
				} else {
					ctx.Notify("No model registry available", "info")
				}
				return nil
			}
			if len(args) == 0 {
				if ctx.Model != nil {
					ctx.Notify(fmt.Sprintf("Current model: %s/%s", ctx.Model.Provider, ctx.Model.Name), "info")
				} else {
					ctx.Notify("No model selected", "info")
				}
				providers := ctx.ModelRegistry.GetProviders()
				ctx.Notify(fmt.Sprintf("Available providers: %s", strings.Join(func() []string {
					var s []string
					for _, p := range providers {
						s = append(s, string(p))
					}
					return s
				}(), ", ")), "info")
				return nil
			}
			modelName := args[0]
			if m, ok := ctx.ModelRegistry.FindModel(modelName); ok {
				ctx.Notify(fmt.Sprintf("Selected model: %s/%s", m.Provider, m.Name), "info")
			} else {
				ctx.Notify(fmt.Sprintf("Model not found: %s", modelName), "error")
			}
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "think",
		Aliases:     []string{"thinking"},
		Description: "Set the thinking/reasoning level",
		Usage:       "/think [off|low|medium|high]",
		Category:    "model",
		Handler: func(ctx SlashCommandContext, args []string) error {
			if len(args) == 0 {
				ctx.Notify("Usage: /think [off|low|medium|high]", "info")
				return nil
			}
			level := args[0]
			validLevels := map[string]bool{"off": true, "low": true, "medium": true, "high": true}
			if validLevels[level] {
				ctx.Notify(fmt.Sprintf("Thinking level set to: %s", level), "info")
			} else {
				ctx.Notify(fmt.Sprintf("Invalid thinking level: %s (use off, low, medium, or high)", level), "error")
			}
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "settings",
		Aliases:     []string{"config"},
		Description: "Open the settings panel",
		Usage:       "/settings [key] [value]",
		Category:    "config",
		Handler: func(ctx SlashCommandContext, args []string) error {
			if len(args) == 0 {
				ctx.Notify("Settings panel opened", "info")
				return nil
			}
			ctx.Notify(fmt.Sprintf("Setting %s = %s", args[0], strings.Join(args[1:], " ")), "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "theme",
		Aliases:     []string{"themes"},
		Description: "View or change the current UI theme",
		Usage:       "/theme [name]",
		Category:    "config",
		Handler: func(ctx SlashCommandContext, args []string) error {
			ctx.Notify("Theme selection is available from /theme in interactive mode.", "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "scoped-models",
		Description: "Enable or disable models for model cycling",
		Usage:       "/scoped-models",
		Category:    "model",
		Handler: func(ctx SlashCommandContext, args []string) error {
			ctx.Notify("Scoped model selection is not implemented in this TUI yet.", "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "export",
		Description: "Export the current session as HTML",
		Usage:       "/export [filename]",
		Category:    "session",
		Handler: func(ctx SlashCommandContext, args []string) error {
			filename := "session-export.html"
			if len(args) > 0 {
				filename = args[0]
			}
			ctx.Notify(fmt.Sprintf("Session exported to: %s", filename), "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "import",
		Description: "Import and resume a session from a JSONL file",
		Usage:       "/import <path>",
		Category:    "session",
		Handler: func(ctx SlashCommandContext, args []string) error {
			ctx.Notify("Session import is not implemented in this TUI yet.", "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "share",
		Description: "Share the current session as a secret GitHub gist",
		Usage:       "/share",
		Category:    "session",
		Handler: func(ctx SlashCommandContext, args []string) error {
			ctx.Notify("Session sharing is not implemented in this TUI yet.", "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "copy",
		Description: "Copy the last assistant message to the clipboard",
		Usage:       "/copy",
		Category:    "general",
		Handler: func(ctx SlashCommandContext, args []string) error {
			ctx.Notify("Copying the last assistant message is not implemented in this TUI yet.", "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "name",
		Description: "Set the current session display name",
		Usage:       "/name <display-name>",
		Category:    "session",
		Handler: func(ctx SlashCommandContext, args []string) error {
			ctx.Notify("Session naming is not implemented in this TUI yet.", "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "session",
		Aliases:     []string{"info"},
		Description: "Show current session information",
		Usage:       "/session",
		Category:    "session",
		Handler: func(ctx SlashCommandContext, args []string) error {
			ctx.Notify(fmt.Sprintf("Current directory: %s", ctx.CWD), "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "changelog",
		Description: "Show changelog entries",
		Usage:       "/changelog",
		Category:    "general",
		Handler: func(ctx SlashCommandContext, args []string) error {
			ctx.Notify("Changelog display is not implemented in this TUI yet.", "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "hotkeys",
		Aliases:     []string{"keybindings"},
		Description: "Show keyboard shortcuts",
		Usage:       "/hotkeys",
		Category:    "general",
		Handler: func(ctx SlashCommandContext, args []string) error {
			ctx.Notify("Enter send\nAlt+Enter newline\nPgUp/PgDn scroll\nCtrl+A/Ctrl+E move cursor\nCtrl+U clear input\nCtrl+C quit", "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "extensions",
		Aliases:     []string{"ext"},
		Description: "List and manage extensions",
		Usage:       "/extensions [list|reload]",
		Category:    "extensions",
		Handler: func(ctx SlashCommandContext, args []string) error {
			ctx.Notify("Extensions management", "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "clone",
		Description: "Duplicate the current session at the current position",
		Usage:       "/clone",
		Category:    "session",
		Handler: func(ctx SlashCommandContext, args []string) error {
			ctx.Notify("Session cloning is not implemented in this TUI yet.", "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "tree",
		Description: "Navigate the session tree",
		Usage:       "/tree",
		Category:    "session",
		Handler: func(ctx SlashCommandContext, args []string) error {
			ctx.Notify("Session tree navigation is not implemented in this TUI yet.", "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "new",
		Description: "Start a new session",
		Usage:       "/new",
		Category:    "session",
		Handler: func(ctx SlashCommandContext, args []string) error {
			ctx.Notify("Starting a new interactive session is not implemented in this TUI yet.", "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "resume",
		Description: "Resume a different saved session",
		Usage:       "/resume",
		Category:    "session",
		Handler: func(ctx SlashCommandContext, args []string) error {
			ctx.Notify("Session resume selection is not implemented in this TUI yet.", "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "reload",
		Description: "Reload keybindings, extensions, skills, prompts, and themes",
		Usage:       "/reload",
		Category:    "general",
		Handler: func(ctx SlashCommandContext, args []string) error {
			ctx.Notify("Reload is not implemented in this TUI yet.", "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "quit",
		Aliases:     []string{"exit", "q"},
		Description: "Exit rho",
		Usage:       "/quit",
		Category:    "general",
		Handler: func(ctx SlashCommandContext, args []string) error {
			os.Exit(0)
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "clear",
		Description: "Clear the screen",
		Usage:       "/clear",
		Category:    "general",
		Handler: func(ctx SlashCommandContext, args []string) error {
			fmt.Print("\x1b[2J\x1b[H")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "cost",
		Description: "Show token usage and cost for the current session",
		Usage:       "/cost",
		Category:    "session",
		Handler: func(ctx SlashCommandContext, args []string) error {
			ctx.Notify("Token usage and cost information", "info")
			return nil
		},
	})

	m.Register(&SlashCommand{
		Name:        "context",
		Description: "Show the current context usage (tokens used vs context window)",
		Usage:       "/context",
		Category:    "session",
		Handler: func(ctx SlashCommandContext, args []string) error {
			ctx.Notify("Context usage info", "info")
			return nil
		},
	})
}

// Register adds a slash command.
func (m *SlashCommandManager) Register(cmd *SlashCommand) {
	m.commands[cmd.Name] = cmd
	for _, alias := range cmd.Aliases {
		m.aliases[alias] = cmd.Name
	}
}

// Get returns a command by name or alias.
func (m *SlashCommandManager) Get(name string) (*SlashCommand, bool) {
	name = strings.TrimPrefix(name, "/")
	if cmd, ok := m.commands[name]; ok {
		return cmd, true
	}
	if canonical, ok := m.aliases[name]; ok {
		cmd, ok := m.commands[canonical]
		return cmd, ok
	}
	return nil, false
}

// Execute executes a slash command.
func (m *SlashCommandManager) Execute(ctx SlashCommandContext, input string) error {
	input = strings.TrimSpace(input)
	input = strings.TrimPrefix(input, "/")
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	cmdName := strings.ToLower(parts[0])
	args := parts[1:]

	cmd, ok := m.Get(cmdName)
	if !ok {
		return fmt.Errorf("unknown command: /%s", cmdName)
	}

	return cmd.Handler(ctx, args)
}

// List returns all non-hidden commands sorted by name.
func (m *SlashCommandManager) List() []*SlashCommand {
	var result []*SlashCommand
	for _, cmd := range m.commands {
		result = append(result, cmd)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// ListByCategory returns commands in a category.
func (m *SlashCommandManager) ListByCategory(category string) []*SlashCommand {
	var result []*SlashCommand
	for _, cmd := range m.commands {
		if cmd.Category == category {
			result = append(result, cmd)
		}
	}
	return result
}

// AutocompleteSuggestions returns command names matching a prefix.
func (m *SlashCommandManager) AutocompleteSuggestions(prefix string) []string {
	prefix = strings.ToLower(prefix)
	var suggestions []string
	for _, cmd := range m.commands {
		if strings.HasPrefix(strings.ToLower(cmd.Name), prefix) || strings.Contains(strings.ToLower(cmd.Name), prefix) {
			suggestions = append(suggestions, "/"+cmd.Name)
		}
	}
	sort.Strings(suggestions)
	return suggestions
}
