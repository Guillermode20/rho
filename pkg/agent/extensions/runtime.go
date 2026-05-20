package extensions

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/agent/skills"
	agentutils "github.com/earendil-works/rho/pkg/agent/utils"
)

// Lock hierarchy (to prevent deadlocks):
//  1. Runtime.mu (highest) – protects extension list, processes, watchers, skills, prompts
//  2. ExtensionProcess.mu – protects process state, ui reference, pending RPC channels
//
// Rules:
//  - Never acquire ExtensionProcess.mu while holding Runtime.mu.
//    Exception: StopProcess releases Runtime.mu before calling proc.Stop().
//  - Event handlers (Fire*) hold Runtime.mu (R lock) while calling user callbacks.
//    Callbacks that need ExtensionProcess.mu must release it before calling proc.Call().
//  - HandleSlashCommand releases the R lock before invoking the handler.
//  - ReloadExtensionFromDir acquires and releases Runtime.mu via StopProcess/Unregister/LoadExtensionFromDir
//    sequentially, never holding both locks at once.

// Runtime manages extension lifecycle and event dispatch.
type Runtime struct {
	mu            sync.RWMutex
	extensions    []*ExtensionDef
	processes     map[string]*ExtensionProcess
	watchers      map[string]*agentutils.FileWatcher
	skills        map[string][]skills.Skill
	promptPatches map[string][]string
	events        *EventBus
}

// NewRuntime creates a new extension runtime.
func NewRuntime() *Runtime {
	return &Runtime{
		processes:     make(map[string]*ExtensionProcess),
		watchers:      make(map[string]*agentutils.FileWatcher),
		skills:        make(map[string][]skills.Skill),
		promptPatches: make(map[string][]string),
		events:        NewEventBus(),
	}
}

// Register adds an extension to the runtime.
func (r *Runtime) Register(ext *ExtensionDef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Prevent duplicate extensions by name during reload
	for i, existing := range r.extensions {
		if existing.Name == ext.Name {
			r.extensions[i] = ext
			return
		}
	}
	r.extensions = append(r.extensions, ext)
}

// Unregister removes an extension from the runtime list.
func (r *Runtime) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, ext := range r.extensions {
		if ext.Name == name {
			r.extensions = append(r.extensions[:i], r.extensions[i+1:]...)
			break
		}
	}
	delete(r.skills, name)
	delete(r.promptPatches, name)
}

// RegisterProcess stores an active process wrapper.
func (r *Runtime) RegisterProcess(name string, proc *ExtensionProcess) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.processes[name] = proc
}

// StopProcess stops and removes an active process.
func (r *Runtime) StopProcess(name string) {
	r.mu.Lock()
	proc, ok := r.processes[name]
	if ok {
		delete(r.processes, name)
	}
	r.mu.Unlock()
	if ok && proc != nil {
		proc.Stop()
	}
}

// StopAllProcesses stops all supervised processes.
func (r *Runtime) StopAllProcesses() {
	r.mu.Lock()
	procs := make([]*ExtensionProcess, 0, len(r.processes))
	for _, proc := range r.processes {
		procs = append(procs, proc)
	}
	r.processes = make(map[string]*ExtensionProcess)

	// Also stop and clear all watchers
	for _, fw := range r.watchers {
		if fw != nil {
			fw.Stop()
		}
	}
	r.watchers = make(map[string]*agentutils.FileWatcher)
	r.mu.Unlock()

	for _, proc := range procs {
		if proc != nil {
			proc.Stop()
		}
	}
}

// RegisterSkills registers skills associated with an extension name.
func (r *Runtime) RegisterSkills(name string, list []skills.Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[name] = list
}

// GetCustomSkills returns all skills registered from extensions.
func (r *Runtime) GetCustomSkills() []skills.Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var all []skills.Skill
	for _, list := range r.skills {
		all = append(all, list...)
	}
	return all
}

// RegisterPromptPatches registers prompt patches for an extension name.
func (r *Runtime) RegisterPromptPatches(name string, patches []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.promptPatches[name] = patches
}

// GetPromptPatches returns all system prompt patches from extensions.
func (r *Runtime) GetPromptPatches() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var all []string
	for _, list := range r.promptPatches {
		all = append(all, list...)
	}
	return all
}

