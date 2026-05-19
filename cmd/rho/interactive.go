package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/agent/extensions"
	"github.com/earendil-works/rho/pkg/agent/tools"
	"github.com/earendil-works/rho/pkg/ai"
	"github.com/earendil-works/rho/pkg/tui"
)

// InteractiveMode is the full TUI agent interface.
type InteractiveMode struct {
	tui            *tui.TUI
	term           *tui.ProcessTerminal
	input          *tui.Input
	messages       *MessageList
	agent          *agent.AgentLoop
	config         *RuntimeConfig
	status         *tui.Text
	extRuntime     *extensions.Runtime
	extCtx         extensions.ExtensionContext
	sessionManager *agent.SessionManager
	sessionID      string
}

// RuntimeConfig holds the runtime configuration.
type RuntimeConfig struct {
	Model        ai.Model
	SystemPrompt string
	APIKey       string
	Provider     ai.Provider
	CWD          string
	ExtDirs      []string
}

// MessageList displays conversation messages.
type MessageList struct {
	messages []agent.AgentMessage
	theme    tui.MarkdownTheme
}

func NewMessageList() *MessageList {
	return &MessageList{
		theme: tui.DefaultMarkdownTheme(),
	}
}

func (ml *MessageList) AddMessage(msg agent.AgentMessage) {
	ml.messages = append(ml.messages, msg)
}

func (ml *MessageList) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	var lines []string
	theme := ml.theme

	for _, msg := range ml.messages {
		if msg.Hide {
			continue
		}

		if msg.Role == ai.RoleUser {
			lines = append(lines, "")
			lines = append(lines, theme.H2("You:")+"\x1b[0m")
			md := tui.NewMarkdown(msg.Content, theme)
			contentLines := md.Render(width - 4)
			for _, l := range contentLines {
				if l != "" {
					lines = append(lines, "  "+l)
				}
			}
		} else if msg.Role == ai.RoleAssistant {
			modelName := msg.Model
			if modelName == "" {
				modelName = "Assistant"
			}
			lines = append(lines, "")
			lines = append(lines, theme.H2(modelName+":")+"\x1b[0m")

			if msg.Content != "" {
				md := tui.NewMarkdown(msg.Content, theme)
				contentLines := md.Render(width - 4)
				for _, l := range contentLines {
					if l != "" {
						lines = append(lines, "  "+l)
					}
				}
			}

			for _, tc := range msg.ToolCalls {
				argsJSON := fmt.Sprintf("%v", tc.Arguments)
				lines = append(lines, "  🔧 "+theme.Code(tc.Name+": "+argsJSON)+"\x1b[0m")
			}

			if msg.ErrorMessage != "" {
				lines = append(lines, "  ⚠ "+theme.Code("Error: "+msg.ErrorMessage)+"\x1b[0m")
			}
		} else if msg.Role == ai.RoleToolResult {
			lines = append(lines, "  ["+msg.ToolName+"]")
			content := strings.Split(msg.Content, "\n")
			maxLines := 2
			if len(content) > maxLines {
				content = content[:maxLines]
			}
			for _, l := range content {
				truncated := l
				if tui.VisibleWidth(truncated) > width-8 {
					truncated = tui.SliceByColumn(truncated, 0, width-8, true)
				}
				lines = append(lines, "  "+truncated)
			}
		}
	}

	return lines
}

func (ml *MessageList) HandleInput(data string) {}
func (ml *MessageList) Invalidate()            {}
func (ml *MessageList) WantsKeyRelease() bool  { return false }

