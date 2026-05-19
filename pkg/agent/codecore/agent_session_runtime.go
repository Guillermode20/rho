package codecore

import (
	"fmt"
	"sync"
	"time"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/agent/auth"
	"github.com/earendil-works/rho/pkg/agent/extensions"
	"github.com/earendil-works/rho/pkg/agent/tools"
	"github.com/earendil-works/rho/pkg/ai"
)

// CreateAgentSessionRuntimeOptions configures a new session runtime.
type CreateAgentSessionRuntimeOptions struct {
	Model        ai.Model
	SystemPrompt string
	APIKey       string
	CWD          string
	Extensions   *extensions.Runtime
	Settings     *SettingsManager
	SessionMgr   *agent.SessionManager
	AuthStorage  *auth.AuthStorage
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

	sessionID   string
	started     bool
	stopped     bool
	turnCount   int
	startTime   time.Time
	totalTokens int
	totalCost   float64
	toolCalls   int

	diagnostics []AgentSessionRuntimeDiagnostic
}

// NewAgentSessionRuntime creates a new session runtime.
func NewAgentSessionRuntime(sessionID string, opts CreateAgentSessionRuntimeOptions) *AgentSessionRuntime {
	loop := agent.NewAgentLoop(agent.AgentLoopConfig{
		Model:        opts.Model,
		SystemPrompt: opts.SystemPrompt,
		APIKey:       opts.APIKey,
	})

	return &AgentSessionRuntime{
		config: AgentSessionConfig{
			Model:        opts.Model,
			SystemPrompt: opts.SystemPrompt,
			APIKey:       opts.APIKey,
			CWD:          opts.CWD,
		},
		agent:      loop,
		extRuntime: opts.Extensions,
		settings:   opts.Settings,
		sessionMgr: opts.SessionMgr,
		auth:       opts.AuthStorage,
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
		SystemPrompt: r.config.SystemPrompt,
		Model:        r.config.Model,
		Tools:        allTools,
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

	// Create prompt
	prompt := agent.AgentMessage{
		Role:      ai.RoleUser,
		Content:   content,
		Timestamp: time.Now().UnixMilli(),
	}

	// Run agent loop
	emit := func(event agent.AgentEvent) error {
		// Track tool calls
		if event.Type == "tool_execution_end" {
			r.mu.Lock()
			r.toolCalls++
			r.mu.Unlock()
		}
		return nil
	}

	results, err := r.agent.Run([]agent.AgentMessage{prompt}, context, emit)
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

	// Track stats
	r.mu.Lock()
	r.turnCount++
	r.messages = append(r.messages, prompt)
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
	return extensions.ExtensionContext{
		HasUI: false,
		CWD:   r.config.CWD,
	}
}
