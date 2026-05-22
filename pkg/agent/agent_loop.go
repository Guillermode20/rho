package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/earendil-works/rho/pkg/ai"
	"github.com/earendil-works/rho/pkg/ai/providers"
)

// NewAgentLoop creates a new agent loop.
func NewAgentLoop(config AgentLoopConfig) *AgentLoop {
	return &AgentLoop{
		config: config,
	}
}

// AgentLoop runs the agent loop that processes messages and executes tools.
type AgentLoop struct {
	config              AgentLoopConfig
	context             AgentContext
	initialMessageCount int
	beforeTool          func(ctx BeforeToolCallContext) (*BeforeToolCallResult, error)
	afterTool           func(ctx AfterToolCallContext) (*AfterToolCallResult, error)
	streamFn            func(model ai.Model, ctx ai.Context, options *ai.SimpleStreamOptions, callback ai.StreamEventCallback) error
}

// SetBeforeTool sets the before-tool-call hook.
func (l *AgentLoop) SetBeforeTool(fn func(ctx BeforeToolCallContext) (*BeforeToolCallResult, error)) {
	l.beforeTool = fn
}

// SetAfterTool sets the after-tool-call hook.
func (l *AgentLoop) SetAfterTool(fn func(ctx AfterToolCallContext) (*AfterToolCallResult, error)) {
	l.afterTool = fn
}

// SetStreamFn sets a custom stream function.
func (l *AgentLoop) SetStreamFn(fn func(model ai.Model, ctx ai.Context, options *ai.SimpleStreamOptions, callback ai.StreamEventCallback) error) {
	l.streamFn = fn
}

// Run starts a new agent loop with the given prompt messages.
func (l *AgentLoop) Run(prompts []AgentMessage, context AgentContext, emit AgentEventCallback) ([]AgentMessage, error) {
	if len(prompts) == 0 {
		return nil, &AgentError{Message: "no prompts provided"}
	}

	// Initialize context
	l.context = AgentContext{
		SystemPrompt:  context.SystemPrompt,
		Model:         context.Model,
		Messages:      append(context.Messages, prompts...),
		Tools:         context.Tools,
		ThinkingLevel: context.ThinkingLevel,
	}
	l.initialMessageCount = len(context.Messages)

	return l.runLoop(emit)
}

// Continue continues an existing agent loop.
func (l *AgentLoop) Continue(context AgentContext, emit AgentEventCallback) ([]AgentMessage, error) {
	if len(context.Messages) == 0 {
		return nil, &AgentError{Message: "cannot continue: no messages in context"}
	}

	lastMsg := context.Messages[len(context.Messages)-1]
	if lastMsg.Role == ai.RoleAssistant {
		return nil, &AgentError{Message: "cannot continue from assistant message"}
	}

	l.context = context
	l.initialMessageCount = len(context.Messages)
	return l.runLoop(emit)
}

func (l *AgentLoop) runLoop(emit AgentEventCallback) ([]AgentMessage, error) {
	maxIterations := 20 // Safety limit to prevent infinite loops
	emit(AgentEvent{Type: "agent_start"})
	defer emit(AgentEvent{Type: "agent_end"})

	for i := 0; i < maxIterations; i++ {
		if l.isAborted() {
			return l.extractNewMessages(), nil
		}
		// Build LLM context
		llmCtx := l.buildLLMContext()

		// Stream response from the model
		msg, err := l.streamResponse(llmCtx, emit)
		if err != nil {
			return nil, fmt.Errorf("stream response failed: %w", err)
		}

		if msg == nil {
			break
		}
		emit(AgentEvent{
			Type:    "message_end",
			Message: msg,
		})

		// Check for tool calls
		if len(msg.ToolCalls) > 0 && msg.StopReason != ai.StopReasonToolUse {
			// Non-tool-use stop but has tool calls - still process them
		}

		if len(msg.ToolCalls) == 0 || msg.StopReason == ai.StopReasonStop || msg.StopReason == ai.StopReasonLength {
			// No tool calls or finished normally
			l.context.Messages = append(l.context.Messages, *msg)

			// Emit final message
			emit(AgentEvent{
				Type:    "message",
				Message: msg,
			})

			// Return new messages only
			return l.extractNewMessages(), nil
		}

		// Has tool calls - execute them
		l.context.Messages = append(l.context.Messages, *msg)

		toolResults, err := l.executeToolCalls(*msg, emit)
		if err != nil {
			return nil, fmt.Errorf("tool execution failed: %w", err)
		}

		// Add tool results to context
		for _, tr := range toolResults {
			trMsg := AgentMessage{
				Role:       ai.RoleToolResult,
				ToolCallID: tr.ToolCallID,
				ToolName:   tr.ToolName,
				Content:    tr.Content,
				IsError:    tr.IsError,
				Timestamp:  time.Now().UnixMilli(),
			}
			l.context.Messages = append(l.context.Messages, trMsg)
		}

		// Compact context if configured
		if l.config.CompactFn != nil {
			compacted, err := l.config.CompactFn(l.context.Messages)
			if err == nil && len(compacted) < len(l.context.Messages) {
				l.context.Messages = compacted
				emit(AgentEvent{
					Type:    "compaction",
					Message: &AgentMessage{Content: fmt.Sprintf("Compacted context from %d to %d messages", len(l.context.Messages), len(compacted))},
				})
			}
		}

		// Continue loop for next turn
	}

	return nil, &AgentError{Message: "agent loop exceeded maximum iterations"}
}

