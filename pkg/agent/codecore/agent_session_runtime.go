package codecore

import (
	"fmt"
	"sync"
	"time"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/agent/auth"
	"github.com/earendil-works/rho/pkg/agent/compaction"
	"github.com/earendil-works/rho/pkg/agent/extensions"
	"github.com/earendil-works/rho/pkg/agent/tools"
	"github.com/earendil-works/rho/pkg/ai"
)

// CreateAgentSessionRuntimeOptions configures a new session runtime.
type CreateAgentSessionRuntimeOptions struct {
	Model         ai.Model
	SystemPrompt  string
	APIKey        string
	CWD           string
	ThinkingLevel ai.ThinkingLevel
	Extensions    *extensions.Runtime
	Settings      *SettingsManager
	SessionMgr    *agent.SessionManager
	AuthStorage   *auth.AuthStorage
	ModelReg      *ModelRegistry
}

// AgentSessionRuntimeDiagnostic is a diagnostic message from the runtime.
type AgentSessionRuntimeDiagnostic struct {
	Type    string `json:"type"` // "info", "warning", "error"
	Message string `json:"message"`
}

// AgentSessionRuntime manages the full session lifecycle.
// It integrates the agent loop, extensions, compaction, settings, and session manager.
type AgentSessionRuntime struct {
	mu         sync.RWMutex
	config     AgentSessionConfig
	agent      *agent.AgentLoop
	context    agent.AgentContext
	messages   []agent.AgentMessage
	extRuntime *extensions.Runtime
	settings   *SettingsManager
	sessionMgr *agent.SessionManager
	auth       *auth.AuthStorage
	modelReg   *ModelRegistry

	sessionID   string
	started     bool
	stopped     bool
	turnCount   int
	startTime   time.Time
	totalTokens int
	totalCost   float64
	toolCalls   int

	uiContext   extensions.ExtensionUIContext
	hasUI       bool

	diagnostics []AgentSessionRuntimeDiagnostic
}

