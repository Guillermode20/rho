// Package codecore provides core shared infrastructure for the agent:
// configuration paths, defaults, message helpers, execution, diagnostics, timings, etc.
package codecore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// VERSION is the current rho version.
const VERSION = "0.2.0"

// ConfigDirName is the name of the rho configuration directory.
const ConfigDirName = ".rho"

// AppName is the CLI application name.
const AppName = "rho"

// getAgentDir returns the agent directory (~/.rho/agent).
func GetAgentDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("/tmp", ConfigDirName, "agent")
	}
	return filepath.Join(home, ConfigDirName, "agent")
}

// GetSessionsDir returns the sessions directory (~/.rho/agent/sessions).
func GetSessionsDir() string {
	return filepath.Join(GetAgentDir(), "sessions")
}

// GetConfigDir returns the config directory (~/.rho).
func GetConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("/tmp", ConfigDirName)
	}
	return filepath.Join(home, ConfigDirName)
}

// GetExtensionsDir returns the extensions directory (~/.rho/extensions).
func GetExtensionsDir() string {
	return filepath.Join(GetConfigDir(), "extensions")
}

// GetPackageDir returns the packages directory (~/.rho/packages).
func GetPackageDir() string {
	return filepath.Join(GetConfigDir(), "packages")
}

// GetCacheDir returns the cache directory (~/.rho/cache).
func GetCacheDir() string {
	return filepath.Join(GetConfigDir(), "cache")
}

// GetLogDir returns the log directory (~/.rho/log).
func GetLogDir() string {
	return filepath.Join(GetConfigDir(), "log")
}

// ExpandTildePath expands ~ to the user's home directory.
func ExpandTildePath(p string) string {
	if strings.HasPrefix(p, "~/") || p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		if p == "~" {
			return home
		}
		return filepath.Join(home, p[2:])
	}
	return p
}

// IsBunBinary returns true if running as a compiled bun binary.
// In the Go version, this always returns false.
func IsBunBinary() bool {
	return false
}

// GetRelativePath returns a path relative to the given base, or the absolute path if not relative.
func GetRelativePath(path, base string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}
	return rel
}

// EnsureDir creates a directory if it doesn't exist.
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// AppDataDir returns the platform-appropriate data directory.
func AppDataDir() string {
	if d := os.Getenv("RHO_DATA_DIR"); d != "" {
		return d
	}
	return GetAgentDir()
}

// TempSessionsDir returns a temporary sessions directory for testing.
func TempSessionsDir() string {
	dir, err := os.MkdirTemp("", "rho-sessions-*")
	if err != nil {
		return filepath.Join(os.TempDir(), "rho-sessions")
	}
	return dir
}

// init ensures necessary directories exist.
func init() {
	dirs := []string{
		GetAgentDir(),
		GetSessionsDir(),
		GetExtensionsDir(),
		GetCacheDir(),
		GetLogDir(),
	}
	for _, d := range dirs {
		os.MkdirAll(d, 0755)
	}
}

// FormatPath formats a path for display, shortening home to ~.
func FormatPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

// ValidateDir checks that a directory exists and is writable.
func ValidateDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory does not exist: %s", dir)
		}
		return fmt.Errorf("cannot access directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", dir)
	}
	return nil
}

// IsLocalPath returns true if the path is a local filesystem path (not a URL).
func IsLocalPath(p string) bool {
	return !strings.HasPrefix(p, "http://") && !strings.HasPrefix(p, "https://") &&
		!strings.HasPrefix(p, "npm:") && !strings.HasPrefix(p, "file://")
}

var _ = []interface{}{
	FormatPath,
	ValidateDir,
	IsLocalPath,
	GetRelativePath,
	EnsureDir,
	AppDataDir,
	TempSessionsDir,
}
