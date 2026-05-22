// Permission Gate Extension
//
// Prompts for user confirmation before running potentially dangerous bash commands.
// Patterns checked: rm -rf, sudo, chmod/chown 777.
//
// Build:  go build -o permission-gate ./examples/extensions/permission-gate/
// Deploy: cp permission-gate ~/.rho/extensions/permission-gate/
//         cp examples/extensions/permission-gate/rho.toml ~/.rho/extensions/permission-gate/
package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/earendil-works/rho/pkg/sdk"
)

var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\brm\s+(-rf?|--recursive)`),
	regexp.MustCompile(`\bsudo\b`),
	regexp.MustCompile(`\b(chmod|chown)\b.*777`),
	regexp.MustCompile(`\bdd\s+if=`),
	regexp.MustCompile(`\b:\(\)\s*\{`),             // fork bomb
	regexp.MustCompile(`\bmv\s+/`),                  // moving root or system dirs
	regexp.MustCompile(`\b>\/dev\/sda`),              // writing directly to block devices
	regexp.MustCompile(`\bformat\s+[a-z]:\\`),       // Windows format
	regexp.MustCompile(`\bmkfs\.`),                  // creating filesystems
}

func main() {
	ext := sdk.New("rho.permission-gate")

	// The permission gate works by providing a tool that checks commands.
	// It also registers a command that can be used to confirm dangerous commands.
	ext.Command("allow-dangerous", func(ctx sdk.Context, args []string) error {
		if len(args) == 0 {
			ctx.Notify("Usage: /allow-dangerous <command>", "info")
			return nil
		}
		cmd := strings.Join(args, " ")
		if isDangerous(cmd) {
			confirmed, err := ctx.Confirm("⚠️ Dangerous Command", fmt.Sprintf("Allow this command?\n\n  %s", cmd))
			if err != nil {
				ctx.Notify(fmt.Sprintf("Error: %v", err), "error")
				return nil
			}
			if confirmed {
				ctx.Notify(fmt.Sprintf("Allowed: %s", cmd), "success")
			} else {
				ctx.Notify("Command blocked by user", "warning")
			}
		} else {
			ctx.Notify("Command does not appear dangerous", "info")
		}
		return nil
	})

	// Register a tool that scans bash commands for dangerous patterns
	ext.Tool("check_dangerous", "Check if a bash command matches any known dangerous patterns (rm -rf, sudo, chmod 777, etc.). If dangerous, prompts the user for confirmation.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The bash command to check",
				},
			},
			"required": []interface{}{"command"},
		},
		func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			command, _ := args["command"].(string)
			if command == "" {
				return "", true, fmt.Errorf("command is required")
			}

			if !isDangerous(command) {
				return "Command appears safe. No dangerous patterns detected.", false, nil
			}

			// Prompt user for confirmation
			if !ctx.HasUI() {
				return "Dangerous command blocked: no UI for confirmation", true, nil
			}

			confirmed, err := ctx.Confirm(
				"⚠️ Dangerous Command Detected",
				fmt.Sprintf("The following command matches dangerous patterns:\n\n  %s\n\nAllow it?", command),
			)
			if err != nil {
				return "", true, fmt.Errorf("confirmation error: %w", err)
			}

			if confirmed {
				return "Command approved by user. Proceed with execution.", false, nil
			}
			return "Command blocked by user. Execution cancelled.", true, nil
		},
	)

	ext.Run()
}

func isDangerous(cmd string) bool {
	cmdLower := strings.ToLower(cmd)
	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(cmdLower) {
			return true
		}
	}
	return false
}
