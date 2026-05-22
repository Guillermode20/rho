package codecore

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ExecOptions configures command execution.
type ExecOptions struct {
	// Working directory for the command.
	CWD string `json:"cwd,omitempty"`
	// Timeout in seconds. 0 means no timeout.
	Timeout int `json:"timeout,omitempty"`
	// Environment variables to set (key=value pairs).
	Env []string `json:"env,omitempty"`
	// Additional PATH entries.
	ExtraPaths []string `json:"extraPaths,omitempty"`
	// Shell to use. Empty uses the default shell.
	Shell string `json:"shell,omitempty"`
	// Whether to merge stderr into stdout.
	MergeStderr bool `json:"mergeStderr,omitempty"`
	// Signal to abort execution.
	Signal context.Context `json:"-"`
}

// ExecResult contains the result of command execution.
type ExecResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
	Duration int64  `json:"duration"` // milliseconds
	Error    string `json:"error,omitempty"`
	Success  bool   `json:"success"`
}

// ExecCommand executes a command and returns the result.
func ExecCommand(command string, opts *ExecOptions) *ExecResult {
	result := &ExecResult{}
	start := time.Now()

	if opts == nil {
		opts = &ExecOptions{}
	}

	ctx := opts.Signal
	var cancel context.CancelFunc
	if opts.Timeout > 0 && ctx == nil {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(opts.Timeout)*time.Second)
		defer cancel()
	} else if ctx == nil {
		ctx = context.Background()
	}

	shell := opts.Shell
	if shell == "" {
		shell = GetDefaultShell()
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, shell, "/c", command)
	} else {
		cmd = exec.CommandContext(ctx, shell, "-c", command)
	}

	if opts.CWD != "" {
		cmd.Dir = opts.CWD
	}

	// Build environment
	env := os.Environ()
	if len(opts.ExtraPaths) > 0 {
		// handled via ShellConfig
	}
	if len(opts.Env) > 0 {
		env = append(env, opts.Env...)
	}
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.Duration = time.Since(start).Milliseconds()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Sprintf("command timed out after %d seconds", opts.Timeout)
			result.ExitCode = -1
		} else {
			result.Error = err.Error()
			result.ExitCode = -1
		}
	}

	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	result.Success = err == nil

	return result
}

// ShellConfig holds shell configuration.
type ShellConfig struct {
	Shell     string   `json:"shell"`
	ShellArgs []string `json:"shellArgs"`
	Env       []string `json:"env,omitempty"`
}

// GetDefaultShell returns the user's default shell.
func GetDefaultShell() string {
	// Check environment
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	// On Unix systems, the user config may specify a shell
	// Platform defaults
	switch runtime.GOOS {
	case "windows":
		return "cmd.exe"
	default:
		return "/bin/bash"
	}
}

// GetShellConfig returns the shell configuration for the current environment.
func GetShellConfig() *ShellConfig {
	shell := GetDefaultShell()
	shellName := filepath.Base(shell)

	var args []string
	switch shellName {
	case "bash":
		args = []string{"--norc", "--noprofile"}
	case "zsh":
		args = []string{"--no-rcs"}
	case "fish":
		args = []string{"--no-config"}
	}

	// Build shell env
	env := os.Environ()
	home, _ := os.UserHomeDir()
	env = append(env, fmt.Sprintf("HOME=%s", home))

	return &ShellConfig{
		Shell:     shell,
		ShellArgs: args,
		Env:       env,
	}
}

// GetShellEnv returns environment variables for shell execution.
func GetShellEnv(extraPaths ...string) []string {
	env := os.Environ()

	if len(extraPaths) > 0 {
		currentPath := os.Getenv("PATH")
		allPaths := append(extraPaths, currentPath)
		env = append(env, fmt.Sprintf("PATH=%s", strings.Join(allPaths, string(filepath.ListSeparator))))
	}

	return env
}
