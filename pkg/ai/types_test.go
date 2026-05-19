package ai

import (
	"testing"
)

func TestNewUserMessage(t *testing.T) {
	msg := NewUserMessage("hello")
	if msg.User == nil {
		t.Fatal("expected User to be non-nil")
	}
	if msg.User.Role != RoleUser {
		t.Errorf("expected role 'user', got '%s'", msg.User.Role)
	}
	if msg.User.Content != "hello" {
		t.Errorf("expected content 'hello', got '%s'", msg.User.Content)
	}
	if msg.User.Timestamp == 0 {
		t.Error("expected non-zero timestamp")
	}
}

func TestNewAssistantMessage(t *testing.T) {
	msg := NewAssistantMessage("anthropic-messages", "anthropic", "claude-sonnet-4")
	if msg.Role != RoleAssistant {
		t.Errorf("expected role 'assistant', got '%s'", msg.Role)
	}
	if string(msg.API) != "anthropic-messages" {
		t.Errorf("expected API 'anthropic-messages', got '%s'", msg.API)
	}
	if string(msg.Provider) != "anthropic" {
		t.Errorf("expected provider 'anthropic', got '%s'", msg.Provider)
	}
	if msg.Model != "claude-sonnet-4" {
		t.Errorf("expected model 'claude-sonnet-4', got '%s'", msg.Model)
	}
}

func TestNewToolResultMessage(t *testing.T) {
	msg := NewToolResultMessage("call_123", "read_file", "file content", false)
	if msg.ToolResult == nil {
		t.Fatal("expected ToolResult to be non-nil")
	}
	if msg.ToolResult.Role != RoleToolResult {
		t.Errorf("expected role 'toolResult', got '%s'", msg.ToolResult.Role)
	}
	if msg.ToolResult.ToolCallID != "call_123" {
		t.Errorf("expected ToolCallID 'call_123', got '%s'", msg.ToolResult.ToolCallID)
	}
	if msg.ToolResult.ToolName != "read_file" {
		t.Errorf("expected ToolName 'read_file', got '%s'", msg.ToolResult.ToolName)
	}
	if msg.ToolResult.IsError {
		t.Error("expected IsError to be false")
	}
}

func TestNewToolResultMessageError(t *testing.T) {
	msg := NewToolResultMessage("call_456", "write_file", "permission denied", true)
	if msg.ToolResult == nil {
		t.Fatal("expected ToolResult to be non-nil")
	}
	if !msg.ToolResult.IsError {
		t.Error("expected IsError to be true")
	}
}

func TestModelRegistry(t *testing.T) {
	registry := NewModelRegistry()

	// Register a model
	model := Model{
		API:      "anthropic-messages",
		Provider: "anthropic",
		Name:     "claude-sonnet-4",
	}
	registry.RegisterModel(model)

	models := registry.GetModels()
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if models[0].Name != "claude-sonnet-4" {
		t.Errorf("expected model name 'claude-sonnet-4', got '%s'", models[0].Name)
	}
}

func TestModelRegistryProvider(t *testing.T) {
	registry := NewModelRegistry()

	// Register a stream function
	streamFn := func(model Model, ctx Context, options *SimpleStreamOptions, callback StreamEventCallback) error {
		return nil
	}
	registry.RegisterProvider("anthropic", streamFn)

	fn, ok := registry.GetStreamFunction("anthropic")
	if !ok {
		t.Fatal("expected to find stream function for anthropic")
	}
	if fn == nil {
		t.Error("expected non-nil stream function")
	}

	// Non-existent provider
	_, ok = registry.GetStreamFunction("nonexistent")
	if ok {
		t.Error("expected not to find stream function for nonexistent provider")
	}
}

func TestProviderError(t *testing.T) {
	err := &ProviderError{
		Provider: "anthropic",
		Message:  "rate limited",
		Code:     429,
	}
	if err.Error() != "rate limited" {
		t.Errorf("expected 'rate limited', got '%s'", err.Error())
	}

	err2 := &ProviderError{
		Provider: "openai",
		Message:  "not found",
	}
	if err2.Error() != "openai: not found" {
		t.Errorf("expected 'openai: not found', got '%s'", err2.Error())
	}
}

func TestConstants(t *testing.T) {
	// Test that key constants are defined
	if APIAnthropicMessages != "anthropic-messages" {
		t.Errorf("unexpected APIAnthropicMessages: %s", APIAnthropicMessages)
	}
	if ProviderAnthropic != "anthropic" {
		t.Errorf("unexpected ProviderAnthropic: %s", ProviderAnthropic)
	}
	if RoleUser != "user" {
		t.Errorf("unexpected RoleUser: %s", RoleUser)
	}
	if RoleAssistant != "assistant" {
		t.Errorf("unexpected RoleAssistant: %s", RoleAssistant)
	}
	if StopReasonStop != "stop" {
		t.Errorf("unexpected StopReasonStop: %s", StopReasonStop)
	}
	if StopReasonToolUse != "toolUse" {
		t.Errorf("unexpected StopReasonToolUse: %s", StopReasonToolUse)
	}
}
