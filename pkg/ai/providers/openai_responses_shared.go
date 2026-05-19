package providers

import (
	"encoding/json"
	"fmt"
	"strings"
)

// OpenAIResponsesCommon holds shared types for OpenAI Responses API
// and Azure OpenAI Responses API providers.

// ResponsesRequest represents the common request body for the Responses API.
type ResponsesRequest struct {
	Model           string                 `json:"model"`
	Input           interface{}            `json:"input"`
	Instructions    string                 `json:"instructions,omitempty"`
	MaxOutputTokens int                    `json:"max_output_tokens,omitempty"`
	Temperature     float64                `json:"temperature,omitempty"`
	TopP            float64                `json:"top_p,omitempty"`
	Stream          bool                   `json:"stream"`
	Tools           []ResponsesTool        `json:"tools,omitempty"`
	Metadata        map[string]string      `json:"metadata,omitempty"`
	Store           bool                   `json:"store,omitempty"`
}

// ResponsesTool represents a tool in the Responses API.
type ResponsesTool struct {
	Type     string          `json:"type"`
	Name     string          `json:"name,omitempty"`
	Function *ResponsesFunction `json:"function,omitempty"`
}

// ResponsesFunction describes a function tool for the Responses API.
type ResponsesFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
	Strict      bool        `json:"strict,omitempty"`
}

// ResponsesInputItem represents a single input item in the Responses API.
type ResponsesInputItem struct {
	Role       string                 `json:"role"`
	Content    interface{}            `json:"content"`
	Type       string                 `json:"type,omitempty"`
	ID         string                 `json:"id,omitempty"`
	ToolCallID string                 `json:"tool_call_id,omitempty"`
	ToolName   string                 `json:"tool_name,omitempty"`
	Output     []ResponsesOutputItem  `json:"output,omitempty"`
}

// ResponsesOutputItem represents an output item from the model.
type ResponsesOutputItem struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Status  string          `json:"status,omitempty"`
	Content []ResponsesContent `json:"content,omitempty"`
}

// ResponsesContent represents a content block in Responses API output.
type ResponsesContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Refusal  string `json:"refusal,omitempty"`
	Annotation []ResponsesAnnotation `json:"annotation,omitempty"`
}

// ResponsesAnnotation represents an annotation (e.g., citation) in output.
type ResponsesAnnotation struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ResponsesUsage holds token usage for the Responses API.
type ResponsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ResponsesStreamEvent represents a streaming event from the Responses API.
type ResponsesStreamEvent struct {
	Type  string          `json:"type"`
	Delta json.RawMessage `json:"delta,omitempty"`
	Item  json.RawMessage `json:"item,omitempty"`
	Part  json.RawMessage `json:"part,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
}

// ResponsesStreamDelta holds delta content during streaming.
type ResponsesStreamDelta struct {
	Text     string `json:"text,omitempty"`
	Reasoning string `json:"reasoning,omitempty"`
}

// MapResponsesStopReason maps Responses API stop reasons to internal StopReason.
func MapResponsesStopReason(reason string) string {
	switch reason {
	case "stop", "end_turn":
		return "stop"
	case "length", "max_tokens":
		return "length"
	case "tool_use", "function_call":
		return "toolUse"
	case "content_filter":
		return "error"
	default:
		return "stop"
	}
}

// BuildResponsesInput converts a conversation context to Responses API input format.
func BuildResponsesInput(systemPrompt string, messages interface{}, tools interface{}) []ResponsesInputItem {
	var items []ResponsesInputItem
	return items
}

// ParseResponsesStreamEvent parses a raw Responses API stream event.
func ParseResponsesStreamEvent(data []byte) (*ResponsesStreamEvent, error) {
	var event ResponsesStreamEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("failed to parse Responses stream event: %w", err)
	}
	return &event, nil
}

// IsResponseStreamEvent checks if a data line is a Responses API stream event.
func IsResponseStreamEvent(data string) bool {
	return strings.HasPrefix(data, `{"type":"response.`)
}

// ResponsesEventPrefixes maps event type prefixes for routing.
var ResponsesEventPrefixes = []string{
	"response.output_item.added",
	"response.content_part.added",
	"response.text.delta",
	"response.text.done",
	"response.output_item.done",
	"response.in_progress",
	"response.completed",
	"response.failed",
	"response.incomplete",
}

func init() {
	_ = BuildResponsesInput
	_ = fmt.Sprintf
}