func (l *AgentLoop) buildLLMContext() ai.Context {
	var messages []ai.Message

	for _, m := range l.context.Messages {
		switch m.Role {
		case ai.RoleUser:
			messages = append(messages, ai.NewUserMessage(m.Content))
		case ai.RoleAssistant:
			amsg := ai.NewAssistantMessage(l.config.Model.API, l.config.Model.Provider, l.config.Model.Name)
			amsg.Content = nil
			amsg.StopReason = m.StopReason
			if m.Content != "" {
				amsg.Content = append(amsg.Content, ai.ContentBlock{
					Text: &ai.TextContent{Type: "text", Text: m.Content},
				})
			}
			for _, tc := range m.ToolCalls {
				tcCopy := tc
				amsg.Content = append(amsg.Content, ai.ContentBlock{
					ToolCall: &tcCopy,
				})
			}
			messages = append(messages, ai.Message{Assistant: &amsg})
		case ai.RoleToolResult:
			messages = append(messages, ai.NewToolResultMessage(m.ToolCallID, m.ToolName, m.Content, m.IsError))
		}
	}

	// Build tools for LLM
	var tools []ai.Tool
	for _, t := range l.context.Tools {
		tools = append(tools, ai.Tool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		})
	}

	return ai.Context{
		SystemPrompt: l.context.SystemPrompt,
		Messages:     messages,
		Tools:        tools,
	}
}

func (l *AgentLoop) streamResponse(llmCtx ai.Context, emit AgentEventCallback) (*AgentMessage, error) {
	streamFn := l.streamFn
	if streamFn == nil {
		streamFn = defaultStreamFn
	}

	options := &ai.SimpleStreamOptions{}
	if l.config.Signal != nil {
		options.Signal = l.config.Signal
	}
	if l.config.MaxTokens > 0 {
		options.MaxTokens = l.config.MaxTokens
	}
	if l.config.Temperature > 0 {
		options.Temperature = l.config.Temperature
	}
	if l.config.APIKey != "" {
		options.APIKey = l.config.APIKey
	}
	if l.config.ThinkingLevel != "" {
		options.Reasoning = l.config.ThinkingLevel
	}

	// Fire before-provider-request hook
	finalLLMCtx := llmCtx
	if l.config.BeforeProviderRequest != nil {
		modifiedCtx, err := l.config.BeforeProviderRequest(llmCtx)
		if err == nil {
			finalLLMCtx = modifiedCtx
		}
	}

	msg := &AgentMessage{
		Role:     ai.RoleAssistant,
		API:      l.config.Model.API,
		Provider: l.config.Model.Provider,
		Model:    l.config.Model.Name,
	}

	// Track partial state
	var currentText string
	var currentToolCalls []ai.ToolCall

	err := streamFn(l.config.Model, finalLLMCtx, options, func(event ai.StreamEvent) error {
		if l.isAborted() {
			msg.StopReason = ai.StopReasonAborted
			msg.ErrorMessage = "Operation aborted"
			return context.Canceled
		}
		switch event.Type {
		case "text_delta":
			currentText += event.Delta
			msg.Content = currentText

			emit(AgentEvent{
				Type:  "text_delta",
				Delta: event.Delta,
				Partial: &AgentMessage{
					Role:    ai.RoleAssistant,
					Content: currentText,
				},
			})

		case "toolcall_start":
			emit(AgentEvent{
				Type:         "toolcall_start",
				ContentIndex: event.ContentIndex,
				ToolCall:     event.ToolCall,
			})

		case "toolcall_delta":
			emit(AgentEvent{
				Type:         "toolcall_delta",
				ContentIndex: event.ContentIndex,
				Delta:        event.Delta,
				ToolCall:     event.ToolCall,
			})

		case "toolcall_end":
			if event.ToolCall != nil {
				currentToolCalls = append(currentToolCalls, *event.ToolCall)
				msg.ToolCalls = currentToolCalls

				emit(AgentEvent{
					Type:     "toolcall_end",
					ToolCall: event.ToolCall,
				})
			}

		case "done":
			if event.Message != nil {
				msg.StopReason = event.Message.StopReason
				msg.Usage = &event.Message.Usage
				msg.ResponseID = event.Message.ResponseID
				msg.ErrorMessage = event.Message.ErrorMessage
			}

		case "error":
			msg.StopReason = ai.StopReasonError
			msg.ErrorMessage = "stream error"
			if event.Error != nil {
				msg.ErrorMessage = event.Error.ErrorMessage
			}

			emit(AgentEvent{
				Type:  "error",
				Error: msg.ErrorMessage,
			})
		}

		return nil
	})

	if err != nil {
		if err == context.Canceled || l.isAborted() {
			msg.StopReason = ai.StopReasonAborted
			msg.ErrorMessage = "Operation aborted"
			return msg, nil
		}
		return nil, err
	}

	// If we got tool calls but no content, set stop reason
	if len(currentToolCalls) > 0 {
		msg.StopReason = ai.StopReasonToolUse
	}

	if msg.StopReason == ai.StopReasonError || msg.ErrorMessage != "" {
		return msg, nil
	}

	return msg, nil
}