// WatchExtensionDir watches the extension directory for changes to trigger hot-reload.
func (r *Runtime) WatchExtensionDir(name string, dir string, ui ExtensionUIContext) {
	r.mu.Lock()
	// Stop existing watcher if any
	if fw, ok := r.watchers[name]; ok && fw != nil {
		fw.Stop()
	}
	r.mu.Unlock()

	fw, err := agentutils.WatchDir(dir, func(path string) {
		// Ignore temporary/hidden files or metadata files that are not code/configs
		base := filepath.Base(path)
		if strings.HasPrefix(base, ".") || strings.HasSuffix(base, "~") {
			return
		}
		// Trigger reload
		ui.Notify(fmt.Sprintf("Reloading extension '%s' due to file changes...", name), "info")
		_ = r.ReloadExtensionFromDir(name, dir, ui)
	})

	if err == nil {
		r.mu.Lock()
		r.watchers[name] = fw
		r.mu.Unlock()
		fw.Start()
	}
}

// ReloadExtensionFromDir stops, re-loads, and restarts the extension in-place.
// On failure, the extension is left unregistered (restoring the old ExtensionDef
// is not viable because the old ExtensionProcess is already stopped/killed).
func (r *Runtime) ReloadExtensionFromDir(name string, dir string, ui ExtensionUIContext) error {
	r.StopProcess(name)
	r.Unregister(name)

	err := LoadExtensionFromDir(dir, r, ui)
	if err != nil {
		ui.Notify(fmt.Sprintf("Failed to reload extension '%s': %v (extension has been removed)", name, err), "error")
		return err
	}

	ui.Notify(fmt.Sprintf("Successfully reloaded extension '%s'!", name), "success")
	return nil
}

// GetAllExtensions returns all registered extensions.
func (r *Runtime) GetAllExtensions() []*ExtensionDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*ExtensionDef, len(r.extensions))
	copy(result, r.extensions)
	return result
}

// GetCustomTools returns all tools from all extensions.
func (r *Runtime) GetCustomTools() []agent.AgentTool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var tools []agent.AgentTool
	for _, ext := range r.extensions {
		for _, td := range ext.CustomTools {
			tools = append(tools, agent.AgentTool{
				Name:        td.Name,
				Description: td.Description,
				Parameters:  td.Parameters,
				Execute:     td.Execute,
			})
		}
	}
	return tools
}

// GetCustomProviders returns all custom providers.
func (r *Runtime) GetCustomProviders() []ProviderConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var providers []ProviderConfig
	for _, ext := range r.extensions {
		providers = append(providers, ext.CustomProviders...)
	}
	return providers
}

// GetSlashCommands returns all slash commands sorted by name.
func (r *Runtime) GetSlashCommands() []RegisteredCommand {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var cmds []RegisteredCommand
	for _, ext := range r.extensions {
		for _, sc := range ext.SlashCommands {
			cmds = append(cmds, RegisteredCommand{
				Name:        sc.Name,
				Description: sc.Description,
				Args:        sc.Args,
				Handler:     sc.Handler,
			})
		}
	}
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })
	return cmds
}

// GetKeybindings returns all extension keybindings.
func (r *Runtime) GetKeybindings() []ExtensionShortcut {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var kbs []ExtensionShortcut
	for _, ext := range r.extensions {
		kbs = append(kbs, ext.Keybindings...)
	}
	return kbs
}

// GetCLIFlags returns all extension CLI flags.
func (r *Runtime) GetCLIFlags() []ExtensionFlag {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var flags []ExtensionFlag
	for _, ext := range r.extensions {
		flags = append(flags, ext.CLIFlags...)
	}
	return flags
}

// GetMessageRenderers returns all message renderers.
func (r *Runtime) GetMessageRenderers() []MessageRenderer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var renderers []MessageRenderer
	for _, ext := range r.extensions {
		renderers = append(renderers, ext.MessageRenderers...)
	}
	return renderers
}

// GetEventBus returns the runtime's event bus for inter-extension communication.
func (r *Runtime) GetEventBus() *EventBus {
	return r.events
}

// ============================================================================
// Event Dispatch
// ============================================================================

