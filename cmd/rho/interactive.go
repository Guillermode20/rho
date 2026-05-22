package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/agent/auth"
	"github.com/earendil-works/rho/pkg/agent/codecore"
	"github.com/earendil-works/rho/pkg/agent/compaction"
	"github.com/earendil-works/rho/pkg/agent/extensions"
	"github.com/earendil-works/rho/pkg/agent/systemprompt"
	agenttheme "github.com/earendil-works/rho/pkg/agent/theme"
	"github.com/earendil-works/rho/pkg/agent/tools"
	"github.com/earendil-works/rho/pkg/agent/ui"
	agentutils "github.com/earendil-works/rho/pkg/agent/utils"
	"github.com/earendil-works/rho/pkg/ai"
	"github.com/earendil-works/rho/pkg/ai/providers"
	"github.com/earendil-works/rho/pkg/tui"
)

// RuntimeConfig holds the runtime configuration.
type RuntimeConfig struct {
	Model          ai.Model
	SystemPrompt   string
	APIKey         string
	Provider       ai.Provider
	CWD            string
	ExtDirs        []string
	AuthStorage    *auth.AuthStorage
	OAuthStore     *auth.OAuthStore
	Settings       *codecore.SettingsManager
	ThemeManager   *agenttheme.ThemeManager
	ClipboardWrite func(text string) error
	OpenURL        func(rawURL string) error
	OAuthExchange  func(provider ai.OAuthProviderID, code string, pkce *ai.PKCE) (*ai.OAuthCredentials, error)
}

type agentLoopEventMsg struct {
	Event agent.AgentEvent
}

type agentLoopFinalMsg struct {
	Message *agent.AgentMessage
}

type uiSelectRequestMsg struct {
	Title   string
	Options []string
	Resp    chan uiStringResponse
}

type uiConfirmRequestMsg struct {
	Title   string
	Message string
	Resp    chan uiBoolResponse
}

type uiInputRequestMsg struct {
	Title       string
	Placeholder string
	Resp        chan uiStringResponse
}

type uiStringResponse struct {
	Value string
	Err   error
}

type uiBoolResponse struct {
	Value bool
	Err   error
}

type AgentExtensionStatusMsg struct {
	Key  string
	Text string
}

// InteractiveMode is the full TUI agent interface.
type InteractiveMode struct {
	program              *tea.Program
	programRunning       bool
	ui                   *ui.ChatModel
	agent                *agent.AgentLoop
	config               *RuntimeConfig
	extRuntime           *extensions.Runtime
	extCtx               extensions.ExtensionContext
	coordinator          *codecore.RuntimeCoordinator
	sessionManager       *agent.SessionManager
	slashCommands        *codecore.SlashCommandManager
	sessionID            string
	pendingLoginProvider string
	extensionStatuses    map[string]string
	streamingMessageIdx  int
	pendingToolMessages  map[string]int
	agentCancel          context.CancelFunc
}

