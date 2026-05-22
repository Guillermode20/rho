package harness

import (
	"fmt"
	"sync"
	"time"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/ai"
)

// AgentHarness wraps the agent loop with session management, skills, and event hooks.
type AgentHarness struct {
	mu     sync.RWMutex
	phase  AgentHarnessPhase
	loop   *agent.AgentLoop
	config AgentHarnessConfig

	// State
	sessionID      string
	turnIndex      int
	eventListeners map[string][]func(event interface{})

	// Resources
	skills          []Skill
	promptTemplates []PromptTemplate
	resources       AgentHarnessResources

	// Stream options
	streamOptions AgentHarnessStreamOptions
}

// AgentHarnessConfig configures the AgentHarness.
type AgentHarnessConfig struct {
	Model         ai.Model                  `json:"model"`
	SystemPrompt  string                    `json:"systemPrompt"`
	APIKey        string                    `json:"apiKey"`
	MaxTokens     int                       `json:"maxTokens"`
	Temperature   float64                   `json:"temperature"`
	ThinkingLevel string                    `json:"thinkingLevel"`
	ToolExecMode  agent.ToolExecutionMode   `json:"toolExecutionMode"`
	StreamOptions AgentHarnessStreamOptions `json:"streamOptions"`
	Compaction    CompactionSettings        `json:"compaction"`
	SessionFile   string                    `json:"sessionFile,omitempty"`
}

// NewAgentHarness creates a new AgentHarness.
func NewAgentHarness(config AgentHarnessConfig) *AgentHarness {
	return &AgentHarness{
		phase:          PhaseIdle,
		config:         config,
		eventListeners: make(map[string][]func(event interface{})),
		streamOptions:  config.StreamOptions,
	}
}

// Phase returns the current harness phase.
func (h *AgentHarness) Phase() AgentHarnessPhase {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.phase
}

// SessionID returns the current session ID.
func (h *AgentHarness) SessionID() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sessionID
}

// TurnIndex returns the current turn index.
func (h *AgentHarness) TurnIndex() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.turnIndex
}

// On registers an event listener.
func (h *AgentHarness) On(eventType string, handler func(event interface{})) func() {
	h.mu.Lock()
	h.eventListeners[eventType] = append(h.eventListeners[eventType], handler)
	idx := len(h.eventListeners[eventType]) - 1
	h.mu.Unlock()
	return func() {
		h.mu.Lock()
		listeners := h.eventListeners[eventType]
		if idx < len(listeners) {
			h.eventListeners[eventType] = append(listeners[:idx], listeners[idx+1:]...)
		}
		h.mu.Unlock()
	}
}

// emit fires an event to all listeners of that type.
func (h *AgentHarness) emit(eventType string, event interface{}) {
	h.mu.RLock()
	listeners := h.eventListeners[eventType]
	h.mu.RUnlock()
	for _, listener := range listeners {
		listener(event)
	}
}

// SetResources sets the harness resources.
func (h *AgentHarness) SetResources(skills []Skill, templates []PromptTemplate) {
	h.mu.Lock()
	old := h.resources
	h.skills = skills
	h.promptTemplates = templates
	h.resources = AgentHarnessResources{
		Skills:          skills,
		PromptTemplates: templates,
	}
	h.mu.Unlock()

	h.emit(EventResourcesUpdate, struct {
		Type              string
		Resources         AgentHarnessResources
		PreviousResources AgentHarnessResources
	}{
		Type:              EventResourcesUpdate,
		Resources:         h.resources,
		PreviousResources: old,
	})
}

// GetSkills returns the harness skills.
func (h *AgentHarness) GetSkills() []Skill {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.skills
}

// GetPromptTemplates returns the prompt templates.
func (h *AgentHarness) GetPromptTemplates() []PromptTemplate {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.promptTemplates
}

// ============================================================================
// Agent Execution
// ============================================================================

