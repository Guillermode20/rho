// Git Checkpoint Extension
//
// Automatically creates a git commit with a summary of changes before the agent
// makes modifications. Helps track what the agent changes and provides rollback points.
//
// Build:  go build -o git-checkpoint ./examples/extensions/git-checkpoint/
// Deploy: cp git-checkpoint ~/.rho/extensions/git-checkpoint/
//         cp examples/extensions/git-checkpoint/rho.toml ~/.rho/extensions/git-checkpoint/
package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/earendil-works/rho/pkg/sdk"
)

func main() {
	ext := sdk.New("rho.git-checkpoint")

	ext.Tool("git_checkpoint", "Create a git checkpoint commit with a summary of current changes. "+
		"Useful before making risky changes to have a rollback point. "+
		"Only works in git repositories with staged/unstaged changes.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Optional commit message. If omitted, a descriptive message is generated.",
				},
				"autoStage": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to automatically stage all changes before committing (default: true)",
				},
			},
		},
		func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			message, _ := args["message"].(string)
			autoStage := true
			if a, ok := args["autoStage"].(bool); ok {
				autoStage = a
			}

			cwd, err := getCWD()
			if err != nil {
				return "", true, fmt.Errorf("cannot determine working directory: %w", err)
			}

			// Check if we're in a git repo
			if !isGitRepo(cwd) {
				return "Not a git repository. Skipping checkpoint.", false, nil
			}

			// Check for changes
			statusOut, err := runGit(cwd, "status", "--porcelain")
			if err != nil {
				return "", true, fmt.Errorf("git status failed: %w", err)
			}
			if strings.TrimSpace(statusOut) == "" {
				return "No changes to checkpoint. Working tree is clean.", false, nil
			}

			// Stage all changes if requested
			if autoStage {
				if _, err := runGit(cwd, "add", "-A"); err != nil {
					return "", true, fmt.Errorf("git add failed: %w", err)
				}
			}

			// Generate commit message
			if message == "" {
				diffStats, err := runGit(cwd, "diff", "--stat", "--cached")
				if err != nil {
					diffStats = ""
				}
				msg := "checkpoint: agent changes"
				if diffStats != "" {
					lines := strings.Split(strings.TrimSpace(diffStats), "\n")
					if len(lines) > 0 {
						msg = "checkpoint: " + lines[len(lines)-1]
					}
				}
				message = msg
			}

			// Create the commit
			out, err := runGit(cwd, "commit", "-m", message)
			if err != nil {
				return "", true, fmt.Errorf("git commit failed: %w\nOutput: %s", err, out)
			}

			return fmt.Sprintf("✅ Checkpoint created: %s\n\n%s", message, out), false, nil
		},
	)

	ext.Run()
}

func getCWD() (string, error) {
	out, err := exec.Command("pwd").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func isGitRepo(cwd string) bool {
	err := exec.Command("git", "-C", cwd, "rev-parse", "--git-dir").Run()
	return err == nil
}

func runGit(cwd string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", cwd}, args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
