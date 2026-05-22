package providers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// GitHubCopilotToken represents the token exchange response from GitHub Copilot.
type GitHubCopilotToken struct {
	Token        string `json:"token"`
	ExpiresAt    int64  `json:"expires_at"`
	MinToken     string `json:"min_token"`
	MinExpiresAt int64  `json:"min_expires_at"`
}

const (
	githubCopilotTokenURL        = "https://api.github.com/copilot_internal/v2/token"
	githubCopilotChatCompletions = "https://api.githubcopilot.com/chat/completions"
)

// CopilotAuthResult holds the result of GitHub Copilot authentication.
type CopilotAuthResult struct {
	Token     string
	ExpiresAt time.Time
	Headers   map[string]string
}

// GitHubCopilotAuthenticate performs the GitHub Copilot token exchange.
// It reads the GitHub token from the environment, then exchanges it for
// a Copilot-specific token via the GitHub API.
func GitHubCopilotAuthenticate() (*CopilotAuthResult, error) {
	// First, get the GitHub token from the environment
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		githubToken = os.Getenv("GH_TOKEN")
	}
	if githubToken == "" {
		// Check for the gh CLI's cached token
		token, err := readGhAuthToken()
		if err != nil || token == "" {
			return nil, fmt.Errorf("GITHUB_TOKEN not set and no gh auth token found")
		}
		githubToken = token
	}

	// Exchange the GitHub token for a Copilot token
	result, err := exchangeCopilotToken(githubToken)
	if err != nil {
		return nil, fmt.Errorf("Copilot token exchange failed: %w", err)
	}

	return result, nil
}

// readGhAuthToken reads the GitHub CLI's cached authentication token.
func readGhAuthToken() (string, error) {
	// Check common locations for gh CLI token storage
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// gh stores tokens in ~/.config/gh/hosts.yml or similar
	// Try common paths
	candidates := []string{
		home + "/.config/gh/hosts.yml",
	}

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		// Simple YAML token extraction (just looks for oauth_token:)
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "oauth_token:") {
				token := strings.TrimSpace(strings.TrimPrefix(line, "oauth_token:"))
				if token != "" {
					return token, nil
				}
			}
		}
	}
	return "", fmt.Errorf("no gh auth token found")
}

// exchangeCopilotToken exchanges a GitHub token for a Copilot API token.
func exchangeCopilotToken(githubToken string) (*CopilotAuthResult, error) {
	req, err := http.NewRequest("GET", githubCopilotTokenURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+githubToken)
	req.Header.Set("User-Agent", "GitHubCopilot/1.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Copilot token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp GitHubCopilotToken
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode Copilot token response: %w", err)
	}

	if tokenResp.Token == "" {
		return nil, fmt.Errorf("empty Copilot token in response")
	}

	// Build dynamic headers required by GitHub Copilot API
	headers := BuildCopilotDynamicHeaders(tokenResp.Token, "")

	return &CopilotAuthResult{
		Token:     tokenResp.Token,
		ExpiresAt: time.Unix(tokenResp.ExpiresAt, 0),
		Headers:   headers,
	}, nil
}

// BuildCopilotDynamicHeaders builds the required HTTP headers for GitHub Copilot API calls.
// The editor-version and copilot-integration-id headers are required by GitHub's API.
func BuildCopilotDynamicHeaders(token string, baseURL string) map[string]string {
	headers := map[string]string{
		"Authorization":         "Bearer " + token,
		"Content-Type":          "application/json",
		"Editor-Version":        "vscode/1.85.0",
		"Editor-Plugin-Version": "copilot-chat/0.17.0",
		"User-Agent":            "GitHubCopilot/1.0",
		"Accept":                "*/*",
	}
	if baseURL != "" {
		headers["Copilot-Integration-Id"] = "vscode-chat"
	}
	return headers
}

// HasCopilotVisionInput checks if the conversation contains images,
// which requires special handling for Copilot's vision API.
func HasCopilotVisionInput(messages interface{}) bool {
	// GitHub Copilot's vision support is provider-specific.
	// This is a stub for the vision detection logic.
	return false
}

// GitHubCopilotBaseURL returns the appropriate base URL for Copilot API calls.
func GitHubCopilotBaseURL() string {
	if u := os.Getenv("GITHUB_COPILOT_BASE_URL"); u != "" {
		return u
	}
	return githubCopilotChatCompletions
}

func init() {
	// GitHub Copilot is typically auto-detected and doesn't need explicit registration.
	// It shares the OpenAI completions API format.
}
