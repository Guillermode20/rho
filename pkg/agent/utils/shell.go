package agentutils

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ShellType represents the type of shell.
type ShellType string

const (
	ShellBash ShellType = "bash"
	ShellZsh  ShellType = "zsh"
	ShellFish ShellType = "fish"
	ShellPWSH ShellType = "pwsh"
	ShellCMD  ShellType = "cmd"
	ShellSh   ShellType = "sh"
	ShellUnknown ShellType = ""
)

// ShellConfig holds shell configuration.
type ShellConfig struct {
	Shell    string   `json:"shell"`
	Args     []string `json:"args,omitempty"`
	ShellType ShellType `json:"shellType"`
	Env      map[string]string `json:"env,omitempty"`
}

// GetShellConfig returns the shell configuration for the current platform.
func GetShellConfig() *ShellConfig {
	cfg := &ShellConfig{
		Env: make(map[string]string),
	}

	switch runtime.GOOS {
	case "windows":
		cfg.Shell = "powershell.exe"
		cfg.Args = []string{"-NoProfile", "-NonInteractive", "-Command"}
		cfg.ShellType = ShellPWSH
	default:
		// Try SHELL env, then fall back to bash
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "bash"
		}
		cfg.Shell = shell

		base := filepath.Base(shell)
		switch base {
		case "zsh":
			cfg.ShellType = ShellZsh
			cfg.Args = []string{"-i"}
		case "fish":
			cfg.ShellType = ShellFish
			cfg.Args = []string{"-i"}
		default:
			cfg.ShellType = ShellBash
			cfg.Args = []string{"-i"}
		}
	}

	// Copy current environment
	for _, env := range os.Environ() {
		if parts := strings.SplitN(env, "=", 2); len(parts) == 2 {
			cfg.Env[parts[0]] = parts[1]
		}
	}

	return cfg
}

// GetDefaultShell returns the default shell path.
func GetDefaultShell() string {
	cfg := GetShellConfig()
	return cfg.Shell
}

// GetShellEnv returns environment variables optimized for the shell.
func GetShellEnv(shellType ShellType) map[string]string {
	env := make(map[string]string)

	switch shellType {
	case ShellBash:
		env["BASH_ENV"] = ""
		env["BASH_SILENCE_DEPRECATION_WARNING"] = "1"
	case ShellZsh:
		env["ZSH_NO_TERM"] = "1"
	case ShellFish:
		env["FISH_ENCODING"] = "UTF-8"
	}

	// Common settings for non-interactive agent usage
	env["TERM"] = os.Getenv("TERM")
	if env["TERM"] == "" {
		env["TERM"] = "xterm-256color"
	}
	env["PAGER"] = "cat"
	env["EDITOR"] = "cat"
	env["GIT_PAGER"] = "cat"
	env["PYTHONUNBUFFERED"] = "1"

	return env
}

// DetectShellType detects the shell type from a shell path.
func DetectShellType(shellPath string) ShellType {
	base := strings.ToLower(filepath.Base(shellPath))
	switch base {
	case "bash":
		return ShellBash
	case "zsh":
		return ShellZsh
	case "fish":
		return ShellFish
	case "pwsh", "powershell", "pwsh.exe", "powershell.exe":
		return ShellPWSH
	case "cmd", "cmd.exe":
		return ShellCMD
	case "sh":
		return ShellSh
	}
	return ShellUnknown
}

// IsShellAvailable checks if a specific shell is installed.
func IsShellAvailable(shellPath string) bool {
	_, err := exec.LookPath(shellPath)
	return err == nil
}

// GetInteractiveShellArgs returns shell arguments for interactive use.
func GetInteractiveShellArgs(shellType ShellType) []string {
	switch shellType {
	case ShellBash:
		return []string{"--norc", "--noprofile"}
	case ShellZsh:
		return []string{"-f"}
	case ShellFish:
		return []string{"-C", "set -g fish_complete_path"}
	default:
		return nil
	}
}