// NewAgentSessionRuntime creates a new session runtime.
func NewAgentSessionRuntime(sessionID string, opts CreateAgentSessionRuntimeOptions) *AgentSessionRuntime {
	// Create compacter for context compaction
	compacter := compaction.NewCompacter(compaction.DefaultCompactionSettings())

	loop := agent.NewAgentLoop(agent.AgentLoopConfig{
		Model:         opts.Model,
		SystemPrompt:  opts.SystemPrompt,
		APIKey:        opts.APIKey,
		ThinkingLevel: opts.ThinkingLevel,
		BeforeProviderRequest: func(ctx ai.Context) (ai.Context, error) {
			if opts.Extensions != nil {
				extCtx := extensions.ExtensionContext{
					HasUI: false,
					CWD:   opts.CWD,
				}
				result, err := opts.Extensions.FireBeforeProviderRequest(extCtx, extensions.BeforeProviderRequestEvent{
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
			// Convert agent messages to compaction messages
			compactMsgs := make([]compaction.Message, len(messages))
			for i, m := range messages {
				compactMsgs[i] = compaction.Message{
					Role:    string(m.Role),
					Content: m.Content,
				}
			}

			// Check if compaction is needed (estimate 100K context window)
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

			// Convert back to agent messages
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

	return &AgentSessionRuntime{
		config: AgentSessionConfig{
			Model:         opts.Model,
			SystemPrompt:  opts.SystemPrompt,
			APIKey:        opts.APIKey,
			CWD:           opts.CWD,
			ThinkingLevel: opts.ThinkingLevel,
		},
		agent:      loop,
		extRuntime: opts.Extensions,
		settings:   opts.Settings,
		sessionMgr: opts.SessionMgr,
		auth:       opts.AuthStorage,
		modelReg:   opts.ModelReg,
		sessionID:  sessionID,
		startTime:  time.Now(),
	}
}

// Start begins the session and fires extension hooks.
func (r *AgentSessionRuntime) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started {
		return fmt.Errorf("session already started")
	}
	r.started = true
	r.startTime = time.Now()

	// Fire session start event
	if r.extRuntime != nil {
		extCtx := r.buildExtensionContext()
		r.extRuntime.FireSessionStart(extCtx, extensions.SessionStartEvent{
			Type: extensions.SessionNew,
		})
	}

	r.AddDiagnostic("info", "Session started")
	return nil
}

// Stop ends the session and fires shutdown hooks.
func (r *AgentSessionRuntime) Stop() error {
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return fmt.Errorf("session already stopped")
	}
	r.stopped = true
	r.mu.Unlock()

	if r.extRuntime != nil {
		extCtx := r.buildExtensionContext()
		r.extRuntime.FireSessionShutdown(extCtx, extensions.SessionShutdownEvent{
			Reason: extensions.SessionQuit,
		})
	}

	r.AddDiagnostic("info", "Session stopped")
	return nil
}

// SendMessage sends a user message and returns the assistant response.
func (r *AgentSessionRuntime) SendMessage(content string) (*agent.AgentMessage, error) {
	return r.SendMessageStream(content, nil, func(event agent.AgentEvent) error { return nil })
}

// SendMessageStream sends a user message and streams assistant response events.
func (r *AgentSessionRuntime) SendMessageStream(content string, images []ai.ImageContent, callback agent.AgentEventCallback) (*agent.AgentMessage, error) {
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return nil, fmt.Errorf("session stopped")
	}
	r.mu.Unlock()

	// Build context with tools
	allTools := tools.AllTools(r.config.CWD)
	if r.extRuntime != nil {
		extTools := r.extRuntime.GetCustomTools()
		allTools = extensions.MergeTools(allTools, extTools)
	}

	// Create context
	context := agent.AgentContext{
		SystemPrompt:  r.config.SystemPrompt,
		Model:         r.config.Model,
		Tools:         allTools,
		ThinkingLevel: r.config.ThinkingLevel,
	}

	// Fire turn start
	turnIndex := r.turnCount
	if r.extRuntime != nil {
		extCtx := r.buildExtensionContext()
		r.extRuntime.FireTurnStart(extCtx, extensions.TurnStartEvent{
			TurnIndex: turnIndex,
		})
	}

	// Fire agent start
	if r.extRuntime != nil {
		extCtx := r.buildExtensionContext()
		r.extRuntime.FireAgentStart(extCtx)
	}

	// Apply extension hooks
	if r.extRuntime != nil {
		extCtx := r.buildExtensionContext()
		extensions.InstallHooks(r.extRuntime, extCtx, r.agent)

		// Fire before-agent-start
		r.extRuntime.FireBeforeAgentStart(extCtx, extensions.BeforeAgentStartEvent{
			Prompt:       content,
			SystemPrompt: r.config.SystemPrompt,
		})
	}

	// Fire context event — allows extensions to inject or modify messages
	allPrompts := []agent.AgentMessage{
		{Role: ai.RoleUser, Content: content, Images: images, Timestamp: time.Now().UnixMilli()},
	}
	if r.extRuntime != nil {
		extCtx := r.buildExtensionContext()
		extraMsgs, err := r.extRuntime.FireContext(extCtx, extensions.ContextEvent{
			Messages: allPrompts,
		})
		if err == nil && len(extraMsgs) > 0 {
			// Prepend context messages before the user prompt
			allPrompts = append(extraMsgs, allPrompts...)
		}
	}

	// Run agent loop
	emit := func(event agent.AgentEvent) error {
		// Track tool calls
		if event.Type == "tool_execution_end" {
			r.mu.Lock()
			r.toolCalls++
			r.mu.Unlock()
		}
		if callback != nil {
			return callback(event)
		}
		return nil
	}

	results, err := r.agent.Run(allPrompts, context, emit)
	if err != nil {
		return nil, fmt.Errorf("agent run failed: %w", err)
	}

	// Find assistant response
	var response *agent.AgentMessage
	for i := len(results) - 1; i >= 0; i-- {
		if results[i].Role == ai.RoleAssistant {
			response = &results[i]
			break
		}
	}

	// Track stats — save all messages (including context-injected ones) to history
	r.mu.Lock()
	r.turnCount++
	r.messages = append(r.messages, allPrompts...) // all prompts including injected context
	if response != nil {
		r.messages = append(r.messages, *response)
		if response.Usage != nil {
			r.totalTokens += response.Usage.TotalTokens
			r.totalCost += response.Usage.Cost.Total
		}
	}
	r.mu.Unlock()

	// Save session
	if r.sessionMgr != nil {
		header := agent.SessionHeader{
			ID:        r.sessionID,
			Timestamp: time.Now().Format(time.RFC3339),
			CWD:       r.config.CWD,
		}
		r.mu.RLock()
		msgs := r.messages
		r.mu.RUnlock()
		r.sessionMgr.Save(r.sessionID, header, msgs)
	}

	// Fire turn end
	if r.extRuntime != nil {
		extCtx := r.buildExtensionContext()
		var respMsg agent.AgentMessage
		if response != nil {
			respMsg = *response
		}
		r.extRuntime.FireTurnEnd(extCtx, extensions.TurnEndEvent{
			TurnIndex: turnIndex,
			Message:   respMsg,
		})
	}

	// Fire agent end
	if r.extRuntime != nil {
		extCtx := r.buildExtensionContext()
		r.extRuntime.FireAgentEnd(extCtx, extensions.AgentEndEvent{
			Messages: r.messages,
		})
	}

	return response, nil
}

// GetStats returns session statistics.
func (r *AgentSessionRuntime) GetStats() SessionStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return SessionStats{
		TotalTurns:    r.turnCount,
		TotalTokens:   r.totalTokens,
		TotalCost:     r.totalCost,
		StartTime:     r.startTime,
		Duration:      time.Since(r.startTime),
		ToolCallCount: r.toolCalls,
	}
}

// GetMessages returns all session messages.
func (r *AgentSessionRuntime) GetMessages() []agent.AgentMessage {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]agent.AgentMessage, len(r.messages))
	copy(result, r.messages)
	return result
}