// NewInteractiveMode creates a new interactive mode.
func NewInteractiveMode(cfg *RuntimeConfig) *InteractiveMode {
	// Create extension runtime
	extRuntime := extensions.NewRuntime()

	// Build extension context
	extCtx := extensions.ExtensionContext{
		HasUI:            true,
		CWD:              cfg.CWD,
		ExtensionRuntime: extRuntime,
		Abort:            nil, // set when agent loop is created
		Shutdown: func() {
			os.Exit(0)
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
	settingsMgr := cfg.Settings
	if settingsMgr == nil {
		settingsMgr = codecore.NewSettingsManager(filepath.Join(rhoDir, "settings"), cfg.CWD)
		cfg.Settings = settingsMgr
	}
	themeMgr := cfg.ThemeManager
	if themeMgr == nil {
		themeMgr = agenttheme.NewThemeManager(filepath.Join(rhoDir, "themes"))
		if err := themeMgr.LoadThemes(); err != nil {
			fmt.Fprintf(os.Stderr, "Theme load error: %s\n", err)
		}
		cfg.ThemeManager = themeMgr
	}
	if selectedTheme := settingsMgr.GetString("theme"); selectedTheme != "" {
		_ = themeMgr.SetActive(selectedTheme)
	}

	services, err := codecore.NewAgentSessionServices(codecore.CreateAgentSessionServicesOptions{
		ExtDirs: cfg.ExtDirs,
	})
	var coordinator *codecore.RuntimeCoordinator
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not initialize agent services: %v\n", err)
	} else {
		services.AuthStorage = cfg.AuthStorage
		services.OAuthStore = cfg.OAuthStore
		services.Settings = cfg.Settings
		services.ModelReg.SetAuthProvider(cfg.AuthStorage)

		coordinator = codecore.NewRuntimeCoordinator(
			services,
			cfg.OAuthStore,
			cfg.AuthStorage,
			themeMgr,
			cfg.Settings,
			sessionMgr,
			cfg.Model,
			cfg.Provider,
			cfg.APIKey,
			cfg.CWD,
			agent.CurrentSessionID(),
		)
	}

	im := &InteractiveMode{
		config:              cfg,
		extRuntime:          extRuntime,
		extCtx:              extCtx,
		sessionManager:      sessionMgr,
		slashCommands:       codecore.NewSlashCommandManager(),
		sessionID:           agent.CurrentSessionID(),
		extensionStatuses:   make(map[string]string),
		streamingMessageIdx: -1,
		pendingToolMessages: make(map[string]int),
		coordinator:         coordinator,
	}

	// Wire UserBash hook to extension runtime via thread-safe registration
	_unregisterBashHook := tools.RegisterUserBashHook(func(command string) error {
		return extRuntime.FireUserBash(extCtx, extensions.UserBashEvent{
			Command:            command,
			ExcludeFromContext: false,
		})
	})
	_ = _unregisterBashHook // keep for future cleanup

	// Set up UI on the BTModel
	im.setupUI()
	im.extCtx.UI = im.extensionUIContext()

	return im
}

// Run starts the interactive mode.
func (im *InteractiveMode) Run() error {
	defer func() {
		im.extCtx.ExtensionRuntime.FireSessionShutdown(im.extCtx, extensions.SessionShutdownEvent{
			Reason: extensions.SessionQuit,
		})
		im.extRuntime.StopAllProcesses()
	}()

	// Fire session start
	im.extCtx.ExtensionRuntime.FireSessionStart(im.extCtx, extensions.SessionStartEvent{
		Type: extensions.SessionStartup,
	})

	im.programRunning = true
	defer func() {
		im.programRunning = false
	}()

	// Start Bubble Tea program (blocks until quit)
	_, err := im.program.Run()
	return err
}

func (im *InteractiveMode) setupUI() {
	status := im.statusText("")
	im.ui = ui.NewChatModel(status)
	im.applyActiveTheme()

	// Initialize metadata footer details
	im.ui.SetModel(im.config.Model.Name, string(im.config.Provider))
	im.ui.SetGitBranch(agentutils.GetGitBranch(im.config.CWD))
	def := providers.GuessModelDefinition(im.config.Provider, im.config.Model.Name)
	im.ui.SetTokenCount(0, def.ContextWindow)

	im.ui.OnMessage = func(msg tea.Msg) {
		im.handleCustomMessage(msg)
	}
	im.ui.OnSubmit = func(value string) {
		im.handleSubmit(value)
	}
	im.ui.OnAutocomplete = func(text string, cursor int) []ui.AutocompleteItem {
		return im.autocomplete(text, cursor)
	}
	im.ui.OnAction = func(action string) bool {
		return im.handleAppAction(action)
	}
	im.program = tea.NewProgram(im.ui, tea.WithAltScreen(), tea.WithMouseCellMotion())

	im.addWelcomeMessage()
}

func (im *InteractiveMode) handleAppAction(action string) bool {
	switch action {
	case "model.select":
		im.ui.ClearInput()
		im.showModelSelector("")
		return true
	case "settings.open":
		im.ui.ClearInput()
		im.showSettingsSelector()
		return true
	case "session.resume":
		im.ui.ClearInput()
		im.showSessionSelector("Resume session")
		return true
	case "thinking.cycle":
		current := im.settingString("thinkingLevel", "off")
		next := nextThinkingLevel(current)
		im.setUserSetting("thinkingLevel", next)
		im.addSystemMessage(fmt.Sprintf("Thinking level: %s", next))
		return true
	case "agent.abort":
		return im.abortAgent()
	}
	return false
}

func (im *InteractiveMode) abortAgent() bool {
	if im.agentCancel == nil {
		return false
	}
	im.agentCancel()
	im.agentCancel = nil
	im.ui.SetStatus(im.statusText("Aborting..."))
	im.addSystemMessage("Operation aborted.")
	return true
}

func (im *InteractiveMode) addWelcomeMessage() {
	if im.config.Model.Name == "" {
		welcome := "rho v" + version + " — Your local coding agent\n\nNo API keys configured. Use /login to set up a provider, or Ctrl+L to select a model."
		im.ui.AddMessage(agent.AgentMessage{
			Role:    ai.RoleAssistant,
			Content: welcome,
			Model:   "rho",
		})
		return
	}
	welcome := "rho v" + version + " — Your local coding agent\nType a message and press Enter to start. Ctrl+C to quit."
	im.ui.AddMessage(agent.AgentMessage{
		Role:    ai.RoleAssistant,
		Content: welcome,
		Model:   "rho",
	})
}

func (im *InteractiveMode) extensionUIContext() extensions.ExtensionUIContext {
	return extensions.ExtensionUIContext{
		Select:    im.extensionSelect,
		Confirm:   im.extensionConfirm,
		Input:     im.extensionInput,
		Notify:    im.extensionNotify,
		SetStatus: im.extensionSetStatus,
	}
}

func (im *InteractiveMode) extensionSelect(title string, options []string) (string, error) {
	if im.program == nil {
		return "", errors.New("extension UI is not running")
	}
	resp := make(chan uiStringResponse, 1)
	im.program.Send(uiSelectRequestMsg{Title: title, Options: options, Resp: resp})
	result := <-resp
	return result.Value, result.Err
}

func (im *InteractiveMode) extensionConfirm(title, message string) (bool, error) {
	if im.program == nil {
		return false, errors.New("extension UI is not running")
	}
	resp := make(chan uiBoolResponse, 1)
	im.program.Send(uiConfirmRequestMsg{Title: title, Message: message, Resp: resp})
	result := <-resp
	return result.Value, result.Err
}

func (im *InteractiveMode) extensionInput(title, placeholder string) (string, error) {
	if im.program == nil {
		return "", errors.New("extension UI is not running")
	}
	resp := make(chan uiStringResponse, 1)
	im.program.Send(uiInputRequestMsg{Title: title, Placeholder: placeholder, Resp: resp})
	result := <-resp
	return result.Value, result.Err
}

func (im *InteractiveMode) extensionNotify(message string, msgType string) {
	if strings.TrimSpace(message) == "" {
		return
	}
	if im.program == nil {
		im.addSystemMessage(message)
		return
	}
	im.program.Send(tui.AddMessageMsg{
		Role:    string(ai.RoleAssistant),
		Content: message,
		Model:   "rho",
	})
}

func (im *InteractiveMode) extensionSetStatus(key, text string) {
	if key == "" {
		return
	}
	if im.program == nil {
		if im.extensionStatuses == nil {
			im.extensionStatuses = make(map[string]string)
		}
		im.extensionStatuses[key] = text
		return
	}
	im.program.Send(AgentExtensionStatusMsg{Key: key, Text: text})
}

func (im *InteractiveMode) handleSubmit(value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}

	if im.pendingLoginProvider != "" {
		im.handlePendingLogin(value)
		return
	}

	// Check for slash commands.
	if strings.HasPrefix(value, "/") {
		if im.handleSlashCommand(value) {
			return
		}
	}

	// If no model is configured, show a helpful message instead of failing
	if im.config.Model.Name == "" {
		im.ui.AddMessage(agent.AgentMessage{
			Role:    ai.RoleAssistant,
			Content: "No model is configured. Use /login to set up an API key, or Ctrl+L to select a model.",
			Model:   "rho",
		})
		im.ui.ClearInput()
		return
	}

	// If no API key is resolved, show a helpful message
	if im.config.APIKey == "" && !im.hasOAuth() {
		im.ui.AddMessage(agent.AgentMessage{
			Role:    ai.RoleAssistant,
			Content: fmt.Sprintf("No API key configured for %s. Use /login %s to set one up, or Ctrl+L to select a different provider.", im.config.Provider, im.config.Provider),
			Model:   "rho",
		})
		im.ui.ClearInput()
		return
	}

	// Fire input event to extensions
	inputResult, err := im.extRuntime.FireInput(im.extCtx, extensions.InputEvent{
		Text:   value,
		Source: "interactive",
	})
	if err != nil {
		im.ui.AddMessage(agent.AgentMessage{
			Role:    ai.RoleAssistant,
			Content: fmt.Sprintf("Extension error: %v", err),
			Model:   im.config.Model.Name,
		})
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
	im.ui.AddMessage(userMsg)
	turnMessages := im.ui.Snapshot()
	im.ui.ClearInput()

	im.ui.SetStatus(im.statusText("Thinking..."))

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		im.agentCancel = cancel
		defer func() {
			im.agentCancel = nil
		}()

		// Fire turn start
		im.extRuntime.FireTurnStart(im.extCtx, extensions.TurnStartEvent{
			TurnIndex: len(turnMessages),
		})

		// Fire agent start
		im.extRuntime.FireAgentStart(im.extCtx)

		// Fire context event before running agent
		contextMsgs, ctxErr := im.extRuntime.FireContext(im.extCtx, extensions.ContextEvent{
			Messages: turnMessages,
		})
		if ctxErr == nil && len(contextMsgs) > 0 {
			// Prepend context messages
			newTurnMessages := make([]agent.AgentMessage, 0, len(contextMsgs)+len(turnMessages))
			newTurnMessages = append(newTurnMessages, contextMsgs...)
			newTurnMessages = append(newTurnMessages, turnMessages...)
			turnMessages = newTurnMessages
		}

		agentMsg, newMessages, err := im.runAgent(ctx, value, turnMessages)
		if err != nil {
			im.program.Send(tui.AddMessageMsg{
				Role:    string(ai.RoleAssistant),
				Content: fmt.Sprintf("Error: %v", err),
				Model:   im.config.Model.Name,
			})
			newMessages = append(newMessages, agent.AgentMessage{
				Role:         ai.RoleAssistant,
				Content:      fmt.Sprintf("Error: %v", err),
				Model:        im.config.Model.Name,
				ErrorMessage: err.Error(),
				Timestamp:    time.Now().UnixMilli(),
			})
		} else if agentMsg != nil {
			im.program.Send(agentLoopFinalMsg{Message: agentMsg})
		}

		// Save session
		priorMessages := priorConversation(turnMessages)
		sessionMessages := append(priorMessages, newMessages...)
		header := agent.SessionHeader{
			ID:        im.sessionID,
			Timestamp: time.Now().Format(time.RFC3339),
			CWD:       im.config.CWD,
		}
		if err := im.sessionManager.Save(im.sessionID, header, sessionMessages); err != nil {
			im.program.Send(tui.AddMessageMsg{
				Role:    string(ai.RoleAssistant),
				Content: fmt.Sprintf("Session save error: %v", err),
				Model:   im.config.Model.Name,
			})
		}

		// Fire turn end
		var respMsg agent.AgentMessage
		if agentMsg != nil {
			respMsg = *agentMsg
		}
		im.extRuntime.FireTurnEnd(im.extCtx, extensions.TurnEndEvent{
			TurnIndex: len(turnMessages),
			Message:   respMsg,
		})

		// Fire agent end
		im.extRuntime.FireAgentEnd(im.extCtx, extensions.AgentEndEvent{
			Messages: sessionMessages,
		})

		im.program.Send(tui.AgentStatusMsg{
			Text: im.statusText(""),
		})
	}()
}

