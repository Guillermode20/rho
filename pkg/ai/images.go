package ai

import "context"

// ImagesApi identifies a known image generation API.
type ImagesApi string

const (
	ImagesAPIOpenRouter ImagesApi = "openrouter-images"
)

// ImagesProvider identifies a known image generation provider.
type ImagesProvider string

const (
	ImagesProviderOpenRouter ImagesProvider = "openrouter"
)

// ImagesInputContent is the input for image generation.
type ImagesInputContent struct {
	Type     string `json:"type"`               // "text" or "image"
	Text     string `json:"text,omitempty"`     // prompt text
	Data     string `json:"data,omitempty"`     // base64 image data (for variation/image-to-image)
	MimeType string `json:"mimeType,omitempty"` // e.g. "image/png"
}

// ImagesOutputContent is a generated image result.
type ImagesOutputContent struct {
	Type     string `json:"type"`           // "image" or "text"
	Data     string `json:"data"`           // base64 encoded image data
	MimeType string `json:"mimeType"`       // MIME type of the result
	Text     string `json:"text,omitempty"` // alt text or caption
}

// ImagesContext holds the inputs for image generation.
type ImagesContext struct {
	Input []ImagesInputContent `json:"input"`
}

// ImagesStopReason describes why image generation stopped.
type ImagesStopReason string

const (
	ImagesStopStop    ImagesStopReason = "stop"
	ImagesStopError   ImagesStopReason = "error"
	ImagesStopAborted ImagesStopReason = "aborted"
)

// AssistantImages represents the result of image generation.
type AssistantImages struct {
	API          ImagesApi             `json:"api"`
	Provider     ImagesProvider        `json:"provider"`
	Model        string                `json:"model"`
	Output       []ImagesOutputContent `json:"output"`
	ResponseID   string                `json:"responseId,omitempty"`
	Usage        *Usage                `json:"usage,omitempty"`
	StopReason   ImagesStopReason      `json:"stopReason"`
	ErrorMessage string                `json:"errorMessage,omitempty"`
	Timestamp    int64                 `json:"timestamp"`
}

// ImageModel describes an image generation model.
type ImageModel struct {
	API         ImagesApi      `json:"api"`
	Provider    ImagesProvider `json:"provider"`
	Name        string         `json:"name"`
	BaseURL     string         `json:"baseUrl,omitempty"`
	Input       []string       `json:"input"` // "text", "image"
	Cost        CostPerMillion `json:"cost"`
	Description string         `json:"description,omitempty"`
}

// ImagesOptions configures image generation requests.
type ImagesOptions struct {
	Signal         context.Context        `json:"-"`
	APIKey         string                 `json:"apiKey,omitempty"`
	Headers        map[string]string      `json:"headers,omitempty"`
	TimeoutMs      int                    `json:"timeoutMs,omitempty"`
	MaxRetries     int                    `json:"maxRetries,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Size           string                 `json:"size,omitempty"`            // e.g. "1024x1024"
	Quality        string                 `json:"quality,omitempty"`         // "standard", "hd"
	Style          string                 `json:"style,omitempty"`           // "vivid", "natural"
	N              int                    `json:"n,omitempty"`               // number of images
	ResponseFormat string                 `json:"response_format,omitempty"` // "b64_json" or "url"
}

// ImagesFunction is the signature for image generation providers.
type ImagesFunction func(model ImageModel, ctx ImagesContext, options *ImagesOptions) (*AssistantImages, error)