// GetSessionID returns the session ID.
func (r *AgentSessionRuntime) GetSessionID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sessionID
}

// AddDiagnostic records a diagnostic message.
func (r *AgentSessionRuntime) AddDiagnostic(dtype, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.diagnostics = append(r.diagnostics, AgentSessionRuntimeDiagnostic{
		Type:    dtype,
		Message: message,
	})
}

// GetDiagnostics returns all diagnostic messages.
func (r *AgentSessionRuntime) GetDiagnostics() []AgentSessionRuntimeDiagnostic {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]AgentSessionRuntimeDiagnostic, len(r.diagnostics))
	copy(result, r.diagnostics)
	return result
}

// buildExtensionContext creates an extension context for hook dispatch.
func (r *AgentSessionRuntime) buildExtensionContext() extensions.ExtensionContext {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return extensions.ExtensionContext{
		UI:               r.uiContext,
		HasUI:            r.hasUI,
		CWD:              r.config.CWD,
		Model:            &r.config.Model,
		ExtensionRuntime: r.extRuntime,
		AgentLoop:        r.agent,
		Shutdown: func() {
			if r.agent != nil {
				// handled by session shutdown
			}
		},
	}
}

// SetUIContext sets the UI context for extensions.
func (r *AgentSessionRuntime) SetUIContext(ui extensions.ExtensionUIContext) {
	r.mu.Lock()
	r.uiContext = ui
	r.hasUI = true
	r.mu.Unlock()
}

// GetModel returns the current model.
func (r *AgentSessionRuntime) GetModel() ai.Model {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config.Model
}

// SetModel updates the model.
func (r *AgentSessionRuntime) SetModel(model ai.Model) {
	r.mu.Lock()
	r.config.Model = model
	if r.agent != nil {
		r.agent.SetModel(model)
	}
	r.mu.Unlock()
}

// GetThinkingLevel returns the current thinking level.
func (r *AgentSessionRuntime) GetThinkingLevel() ai.ThinkingLevel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config.ThinkingLevel
}

// SetThinkingLevel updates the thinking level.
func (r *AgentSessionRuntime) SetThinkingLevel(level ai.ThinkingLevel) {
	r.mu.Lock()
	r.config.ThinkingLevel = level
	if r.agent != nil {
		r.agent.SetThinkingLevel(level)
	}
	r.mu.Unlock()
}

// GetSystemPrompt returns the system prompt.
func (r *AgentSessionRuntime) GetSystemPrompt() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config.SystemPrompt
}

// SetSystemPrompt updates the system prompt.
func (r *AgentSessionRuntime) SetSystemPrompt(prompt string) {
	r.mu.Lock()
	r.config.SystemPrompt = prompt
	if r.agent != nil {
		r.agent.SetSystemPrompt(prompt)
	}
	r.mu.Unlock()
}

// GetAPIKey returns the API key.
func (r *AgentSessionRuntime) GetAPIKey() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config.APIKey
}

// SetAPIKey updates the API key.
func (r *AgentSessionRuntime) SetAPIKey(key string) {
	r.mu.Lock()
	r.config.APIKey = key
	if r.agent != nil {
		r.agent.SetAPIKey(key)
	}
	r.mu.Unlock()
}

// GetModelRegistry returns the model registry.
func (r *AgentSessionRuntime) GetModelRegistry() *ModelRegistry {
	return r.modelReg
}