func (im *InteractiveMode) addSystemMessage(content string) {
	im.ui.AddMessage(agent.AgentMessage{
		Role:      ai.RoleAssistant,
		Content:   content,
		Model:     "rho",
		Timestamp: time.Now().UnixMilli(),
	})
	// Show brief messages as a toast overlay
	if len(content) < 80 && !strings.Contains(content, "\n") {
		im.ui.ShowToast(content)
	}
}

func (im *InteractiveMode) statusText(activity string) string {
	th := tui.DefaultTheme
	modelLabel := "no model"
	if im.config.Model.Name != "" {
		modelLabel = fmt.Sprintf("%s/%s", im.config.Provider, im.config.Model.Name)
	}
	var parts []string
	// Left: app name + model
	parts = append(parts, th.BoldAccent("ρ"))
	if im.config.Model.Name != "" {
		parts = append(parts, th.Tag(modelLabel))
	}
	// Center: activity or cwd
	if activity != "" {
		parts = append(parts, th.Accent(activity))
	} else {
		parts = append(parts, th.Muted(shortenPath(im.config.CWD)))
	}
	// Right: session name
	if name := im.sessionName(im.sessionID); name != "" {
		parts = append(parts, th.Muted("@"+name))
	}
	// Extension statuses
	for _, text := range im.extensionStatuses {
		if strings.TrimSpace(text) != "" {
			parts = append(parts, th.Muted(text))
		}
	}
	sep := th.Muted(" · ")
	return strings.Join(parts, sep)
}