// buildContext builds the ExtensionContext for all handlers.
func (r *Runtime) buildContext(opts ExtensionContextOpts) ExtensionContext {
	return ExtensionContext{
		HasUI:            opts.HasUI,
		CWD:              opts.CWD,
		Abort:            opts.Abort,
		Shutdown:         opts.Shutdown,
		ExtensionRuntime: r,
		UI: ExtensionUIContext{
			Select:    opts.Select,
			Confirm:   opts.Confirm,
			Input:     opts.Input,
			Notify:    opts.Notify,
			SetStatus: opts.SetStatus,
		},
	}
}

// ExtensionContextOpts provides dependencies for building ExtensionContext.
type ExtensionContextOpts struct {
	HasUI     bool
	CWD       string
	Abort     func()
	Shutdown  func()
	Select    func(title string, options []string) (string, error)
	Confirm   func(title, message string) (bool, error)
	Input     func(title, placeholder string) (string, error)
	Notify    func(message string, msgType string)
	SetStatus func(key, text string)
}

// FireSessionStart dispatches session start to all extensions.
func (r *Runtime) FireSessionStart(ctx ExtensionContext, event SessionStartEvent) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ext := range r.extensions {
		if ext.OnSessionStart != nil {
			if err := ext.OnSessionStart(ctx, event); err != nil {
				return r.wrapError(ext.Name, "session_start", err)
			}
		}
	}
	return nil
}

// FireSessionShutdown dispatches session shutdown.
func (r *Runtime) FireSessionShutdown(ctx ExtensionContext, event SessionShutdownEvent) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ext := range r.extensions {
		if ext.OnSessionShutdown != nil {
			if err := ext.OnSessionShutdown(ctx, event); err != nil {
				return r.wrapError(ext.Name, "session_shutdown", err)
			}
		}
	}
	return nil
}

// FireAgentStart dispatches agent start.
func (r *Runtime) FireAgentStart(ctx ExtensionContext) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ext := range r.extensions {
		if ext.OnAgentStart != nil {
			if err := ext.OnAgentStart(ctx); err != nil {
				return r.wrapError(ext.Name, "agent_start", err)
			}
		}
	}
	return nil
}

// FireAgentEnd dispatches agent end.
func (r *Runtime) FireAgentEnd(ctx ExtensionContext, event AgentEndEvent) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ext := range r.extensions {
		if ext.OnAgentEnd != nil {
			if err := ext.OnAgentEnd(ctx, event); err != nil {
				return r.wrapError(ext.Name, "agent_end", err)
			}
		}
	}
	return nil
}

// FireTurnStart dispatches turn start.
func (r *Runtime) FireTurnStart(ctx ExtensionContext, event TurnStartEvent) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ext := range r.extensions {
		if ext.OnTurnStart != nil {
			if err := ext.OnTurnStart(ctx, event); err != nil {
				return r.wrapError(ext.Name, "turn_start", err)
			}
		}
	}
	return nil
}

// FireTurnEnd dispatches turn end.
func (r *Runtime) FireTurnEnd(ctx ExtensionContext, event TurnEndEvent) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ext := range r.extensions {
		if ext.OnTurnEnd != nil {
			if err := ext.OnTurnEnd(ctx, event); err != nil {
				return r.wrapError(ext.Name, "turn_end", err)
			}
		}
	}
	return nil
}

// FireContext dispatches context event (can modify messages).
func (r *Runtime) FireContext(ctx ExtensionContext, event ContextEvent) ([]agent.AgentMessage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	messages := event.Messages
	for _, ext := range r.extensions {
		if ext.OnContext != nil {
			result, err := ext.OnContext(ctx, ContextEvent{Messages: messages})
			if err != nil {
				return nil, r.wrapError(ext.Name, "context", err)
			}
			if result != nil {
				messages = result
			}
		}
	}
	return messages, nil
}

// FireBeforeProviderRequest dispatches before-provider-request.
func (r *Runtime) FireBeforeProviderRequest(ctx ExtensionContext, event BeforeProviderRequestEvent) (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	payload := event.Payload
	for _, ext := range r.extensions {
		if ext.OnBeforeProviderRequest != nil {
			result, err := ext.OnBeforeProviderRequest(ctx, BeforeProviderRequestEvent{Payload: payload})
			if err != nil {
				return nil, r.wrapError(ext.Name, "before_provider_request", err)
			}
			if result != nil {
				payload = result
			}
		}
	}
	return payload, nil
}

