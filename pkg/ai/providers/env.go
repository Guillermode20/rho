package providers

import (
	"os"
	"strings"
)

// GetEnvAPIKey resolves an API key from multiple environment variables.
func GetEnvAPIKey(vars ...string) string {
	for _, v := range vars {
		if key := os.Getenv(v); key != "" {
			return key
		}
	}
	return ""
}

// BaseURLFromEnv returns a base URL from an environment variable, or a default.
func BaseURLFromEnv(envVar, defaultURL string) string {
	if u := os.Getenv(envVar); u != "" {
		return strings.TrimRight(u, "/")
	}
	return defaultURL
}

// EnvBool returns true if the env var is truthy, otherwise the default.
func EnvBool(envVar string, defaultVal bool) bool {
	switch strings.ToLower(os.Getenv(envVar)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return defaultVal
}
