package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/earendil-works/rho/pkg/ai"
)

// AnthropicEffort represents thinking effort levels.
type AnthropicEffort string

const (
	EffortLow    AnthropicEffort = "low"
	EffortMedium AnthropicEffort = "medium"
	EffortHigh   AnthropicEffort = "high"
	EffortXHigh  AnthropicEffort = "xhigh"
	EffortMax    AnthropicEffort = "max"
)

// AnthropicOptions extends the base stream options.
type AnthropicOptions struct {
	ai.StreamOptions
	ThinkingEnabled     bool              `json:"thinking_enabled,omitempty"`
	ThinkingBudgetTokens int              `json:"thinking_budget_tokens,omitempty"`
	Effort              AnthropicEffort   `json:"effort,omitempty"`
	ToolChoice          interface{}       `json:"tool_choice,omitempty"`
	BetaHeaders         []string          `json:"-"`
}

const (
	anthropicDefaultBaseURL = "https://api.anthropic.com"
	anthropicAPIVersion     = "2023-06-01"
)

// StreamAnthropic streams a conversation against the Anthropic Messages API.
func StreamAnthropic(model ai.Model, ctx ai.Context, options *ai.StreamOptions, callback ai.StreamEventCallback) error {
	opts := &AnthropicOptions{}
	if options != nil {
		opts.StreamOptions = *options
	}
	return streamAnthropicMessages(model, ctx, opts, callback)
}

// StreamSimpleAnthropic is the simple version of the Anthropic stream function.
func StreamSimpleAnthropic(model ai.Model, ctx ai.Context, options *ai.SimpleStreamOptions, callback ai.StreamEventCallback) error {
	opts := &AnthropicOptions{}
	if options != nil {
		opts.StreamOptions = options.StreamOptions
		if options.Reasoning != "" {
			opts.ThinkingEnabled = true
			opts.Effort = AnthropicEffort(options.Reasoning)
		}
	}
	return streamAnthropicMessages(model, ctx, opts, callback)
}

func streamAnthropicMessages(model ai.Model, ctx ai.Context, opts *AnthropicOptions, callback ai.StreamEventCallback) error {
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = GetEnvAPIKey("ANTHROPIC_API_KEY", "CLAUDE_API_KEY")
	}
	if apiKey == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	baseURL := model.BaseURL
	if baseURL == "" {
		baseURL = BaseURLFromEnv("ANTHROPIC_BASE_URL", anthropicDefaultBaseURL)
	}

	// Build the request body
	body := buildAnthropicRequest(model, ctx, opts)
	jsonData, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(context.Background(), "POST", baseURL+"/v1/messages", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)

	// Add beta headers
	for _, beta := range opts.BetaHeaders {
		req.Header.Add("anthropic-beta", beta)
	}

	// Send request
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Anthropic API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse SSE stream
	return parseAnthropicSSE(resp.Body, callback, model)
}

func buildAnthropicRequest(model ai.Model, ctx ai.Context, opts *AnthropicOptions) map[string]interface{} {
	body := map[string]interface{}{
		"model":      model.Name,
		"max_tokens": 8192,
	}

	if opts.MaxTokens > 0 {
		body["max_tokens"] = opts.MaxTokens
	}

	// Convert messages
	var messages []map[string]interface{}
	systemPrompt := ""

	for _, msg := range ctx.Messages {
		switch {
		case msg.User != nil:
			m := map[string]interface{}{
				"role": "user",
			}
			content := msg.User.Content
			if s, ok := content.(string); ok {
				m["content"] = s
			} else {
				m["content"] = content
			}
			messages = append(messages, m)

		case msg.Assistant != nil:
			blocks := convertContentBlocks(msg.Assistant.Content)
			m := map[string]interface{}{
				"role":    "assistant",
				"content": blocks,
			}
			messages = append(messages, m)

		case msg.ToolResult != nil:
			tr := msg.ToolResult
			var content interface{} = tr.Content
			if len(tr.Content) == 1 && tr.Content[0].Text != nil {
				content = tr.Content[0].Text.Text
			}
			messages = append(messages, map[string]interface{}{
				"role":        "user",
				"content": []map[string]interface{}{
					{
						"type":        "tool_result",
						"tool_use_id": tr.ToolCallID,
						"content":     content,
						"is_error":    tr.IsError,
					},
				},
			})
		}
	}

	body["messages"] = messages

	// System prompt
	if ctx.SystemPrompt != "" {
		systemPrompt = ctx.SystemPrompt
	}
	if systemPrompt != "" {
		body["system"] = []map[string]interface{}{
			{"type": "text", "text": systemPrompt},
		}
	}

	// Tools
	if len(ctx.Tools) > 0 {
		var tools []map[string]interface{}
		for _, t := range ctx.Tools {
			tools = append(tools, map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
				"input_schema": t.Parameters,
			})
		}
		body["tools"] = tools
	}

	// Thinking/reasoning
	if opts.ThinkingEnabled {
		thinking := map[string]interface{}{
			"type": "enabled",
		}
		if opts.Effort != "" {
			thinking["effort"] = string(opts.Effort)
		}
		if opts.ThinkingBudgetTokens > 0 {
			thinking["budget_tokens"] = opts.ThinkingBudgetTokens
		}
		body["thinking"] = thinking
	}

	// Temperature
	if opts.Temperature > 0 {
		body["temperature"] = opts.Temperature
	}

	return body
}

