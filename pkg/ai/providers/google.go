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

const googleDefaultBaseURL = "https://generativelanguage.googleapis.com"

// GoogleOptions extend base options.
type GoogleOptions struct {
	ai.StreamOptions
	SafetySettings []map[string]interface{} `json:"safety_settings,omitempty"`
}

// StreamGoogle streams against the Google Generative AI API.
func StreamGoogle(model ai.Model, ctx ai.Context, options *ai.StreamOptions, callback ai.StreamEventCallback) error {
	opts := &GoogleOptions{}
	if options != nil {
		opts.StreamOptions = *options
	}
	return streamGoogleGenAI(model, ctx, opts, callback)
}

// StreamSimpleGoogle is the simple version.
func StreamSimpleGoogle(model ai.Model, ctx ai.Context, options *ai.SimpleStreamOptions, callback ai.StreamEventCallback) error {
	opts := &GoogleOptions{}
	if options != nil {
		opts.StreamOptions = options.StreamOptions
	}
	return streamGoogleGenAI(model, ctx, opts, callback)
}

func streamGoogleGenAI(model ai.Model, ctx ai.Context, opts *GoogleOptions, callback ai.StreamEventCallback) error {
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = GetEnvAPIKey("GOOGLE_API_KEY", "GEMINI_API_KEY")
	}
	if apiKey == "" {
		return fmt.Errorf("GOOGLE_API_KEY not set")
	}

	baseURL := model.BaseURL
	if baseURL == "" {
		baseURL = BaseURLFromEnv("GOOGLE_GENAI_BASE_URL", googleDefaultBaseURL)
	}

	// Build request
	body := buildGoogleRequest(model, ctx, opts)
	jsonData, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s", baseURL, model.Name, apiKey)

	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Google API error (status %d): %s", resp.StatusCode, string(b))
	}

	return parseGoogleSSE(resp.Body, callback, model)
}

func buildGoogleRequest(model ai.Model, ctx ai.Context, opts *GoogleOptions) map[string]interface{} {
	body := map[string]interface{}{
		"contents": buildGoogleContents(ctx),
		"generationConfig": map[string]interface{}{},
	}

	if ctx.SystemPrompt != "" {
		body["system_instruction"] = map[string]interface{}{
			"parts": []map[string]interface{}{
				{"text": ctx.SystemPrompt},
			},
		}
	}

	if opts.MaxTokens > 0 {
		body["generationConfig"].(map[string]interface{})["max_output_tokens"] = opts.MaxTokens
	}
	if opts.Temperature > 0 {
		body["generationConfig"].(map[string]interface{})["temperature"] = opts.Temperature
	}

	if len(ctx.Tools) > 0 {
		var tools []map[string]interface{}
		for _, t := range ctx.Tools {
			tools = append(tools, map[string]interface{}{
				"functionDeclarations": []map[string]interface{}{
					{
						"name":        t.Name,
						"description": t.Description,
						"parameters":  t.Parameters,
					},
				},
			})
		}
		body["tools"] = tools
	}

	return body
}

func buildGoogleContents(ctx ai.Context) []map[string]interface{} {
	var contents []map[string]interface{}

	for _, msg := range ctx.Messages {
		switch {
		case msg.User != nil:
			content := msg.User.Content
			parts := []map[string]interface{}{
				{"text": content},
			}
			contents = append(contents, map[string]interface{}{
				"role":  "user",
				"parts": parts,
			})

		case msg.Assistant != nil:
			var parts []map[string]interface{}
			for _, block := range msg.Assistant.Content {
				switch {
				case block.Text != nil:
					parts = append(parts, map[string]interface{}{"text": block.Text.Text})
				case block.ToolCall != nil:
					argsJSON, _ := json.Marshal(block.ToolCall.Arguments)
					parts = append(parts, map[string]interface{}{
						"functionCall": map[string]interface{}{
							"name": block.ToolCall.Name,
							"args": json.RawMessage(argsJSON),
						},
					})
				}
			}
			if len(parts) > 0 {
				contents = append(contents, map[string]interface{}{
					"role":  "model",
					"parts": parts,
				})
			}

		case msg.ToolResult != nil:
			tr := msg.ToolResult
			content := ""
			if len(tr.Content) > 0 && tr.Content[0].Text != nil {
				content = tr.Content[0].Text.Text
			}
			contents = append(contents, map[string]interface{}{
				"role": "function",
				"parts": []map[string]interface{}{
					{
						"functionResponse": map[string]interface{}{
							"name": tr.ToolName,
							"response": map[string]interface{}{
								"content": content,
							},
						},
					},
				},
			})
		}
	}

	return contents
}

// Google SSE response types
type googleResponse struct {
	Candidates []struct {
		Index          int `json:"index"`
		Content        struct {
			Parts []struct {
				Text         string `json:"text"`
				FunctionCall *struct {
					Name string          `json:"name"`
					Args json.RawMessage `json:"args"`
				} `json:"functionCall"`
			} `json:"parts"`
			Role string `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

func parseGoogleSSE(body io.ReadCloser, callback ai.StreamEventCallback, model ai.Model) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var msg ai.AssistantMessage
	msg.Role = ai.RoleAssistant
	msg.API = model.API
	msg.Provider = model.Provider
	msg.Model = model.Name
	msg.Timestamp = time.Now().UnixMilli()

	var textBuilder strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var gr googleResponse
		if err := json.Unmarshal([]byte(data), &gr); err != nil {
			continue
		}

		if gr.UsageMetadata != nil {
			msg.Usage.Input = gr.UsageMetadata.PromptTokenCount
			msg.Usage.Output = gr.UsageMetadata.CandidatesTokenCount
			msg.Usage.TotalTokens = gr.UsageMetadata.TotalTokenCount
		}

		for _, candidate := range gr.Candidates {
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					textBuilder.WriteString(part.Text)
					msg.Content = []ai.ContentBlock{
						{Text: &ai.TextContent{Type: "text", Text: textBuilder.String()}},
					}
					callback(ai.StreamEvent{
						Type:  "text_delta",
						Delta: part.Text,
					})
				}

				if part.FunctionCall != nil {
					var args map[string]interface{}
					json.Unmarshal(part.FunctionCall.Args, &args)
					tc := ai.ToolCall{
						Type:      "toolCall",
						ID:        part.FunctionCall.Name,
						Name:      part.FunctionCall.Name,
						Arguments: args,
					}
					msg.Content = append(msg.Content, ai.ContentBlock{
						ToolCall: &tc,
					})
					callback(ai.StreamEvent{
						Type:    "toolcall_end",
						ToolCall: &tc,
					})
				}
			}

			switch candidate.FinishReason {
			case "STOP":
				msg.StopReason = ai.StopReasonStop
			case "MAX_TOKENS":
				msg.StopReason = ai.StopReasonLength
			case "SAFETY", "RECITATION", "OTHER":
				msg.StopReason = ai.StopReasonStop
			}
		}
	}

	if msg.StopReason == "" {
		msg.StopReason = ai.StopReasonStop
	}

	callback(ai.StreamEvent{
		Type:    "done",
		Message: &msg,
	})

	return scanner.Err()
}

func init() {
	Register(&StreamProvider{
		API:          ai.APIGoogleGenerativeAI,
		Stream:       StreamGoogle,
		StreamSimple: StreamSimpleGoogle,
	})
}