func (im *InteractiveMode) runAgent(ctx context.Context, prompt string, turnMessages []agent.AgentMessage) (*agent.AgentMessage, []agent.AgentMessage, error) {
	// Create compacter for context compaction
	compacter := compaction.NewCompacter(compaction.DefaultCompactionSettings())

	loop := agent.NewAgentLoop(agent.AgentLoopConfig{
		Model:             im.config.Model,
		SystemPrompt:      im.config.SystemPrompt,
		APIKey:            im.config.APIKey,
		ToolExecutionMode: agent.ToolExecutionSequential,
		Signal:            ctx,
		BeforeProviderRequest: func(ctx ai.Context) (ai.Context, error) {
			if im.extRuntime != nil {
				result, err := im.extRuntime.FireBeforeProviderRequest(im.extCtx, extensions.BeforeProviderRequestEvent{
					Payload: ctx,
				})
				if err == nil && result != nil {
					if modifiedCtx, ok := result.(ai.Context); ok {
						return modifiedCtx, nil
					}
				}
			}
			return ctx, nil
		},
		CompactFn: func(messages []agent.AgentMessage) ([]agent.AgentMessage, error) {
			compactMsgs := make([]compaction.Message, len(messages))
			for i, m := range messages {
				compactMsgs[i] = compaction.Message{
					Role:    string(m.Role),
					Content: m.Content,
				}
			}
			if !compacter.ShouldCompactWith(compactMsgs, 100000) {
				return messages, nil
			}
			cutPoint := compaction.FindCutPoint(compactMsgs, 100000, compaction.DefaultCompactionSettings())
			if cutPoint == nil {
				return messages, nil
			}
			result, err := compaction.Compact(compactMsgs, cutPoint, nil)
			if err != nil {
				return messages, nil
			}
			newMsgs := make([]agent.AgentMessage, len(result.Messages))
			for i, m := range result.Messages {
				newMsgs[i] = agent.AgentMessage{
					Role:    ai.Role(m.Role),
					Content: m.Content,
				}
			}
			return newMsgs, nil
		},
	})

	// Apply extension hooks to the agent loop
	extensions.InstallHooks(im.extRuntime, im.extCtx, loop)

	// Merge built-in tools with extension custom tools
	builtinTools := tools.AllTools(im.config.CWD)
	extTools := im.extRuntime.GetCustomTools()
	allTools := extensions.MergeTools(builtinTools, extTools)
	systemPromptText := systemprompt.Build(systemprompt.BuildOptions{
		BaseTemplate:          im.config.SystemPrompt,
		Tools:                 allTools,
		CWD:                   im.config.CWD,
		ModelName:             im.config.Model.Name,
		ProviderName:          string(im.config.Provider),
		ExtensionInstructions: im.extRuntime.GetPromptPatches(),
		Skills:                im.extRuntime.GetCustomSkills(),
	})

	context := agent.AgentContext{
		SystemPrompt: systemPromptText,
		Model:        im.config.Model,
		Messages:     priorConversation(turnMessages),
		Tools:        allTools,
	}

	prompts := []agent.AgentMessage{
		{Role: ai.RoleUser, Content: prompt, Timestamp: time.Now().UnixMilli()},
	}

	emit := func(event agent.AgentEvent) error {
		im.program.Send(agentLoopEventMsg{Event: event})
		return nil
	}

	results, err := loop.Run(prompts, context, emit)
	if err != nil {
		return nil, nil, err
	}

	for i := len(results) - 1; i >= 0; i-- {
		if results[i].Role == ai.RoleAssistant {
			return &results[i], results, nil
		}
	}

	msg := &agent.AgentMessage{
		Role:    ai.RoleAssistant,
		Content: "No response generated.",
		Model:   im.config.Model.Name,
	}
	return msg, append(results, *msg), nil
}

