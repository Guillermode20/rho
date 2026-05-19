package providers

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/earendil-works/rho/pkg/ai"
)

// CloudflareProviderType auto-detects which API to use based on the base URL.
type CloudflareProviderType int

const (
	CloudflareUnknown CloudflareProviderType = iota
	CloudflareAnthropic
	CloudflareOpenAI
)

// DetectCloudflareProvider parses a Cloudflare AI Gateway base URL to determine
// which upstream provider it routes to.
//
// Cloudflare AI Gateway URLs follow the pattern:
//
//	https://gateway.ai.cloudflare.com/v1/{account_id}/{gateway}/{provider}
//
// Where {provider} can be "anthropic", "openai", "azure-openai", etc.
func DetectCloudflareProvider(rawURL string) (CloudflareProviderType, string) {
	if rawURL == "" {
		return CloudflareUnknown, ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return CloudflareUnknown, ""
	}

	// Check if this is a Cloudflare AI Gateway URL
	if !strings.Contains(parsed.Host, "cloudflare.com") &&
		!strings.Contains(parsed.Host, "gateway.ai") {
		return CloudflareUnknown, ""
	}

	// Extract the provider segment from the path
	// Path: /v1/{account_id}/{gateway}/{provider}/...
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i, part := range parts {
		switch part {
		case "anthropic":
			return CloudflareAnthropic, strings.Join(parts[:i+1], "/")
		case "openai":
			return CloudflareOpenAI, strings.Join(parts[:i+1], "/")
		}
	}
	return CloudflareUnknown, parsed.Host
}

// ResolveCloudflareBaseURL resolves a Cloudflare AI Gateway base URL for a given provider.
// If the base URL already points to Cloudflare, it returns it as-is. If the model
// has a cloudflare-ai-gateway provider type configured, it constructs the URL.
func ResolveCloudflareBaseURL(model ai.Model) string {
	if model.BaseURL != "" {
		providerType, _ := DetectCloudflareProvider(model.BaseURL)
		if providerType != CloudflareUnknown {
			return model.BaseURL
		}
	}
	return ""
}

// IsCloudflareProvider checks if a model is routed through Cloudflare AI Gateway.
func IsCloudflareProvider(model ai.Model) bool {
	if model.Provider == "cloudflare-ai-gateway" || model.Provider == "cloudflare-workers-ai" {
		return true
	}
	if model.BaseURL != "" {
		pt, _ := DetectCloudflareProvider(model.BaseURL)
		return pt != CloudflareUnknown
	}
	return false
}

// CloudflareAPIKey resolves the API key for Cloudflare AI Gateway.
func CloudflareAPIKey() string {
	key := GetEnvAPIKey("CLOUDFLARE_API_KEY", "CLOUDFLARE_AI_TOKEN")
	if key != "" {
		return key
	}
	return ""
}

// IsCloudflareWorkersAI checks if the model is a Cloudflare Workers AI model.
func IsCloudflareWorkersAI(model ai.Model) bool {
	return model.Provider == "cloudflare-workers-ai" ||
		strings.Contains(model.BaseURL, "workers-ai")
}

// RuntimeErr represents an error message from Cloudflare.
type RuntimeErr struct {
	Message string `json:"message"`
	Code    int    `json:"code,omitempty"`
}

func (e *RuntimeErr) Error() string {
	if e.Code > 0 {
		return fmt.Sprintf("Cloudflare error (code %d): %s", e.Code, e.Message)
	}
	return "Cloudflare: " + e.Message
}

// isCloudflareProvider is a check used by other providers to detect Cloudflare routing.
func isCloudflareProvider(model ai.Model) bool {
	return IsCloudflareProvider(model)
}
