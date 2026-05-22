// Package agent provides a general-purpose AI agent loop with tool execution,
// state management, and session persistence.
package agent

import (
	"context"

	"github.com/earendil-works/rho/pkg/ai"
)

// AgentToolCall represents a single tool call from an assistant message.
type AgentToolCall = ai.ToolCall

// AgentToolResult represents the result of executing a tool.
type AgentToolResult struct {
	ToolCallID string      `json:"toolCallId"`
	ToolName   string      `json:"toolName"`
	Content    string      `json:"content"`
	IsError    bool        `json:"isError"`
	Details    interface{} `json:"details,omitempty"`
}

// AgentMessage represents a message in the agent context.
type AgentMessage struct {
	Role         ai.Role       `json:"role"`
	Content      string        `json:"content,omitempty"`
	ToolCallID   string        `json:"toolCallId,omitempty"`
	ToolName     string        `json:"toolName,omitempty"`
	IsError      bool          `json:"isError,omitempty"`
	ToolCalls    []ai.ToolCall `json:"toolCalls,omitempty"`
	Usage        *ai.Usage     `json:"usage,omitempty"`
	StopReason   ai.StopReason `json:"stopReason,omitempty"`
	ErrorMessage string        `json:"errorMessage,omitempty"`
	ResponseID   string        `json:"responseId,omitempty"`
	API          ai.API        `json:"api,omitempty"`
	Provider     ai.Provider   `json:"provider,omitempty"`
	Model        string        `json:"model,omitempty"`
	Hide         bool          `json:"hide,omitempty"`
	Timestamp    int64         `json:"timestamp,omitempty"`
}

// ToolExecutionMode controls how tool calls are executed.
type ToolExecutionMode string

const (
	ToolExecutionSequential ToolExecutionMode = "sequential"
	ToolExecutionParallel   ToolExecutionMode = "parallel"
)

// QueueMode controls how queued user messages are injected.
type QueueMode string

const (
	QueueAll        QueueMode = "all"
	QueueOneAtATime QueueMode = "one-at-a-time"
)

// AgentTool defines a tool available to the agent.
type AgentTool struct {
	Name        string                                                  `json:"name"`
	Description string                                                  `json:"description"`
	Parameters  interface{}                                             `json:"parameters"` // JSON schema
	Execute     func(args map[string]interface{}) (string, bool, error) `json:"-"`
}

// AgentContext holds the current state of the agent.
type AgentContext struct {
	SystemPrompt  string           `json:"systemPrompt"`
	Model         ai.Model         `json:"model"`
	Messages      []AgentMessage   `json:"messages"`
	Tools         []AgentTool      `json:"tools"`
	ThinkingLevel ai.ThinkingLevel `json:"thinkingLevel,omitempty"`
}

// AgentState is a snapshot of the agent's state.
type AgentState struct {
	SystemPrompt  string           `json:"systemPrompt"`
	Model         ai.Model         `json:"model"`
	Messages      []AgentMessage   `json:"messages"`
	Tools         []AgentTool      `json:"tools"`
	ThinkingLevel ai.ThinkingLevel `json:"thinkingLevel,omitempty"`
	IsStreaming   bool             `json:"isStreaming"`
	ErrorMessage  string           `json:"errorMessage,omitempty"`
}

// AgentEvent represents events emitted during the agent loop.
type AgentEvent struct {
	Type         string        `json:"type"`
	ContentIndex int           `json:"contentIndex,omitempty"`
	Delta        string        `json:"delta,omitempty"`
	Content      string        `json:"content,omitempty"`
	ToolCall     *ai.ToolCall  `json:"toolCall,omitempty"`
	Message      *AgentMessage `json:"message,omitempty"`
	Partial      *AgentMessage `json:"partial,omitempty"`
	Error        string        `json:"error,omitempty"`
	IsError      bool          `json:"isError,omitempty"`
}

// AgentEventCallback is called for each agent event.
type AgentEventCallback func(event AgentEvent) error

// AgentLoopConfig configures the agent loop.
type AgentLoopConfig struct {
	Model                 ai.Model                                              `json:"model"`
	SystemPrompt          string                                                `json:"systemPrompt,omitempty"`
	MaxTokens             int                                                   `json:"maxTokens,omitempty"`
	Temperature           float64                                               `json:"temperature,omitempty"`
	ThinkingLevel         ai.ThinkingLevel                                      `json:"thinkingLevel,omitempty"`
	ToolExecutionMode     ToolExecutionMode                                     `json:"toolExecutionMode,omitempty"`
	APIKey                string                                                `json:"apiKey,omitempty"`
	MaxRetries            int                                                   `json:"maxRetries,omitempty"`
	Signal                context.Context                                       `json:"-"`
	BeforeProviderRequest func(ctx ai.Context) (ai.Context, error)              `json:"-"`
	CompactFn             func(messages []AgentMessage) ([]AgentMessage, error) `json:"-"`
}

// BeforeToolCallContext provides context for beforeToolCall hooks.
type BeforeToolCallContext struct {
	AssistantMessage AgentMessage
	ToolCall         ai.ToolCall
	Args             map[string]interface{}
	Context          AgentContext
}

// BeforeToolCallResult controls tool execution.
type BeforeToolCallResult struct {
	Block  bool
	Reason string
}

// AfterToolCallContext provides context for afterToolCall hooks.
type AfterToolCallContext struct {
	AssistantMessage AgentMessage
	ToolCall         ai.ToolCall
	Args             map[string]interface{}
	Result           AgentToolResult
	IsError          bool
	Context          AgentContext
}

// AfterToolCallResult allows overriding tool results.
type AfterToolCallResult struct {
	Content   string
	IsError   *bool
	Terminate *bool
}

// Agent loop errors.
type AgentError struct {
	Message string
	Err     error
}

func (e *AgentError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *AgentError) Unwrap() error {
	return e.Err
}