// Run starts the agent with a prompt and returns the response messages.
func (h *AgentHarness) Run(prompt string, userImages []ai.ImageContent, tools []agent.AgentTool) ([]agent.AgentMessage, error) {
	h.mu.Lock()
	if h.phase != PhaseIdle {
		h.mu.Unlock()
		return nil, &AgentHarnessError{Code: HarnessErrBusy, Message: "harness is busy"}
	}
	h.phase = PhaseTurn
	h.turnIndex++
	sessionID := fmt.Sprintf("session_%d", time.Now().UnixNano())
	h.sessionID = sessionID
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		h.phase = PhaseIdle
		h.mu.Unlock()
	}()

	// Build system prompt
	sysPrompt := BuildSystemPrompt(h.config.SystemPrompt, h.skills, toolNames(tools), nil)

	// Create agent loop
	loop := agent.NewAgentLoop(agent.AgentLoopConfig{
		Model:             h.config.Model,
		SystemPrompt:      sysPrompt,
		APIKey:            h.config.APIKey,
		MaxTokens:         h.config.MaxTokens,
		Temperature:       h.config.Temperature,
		ToolExecutionMode: h.config.ToolExecMode,
	})
	h.loop = loop

	// Emit before_agent_start
	h.emit(EventBeforeAgentStart, struct {
		Type         string
		Prompt       string
		SystemPrompt string
		Resources    AgentHarnessResources
	}{
		Type: EventBeforeAgentStart, Prompt: prompt,
		SystemPrompt: sysPrompt, Resources: h.resources,
	})

	// Convert images to content
	content := prompt
	_ = userImages

	promptMessages := []agent.AgentMessage{
		{Role: ai.RoleUser, Content: content, Timestamp: time.Now().UnixMilli()},
	}

	context := agent.AgentContext{
		SystemPrompt: sysPrompt,
		Model:        h.config.Model,
		Tools:        tools,
	}

	var results []agent.AgentMessage

	emit := func(event agent.AgentEvent) error {
		// Forward to harness listeners
		h.emit(event.Type, event)
		return nil
	}

	results, err := loop.Run(promptMessages, context, emit)
	if err != nil {
		h.emit(EventAbort, AbortEvent{
			Type: EventAbort,
		})
		return nil, err
	}

	// Emit agent_end
	h.emit("agent_end", struct {
		Type     string
		Messages []agent.AgentMessage
	}{
		Type: "agent_end", Messages: results,
	})

	return results, nil
}

// Continue continues the agent from existing context.
func (h *AgentHarness) Continue(context agent.AgentContext, tools []agent.AgentTool) ([]agent.AgentMessage, error) {
	h.mu.Lock()
	if h.phase != PhaseIdle {
		h.mu.Unlock()
		return nil, &AgentHarnessError{Code: HarnessErrBusy, Message: "harness is busy"}
	}
	h.phase = PhaseTurn
	h.turnIndex++
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		h.phase = PhaseIdle
		h.mu.Unlock()
	}()

	loop := agent.NewAgentLoop(agent.AgentLoopConfig{
		Model:             h.config.Model,
		SystemPrompt:      h.config.SystemPrompt,
		APIKey:            h.config.APIKey,
		MaxTokens:         h.config.MaxTokens,
		Temperature:       h.config.Temperature,
		ToolExecutionMode: h.config.ToolExecMode,
	})
	h.loop = loop

	var results []agent.AgentMessage
	results, err := loop.Continue(context, func(event agent.AgentEvent) error {
		h.emit(event.Type, event)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return results, nil
}

// Abort aborts the current agent execution.
func (h *AgentHarness) Abort() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.loop != nil {
		h.phase = PhaseIdle
		h.emit(EventAbort, AbortEvent{Type: EventAbort})
	}
}

// IsBusy returns whether the harness is currently processing.
func (h *AgentHarness) IsBusy() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.phase != PhaseIdle
}

func toolNames(tools []agent.AgentTool) []string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	return names
}