// FireBeforeAgentStart dispatches before-agent-start.
func (r *Runtime) FireBeforeAgentStart(ctx ExtensionContext, event BeforeAgentStartEvent) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ext := range r.extensions {
		if ext.OnBeforeAgentStart != nil {
			if err := ext.OnBeforeAgentStart(ctx, event); err != nil {
				return r.wrapError(ext.Name, "before_agent_start", err)
			}
		}
	}
	return nil
}

// FireInput dispatches input events. First handler that returns "handled" or "transform" wins.
func (r *Runtime) FireInput(ctx ExtensionContext, event InputEvent) (*InputEventResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ext := range r.extensions {
		if ext.OnInput != nil {
			result, err := ext.OnInput(ctx, event)
			if err != nil {
				return nil, r.wrapError(ext.Name, "input", err)
			}
			if result != nil && result.Action != "continue" {
				return result, nil
			}
		}
	}
	return nil, nil
}

// FireToolCall dispatches tool call events. First block wins.
func (r *Runtime) FireToolCall(ctx ExtensionContext, event ToolCallEvent) (*ToolCallEventResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ext := range r.extensions {
		if ext.OnToolCall != nil {
			result, err := ext.OnToolCall(ctx, event)
			if err != nil {
				return nil, r.wrapError(ext.Name, "tool_call", err)
			}
			if result != nil && result.Block {
				return result, nil
			}
		}
	}
	return nil, nil
}

// FireToolResult dispatches tool result events.
func (r *Runtime) FireToolResult(ctx ExtensionContext, event ToolResultEvent) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ext := range r.extensions {
		if ext.OnToolResult != nil {
			if err := ext.OnToolResult(ctx, event); err != nil {
				return r.wrapError(ext.Name, "tool_result", err)
			}
		}
	}
	return nil
}

// FireUserBash dispatches user bash events.
func (r *Runtime) FireUserBash(ctx ExtensionContext, event UserBashEvent) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ext := range r.extensions {
		if ext.OnUserBash != nil {
			if err := ext.OnUserBash(ctx, event); err != nil {
				return r.wrapError(ext.Name, "user_bash", err)
			}
		}
	}
	return nil
}

// HandleSlashCommand finds and executes a slash command by name.
func (r *Runtime) HandleSlashCommand(ctx ExtensionContext, name string, args []string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ext := range r.extensions {
		for _, sc := range ext.SlashCommands {
			if sc.Name == name {
				if sc.Handler != nil {
					r.mu.RUnlock()
					err := sc.Handler(ctx, args)
					r.mu.RLock()
					return err
				}
			}
		}
	}
	return fmt.Errorf("unknown slash command: %s", name)
}

func (r *Runtime) wrapError(extName, eventName string, err error) error {
	return &ExtensionError{
		ExtensionName: extName,
		Message:       fmt.Sprintf("error in %s handler", eventName),
		Err:           err,
	}
}

// ============================================================================
// EventBus for inter-extension communication
// ============================================================================

// EventBus allows extensions to emit and listen for custom events.
type EventBus struct {
	mu        sync.RWMutex
	listeners map[string][]func(event interface{})
}

// NewEventBus creates a new EventBus.
func NewEventBus() *EventBus {
	return &EventBus{
		listeners: make(map[string][]func(event interface{})),
	}
}

// On subscribes to a custom event type.
func (eb *EventBus) On(eventType string, handler func(event interface{})) func() {
	eb.mu.Lock()
	eb.listeners[eventType] = append(eb.listeners[eventType], handler)
	idx := len(eb.listeners[eventType]) - 1
	eb.mu.Unlock()
	return func() {
		eb.mu.Lock()
		listeners := eb.listeners[eventType]
		if idx < len(listeners) {
			eb.listeners[eventType] = append(listeners[:idx], listeners[idx+1:]...)
		}
		eb.mu.Unlock()
	}
}

// Emit dispatches an event to all listeners of its type.
func (eb *EventBus) Emit(eventType string, event interface{}) {
	eb.mu.RLock()
	listeners := eb.listeners[eventType]
	eb.mu.RUnlock()
	for _, listener := range listeners {
		listener(event)
	}
}