// conversationMessages returns non-hidden, non-system messages from the given slice.
func conversationMessages(messages []agent.AgentMessage) []agent.AgentMessage {
	if len(messages) == 0 {
		return nil
	}
	out := make([]agent.AgentMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.Hide || msg.Model == "rho" {
			continue
		}
		out = append(out, msg)
	}
	return out
}

// priorConversation returns messages from the conversation, excluding the trailing user message.
func priorConversation(messages []agent.AgentMessage) []agent.AgentMessage {
	all := conversationMessages(messages)
	if len(all) == 0 {
		return nil
	}
	if all[len(all)-1].Role == ai.RoleUser {
		return all[:len(all)-1]
	}
	return all
}

// ─── Handle custom messages ────────────────────────────────────────────────

// This function is called from the Bubble Tea update loop to process
// asynchronous agent messages.
func (im *InteractiveMode) handleCustomMessage(msg tea.Msg) {
	switch m := msg.(type) {
	case tui.AddMessageMsg:
		amsg := agent.AgentMessage{
			Role:    ai.Role(m.Role),
			Content: m.Content,
			Model:   m.Model,
		}
		if m.ToolCall != "" {
			amsg.ToolName = m.ToolCall
		}
		im.ui.AddMessage(amsg)

	case tui.AgentStatusMsg:
		im.ui.SetStatus(m.Text)

	case agentLoopEventMsg:
		im.applyAgentLoopEvent(m.Event)

	case agentLoopFinalMsg:
		im.applyAgentLoopFinal(m.Message)

	case uiSelectRequestMsg:
		items := make([]ui.AutocompleteItem, 0, len(m.Options))
		for _, option := range m.Options {
			items = append(items, ui.AutocompleteItem{Value: option, Label: option})
		}
		if len(items) == 0 {
			m.Resp <- uiStringResponse{Err: errors.New("no options available")}
			return
		}
		im.ui.OpenModalSelector(m.Title, items, func(item ui.AutocompleteItem) {
			m.Resp <- uiStringResponse{Value: item.Value}
		}, func() {
			m.Resp <- uiStringResponse{Err: errors.New("selection cancelled")}
		})

	case uiConfirmRequestMsg:
		im.ui.OpenModalConfirm(m.Title, m.Message, func() {
			m.Resp <- uiBoolResponse{Value: true}
		}, func() {
			m.Resp <- uiBoolResponse{Value: false}
		})

	case uiInputRequestMsg:
		im.ui.OpenModalPrompt(m.Title, m.Placeholder, func(value string) {
			m.Resp <- uiStringResponse{Value: value}
		}, func() {
			m.Resp <- uiStringResponse{Err: errors.New("input cancelled")}
		})

	case AgentExtensionStatusMsg:
		if im.extensionStatuses == nil {
			im.extensionStatuses = make(map[string]string)
		}
		if strings.TrimSpace(m.Text) == "" {
			delete(im.extensionStatuses, m.Key)
		} else {
			im.extensionStatuses[m.Key] = m.Text
		}
		im.ui.SetStatus(im.statusText(""))

	case uiClosePromptMsg:
		im.ui.CloseModal()
	}
}

