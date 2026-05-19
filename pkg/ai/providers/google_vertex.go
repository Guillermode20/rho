package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/earendil-works/rho/pkg/ai"
)

const vertexDefaultBaseURL = "https://us-central1-aiplatform.googleapis.com"

// VertexOptions extends base options.
type VertexOptions struct {
	ai.StreamOptions
	ProjectID string `json:"projectId,omitempty"`
	Location  string `json:"location,omitempty"`
}

// StreamGoogleVertex streams against Google Vertex AI.
func StreamGoogleVertex(model ai.Model, ctx ai.Context, options *ai.StreamOptions, callback ai.StreamEventCallback) error {
	opts := &VertexOptions{}
	if options != nil {
		opts.StreamOptions = *options
	}
	return streamVertex(model, ctx, opts, callback)
}

// StreamSimpleGoogleVertex is the simple version.
func StreamSimpleGoogleVertex(model ai.Model, ctx ai.Context, options *ai.SimpleStreamOptions, callback ai.StreamEventCallback) error {
	opts := &VertexOptions{}
	if options != nil {
		opts.StreamOptions = options.StreamOptions
	}
	return streamVertex(model, ctx, opts, callback)
}

func streamVertex(model ai.Model, ctx ai.Context, opts *VertexOptions, callback ai.StreamEventCallback) error {
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = GetEnvAPIKey("GOOGLE_VERTEX_API_KEY", "VERTEX_API_KEY")
	}

	projectID := opts.ProjectID
	if projectID == "" {
		projectID = GetEnvAPIKey("GOOGLE_CLOUD_PROJECT", "VERTEX_PROJECT_ID")
	}
	if projectID == "" {
		return fmt.Errorf("Vertex AI requires GOOGLE_CLOUD_PROJECT or projectId")
	}

	location := opts.Location
	if location == "" {
		location = GetEnvAPIKey("GOOGLE_CLOUD_REGION", "VERTEX_LOCATION")
	}
	if location == "" {
		location = "us-central1"
	}

	baseURL := model.BaseURL
	if baseURL == "" {
		baseURL = BaseURLFromEnv("VERTEX_BASE_URL", vertexDefaultBaseURL)
	}

	body := map[string]interface{}{
		"contents": buildVertexContents(ctx),
		"generationConfig": map[string]interface{}{},
	}
	if ctx.SystemPrompt != "" {
		body["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]interface{}{{"text": ctx.SystemPrompt}},
		}
	}
	if opts.MaxTokens > 0 {
		body["generationConfig"].(map[string]interface{})["maxOutputTokens"] = opts.MaxTokens
	}
	if opts.Temperature > 0 {
		body["generationConfig"].(map[string]interface{})["temperature"] = opts.Temperature
	}
	if len(ctx.Tools) > 0 {
		var tools []map[string]interface{}
		for _, t := range ctx.Tools {
			tools = append(tools, map[string]interface{}{
				"functionDeclarations": []map[string]interface{}{
					{"name": t.Name, "description": t.Description, "parameters": t.Parameters},
				},
			})
		}
		body["tools"] = tools
	}

	jsonData, _ := json.Marshal(body)
	publisher := "google"
	url := fmt.Sprintf("%s/v1/projects/%s/locations/%s/publishers/%s/models/%s:streamGenerateContent?alt=sse",
		baseURL, projectID, location, publisher, model.Name)
	if apiKey != "" {
		url += "&key=" + apiKey
	}

	req, _ := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Vertex AI request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Vertex AI API error (status %d): %s", resp.StatusCode, string(b))
	}

	// Reuse Google's SSE parser (same response format)
	return parseGoogleSSE(resp.Body, callback, model)
}

func buildVertexContents(ctx ai.Context) []map[string]interface{} {
	return buildGoogleContents(ctx)
}

func init() {
	Register(&StreamProvider{
		API:          ai.APIGoogleVertex,
		Stream:       StreamGoogleVertex,
		StreamSimple: StreamSimpleGoogleVertex,
	})
}
