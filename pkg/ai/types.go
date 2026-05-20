// Package ai provides a unified LLM API for interacting with various AI providers.
package ai

import (
	"context"
	"time"
)

// Known API types.
type API string

const (
	APIOpenAICompletions     API = "openai-completions"
	APIMistralConversations  API = "mistral-conversations"
	APIOpenAIResponses       API = "openai-responses"
	APIAzureOpenAIResponses  API = "azure-openai-responses"
	APIOpenAICodexResponses  API = "openai-codex-responses"
	APIAnthropicMessages     API = "anthropic-messages"
	APIBedrockConverseStream API = "bedrock-converse-stream"
	APIGoogleGenerativeAI    API = "google-generative-ai"
	APIGoogleVertex          API = "google-vertex"
)

// Known providers.
type Provider string

const (
	ProviderAmazonBedrock        Provider = "amazon-bedrock"
	ProviderAnthropic            Provider = "anthropic"
	ProviderGoogle               Provider = "google"
	ProviderGoogleVertex         Provider = "google-vertex"
	ProviderOpenAI               Provider = "openai"
	ProviderAzureOpenAIResponses Provider = "azure-openai-responses"
	ProviderOpenAICodex          Provider = "openai-codex"
	ProviderDeepSeek             Provider = "deepseek"
	ProviderGitHubCopilot        Provider = "github-copilot"
	ProviderXAI                  Provider = "xai"
	ProviderGroq                 Provider = "groq"
	ProviderCerebras             Provider = "cerebras"
	ProviderCrof                 Provider = "crof"
	ProviderOpenRouter           Provider = "openrouter"
	ProviderVercelAIGateway      Provider = "vercel-ai-gateway"
	ProviderMistral              Provider = "mistral"
	ProviderFireworks            Provider = "fireworks"
	ProviderTogether             Provider = "together"
	ProviderCloudflareAIGateway  Provider = "cloudflare-ai-gateway"
	ProviderCloudflareWorkersAI  Provider = "cloudflare-workers-ai"
)

// ThinkingLevel represents the reasoning effort.
type ThinkingLevel string

const (
	ThinkingMinimal ThinkingLevel = "minimal"
	ThinkingLow     ThinkingLevel = "low"
	ThinkingMedium  ThinkingLevel = "medium"
	ThinkingHigh    ThinkingLevel = "high"
	ThinkingXHigh   ThinkingLevel = "xhigh"
)

// CacheRetention controls prompt caching behavior.
type CacheRetention string

const (
	CacheRetentionNone  CacheRetention = "none"
	CacheRetentionShort CacheRetention = "short"
	CacheRetentionLong  CacheRetention = "long"
)

// Transport for streaming.
type Transport string

const (
	TransportSSE             Transport = "sse"
	TransportWebSocket       Transport = "websocket"
	TransportWebSocketCached Transport = "websocket-cached"
	TransportAuto            Transport = "auto"
)

// StopReason describes why message generation stopped.
type StopReason string

const (
	StopReasonStop    StopReason = "stop"
	StopReasonLength  StopReason = "length"
	StopReasonToolUse StopReason = "toolUse"
	StopReasonError   StopReason = "error"
	StopReasonAborted StopReason = "aborted"
)

// Role identifies the sender of a message.
type Role string

const (
	RoleUser       Role = "user"
	RoleAssistant  Role = "assistant"
	RoleToolResult Role = "toolResult"
)

// Content types for messages.

// TextContent represents a text content block.
type TextContent struct {
	Type          string `json:"type"`
	Text          string `json:"text"`
	TextSignature string `json:"textSignature,omitempty"`
}

// ThinkingContent represents a reasoning/thinking content block.
type ThinkingContent struct {
	Type              string `json:"type"`
	Thinking          string `json:"thinking"`
	ThinkingSignature string `json:"thinkingSignature,omitempty"`
	Redacted          bool   `json:"redacted,omitempty"`
}

