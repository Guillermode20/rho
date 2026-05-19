package agent

import (
	"testing"

	"github.com/earendil-works/rho/pkg/ai"
)

func TestAgentToolCall(t *testing.T) {
	tc := ai.ToolCall{
		ID:        "call_123",
		Name:      "read_file",
		Arguments: map[string]interface{}{"path": "/tmp/test.txt"},
	}
	if tc.ID != "call_123" {
		t.Errorf("expected ID 'call_123', got '%s'", tc.ID)
	}
	if tc.Name != "read_file" {
		t.Errorf("expected name 'read_file', got '%s'", tc.Name)
	}
}

func TestAgentMessage(t *testing.T) {
	msg := AgentMessage{
		Role:    ai.RoleUser,
		Content: "Hello",
	}
	if msg.Role != ai.RoleUser {
		t.Errorf("expected role 'user', got '%s'", msg.Role)
	}
	if msg.Content != "Hello" {
		t.Errorf("expected content 'Hello', got '%s'", msg.Content)
	}
}

func TestAgentMessageAssistant(t *testing.T) {
	msg := AgentMessage{
		Role:    ai.RoleAssistant,
		Content: "Hello there!",
		Model:   "claude-sonnet-4",
		Provider: ai.ProviderAnthropic,
		StopReason: ai.StopReasonStop,
	}
	if msg.StopReason != ai.StopReasonStop {
		t.Errorf("expected stop reason 'stop', got '%s'", msg.StopReason)
	}
	if msg.Model != "claude-sonnet-4" {
		t.Errorf("expected model 'claude-sonnet-4', got '%s'", msg.Model)
	}
}

func TestAgentMessageToolResult(t *testing.T) {
	msg := AgentMessage{
		Role:       ai.RoleToolResult,
		ToolCallID: "call_123",
		ToolName:   "read_file",
		Content:    "file contents",
		IsError:    false,
	}
	if msg.ToolCallID != "call_123" {
		t.Errorf("expected ToolCallID 'call_123', got '%s'", msg.ToolCallID)
	}
	if msg.ToolName != "read_file" {
		t.Errorf("expected ToolName 'read_file', got '%s'", msg.ToolName)
	}
	if msg.IsError {
		t.Error("expected IsError to be false")
	}
}

func TestAgentContext(t *testing.T) {
	ctx := AgentContext{
		SystemPrompt: "You are a helpful assistant.",
		Model: ai.Model{
			API:      "anthropic-messages",
			Provider: "anthropic",
			Name:     "claude-sonnet-4",
		},
		Tools: []AgentTool{
			{
				Name:        "read_file",
				Description: "Read the contents of a file",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}
	if ctx.SystemPrompt != "You are a helpful assistant." {
		t.Errorf("unexpected system prompt")
	}
	if len(ctx.Tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(ctx.Tools))
	}
	if ctx.Tools[0].Name != "read_file" {
		t.Errorf("expected tool 'read_file', got '%s'", ctx.Tools[0].Name)
	}
}

func TestConvertToLLMMessages(t *testing.T) {
	messages := []AgentMessage{
		{Role: ai.RoleUser, Content: "Hello"},
		{Role: ai.RoleAssistant, Content: "Hi there!", Model: "gpt-4", API: ai.APIOpenAICompletions, Provider: ai.ProviderOpenAI},
		{Role: ai.RoleToolResult, ToolCallID: "call_1", ToolName: "test", Content: "result"},
	}

	llmMessages := ConvertToLLMMessages(messages)
	if len(llmMessages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(llmMessages))
	}

	if llmMessages[0].User == nil {
		t.Error("expected first message to be a user message")
	}
	if llmMessages[1].Assistant == nil {
		t.Error("expected second message to be an assistant message")
	}
	if llmMessages[2].ToolResult == nil {
		t.Error("expected third message to be a tool result message")
	}
}

func TestAgentError(t *testing.T) {
	err := &AgentError{Message: "something went wrong"}
	if err.Error() != "something went wrong" {
		t.Errorf("expected 'something went wrong', got '%s'", err.Error())
	}

	cause := &AgentError{Message: "inner error"}
	outer := &AgentError{Message: "outer", Err: cause}
	if outer.Error() != "outer: inner error" {
		t.Errorf("expected 'outer: inner error', got '%s'", outer.Error())
	}
	if outer.Unwrap() != cause {
		t.Error("expected Unwrap to return the inner error")
	}
}

func TestToolExecutionModes(t *testing.T) {
	if ToolExecutionSequential != "sequential" {
		t.Errorf("expected 'sequential', got '%s'", ToolExecutionSequential)
	}
	if ToolExecutionParallel != "parallel" {
		t.Errorf("expected 'parallel', got '%s'", ToolExecutionParallel)
	}
}

func TestQueueModes(t *testing.T) {
	if QueueAll != "all" {
		t.Errorf("expected 'all', got '%s'", QueueAll)
	}
	if QueueOneAtATime != "one-at-a-time" {
		t.Errorf("expected 'one-at-a-time', got '%s'", QueueOneAtATime)
	}
}

func TestAgentLoopConfig(t *testing.T) {
	config := AgentLoopConfig{
		Model: ai.Model{
			API:      "anthropic-messages",
			Provider: "anthropic",
			Name:     "claude-sonnet-4",
		},
		SystemPrompt:      "You are a coding agent.",
		MaxTokens:         8192,
		Temperature:       0.7,
		ToolExecutionMode: ToolExecutionSequential,
	}
	if config.MaxTokens != 8192 {
		t.Errorf("expected 8192 max tokens, got %d", config.MaxTokens)
	}
	if config.Temperature != 0.7 {
		t.Errorf("expected 0.7 temperature, got %f", config.Temperature)
	}
}