func buildAnthropicSystemPrompt(ctx ai.Context) string {
	if ctx.SystemPrompt != "" {
		return ctx.SystemPrompt
	}
	return ""
}

// convertContentBlocks converts AI content blocks to Anthropic-format content blocks.
func convertContentBlocks(blocks []ai.ContentBlock) []map[string]interface{} {
	var result []map[string]interface{}
	for _, block := range blocks {
		switch {
		case block.Text != nil:
			result = append(result, map[string]interface{}{
				"type": "text",
				"text": block.Text.Text,
			})
		case block.ToolCall != nil:
			tc := block.ToolCall
			result = append(result, map[string]interface{}{
				"type":  "tool_use",
				"id":    tc.ID,
				"name":  tc.Name,
				"input": tc.Arguments,
			})
		case block.Thinking != nil:
			result = append(result, map[string]interface{}{
				"type":     "thinking",
				"thinking": block.Thinking.Thinking,
			})
		case block.Image != nil:
			result = append(result, map[string]interface{}{
				"type": "image",
				"source": map[string]interface{}{
					"type":      "base64",
					"media_type": block.Image.MimeType,
					"data":      block.Image.Data,
				},
			})
		}
	}
	return result
}

// parseAnthropicSSE parses the Anthropic SSE stream.
func parseAnthropicSSE(body io.ReadCloser, callback ai.StreamEventCallback, model ai.Model) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var msg ai.AssistantMessage
	msg.Role = ai.RoleAssistant
	msg.API = model.API
	msg.Provider = model.Provider
	msg.Model = model.Name
	msg.Timestamp = time.Now().UnixMilli()

	var currentBlockType string
	var contentIndex int
	var toolCallID, toolName string
	var toolArgsBuilder strings.Builder
	var thinkingBuilder strings.Builder
	var textBuilder strings.Builder

	emitPartial := func() {
		partial := msg
		callback(ai.StreamEvent{
			Type:    "start",
			ContentIndex: contentIndex,
			Partial: &partial,
		})
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "event: ") {
			continue // Event type is in the next "data:" line
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		var event struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "message_start":
			var start struct {
				Message struct {
					ID      string `json:"id"`
					Content []struct {
						Type string `json:"type"`
					} `json:"content"`
					Usage struct {
						InputTokens  int `json:"input_tokens"`
						OutputTokens int `json:"output_tokens"`
					} `json:"usage"`
				} `json:"message"`
			}
			json.Unmarshal([]byte(data), &start)
			msg.ResponseID = start.Message.ID
			msg.Usage.Input = start.Message.Usage.InputTokens
			msg.Usage.Output = start.Message.Usage.OutputTokens
			emitPartial()

		case "message_delta":
			var delta struct {
				Delta struct {
					StopReason string `json:"stop_reason"`
				} `json:"delta"`
				Usage struct {
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			json.Unmarshal([]byte(data), &delta)
			msg.Usage.Output = delta.Usage.OutputTokens
			switch delta.Delta.StopReason {
			case "end_turn":
				msg.StopReason = ai.StopReasonStop
			case "max_tokens":
				msg.StopReason = ai.StopReasonLength
			case "tool_use":
				msg.StopReason = ai.StopReasonToolUse
			}

		case "message_stop":
			// Finalize
			msg.Content = buildContentBlocks(currentBlockType, contentIndex, textBuilder.String(), toolCallID, toolName, toolArgsBuilder.String(), thinkingBuilder.String())
			if msg.StopReason == "" {
				msg.StopReason = ai.StopReasonStop
			}
			callback(ai.StreamEvent{
				Type:    "done",
				Message: &msg,
			})
			return nil

		case "content_block_start":
			var start struct {
				Index     int             `json:"index"`
				ContentBlock json.RawMessage `json:"content_block"`
			}
			json.Unmarshal([]byte(data), &start)
			contentIndex = start.Index

			var block struct {
				Type string `json:"type"`
			}
			json.Unmarshal(start.ContentBlock, &block)
			currentBlockType = block.Type

			switch block.Type {
			case "tool_use":
				var tool struct {
					ID   string          `json:"id"`
					Name string          `json:"name"`
					Input json.RawMessage `json:"input"`
				}
				json.Unmarshal(start.ContentBlock, &tool)
				toolCallID = tool.ID
				toolName = tool.Name
				toolArgsBuilder.Reset()
				toolArgsBuilder.WriteString(string(tool.Input))

				tc := ai.ToolCall{
					Type:      "toolCall",
					ID:        toolCallID,
					Name:      toolName,
					Arguments: make(map[string]interface{}),
				}
				json.Unmarshal(tool.Input, &tc.Arguments)

				msg.Content = append(msg.Content, ai.ContentBlock{ToolCall: &tc})
				callback(ai.StreamEvent{
					Type:    "toolcall_start",
					ContentIndex: contentIndex,
					ToolCall: &tc,
				})

			case "text":
				textBuilder.Reset()

			case "thinking":
				thinkingBuilder.Reset()
				callback(ai.StreamEvent{
					Type:    "thinking_start",
					ContentIndex: contentIndex,
				})
			}

		case "content_block_delta":
			var delta struct {
				Index int             `json:"index"`
				Delta json.RawMessage `json:"delta"`
			}
			json.Unmarshal([]byte(data), &delta)

			var d struct {
				Type string `json:"type"`
			}
			json.Unmarshal(delta.Delta, &d)

			switch d.Type {
			case "text_delta":
				var td struct {
					Text string `json:"text"`
				}
				json.Unmarshal(delta.Delta, &td)
				textBuilder.WriteString(td.Text)

				msg.Content = buildContentBlocks("text", contentIndex, textBuilder.String(), toolCallID, toolName, toolArgsBuilder.String(), thinkingBuilder.String())
				callback(ai.StreamEvent{
					Type:    "text_delta",
					ContentIndex: contentIndex,
					Delta:   td.Text,
					Partial: &ai.AssistantMessage{
						Role:    ai.RoleAssistant,
						Content: msg.Content,
					},
				})

			case "input_json_delta":
				var id struct {
					PartialJSON string `json:"partial_json"`
				}
				json.Unmarshal(delta.Delta, &id)
				toolArgsBuilder.WriteString(id.PartialJSON)

			case "thinking_delta":
				var td struct {
					Thinking string `json:"thinking"`
				}
				json.Unmarshal(delta.Delta, &td)
				thinkingBuilder.WriteString(td.Thinking)

				msg.Content = buildContentBlocks("thinking", contentIndex, textBuilder.String(), toolCallID, toolName, toolArgsBuilder.String(), thinkingBuilder.String())
				callback(ai.StreamEvent{
					Type:    "thinking_delta",
					ContentIndex: contentIndex,
					Delta:   td.Thinking,
				})
			}

		case "content_block_stop":
			switch currentBlockType {
			case "text":
				bt := ai.TextContent{Type: "text", Text: textBuilder.String()}
				msg.Content = append(msg.Content, ai.ContentBlock{Text: &bt})
				callback(ai.StreamEvent{
					Type:    "text_end",
					ContentIndex: contentIndex,
				})
			case "tool_use":
				var args map[string]interface{}
				json.Unmarshal([]byte(toolArgsBuilder.String()), &args)
				tc := ai.ToolCall{
					Type:      "toolCall",
					ID:        toolCallID,
					Name:      toolName,
					Arguments: args,
				}
				msg.Content = append(msg.Content, ai.ContentBlock{ToolCall: &tc})
				callback(ai.StreamEvent{
					Type:    "toolcall_end",
					ContentIndex: contentIndex,
					ToolCall: &tc,
				})
			case "thinking":
				bt := ai.ThinkingContent{Type: "thinking", Thinking: thinkingBuilder.String()}
				msg.Content = append(msg.Content, ai.ContentBlock{Thinking: &bt})
				callback(ai.StreamEvent{
					Type:    "thinking_end",
					ContentIndex: contentIndex,
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("SSE read error: %w", err)
	}

	// If we got here without message_stop, emit done
	if msg.StopReason == "" {
		msg.StopReason = ai.StopReasonStop
	}
	callback(ai.StreamEvent{
		Type:    "done",
		Message: &msg,
	})
	return nil
}

func buildContentBlocks(blockType string, contentIndex int, text, toolCallID, toolName, toolArgs, thinking string) []ai.ContentBlock {
	var blocks []ai.ContentBlock
	if text != "" {
		blocks = append(blocks, ai.ContentBlock{
			Text: &ai.TextContent{Type: "text", Text: text},
		})
	}
	if toolCallID != "" {
		var args map[string]interface{}
		json.Unmarshal([]byte(toolArgs), &args)
		blocks = append(blocks, ai.ContentBlock{
			ToolCall: &ai.ToolCall{
				Type:      "toolCall",
				ID:        toolCallID,
				Name:      toolName,
				Arguments: args,
			},
		})
	}
	if thinking != "" {
		blocks = append(blocks, ai.ContentBlock{
			Thinking: &ai.ThinkingContent{Type: "thinking", Thinking: thinking},
		})
	}
	return blocks
}

func init() {
	Register(&StreamProvider{
		API:          ai.APIAnthropicMessages,
		Stream:       StreamAnthropic,
		StreamSimple: StreamSimpleAnthropic,
	})
}