func (im *InteractiveMode) applyAgentLoopEvent(event agent.AgentEvent) {
	switch event.Type {
	case "agent_start":
		im.streamingMessageIdx = -1
		im.pendingToolMessages = make(map[string]int)
		im.ensureStreamingAssistantMessage()
		im.ui.SetStatus(im.statusText("Thinking..."))
	case "text_delta":
		msg := im.ensureStreamingAssistantMessage()
		msg.Content += event.Delta
		im.ui.UpdateMessage(im.streamingMessageIdx, msg)
		im.ui.SetStatus(im.statusText("Receiving..."))
	case "toolcall_start", "toolcall_delta":
		if event.ToolCall == nil || event.ToolCall.Name == "" {
			return
		}
		msg := im.ensureStreamingAssistantMessage()
		if event.ToolCall.ID != "" && !messageHasToolCall(msg, event.ToolCall.ID) {
			msg.ToolCalls = append(msg.ToolCalls, *event.ToolCall)
			im.ui.UpdateMessage(im.streamingMessageIdx, msg)
		}
		im.ensureToolMessage(event.ToolCall, "Preparing "+formatToolArgs(event.ToolCall.Arguments), false)
	case "toolcall_end":
		if event.ToolCall == nil {
			return
		}
		msg := im.ensureStreamingAssistantMessage()
		if !messageHasToolCall(msg, event.ToolCall.ID) {
			msg.ToolCalls = append(msg.ToolCalls, *event.ToolCall)
			im.ui.UpdateMessage(im.streamingMessageIdx, msg)
		}
		im.ensureToolMessage(event.ToolCall, "Queued "+formatToolArgs(event.ToolCall.Arguments), false)
	case "message_end":
		if event.Message == nil {
			return
		}
		if event.Message.StopReason == ai.StopReasonToolUse {
			if im.streamingMessageIdx >= 0 {
				im.ui.UpdateMessage(im.streamingMessageIdx, *event.Message)
			}
			im.streamingMessageIdx = -1
		}
		if event.Message.StopReason == ai.StopReasonAborted {
			msg := im.ensureStreamingAssistantMessage()
			msg.StopReason = ai.StopReasonAborted
			msg.ErrorMessage = "Operation aborted"
			im.ui.UpdateMessage(im.streamingMessageIdx, msg)
			im.streamingMessageIdx = -1
		}
	case "tool_execution_start":
		if event.ToolCall == nil {
			return
		}
		im.ensureToolMessage(event.ToolCall, "Running...", false)
		im.ui.SetStatus(im.statusText("Running " + event.ToolCall.Name + "..."))
	case "tool_execution_end":
		if event.ToolCall == nil {
			return
		}
		content := event.Content
		if strings.TrimSpace(content) == "" {
			content = "Done."
		}
		im.ensureToolMessage(event.ToolCall, content, event.IsError)
		im.ui.SetStatus(im.statusText("Thinking..."))
	case "error":
		msg := im.ensureStreamingAssistantMessage()
		msg.ErrorMessage = event.Error
		if msg.Content == "" {
			msg.Content = event.Error
		}
		im.ui.UpdateMessage(im.streamingMessageIdx, msg)
		im.ui.SetStatus(im.statusText("Error"))
	case "agent_end":
		im.ui.SetStatus(im.statusText(""))
	}
}