// NewInteractiveMode creates a new interactive mode.
func NewInteractiveMode(cfg *RuntimeConfig) *InteractiveMode {
	term := tui.NewProcessTerminal()
	t := tui.NewTUI(term)

	// Create extension runtime
	extRuntime := extensions.NewRuntime()

	// Build extension context
	extCtx := extensions.ExtensionContext{
		HasUI:            true,
		CWD:              cfg.CWD,
		ExtensionRuntime: extRuntime,
		Abort:            nil, // set when agent loop is created
		Shutdown: func() {
			t.Stop()
			os.Exit(0)
		},
		UI: extensions.ExtensionUIContext{
			Select:  func(title string, options []string) (string, error) { return "", nil },
			Confirm: func(title, message string) (bool, error) { return true, nil },
			Input:   func(title, placeholder string) (string, error) { return "", nil },
			Notify:  func(message string, msgType string) {},
			SetStatus: func(key, text string) {},
		},
	}

	// Load extensions from configured directories
	if len(cfg.ExtDirs) > 0 {
		result := extensions.LoadExtensions(cfg.ExtDirs, extRuntime)
		for _, name := range result.Loaded {
			fmt.Fprintf(os.Stderr, "Loaded extension: %s\n", name)
		}
		for _, err := range result.Errors {
			fmt.Fprintf(os.Stderr, "Extension error: %s\n", err)
		}
	}

	// Create session manager
	rhoDir := filepath.Join(os.Getenv("HOME"), ".rho")
	sessionMgr := agent.NewSessionManager(filepath.Join(rhoDir, "sessions"))

	return &InteractiveMode{
		tui:            t,
		term:           term,
		input:          tui.NewInput(),
		config:         cfg,
		extRuntime:     extRuntime,
		extCtx:         extCtx,
		sessionManager: sessionMgr,
		sessionID:      agent.CurrentSessionID(),
	}
}

// Run starts the interactive mode.
func (im *InteractiveMode) Run() error {
	im.setupUI()
	im.setupSignalHandling()

	// Fire session start
	im.extCtx.ExtensionRuntime.FireSessionStart(im.extCtx, extensions.SessionStartEvent{
		Type: extensions.SessionStartup,
	})

	im.tui.Start()

	// Block until TUI stops (signal handler calls os.Exit)
	<-make(chan struct{})
	return nil
}

func (im *InteractiveMode) setupUI() {
	im.status = tui.NewText(fmt.Sprintf("rho | %s/%s | %s",
		im.config.Provider, im.config.Model.Name, shortenPath(im.config.CWD)))
	im.tui.AddChild(im.status)
	im.tui.AddChild(tui.NewSpacer(1))

	im.messages = NewMessageList()
	im.tui.AddChild(im.messages)

	im.tui.AddChild(tui.NewText("\x1b[2m" + strings.Repeat("─", 60) + "\x1b[0m"))

	im.input.SetPlaceholder("Type your message...")
	im.input.SetOnSubmit(func(value string) {
		im.handleSubmit(value)
	})
	im.input.SetFocused(true)
	im.tui.SetFocus(im.input)
	im.tui.AddChild(im.input)

	welcome := "rho v" + version + " — Your local coding agent\nType a message and press Enter to start. Ctrl+C to quit."
	im.messages.AddMessage(agent.AgentMessage{
		Role:    ai.RoleAssistant,
		Content: welcome,
		Model:   "rho",
	})
}

