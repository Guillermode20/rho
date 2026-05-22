// Protected Paths Extension
//
// Protects specified files and directories from being modified by the agent.
// Useful for safeguarding critical configuration files, credentials, or build outputs.
//
// Build:  go build -o protected-paths ./examples/extensions/protected-paths/
// Deploy: cp protected-paths ~/.rho/extensions/protected-paths/
//         cp examples/extensions/protected-paths/rho.toml ~/.rho/extensions/protected-paths/
package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/earendil-works/rho/pkg/sdk"
)

var defaultProtectedPatterns = []string{
	".env*",
	"*.pem",
	"*.key",
	"id_rsa*",
	"id_ed25519*",
	"*.secret.*",
	"credentials*",
	"*.local.*",
	"node_modules/",
	"vendor/",
	".git/",
	"dist/",
	"build/",
	".next/",
	"target/",
	"*.lock",
	"package-lock.json",
	"yarn.lock",
	"go.sum",
	"rho",
	".rho/",
	".pi/",
}

func main() {
	ext := sdk.New("rho.protected-paths")

	ext.Command("protect-add", func(ctx sdk.Context, args []string) error {
		if len(args) == 0 {
			ctx.Notify("Usage: /protect-add <pattern> [pattern...]", "info")
			return nil
		}
		for _, pattern := range args {
			defaultProtectedPatterns = append(defaultProtectedPatterns, pattern)
		}
		ctx.Notify(fmt.Sprintf("Added %d protected pattern(s): %s", len(args), strings.Join(args, ", ")), "success")
		return nil
	})

	ext.Command("protect-list", func(ctx sdk.Context, args []string) error {
		var msg strings.Builder
		msg.WriteString(fmt.Sprintf("Protected paths (%d patterns):\n", len(defaultProtectedPatterns)))
		for _, p := range defaultProtectedPatterns {
			msg.WriteString(fmt.Sprintf("  - %s\n", p))
		}
		ctx.Notify(msg.String(), "info")
		return nil
	})

	ext.Tool("check_path_protected", "Check if a file path matches any protected patterns. "+
		"Protected paths (e.g., .env files, credentials, vendor directories) should not be modified.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path to check against protected patterns",
				},
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "The operation being checked (read, write, edit, etc.)",
				},
			},
			"required": []interface{}{"path"},
		},
		func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			path, _ := args["path"].(string)
			operation, _ := args["operation"].(string)

			if path == "" {
				return "", true, fmt.Errorf("path is required")
			}

			if operation == "" {
				operation = "modify"
			}

			protected, pattern := isProtected(path)
			if !protected {
				return fmt.Sprintf("Path '%s' is not protected. Safe to %s.", path, operation), false, nil
			}

			if ctx.HasUI() {
				confirmed, err := ctx.Confirm(
					"🛡️ Protected Path",
					fmt.Sprintf("Path '%s' matches protected pattern '%s'.\n\nOperation: %s\n\nOverride protection?", path, pattern, operation),
				)
				if err != nil {
					return "", true, fmt.Errorf("confirmation error: %w", err)
				}
				if confirmed {
					return fmt.Sprintf("⚠️ Override: Path '%s' is protected (pattern: %s) but was allowed by user.", path, pattern), false, nil
				}
			}

			return fmt.Sprintf("🛡️ BLOCKED: Path '%s' matches protected pattern '%s'. Operation '%s' was prevented.", path, pattern, operation), true, nil
		},
	)

	ext.Run()
}

func isProtected(path string) (bool, string) {
	name := filepath.Base(path)
	for _, pattern := range defaultProtectedPatterns {
		// Check pattern as glob
		if matched, _ := filepath.Match(pattern, name); matched {
			return true, pattern
		}
		// Check pattern as directory prefix
		if strings.HasSuffix(pattern, "/") {
			dir := strings.TrimSuffix(pattern, "/")
			if strings.Contains(path, "/"+dir+"/") || strings.HasPrefix(path, dir+"/") {
				return true, pattern
			}
		}
		// Check pattern as path suffix
		if strings.HasPrefix(pattern, "*") && strings.HasSuffix(path, pattern[1:]) {
			return true, pattern
		}
	}
	return false, ""
}
