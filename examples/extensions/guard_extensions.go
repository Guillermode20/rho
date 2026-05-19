
package exampleext

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/earendil-works/rho/pkg/agent/extensions"
)

// RegisterPermissionGate registers an extension that gates tools by name.
func RegisterPermissionGate(runtime *extensions.Runtime) {
	allowedTools := map[string]bool{
		"Read": true, "Write": true, "Edit": true,
		"Grep": true, "Find": true, "Glob": true, "Ls": true,
	}

	runtime.Register(&extensions.ExtensionDef{
		Name:        "permission-gate",
		Description: "Gate which tools the agent is allowed to use",
		Version:     "1.0.0",
		OnToolCall: func(ctx extensions.ExtensionContext, event extensions.ToolCallEvent) (*extensions.ToolCallEventResult, error) {
			if !allowedTools[event.ToolName] {
				return &extensions.ToolCallEventResult{
					Block:  true,
					Reason: fmt.Sprintf("Tool %q is not in the allowed list. Allowed: Read, Write, Edit, Grep, Find, Glob, Ls", event.ToolName),
				}, nil
			}
			return nil, nil
		},
		SlashCommands: []extensions.SlashCommand{
			{
				Name:        "allow-tool",
				Description: "Add a tool to the allowed list",
				Handler: func(ctx extensions.ExtensionContext, args []string) error {
					if len(args) == 0 {
						return fmt.Errorf("usage: /allow-tool <toolname>")
					}
					allowedTools[args[0]] = true
					ctx.UI.Notify(fmt.Sprintf("Tool %q added to allowed list", args[0]), "info")
					return nil
				},
			},
		},
	})
}

// RegisterDirtyRepoGuard registers an extension that blocks the agent on dirty git repos.
func RegisterDirtyRepoGuard(runtime *extensions.Runtime) {
	runtime.Register(&extensions.ExtensionDef{
		Name:        "dirty-repo-guard",
		Description: "Block the agent from running when the git repo has uncommitted changes",
		Version:     "1.0.0",
		OnBeforeAgentStart: func(ctx extensions.ExtensionContext, event extensions.BeforeAgentStartEvent) error {
			dirty, err := isGitDirty(ctx.CWD)
			if err != nil {
				return nil // Not a git repo or git not available
			}
			if dirty {
				confirmed, err := ctx.UI.Confirm("Dirty Repository",
					"You have uncommitted changes. The agent may modify files. Continue?")
				if err != nil {
					return err
				}
				if !confirmed {
					return fmt.Errorf("agent blocked: dirty repository")
				}
			}
			return nil
		},
	})
}

func isGitDirty(dir string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// RegisterGitCheckpoint registers an extension that auto-commits before agent actions.
func RegisterGitCheckpoint(runtime *extensions.Runtime) {
	runtime.Register(&extensions.ExtensionDef{
		Name:        "git-checkpoint",
		Description: "Automatically create a git checkpoint before the agent runs",
		Version:     "1.0.0",
		OnBeforeAgentStart: func(ctx extensions.ExtensionContext, event extensions.BeforeAgentStartEvent) error {
			if !isGitRepo(ctx.CWD) {
				return nil
			}
			cmd := exec.Command("git", "add", "-A")
			cmd.Dir = ctx.CWD
			cmd.Run()

			cmd2 := exec.Command("git", "commit", "--allow-empty", "-m", "rho checkpoint before agent run")
			cmd2.Dir = ctx.CWD
			cmd2.Run()

			ctx.UI.Notify("Git checkpoint created", "info")
			return nil
		},
	})
}

func isGitRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	return cmd.Run() == nil
}

// RegisterSSHExt registers an extension for SSH command execution.
func RegisterSSHExt(runtime *extensions.Runtime) {
	runtime.Register(&extensions.ExtensionDef{
		Name:        "ssh-extension",
		Description: "Execute commands on remote hosts via SSH",
		Version:     "1.0.0",
		CustomTools: []extensions.ToolDefinition{
			{
				Name:        "SSH",
				Label:       "SSH",
				Description: "Execute a command on a remote host via SSH",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"host": map[string]interface{}{
							"type":        "string",
							"description": "Remote host (user@hostname)",
						},
						"command": map[string]interface{}{
							"type":        "string",
							"description": "Command to execute on the remote host",
						},
					},
					"required": []interface{}{"host", "command"},
				},
				Execute: func(args map[string]interface{}) (string, bool, error) {
					host, _ := args["host"].(string)
					command, _ := args["command"].(string)
					if host == "" || command == "" {
						return "", true, fmt.Errorf("host and command are required")
					}
					cmd := exec.Command("ssh", host, command)
					out, err := cmd.CombinedOutput()
					if err != nil {
						return string(out), true, fmt.Errorf("ssh failed: %w", err)
					}
					return string(out), false, nil
				},
			},
		},
	})
}
