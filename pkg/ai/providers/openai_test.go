package providers

import (
	"io"
	"strings"
	"testing"

	"github.com/earendil-works/rho/pkg/ai"
)

func TestParseOpenAIStreamEmitsToolCallProgress(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"id":"resp_1","model":"glm-5.1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"Grep","arguments":"{\"pattern\""}}]},"finish_reason":null}]}`,
		`data: {"id":"resp_1","model":"glm-5.1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":":\"TODO\"}"}}]},"finish_reason":null}]}`,
		`data: {"id":"resp_1","model":"glm-5.1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		`data: [DONE]`,
		"",
	}, "\n")

	var events []ai.StreamEvent
	err := parseOpenAIStream(io.NopCloser(strings.NewReader(stream)), func(event ai.StreamEvent) error {
		events = append(events, event)
		return nil
	}, ai.Model{API: ai.APIOpenAICompletions, Provider: ai.ProviderCrof, Name: "glm-5.1"})
	if err != nil {
		t.Fatalf("parseOpenAIStream error: %v", err)
	}

	if !hasEvent(events, "toolcall_start") {
		t.Fatalf("missing toolcall_start: %#v", events)
	}
	if !hasEvent(events, "toolcall_delta") {
		t.Fatalf("missing toolcall_delta: %#v", events)
	}
	var final *ai.ToolCall
	for _, event := range events {
		if event.Type == "toolcall_end" {
			final = event.ToolCall
		}
	}
	if final == nil {
		t.Fatalf("missing toolcall_end: %#v", events)
	}
	if final.ID != "call_1" || final.Name != "Grep" || final.Arguments["pattern"] != "TODO" {
		t.Fatalf("final tool call = %#v", final)
	}
}

func TestOpenAICompletionsRequestEnablesAutoToolChoice(t *testing.T) {
	body := buildOpenAICompletionsRequest(ai.Model{Name: "glm-5.1"}, ai.Context{
		Messages: []ai.Message{ai.NewUserMessage("hello")},
		Tools: []ai.Tool{
			{
				Name:        "Read",
				Description: "read file",
				Parameters:  map[string]interface{}{"type": "object"},
			},
		},
	}, &OpenAICompletionsOptions{})

	if body["tool_choice"] != "auto" {
		t.Fatalf("tool_choice = %#v, want auto", body["tool_choice"])
	}
	tools, ok := body["tools"].([]map[string]interface{})
	if !ok || len(tools) != 1 {
		t.Fatalf("tools = %#v, want one tool", body["tools"])
	}
}

func hasEvent(events []ai.StreamEvent, typ string) bool {
	for _, event := range events {
		if event.Type == typ {
			return true
		}
	}
	return false
}