func (l *AgentLoop) executeToolCalls(msg AgentMessage, emit AgentEventCallback) ([]AgentToolResult, error) {
	var results []AgentToolResult

	toolMap := make(map[string]AgentTool)
	for _, t := range l.context.Tools {
		toolMap[t.Name] = t
	}

	for _, tc := range msg.ToolCalls {
		if l.isAborted() {
			return results, nil
		}
		// Check before-tool hook
		if l.beforeTool != nil {
			var args map[string]interface{}
			if tc.Arguments != nil {
				args = tc.Arguments
			}

			result, err := l.beforeTool(BeforeToolCallContext{
				AssistantMessage: msg,
				ToolCall:         tc,
				Args:             args,
				Context:          l.context,
			})
			if err != nil {
				return nil, err
			}
			if result != nil && result.Block {
				reason := result.Reason
				if reason == "" {
					reason = "Tool call blocked"
				}
				results = append(results, AgentToolResult{
					ToolCallID: tc.ID,
					ToolName:   tc.Name,
					Content:    reason,
					IsError:    true,
				})
				continue
			}
		}

		// Find and execute the tool
		tool, ok := toolMap[tc.Name]
		if !ok {
			results = append(results, AgentToolResult{
				ToolCallID: tc.ID,
				ToolName:   tc.Name,
				Content:    fmt.Sprintf("Unknown tool: %s", tc.Name),
				IsError:    true,
			})
			continue
		}

		emit(AgentEvent{
			Type: "tool_execution_start",
			ToolCall: &ai.ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			},
		})

		content, isError, err := tool.Execute(tc.Arguments)
		if err != nil {
			content = fmt.Sprintf("Error: %v", err)
			isError = true
		}

		result := AgentToolResult{
			ToolCallID: tc.ID,
			ToolName:   tc.Name,
			Content:    content,
			IsError:    isError,
		}

		// Check after-tool hook
		if l.afterTool != nil {
			override, err := l.afterTool(AfterToolCallContext{
				AssistantMessage: msg,
				ToolCall:         tc,
				Args:             tc.Arguments,
				Result:           result,
				IsError:          isError,
				Context:          l.context,
			})
			if err != nil {
				return nil, err
			}
			if override != nil {
				if override.Content != "" {
					result.Content = override.Content
				}
				if override.IsError != nil {
					result.IsError = *override.IsError
				}
			}
		}

		results = append(results, result)

		emit(AgentEvent{
			Type: "tool_execution_end",
			ToolCall: &ai.ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			},
			Content: result.Content,
			IsError: result.IsError,
		})
	}

	return results, nil
}

func (l *AgentLoop) isAborted() bool {
	if l.config.Signal == nil {
		return false
	}
	select {
	case <-l.config.Signal.Done():
		return true
	default:
		return false
	}
}

func (l *AgentLoop) extractNewMessages() []AgentMessage {
	if l.initialMessageCount < 0 || l.initialMessageCount > len(l.context.Messages) {
		return append([]AgentMessage(nil), l.context.Messages...)
	}
	return append([]AgentMessage(nil), l.context.Messages[l.initialMessageCount:]...)
}

// defaultStreamFn is the default streaming function that uses the AI registry.
func defaultStreamFn(model ai.Model, ctx ai.Context, options *ai.SimpleStreamOptions, callback ai.StreamEventCallback) error {
	return providers.StreamSimple(model, ctx, options, callback)
}

// JSONSchemaTool generates a JSON schema for a simple tool.
func JSONSchemaTool(name, description string, params map[string]interface{}) AgentTool {
	return AgentTool{
		Name:        name,
		Description: description,
		Parameters:  params,
	}
}

// convertToLLMMessages converts agent messages to AI messages, filtering non-convertible ones.
func ConvertToLLMMessages(messages []AgentMessage) []ai.Message {
	var result []ai.Message
	for _, m := range messages {
		switch m.Role {
		case ai.RoleUser:
			result = append(result, ai.NewUserMessage(m.Content))
		case ai.RoleAssistant:
			amsg := ai.NewAssistantMessage(m.API, m.Provider, m.Model)
			if m.Content != "" {
				amsg.Content = append(amsg.Content, ai.ContentBlock{
					Text: &ai.TextContent{Type: "text", Text: m.Content},
				})
			}
			for _, tc := range m.ToolCalls {
				tcCopy := tc
				amsg.Content = append(amsg.Content, ai.ContentBlock{
					ToolCall: &tcCopy,
				})
			}
			result = append(result, ai.Message{Assistant: &amsg})
		case ai.RoleToolResult:
			result = append(result, ai.NewToolResultMessage(m.ToolCallID, m.ToolName, m.Content, m.IsError))
		}
	}
	return result
}

// init ensures encoding/json is used.
var _ = json.Marshal
