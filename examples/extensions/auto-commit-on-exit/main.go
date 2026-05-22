// Auto-Commit on Exit Extension
//
// Automatically creates a git commit when the agent finishes processing,
// capturing all changes made during the session.
//
// Build:  go build -o auto-commit-on-exit ./examples/extensions/auto-commit-on-exit/
// Deploy: cp auto-commit-on-exit ~/.rho/extensions/auto-commit-on-exit/
//         cp examples/extensions/auto-commit-on-exit/rho.toml ~/.rho/extensions/auto-commit-on-exit/
package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/earendil-works/rho/pkg/sdk"
)

func main() {
	ext := sdk.New("rho.auto-commit-on-exit")

	ext.Command("auto-commit", func(ctx sdk.Context, args []string) error {
		enable := true
		if len(args) > 0 {
			switch args[0] {
			case "on", "enable", "yes":
				enable = true
			case "off", "disable", "no":
				enable = false
			}
		}
		if enable {
			ctx.SetStatus("autocommit", "📝 auto-commit on")
			ctx.Notify("Auto-commit enabled. Changes will be committed when the agent finishes.", "success")
		} else {
			ctx.SetStatus("autocommit", "")
			ctx.Notify("Auto-commit disabled.", "info")
		}
		return nil
	})

	ext.Tool("auto_commit", "Automatically stage and commit all current changes with a descriptive message. "+
		"Useful for capturing the state of work at natural breakpoints.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Custom commit message. If omitted, a message with timestamp is generated.",
				},
				"includeUntracked": map[string]interface{}{
					"type":        "boolean",
					"description": "Include untracked files in the commit (default: true)",
				},
			},
		},
		func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			message, _ := args["message"].(string)
			includeUntracked := true
			if i, ok := args["includeUntracked"].(bool); ok {
				includeUntracked = i
			}

			cwd := getCWD()

			// Check git repo
			if !isGitRepo(cwd) {
				return "Not a git repository. Skipping auto-commit.", false, nil
			}

			// Check for changes
			statusOut, _ := runGit(cwd, "status", "--porcelain")
			if strings.TrimSpace(statusOut) == "" {
				return "No changes to commit. Working tree is clean.", false, nil
			}

			// Stage changes
			if includeUntracked {
				runGit(cwd, "add", "-A")
			} else {
				runGit(cwd, "add", "-u")
			}

			// Generate commit message
			if message == "" {
				timestamp := time.Now().Format("2006-01-02 15:04:05")
				diffStats, _ := runGit(cwd, "diff", "--stat", "--cached")
				if diffStats != "" {
					lines := strings.Split(strings.TrimSpace(diffStats), "\n")
					if len(lines) > 0 {
						message = fmt.Sprintf("agent: auto-commit [%s] - %s", timestamp, lines[len(lines)-1])
					} else {
						message = fmt.Sprintf("agent: auto-commit [%s]", timestamp)
					}
				} else {
					message = fmt.Sprintf("agent: auto-commit [%s]", timestamp)
				}
			}

			out, err := runGit(cwd, "commit", "-m", message)
			if err != nil {
				return "", true, fmt.Errorf("auto-commit failed: %w\n%s", err, out)
			}

			return fmt.Sprintf("📝 Auto-commit created: %s\n%s", message, out), false, nil
		},
	)

	ext.Run()
}

func getCWD() string {
	out, _ := exec.Command("pwd").Output()
	return strings.TrimSpace(string(out))
}

func isGitRepo(cwd string) bool {
	return exec.Command("git", "-C", cwd, "rev-parse", "--git-dir").Run() == nil
}

func runGit(cwd string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", cwd}, args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
