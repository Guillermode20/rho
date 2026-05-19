// Package env provides environment helpers compatible with Node.js conventions.
package env

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// GetEnv retrieves an environment variable, with fallback.
func GetEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

// GetEnvBool checks if an environment variable is truthy.
func GetEnvBool(key string, defaultVal bool) bool {
	switch strings.ToLower(os.Getenv(key)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return defaultVal
}

// HomeDir returns the user's home directory.
func HomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp"
	}
	return home
}

// ConfigDir returns the platform-specific config directory.
func ConfigDir() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(HomeDir(), "Library", "Application Support")
	case "windows":
		if d := os.Getenv("APPDATA"); d != "" {
			return d
		}
		return filepath.Join(HomeDir(), "AppData", "Roaming")
	default:
		if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
			return d
		}
		return filepath.Join(HomeDir(), ".config")
	}
}

// DataDir returns the platform-specific data directory.
func DataDir() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(HomeDir(), "Library", "Application Support")
	case "windows":
		if d := os.Getenv("APPDATA"); d != "" {
			return d
		}
		return filepath.Join(HomeDir(), "AppData", "Roaming")
	default:
		if d := os.Getenv("XDG_DATA_HOME"); d != "" {
			return d
		}
		return filepath.Join(HomeDir(), ".local", "share")
	}
}

// CacheDir returns the platform-specific cache directory.
func CacheDir() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(HomeDir(), "Library", "Caches")
	case "windows":
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return d
		}
		return filepath.Join(HomeDir(), "AppData", "Local")
	default:
		if d := os.Getenv("XDG_CACHE_HOME"); d != "" {
			return d
		}
		return filepath.Join(HomeDir(), ".cache")
	}
}

// ExpandPath expands ~ to the user's home directory.
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(HomeDir(), path[2:])
	}
	if path == "~" {
		return HomeDir()
	}
	return path
}

// IsTerminal checks if stdin is a terminal.
func IsTerminal() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// Platform returns the OS name in Node.js format.
func Platform() string {
	switch runtime.GOOS {
	case "darwin":
		return "darwin"
	case "windows":
		return "win32"
	default:
		return "linux"
	}
}

// Arch returns the architecture in Node.js format.
func Arch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "arm64":
		return "arm64"
	default:
		return runtime.GOARCH
	}
}

// PID returns the process ID.
func PID() int {
	return os.Getpid()
}

// CWD returns the current working directory.
func CWD() string {
	wd, err := os.Getwd()
	if err != nil {
		return "/"
	}
	return wd
}

// Environ returns environment variables as a map.
func Environ() map[string]string {
	result := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}
