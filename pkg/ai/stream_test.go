package ai

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestEventStreamReader(t *testing.T) {
	sseData := "event: message\ndata: {\"hello\":\"world\"}\n\nevent: done\ndata: [DONE]\n\n"
	reader := NewEventStreamReader(io.NopCloser(strings.NewReader(sseData)))

	// First event
	event, err := reader.ReadEvent()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Event != "message" {
		t.Errorf("expected event 'message', got '%s'", event.Event)
	}
	if event.Data != `{"hello":"world"}` {
		t.Errorf("expected data '%s', got '%s'", `{"hello":"world"}`, event.Data)
	}

	// Second event
	event, err = reader.ReadEvent()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Event != "done" {
		t.Errorf("expected event 'done', got '%s'", event.Event)
	}
	if event.Data != "[DONE]" {
		t.Errorf("expected data '[DONE]', got '%s'", event.Data)
	}

	// EOF
	_, err = reader.ReadEvent()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}

	reader.Close()
}

func TestEventStreamReaderEmptyData(t *testing.T) {
	// Empty SSE with just a comment
	sseData := ":comment\n\n"
	reader := NewEventStreamReader(io.NopCloser(strings.NewReader(sseData)))

	_, err := reader.ReadEvent()
	if err != io.EOF {
		t.Errorf("expected io.EOF for comment-only stream, got %v", err)
	}

	reader.Close()
}

func TestDoHTTPRequest(t *testing.T) {
	// We can't easily test this without a real server, but we can test error cases
	ctx := context.Background()
	_, err := doHTTPRequest(ctx, "GET", "http://127.0.0.1:1/nonexistent", nil, nil)
	if err == nil {
		t.Error("expected error for invalid request")
	}
}

func TestStreamEventTypes(t *testing.T) {
	// Verify event type constants via struct creation
	events := []StreamEvent{
		{Type: "text_delta", Delta: "hello"},
		{Type: "done", Message: &AssistantMessage{StopReason: StopReasonStop}},
		{Type: "error", Error: &AssistantMessage{StopReason: StopReasonError, ErrorMessage: "test error"}},
		{Type: "toolcall_start", ContentIndex: 0},
		{Type: "toolcall_end", ToolCall: &ToolCall{ID: "call_1", Name: "test"}},
	}

	if len(events) != 5 {
		t.Errorf("expected 5 events, got %d", len(events))
	}

	// Verify done event message
	if events[1].Message != nil && events[1].Message.StopReason != StopReasonStop {
		t.Errorf("expected stop reason 'stop', got '%s'", events[1].Message.StopReason)
	}
}

func TestStreamOptionsDefaults(t *testing.T) {
	opts := SimpleStreamOptions{}
	if opts.MaxTokens != 0 {
		t.Errorf("expected 0 max tokens, got %d", opts.MaxTokens)
	}
	if opts.Temperature != 0 {
		t.Errorf("expected 0 temperature, got %f", opts.Temperature)
	}

	opts2 := SimpleStreamOptions{
		StreamOptions: StreamOptions{
			MaxTokens:   4096,
			Temperature: 0.7,
			APIKey:      "sk-test",
		},
		Reasoning: ThinkingHigh,
	}

	if opts2.MaxTokens != 4096 {
		t.Errorf("expected 4096 max tokens, got %d", opts2.MaxTokens)
	}
	if opts2.Temperature != 0.7 {
		t.Errorf("expected 0.7 temperature, got %f", opts2.Temperature)
	}
	if opts2.APIKey != "sk-test" {
		t.Errorf("expected 'sk-test' API key, got '%s'", opts2.APIKey)
	}
	if opts2.Reasoning != ThinkingHigh {
		t.Errorf("expected ThinkingHigh, got '%s'", opts2.Reasoning)
	}
}

func TestModelAndProviderConstants(t *testing.T) {
	// Verify model-related constants
	providers := []Provider{
		ProviderAnthropic,
		ProviderOpenAI,
		ProviderGoogle,
		ProviderDeepSeek,
		ProviderMistral,
	}

	expected := []string{
		"anthropic",
		"openai",
		"google",
		"deepseek",
		"mistral",
	}

	for i, p := range providers {
		if string(p) != expected[i] {
			t.Errorf("provider %d: expected '%s', got '%s'", i, expected[i], p)
		}
	}

	// Verify API constants
	apis := []API{
		APIAnthropicMessages,
		APIOpenAICompletions,
		APIOpenAIResponses,
	}

	apiExpected := []string{
		"anthropic-messages",
		"openai-completions",
		"openai-responses",
	}

	for i, a := range apis {
		if string(a) != apiExpected[i] {
			t.Errorf("api %d: expected '%s', got '%s'", i, apiExpected[i], a)
		}
	}
}

func TestThinkingLevels(t *testing.T) {
	levels := []ThinkingLevel{
		ThinkingMinimal,
		ThinkingLow,
		ThinkingMedium,
		ThinkingHigh,
		ThinkingXHigh,
	}

	expected := []string{"minimal", "low", "medium", "high", "xhigh"}
	for i, l := range levels {
		if string(l) != expected[i] {
			t.Errorf("thinking level %d: expected '%s', got '%s'", i, expected[i], l)
		}
	}
}
