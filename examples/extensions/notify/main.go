// Notify Extension
//
// Sends desktop notifications when long-running tasks complete.
// Supports both terminal bell and system notifications (via notify-send).
//
// Build:  go build -o notify ./examples/extensions/notify/
// Deploy: cp notify ~/.rho/extensions/notify/
//
//	cp examples/extensions/notify/rho.toml ~/.rho/extensions/notify/
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/earendil-works/rho/pkg/sdk"
)

func main() {
	ext := sdk.New("rho.notify")

	ext.Tool("notify", "Send a desktop notification (or terminal bell) with an optional message. "+
		"Useful for alerting the user when a long-running task completes.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "The notification message to display",
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Optional notification title (default: 'rho')",
				},
				"urgency": map[string]interface{}{
					"type":        "string",
					"description": "Urgency level: 'low', 'normal', or 'critical' (default: 'normal')",
					"enum":        []interface{}{"low", "normal", "critical"},
				},
				"bell": map[string]interface{}{
					"type":        "boolean",
					"description": "Also ring the terminal bell (default: false)",
				},
			},
			"required": []interface{}{"message"},
		},
		func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			message, _ := args["message"].(string)
			title, _ := args["title"].(string)
			urgency, _ := args["urgency"].(string)
			bell, _ := args["bell"].(bool)

			if message == "" {
				return "", true, fmt.Errorf("message is required")
			}
			if title == "" {
				title = "rho"
			}
			if urgency == "" {
				urgency = "normal"
			}

			// Try desktop notification (Linux)
			notifySent := false
			if exec.Command("which", "notify-send").Run() == nil {
				_ = exec.Command("notify-send",
					"-u", urgency,
					"-a", "rho",
					title,
					message,
				).Run()
				notifySent = true
			}

			// Try macOS notification
			if !notifySent && exec.Command("which", "osascript").Run() == nil {
				script := fmt.Sprintf(`display notification "%s" with title "%s"`, escapeAppleScript(message), escapeAppleScript(title))
				_ = exec.Command("osascript", "-e", script).Run()
				notifySent = true
			}

			// Terminal bell
			if bell {
				fmt.Fprint(os.Stderr, "\a")
			}

			platform := "terminal"
			if notifySent {
				platform = "desktop"
			}

			return fmt.Sprintf("✅ Notification sent (%s): %s", platform, message), false, nil
		},
	)

	ext.Tool("notify_after", "Execute a command and send a notification when it completes. "+
		"Useful for running long tasks and being alerted when they finish.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The command to run",
				},
				"notifyMessage": map[string]interface{}{
					"type":        "string",
					"description": "Message to show in the completion notification (default: 'Command completed')",
				},
				"timeout": map[string]interface{}{
					"type":        "number",
					"description": "Timeout in seconds",
				},
			},
			"required": []interface{}{"command"},
		},
		func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			command, _ := args["command"].(string)
			notifyMsg, _ := args["notifyMessage"].(string)
			timeoutSec := 0
			if t, ok := args["timeout"].(float64); ok && t > 0 {
				timeoutSec = int(t)
			}

			if command == "" {
				return "", true, fmt.Errorf("command is required")
			}
			if notifyMsg == "" {
				notifyMsg = "Command completed"
			}

			start := time.Now()

			cmd := exec.Command("bash", "-c", command)
			var outBuf strings.Builder
			cmd.Stdout = &outBuf
			cmd.Stderr = &outBuf

			var err error
			if timeoutSec > 0 {
				timer := time.AfterFunc(time.Duration(timeoutSec)*time.Second, func() {
					if cmd.Process != nil {
						cmd.Process.Kill()
					}
				})
				err = cmd.Run()
				timer.Stop()
			} else {
				err = cmd.Run()
			}

			duration := time.Since(start)
			status := "completed"
			if err != nil {
				status = "failed"
			}

			// Send notification
			notifTitle := fmt.Sprintf("rho: %s", status)
			notifBody := fmt.Sprintf("%s (%s)", notifyMsg, duration.Round(time.Second))
			if exec.Command("which", "notify-send").Run() == nil {
				_ = exec.Command("notify-send", "-a", "rho", notifTitle, notifBody).Run()
			}

			result := fmt.Sprintf("Command %s in %s\n%s", status, duration.Round(time.Second), outBuf.String())
			if err != nil {
				return result, true, err
			}
			return result, false, nil
		},
	)

	ext.Run()
}

func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}
