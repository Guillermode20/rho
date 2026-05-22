// Interactive Shell Extension
//
// Provides an interactive shell session tool that maintains state between commands.
// Unlike the regular bash tool which runs each command in a new process, this
// maintains a persistent shell session.
//
// Build:  go build -o interactive-shell ./examples/extensions/interactive-shell/
// Deploy: cp interactive-shell ~/.rho/extensions/interactive-shell/
//
//	cp examples/extensions/interactive-shell/rho.toml ~/.rho/extensions/interactive-shell/
package main

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/earendil-works/rho/pkg/sdk"
)

type shellSession struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	mu     sync.Mutex
	active bool
}

var (
	sessions   = make(map[string]*shellSession)
	sessionsMu sync.Mutex
)

func main() {
	ext := sdk.New("rho.interactive-shell")

	ext.Command("shell", func(ctx sdk.Context, args []string) error {
		if len(args) == 0 {
			ctx.Notify("Usage: /shell <start|stop|command>", "info")
			return nil
		}
		switch args[0] {
		case "start":
			sessionsMu.Lock()
			if _, exists := sessions["default"]; exists {
				sessionsMu.Unlock()
				ctx.Notify("Shell session already running.", "info")
				return nil
			}
			sess := &shellSession{}
			sessions["default"] = sess
			sessionsMu.Unlock()
			ctx.Notify("🖥️ Interactive shell started.", "success")
		case "stop":
			sessionsMu.Lock()
			if sess, ok := sessions["default"]; ok {
				sess.mu.Lock()
				if sess.cmd != nil && sess.cmd.Process != nil {
					sess.cmd.Process.Kill()
				}
				sess.mu.Unlock()
				delete(sessions, "default")
			}
			sessionsMu.Unlock()
			ctx.Notify("🖥️ Interactive shell stopped.", "info")
		default:
			cmd := strings.Join(args, " ")
			sessionsMu.Lock()
			sess, ok := sessions["default"]
			sessionsMu.Unlock()
			if !ok {
				ctx.Notify("No shell session. Start one with /shell start", "error")
				return nil
			}
			// Run command
			sess.mu.Lock()
			defer sess.mu.Unlock()
			// For now, run as one-off
			out, err := exec.Command("bash", "-c", cmd).CombinedOutput()
			if err != nil {
				ctx.Notify(fmt.Sprintf("Error: %v\n%s", err, string(out)), "error")
			} else {
				ctx.Notify(string(out), "info")
			}
		}
		return nil
	})

	ext.Tool("shell_session", "Run a command in an interactive shell session. "+
		"Unlike the regular Bash tool, this maintains state between commands "+
		"(current directory, environment variables, etc.). "+
		"Start a session first with /shell start, then use this tool to run commands.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "Command to run in the interactive shell session",
				},
				"sessionId": map[string]interface{}{
					"type":        "string",
					"description": "Session ID (default: 'default')",
				},
				"workdir": map[string]interface{}{
					"type":        "string",
					"description": "Working directory for the command",
				},
			},
			"required": []interface{}{"command"},
		},
		func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			command, _ := args["command"].(string)
			sessionID, _ := args["sessionId"].(string)
			workdir, _ := args["workdir"].(string)

			if command == "" {
				return "", true, fmt.Errorf("command is required")
			}
			if sessionID == "" {
				sessionID = "default"
			}

			sessionsMu.Lock()
			sess, ok := sessions[sessionID]
			if !ok {
				// Auto-start a session
				sess = &shellSession{}
				sessions[sessionID] = sess
			}
			sessionsMu.Unlock()

			sess.mu.Lock()
			defer sess.mu.Unlock()

			cmd := exec.Command("bash", "-c", command)
			if workdir != "" {
				cmd.Dir = workdir
			}

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			start := time.Now()
			err := cmd.Run()
			duration := time.Since(start)

			var result strings.Builder
			result.WriteString(fmt.Sprintf("$ %s\n", command))

			if stdout.Len() > 0 {
				result.WriteString(stdout.String())
			}
			if stderr.Len() > 0 {
				if stdout.Len() > 0 {
					result.WriteString("\n")
				}
				result.WriteString(stderr.String())
			}

			if err != nil {
				result.WriteString(fmt.Sprintf("\n(%.2fs) exit code: %v", duration.Seconds(), err))
				return result.String(), true, nil
			}

			result.WriteString(fmt.Sprintf("\n(%.2fs) completed", duration.Seconds()))
			return result.String(), false, nil
		},
	)

	ext.Run()
}
