package agent

import (
	"testing"

	"github.com/earendil-works/rho/pkg/ai"
)

func TestAgentLoopExecutesToolCallsAndContinues(t *testing.T) {
	loop := NewAgentLoop(AgentLoopConfig{
		Model: ai.Model{
			API:      ai.APIOpenAICompletions,
			Provider: ai.ProviderCrof,
			Name:     "glm-5.1",
		},
	})

	streamCalls := 0
	loop.SetStreamFn(func(model ai.Model, ctx ai.Context, options *ai.SimpleStreamOptions, callback ai.StreamEventCallback) error {
		streamCalls++
		switch streamCalls {
		case 1:
			callback(ai.StreamEvent{
				Type: "toolcall_end",
				ToolCall: &ai.ToolCall{
					Type:      "toolCall",
					ID:        "call_1",
					Name:      "Read",
					Arguments: map[string]interface{}{"path": "README.md"},
				},
			})
			callback(ai.StreamEvent{
				Type: "done",
				Message: &ai.AssistantMessage{
					Role:       ai.RoleAssistant,
					StopReason: ai.StopReasonToolUse,
				},
			})
		case 2:
			callback(ai.StreamEvent{Type: "text_delta", Delta: "I read it."})
			callback(ai.StreamEvent{
				Type: "done",
				Message: &ai.AssistantMessage{
					Role:       ai.RoleAssistant,
					StopReason: ai.StopReasonStop,
				},
			})
		default:
			t.Fatalf("unexpected stream call %d", streamCalls)
		}
		return nil
	})

	var executed bool
	var events []string
	results, err := loop.Run([]AgentMessage{{Role: ai.RoleUser, Content: "read README"}}, AgentContext{
		SystemPrompt: "test",
		Model:        loop.config.Model,
		Tools: []AgentTool{
			{
				Name:        "Read",
				Description: "read file",
				Parameters:  map[string]interface{}{"type": "object"},
				Execute: func(args map[string]interface{}) (string, bool, error) {
					executed = true
					if args["path"] != "README.md" {
						t.Fatalf("path arg = %v, want README.md", args["path"])
					}
					return "README contents", false, nil
				},
			},
		},
	}, func(event AgentEvent) error {
		events = append(events, event.Type)
		return nil
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if streamCalls != 2 {
		t.Fatalf("stream calls = %d, want 2", streamCalls)
	}
	if !executed {
		t.Fatal("tool was not executed")
	}
	if len(results) != 4 {
		t.Fatalf("result count = %d, want user/assistant/tool/assistant", len(results))
	}
	if results[0].Role != ai.RoleUser {
		t.Fatalf("first result = %#v", results[0])
	}
	if results[1].Role != ai.RoleAssistant || len(results[1].ToolCalls) != 1 {
		t.Fatalf("assistant tool call result = %#v", results[1])
	}
	if results[2].Role != ai.RoleToolResult || results[2].Content != "README contents" {
		t.Fatalf("tool result = %#v", results[2])
	}
	if results[3].Role != ai.RoleAssistant || results[3].Content != "I read it." {
		t.Fatalf("final assistant = %#v", results[3])
	}
	if !containsEvent(events, "tool_execution_start") || !containsEvent(events, "tool_execution_end") {
		t.Fatalf("missing tool execution events: %#v", events)
	}
}

func containsEvent(events []string, want string) bool {
	for _, event := range events {
		if event == want {
			return true
		}
	}
	return false
}