// ImageContent represents an image content block.
type ImageContent struct {
	Type     string `json:"type"`
	Data     string `json:"data"`     // base64 encoded
	MimeType string `json:"mimeType"` // e.g., "image/jpeg"
}

// ToolCall represents a tool use by the model.
type ToolCall struct {
	Type             string                 `json:"type"`
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Arguments        map[string]interface{} `json:"arguments"`
	ThoughtSignature string                 `json:"thoughtSignature,omitempty"`
}

// Usage contains token usage information.
type Usage struct {
	Input       int  `json:"input"`
	Output      int  `json:"output"`
	CacheRead   int  `json:"cacheRead"`
	CacheWrite  int  `json:"cacheWrite"`
	TotalTokens int  `json:"totalTokens"`
	Cost        Cost `json:"cost"`
}

// Cost contains pricing information.
type Cost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
	Total      float64 `json:"total"`
}

// Messages.

// UserMessage represents a message from the user.
type UserMessage struct {
	Role      Role        `json:"role"`
	Content   interface{} `json:"content"` // string or []ContentBlock
	Timestamp int64       `json:"timestamp"`
}

// AssistantMessage represents a message from the assistant.
type AssistantMessage struct {
	Role          Role           `json:"role"`
	Content       []ContentBlock `json:"content"`
	API           API            `json:"api"`
	Provider      Provider       `json:"provider"`
	Model         string         `json:"model"`
	ResponseModel string         `json:"responseModel,omitempty"`
	ResponseID    string         `json:"responseId,omitempty"`
	Usage         Usage          `json:"usage"`
	StopReason    StopReason     `json:"stopReason"`
	ErrorMessage  string         `json:"errorMessage,omitempty"`
	Timestamp     int64          `json:"timestamp"`
}

// ToolResultMessage represents the result of a tool call.
type ToolResultMessage struct {
	Role       Role           `json:"role"`
	ToolCallID string         `json:"toolCallId"`
	ToolName   string         `json:"toolName"`
	Content    []ContentBlock `json:"content"`
	IsError    bool           `json:"isError"`
	Timestamp  int64          `json:"timestamp"`
}

// ContentBlock is a union type for content blocks.
type ContentBlock struct {
	Text     *TextContent     `json:"text,omitempty"`
	Thinking *ThinkingContent `json:"thinking,omitempty"`
	Image    *ImageContent    `json:"image,omitempty"`
	ToolCall *ToolCall        `json:"toolCall,omitempty"`
}

// Message is a union type for all message types.
type Message struct {
	User       *UserMessage       `json:"user,omitempty"`
	Assistant  *AssistantMessage  `json:"assistant,omitempty"`
	ToolResult *ToolResultMessage `json:"toolResult,omitempty"`
}

// Tool defines a tool that the model can call.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"` // JSON schema
}

// Context is the input to a model call.
type Context struct {
	SystemPrompt string    `json:"systemPrompt,omitempty"`
	Messages     []Message `json:"messages"`
	Tools        []Tool    `json:"tools,omitempty"`
}

// Model represents a registered AI model.
type Model struct {
	API      API      `json:"api"`
	Provider Provider `json:"provider"`
	Name     string   `json:"name"`
	BaseURL  string   `json:"baseUrl,omitempty"`
}

