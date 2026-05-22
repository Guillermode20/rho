package providers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// CacheKeyConfig configures how cache keys are generated for prompt caching.
type CacheKeyConfig struct {
	// Whether to enable prompt caching
	Enabled bool
	// The base cache key prefix (e.g., model name)
	BaseKey string
	// Session ID for session-level caching
	SessionID string
	// Whether to use long retention (24h vs ephemeral)
	LongRetention bool
}

// PromptCacheEntry represents a cached prompt entry.
type PromptCacheEntry struct {
	Key      string `json:"key"`
	Content  string `json:"content"`
	Source   string `json:"source"`   // "system", "tools", "messages"
	Position int    `json:"position"` // Order in the prompt
}

// GenerateCacheKey creates a deterministic cache key for a prompt segment.
func GenerateCacheKey(content string, config CacheKeyConfig) string {
	hash := sha256.Sum256([]byte(content))
	keyParts := []string{
		"prompt",
		config.BaseKey,
	}
	if config.SessionID != "" {
		keyParts = append(keyParts, config.SessionID)
	}
	keyParts = append(keyParts, hex.EncodeToString(hash[:8]))
	return strings.Join(keyParts, ":")
}

// CacheControlHeader generates the appropriate cache control headers
// for OpenAI-compatible APIs based on cache retention preference.
func CacheControlHeader(cacheRetention string) map[string]string {
	headers := make(map[string]string)
	switch cacheRetention {
	case "long":
		// Long retention: 24h cache
		headers["prompt-cache-retention"] = "24h"
	case "short":
		// Short retention: ephemeral (default)
		headers["prompt-cache-retention"] = "ephemeral"
	case "none":
		// No caching
		headers["prompt-cache-retention"] = "none"
	}
	return headers
}

// CacheControlForModel returns model-specific cache control settings.
func CacheControlForModel(modelName, retention string) (string, string) {
	// Different models support different cache retention durations
	longRetentionModels := []string{"gpt-4o", "gpt-4o-mini"}

	switch retention {
	case "long":
		for _, m := range longRetentionModels {
			if strings.Contains(modelName, m) {
				return "24h", "ephemeral"
			}
		}
		return "ephemeral", "ephemeral"
	case "short":
		return "ephemeral", "ephemeral"
	default:
		return "", ""
	}
}

// ClampOpenAIPromptCacheKey truncates or adjusts a cache key to fit
// within provider limits. Some providers have maximum key lengths.
func ClampOpenAIPromptCacheKey(key string, maxLen int) string {
	if maxLen <= 0 || len(key) <= maxLen {
		return key
	}
	// Truncate but keep enough for uniqueness
	if maxLen < 8 {
		return key[:maxLen]
	}
	hash := sha256.Sum256([]byte(key))
	hashStr := hex.EncodeToString(hash[:4]) // 8 chars of hash
	return key[:maxLen-9] + ":" + hashStr
}

// SessionAffinityHeader returns the session affinity header for
// OpenAI-compatible APIs that support it.
func SessionAffinityHeader(sessionID string) map[string]string {
	if sessionID == "" {
		return nil
	}
	return map[string]string{
		"x-session-affinity": sessionID,
	}
}

// SendSessionAffinityHeaders checks whether session affinity headers should
// be sent based on provider type and capabilities.
func SendSessionAffinityHeaders(providerName string) bool {
	// Providers known to support session affinity for prompt caching
	supported := map[string]bool{
		"openai":    true,
		"azure":     true,
		"fireworks": true,
		"together":  false,
		"deepseek":  false,
	}
	// Default to false for unknown providers
	return supported[strings.ToLower(providerName)]
}

func init() {
	_ = fmt.Sprintf
	_ = ClampOpenAIPromptCacheKey
}