func (im *InteractiveMode) handleSubmit(value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}

	// Check for slash commands (handled by extensions)
	if strings.HasPrefix(value, "/") {
		parts := strings.Fields(value)
		cmdName := strings.TrimPrefix(parts[0], "/")
		cmdArgs := parts[1:]
		if err := im.extRuntime.HandleSlashCommand(im.extCtx, cmdName, cmdArgs); err == nil {
			return
		}
	}

	// Fire input event to extensions
	inputResult, err := im.extRuntime.FireInput(im.extCtx, extensions.InputEvent{
		Text:   value,
		Source: "interactive",
	})
	if err != nil {
		im.messages.AddMessage(agent.AgentMessage{
			Role:    ai.RoleAssistant,
			Content: fmt.Sprintf("Extension error: %v", err),
			Model:   im.config.Model.Name,
		})
		im.tui.RequestRender(true)
		return
	}
	if inputResult != nil && inputResult.Action == "handled" {
		return
	}
	if inputResult != nil && inputResult.Action == "transform" {
		value = inputResult.Text
	}

	userMsg := agent.AgentMessage{
		Role:      ai.RoleUser,
		Content:   value,
		Timestamp: time.Now().UnixMilli(),
	}
	im.messages.AddMessage(userMsg)
	im.input.SetValue("")

	im.status.SetContent(fmt.Sprintf("rho | %s/%s | Thinking...",
		im.config.Provider, im.config.Model.Name))
	im.tui.RequestRender(true)

	go func() {
		// Fire agent start
		im.extRuntime.FireAgentStart(im.extCtx)

		agentMsg, err := im.runAgent(value)
		if err != nil {
			im.messages.AddMessage(agent.AgentMessage{
				Role:    ai.RoleAssistant,
				Content: fmt.Sprintf("Error: %v", err),
				Model:   im.config.Model.Name,
			})
		} else if agentMsg != nil {
			im.messages.AddMessage(*agentMsg)
		}

		// Save session
		header := agent.SessionHeader{
			ID:        im.sessionID,
			Timestamp: time.Now().Format(time.RFC3339),
			CWD:       im.config.CWD,
		}
		im.sessionManager.Save(im.sessionID, header, im.messages.messages)

		// Fire agent end
		im.extRuntime.FireAgentEnd(im.extCtx, extensions.AgentEndEvent{
			Messages: im.messages.messages,
		})

		im.status.SetContent(fmt.Sprintf("rho | %s/%s | %s",
			im.config.Provider, im.config.Model.Name, shortenPath(im.config.CWD)))
		im.tui.RequestRender(true)
	}()
}

func (im *InteractiveMode) runAgent(prompt string) (*agent.AgentMessage, error) {
	loop := agent.NewAgentLoop(agent.AgentLoopConfig{
		Model:             im.config.Model,
		SystemPrompt:      im.config.SystemPrompt,
		APIKey:            im.config.APIKey,
		ToolExecutionMode: agent.ToolExecutionSequential,
	})

	// Apply extension hooks to the agent loop
	extensions.InstallHooks(im.extRuntime, im.extCtx, loop)

	// Merge built-in tools with extension custom tools
	builtinTools := tools.AllTools(im.config.CWD)
	extTools := im.extRuntime.GetCustomTools()
	allTools := extensions.MergeTools(builtinTools, extTools)

	context := agent.AgentContext{
		SystemPrompt: im.config.SystemPrompt,
		Model:        im.config.Model,
		Tools:        allTools,
	}

	prompts := []agent.AgentMessage{
		{Role: ai.RoleUser, Content: prompt, Timestamp: time.Now().UnixMilli()},
	}

	emit := func(event agent.AgentEvent) error {
		switch event.Type {
		case "tool_execution_start":
			if event.ToolCall != nil {
				im.messages.AddMessage(agent.AgentMessage{
					Role:    ai.RoleAssistant,
					Content: fmt.Sprintf("Running tool: %s...", event.ToolCall.Name),
					Model:   im.config.Model.Name,
				})
				im.tui.RequestRender(true)
			}
		case "tool_execution_end":
			if event.ToolCall != nil && event.Content != "" {
				truncated := event.Content
				if len(truncated) > 200 {
					truncated = truncated[:200] + "..."
				}
				im.messages.AddMessage(agent.AgentMessage{
					Role:     ai.RoleToolResult,
					ToolName: event.ToolCall.Name,
					Content:  truncated,
				})
				im.tui.RequestRender(true)
			}
		}
		return nil
	}

	results, err := loop.Run(prompts, context, emit)
	if err != nil {
		return nil, err
	}

	for i := len(results) - 1; i >= 0; i-- {
		if results[i].Role == ai.RoleAssistant {
			return &results[i], nil
		}
	}

	return &agent.AgentMessage{
		Role:    ai.RoleAssistant,
		Content: "No response generated.",
		Model:   im.config.Model.Name,
	}, nil
}

func (im *InteractiveMode) setupSignalHandling() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		im.extRuntime.FireSessionShutdown(im.extCtx, extensions.SessionShutdownEvent{
			Reason: extensions.SessionQuit,
		})
		im.tui.Stop()
		os.Exit(0)
	}()
}

func shortenPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}