// StreamOptions configures streaming model calls.
type StreamOptions struct {
	Temperature     float64                `json:"temperature,omitempty"`
	MaxTokens       int                    `json:"maxTokens,omitempty"`
	Signal          context.Context        `json:"-"` // For cancellation
	APIKey          string                 `json:"apiKey,omitempty"`
	Transport       Transport              `json:"transport,omitempty"`
	CacheRetention  CacheRetention         `json:"cacheRetention,omitempty"`
	SessionID       string                 `json:"sessionId,omitempty"`
	Headers         map[string]string      `json:"headers,omitempty"`
	TimeoutMs       int                    `json:"timeoutMs,omitempty"`
	MaxRetries      int                    `json:"maxRetries,omitempty"`
	MaxRetryDelayMs int                    `json:"maxRetryDelayMs,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// SimpleStreamOptions extends StreamOptions with reasoning settings.
type SimpleStreamOptions struct {
	StreamOptions
	Reasoning       ThinkingLevel         `json:"reasoning,omitempty"`
	ThinkingBudgets map[ThinkingLevel]int `json:"thinkingBudgets,omitempty"`
}

// StreamEvent represents a single event in the response stream.
type StreamEvent struct {
	Type         string            `json:"type"`
	ContentIndex int               `json:"contentIndex,omitempty"`
	Delta        string            `json:"delta,omitempty"`
	Content      string            `json:"content,omitempty"`
	ToolCall     *ToolCall         `json:"toolCall,omitempty"`
	Partial      *AssistantMessage `json:"partial,omitempty"`
	Message      *AssistantMessage `json:"message,omitempty"`
	Error        *AssistantMessage `json:"error,omitempty"`
}

// StreamEventCallback is called for each event in the stream.
type StreamEventCallback func(event StreamEvent) error

// StreamFunction is the signature for streaming model calls.
type StreamFunction func(model Model, ctx Context, options *SimpleStreamOptions, callback StreamEventCallback) error

// ModelRegistry manages available models and providers.
type ModelRegistry struct {
	models    []Model
	providers map[Provider]StreamFunction
}

// NewModelRegistry creates a new model registry.
func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		providers: make(map[Provider]StreamFunction),
	}
}

// RegisterModel adds a model to the registry.
func (r *ModelRegistry) RegisterModel(m Model) {
	r.models = append(r.models, m)
}

// RegisterProvider registers a stream function for a provider.
func (r *ModelRegistry) RegisterProvider(provider Provider, fn StreamFunction) {
	r.providers[provider] = fn
}

// GetModels returns all registered models.
func (r *ModelRegistry) GetModels() []Model {
	return r.models
}

// GetStreamFunction returns the stream function for a given provider.
func (r *ModelRegistry) GetStreamFunction(provider Provider) (StreamFunction, bool) {
	fn, ok := r.providers[provider]
	return fn, ok
}

// Stream calls the appropriate provider's stream function.
func (r *ModelRegistry) Stream(model Model, ctx Context, options *SimpleStreamOptions, callback StreamEventCallback) error {
	fn, ok := r.providers[model.Provider]
	if !ok {
		return &ProviderError{Provider: string(model.Provider), Message: "no stream function registered for provider"}
	}
	return fn(model, ctx, options, callback)
}

// ProviderError represents an error from an AI provider.
type ProviderError struct {
	Provider string
	Message  string
	Code     int
}

func (e *ProviderError) Error() string {
	if e.Code > 0 {
		return e.Message
	}
	return e.Provider + ": " + e.Message
}

// NewUserMessage creates a new user message.
func NewUserMessage(content string) Message {
	return Message{
		User: &UserMessage{
			Role:      RoleUser,
			Content:   content,
			Timestamp: time.Now().UnixMilli(),
		},
	}
}

// NewAssistantMessage creates a new assistant message.
func NewAssistantMessage(api API, provider Provider, modelName string) AssistantMessage {
	return AssistantMessage{
		Role:      RoleAssistant,
		Content:   nil,
		API:       api,
		Provider:  provider,
		Model:     modelName,
		Timestamp: time.Now().UnixMilli(),
	}
}

// NewToolResultMessage creates a new tool result message.
func NewToolResultMessage(toolCallID, toolName string, content string, isError bool) Message {
	return Message{
		ToolResult: &ToolResultMessage{
			Role:       RoleToolResult,
			ToolCallID: toolCallID,
			ToolName:   toolName,
			Content: []ContentBlock{
				{Text: &TextContent{Type: "text", Text: content}},
			},
			IsError:   isError,
			Timestamp: time.Now().UnixMilli(),
		},
	}
}
