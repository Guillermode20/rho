package main

import (
	"fmt"
	"strings"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/agent/codecore"
	"github.com/earendil-works/rho/pkg/agent/extensions"
	"github.com/earendil-works/rho/pkg/agent/tools"
	agentutils "github.com/earendil-works/rho/pkg/agent/utils"
	"github.com/earendil-works/rho/pkg/ai"
	"github.com/earendil-works/rho/pkg/tui"
)

func (im *InteractiveMode) handleSlashCommand(value string) bool {
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return true
	}
	cmdName := strings.ToLower(strings.TrimPrefix(parts[0], "/"))
	args := parts[1:]

	switch cmdName {
	case "login":
		im.handleLoginCommand(args)
		return true
	case "logout":
		im.handleLogoutCommand(args)
		return true
	case "model", "models":
		im.handleModelCommand(args)
		return true
	case "settings", "config":
		im.handleSettingsCommand(args)
		return true
	case "theme", "themes":
		im.handleThemeCommand(args)
		return true
	case "extensions", "ext":
		im.showExtensionsSelector()
		return true
	case "copy":
		im.copyLastAssistantMessage()
		return true
	case "sessions", "session", "resume":
		im.handleSessionsCommand(args, "Resume session")
		return true
	case "name":
		im.handleNameCommand(args)
		return true
	case "tree":
		im.handleSessionsCommand(args, "Session tree")
		return true
	case "fork":
		im.forkSession()
		return true
	case "clone":
		im.cloneSession()
		return true
	case "share":
		im.shareSession(args)
		return true
	case "import":
		im.importSession(args)
		return true
	case "export":
		im.exportSession(args)
		return true
	case "new":
		im.startNewSession()
		return true
	case "commands":
		im.showCommandList()
		im.ui.ClearInput()
		return true
	case "tools":
		im.showAvailableTools()
		im.ui.ClearInput()
		return true
	case "reload":
		im.handleReloadCommand()
		return true
	}

	if im.hasExtensionCommand(cmdName) {
		im.ui.ClearInput()
		go func() {
			if err := im.extRuntime.HandleSlashCommand(im.extCtx, cmdName, args); err != nil {
				im.program.Send(tui.AddMessageMsg{
					Role:    string(ai.RoleAssistant),
					Content: fmt.Sprintf("Extension command error: %v", err),
					Model:   "rho",
				})
			}
		}()
		return true
	}

	if im.slashCommands != nil {
		ctx := codecore.SlashCommandContext{
			CWD:            im.config.CWD,
			SessionManager: im.sessionManager,
			Model:          &im.config.Model,
			SystemPrompt:   im.config.SystemPrompt,
			Notify: func(message string, msgType string) {
				im.addSystemMessage(message)
			},
		}
		if err := im.slashCommands.Execute(ctx, value); err == nil {
			im.ui.ClearInput()
			return true
		}
	}

	im.addSystemMessage(fmt.Sprintf("Unknown command: /%s. Type /help to see available commands.", cmdName))
	im.ui.ClearInput()
	return true
}

func (im *InteractiveMode) hasExtensionCommand(name string) bool {
	for _, cmd := range im.extRuntime.GetSlashCommands() {
		if cmd.Name == name {
			return true
		}
	}
	return false
}

func (im *InteractiveMode) handleSettingsCommand(args []string) {
	im.ui.ClearInput()
	if len(args) > 0 {
		im.addSystemMessage("Interactive settings editing is available from /settings. Select an item to change related state.")
		return
	}
	im.showSettingsSelector()
}

func (im *InteractiveMode) handleThemeCommand(args []string) {
	im.ui.ClearInput()
	if len(args) == 0 {
		im.showThemeSelector()
		return
	}
	im.selectTheme(strings.Join(args, " "))
}

func (im *InteractiveMode) showAvailableTools() {
	allTools := extensions.MergeTools(tools.AllTools(im.config.CWD), im.extRuntime.GetCustomTools())
	var lines []string
	for _, tool := range allTools {
		lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.Theme.Accent(tool.Name), im.ui.Theme.Muted(tool.Description)))
	}
	if len(lines) == 0 {
		lines = []string{"  No tools available."}
	}
	im.ui.OpenModalInfo("📋 Available tools", lines, nil)
}

func (im *InteractiveMode) copyLastAssistantMessage() {
	im.ui.ClearInput()
	msg, ok := im.lastAssistantMessage()
	if !ok {
		im.addSystemMessage("No assistant message to copy.")
		return
	}
	write := im.config.ClipboardWrite
	if write == nil {
		write = agentutils.DefaultClipboard().Write
	}
	if err := write(msg.Content); err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not copy assistant message: %v", err))
		return
	}
	im.addSystemMessage("Copied last assistant message to clipboard.")
}

func (im *InteractiveMode) lastAssistantMessage() (agent.AgentMessage, bool) {
	for i := len(im.ui.Messages) - 1; i >= 0; i-- {
		msg := im.ui.Messages[i]
		if msg.Hide || msg.Model == "rho" {
			continue
		}
		if msg.Role == ai.RoleAssistant && strings.TrimSpace(msg.Content) != "" {
			return msg, true
		}
	}
	return agent.AgentMessage{}, false
}

func (im *InteractiveMode) showCommandList() {
	var lines []string
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.Theme.Accent("/login"), im.ui.Theme.Muted("Set up a provider or API key")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.Theme.Accent("/logout"), im.ui.Theme.Muted("Remove stored credentials")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.Theme.Accent("/model"), im.ui.Theme.Muted("Select a model")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.Theme.Accent("/settings"), im.ui.Theme.Muted("Open settings")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.Theme.Accent("/theme"), im.ui.Theme.Muted("Select a theme")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.Theme.Accent("/new"), im.ui.Theme.Muted("Start a new session")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.Theme.Accent("/resume"), im.ui.Theme.Muted("Resume a previous session")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.Theme.Accent("/fork"), im.ui.Theme.Muted("Fork the current session")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.Theme.Accent("/name"), im.ui.Theme.Muted("Name the current session")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.Theme.Accent("/copy"), im.ui.Theme.Muted("Copy last assistant message")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.Theme.Accent("/commands"), im.ui.Theme.Muted("Show all commands")))
	lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.Theme.Accent("/tools"), im.ui.Theme.Muted("Show available tools")))

	// Add extension slash commands
	for _, cmd := range im.extRuntime.GetSlashCommands() {
		lines = append(lines, fmt.Sprintf("  %s  %s", im.ui.Theme.Accent("/"+cmd.Name), im.ui.Theme.Muted(cmd.Description)))
	}

	im.ui.OpenModalInfo("📋 Slash Commands", lines, nil)
}

func messageHasToolCall(msg agent.AgentMessage, id string) bool {
	for _, tc := range msg.ToolCalls {
		if tc.ID == id {
			return true
		}
	}
	return false
}

func formatToolArgs(args map[string]interface{}) string {
	if len(args) == 0 {
		return "{}"
	}
	var parts []string
	for key, value := range args {
		parts = append(parts, fmt.Sprintf("%s=%v", key, value))
	}
	return strings.Join(parts, ", ")
}
