// Dirty Repo Guard Extension
//
// Prevents the agent from making changes when the git repository has
// uncommitted changes, helping avoid mixing agent changes with user changes.
//
// Build:  go build -o dirty-repo-guard ./examples/extensions/dirty-repo-guard/
// Deploy: cp dirty-repo-guard ~/.rho/extensions/dirty-repo-guard/
//
//	cp examples/extensions/dirty-repo-guard/rho.toml ~/.rho/extensions/dirty-repo-guard/
package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/earendil-works/rho/pkg/sdk"
)

func main() {
	ext := sdk.New("rho.dirty-repo-guard")

	ext.Command("repo-status", func(ctx sdk.Context, args []string) error {
		cwd := getCWD()
		if !isGitRepo(cwd) {
			ctx.Notify("Not a git repository", "info")
			return nil
		}

		statusOut, _ := runGit(cwd, "status", "--porcelain")
		lines := strings.Split(strings.TrimSpace(statusOut), "\n")

		if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
			ctx.Notify("✅ Working tree is clean", "success")
			return nil
		}

		var msg strings.Builder
		msg.WriteString(fmt.Sprintf("📝 %d uncommitted changes:\n", len(lines)))
		for _, line := range lines {
			if line != "" {
				msg.WriteString(fmt.Sprintf("  %s\n", line))
			}
		}
		ctx.Notify(msg.String(), "warning")
		return nil
	})

	ext.Tool("check_repo_clean", "Check if the git repository has uncommitted changes. "+
		"Returns a warning if the repo is dirty, helping avoid mixing agent changes with user changes.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Optional context message about what the agent is about to do",
				},
			},
		},
		func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			message, _ := args["message"].(string)

			cwd := getCWD()
			if !isGitRepo(cwd) {
				return "Not a git repository, no guard needed.", false, nil
			}

			statusOut, _ := runGit(cwd, "status", "--porcelain")
			if strings.TrimSpace(statusOut) == "" {
				return "✅ Repository is clean. Safe to proceed.", false, nil
			}

			lines := strings.Split(strings.TrimSpace(statusOut), "\n")
			warning := fmt.Sprintf("⚠️ WARNING: Repository has %d uncommitted change(s):\n", countNonEmpty(lines))
			for _, line := range lines {
				if line != "" {
					warning += fmt.Sprintf("  %s\n", line)
				}
			}
			if message != "" {
				warning += fmt.Sprintf("\nContext: %s\n", message)
			}
			warning += "\nRecommendation: Commit or stash changes before proceeding, or use git_checkpoint to create a checkpoint."

			return warning, false, nil
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

func countNonEmpty(lines []string) int {
	count := 0
	for _, l := range lines {
		if l != "" {
			count++
		}
	}
	return count
}