func (im *InteractiveMode) applyAgentLoopFinal(msg *agent.AgentMessage) {
	if msg == nil {
		im.streamingMessageIdx = -1
		return
	}
	if msg.Model == "" {
		msg.Model = im.config.Model.Name
	}
	if im.streamingMessageIdx >= 0 {
		im.ui.UpdateMessage(im.streamingMessageIdx, *msg)
	} else {
		im.ui.AddMessage(*msg)
	}
	im.streamingMessageIdx = -1
	im.ui.SetStatus(im.statusText(""))
}

func (im *InteractiveMode) ensureStreamingAssistantMessage() agent.AgentMessage {
	if msg, ok := im.ui.Message(im.streamingMessageIdx); ok {
		return msg
	}
	msg := agent.AgentMessage{
		Role:      ai.RoleAssistant,
		Model:     im.config.Model.Name,
		Provider:  im.config.Provider,
		Timestamp: time.Now().UnixMilli(),
	}
	im.ui.AddMessage(msg)
	im.streamingMessageIdx = im.ui.LastMessageIndex()
	return msg
}

func (im *InteractiveMode) ensureToolMessage(tc *ai.ToolCall, content string, isError bool) {
	if tc == nil {
		return
	}
	msg := agent.AgentMessage{
		Role:       ai.RoleToolResult,
		ToolCallID: tc.ID,
		ToolName:   tc.Name,
		Content:    content,
		IsError:    isError,
		Model:      im.config.Model.Name,
		Timestamp:  time.Now().UnixMilli(),
	}
	if im.pendingToolMessages == nil {
		im.pendingToolMessages = make(map[string]int)
	}
	if idx, ok := im.pendingToolMessages[tc.ID]; ok {
		im.ui.UpdateMessage(idx, msg)
		return
	}
	im.ui.AddMessage(msg)
	im.pendingToolMessages[tc.ID] = im.ui.LastMessageIndex()
}

func (im *InteractiveMode) canSendUI() bool {
	return im.program != nil && im.programRunning
}

func (im *InteractiveMode) postSystemMessage(message string) {
	if im.canSendUI() {
		im.program.Send(tui.AddMessageMsg{
			Role:    string(ai.RoleAssistant),
			Content: message,
			Model:   "rho",
		})
	} else {
		im.addSystemMessage(message)
	}
}

type uiClosePromptMsg struct{}
