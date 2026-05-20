// Subagent extension for rho.
//
// Provides a SpawnSubagent tool that lets the LLM delegate tasks to
// sub-processes running rho in print mode. Also supports parallel
// subagent spawning, status tracking, and result collection.
//
// Build:  go build -o subagent ./examples/extensions/subagent/
// Deploy: cp subagent ~/.rho/extensions/subagent/
//         cp examples/extensions/subagent/rho.toml ~/.rho/extensions/subagent/
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/earendil-works/rho/pkg/sdk"
)

// subagentTask tracks a running or completed subagent task.
type subagentTask struct {
	ID        uint64 `json:"id"`
	Task      string `json:"task"`
	Status    string `json:"status"` // "running", "completed", "failed"
	Result    string `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
	StartedAt int64  `json:"started_at"`
	Duration  string `json:"duration,omitempty"`
}

var (
	tasks     []*subagentTask
	tasksMu   sync.Mutex
	nextID    atomic.Uint64
)

func main() {
	ext := sdk.New("rho.subagent")

	// ========================================================================
	// SpawnSubagent – run a single task in a sub-process
	// ========================================================================
	ext.Tool("SpawnSubagent", "Spawn a sub-agent to work on a task independently. "+
		"The sub-agent runs rho in print mode and returns its response.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task": map[string]interface{}{
					"type":        "string",
					"description": "The task for the sub-agent to complete",
				},
				"instructions": map[string]interface{}{
					"type":        "string",
					"description": "Detailed instructions or context for the sub-agent",
				},
				"model": map[string]interface{}{
					"type":        "string",
					"description": "Model to use (e.g., 'claude-sonnet-4-20250514'). Defaults to the parent's model.",
				},
				"timeout": map[string]interface{}{
					"type":        "number",
					"description": "Timeout in seconds for the sub-agent (default: 120)",
				},
			},
			"required": []interface{}{"task"},
		}, func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			taskStr, _ := args["task"].(string)
			instructions, _ := args["instructions"].(string)
			model, _ := args["model"].(string)
			timeoutSec := 120
			if t, ok := args["timeout"].(float64); ok && t > 0 {
				timeoutSec = int(t)
			}

			if taskStr == "" {
				return "", true, fmt.Errorf("task is required")
			}

			// Build the prompt for the sub-agent
			prompt := taskStr
			if instructions != "" {
				prompt = fmt.Sprintf("%s\n\nInstructions:\n%s", taskStr, instructions)
			}

			// Find the rho binary
			rhoBin, err := findRhoBinary()
			if err != nil {
				return "", true, fmt.Errorf("cannot find rho binary: %w", err)
			}

			// Spawn rho in print mode
			argsList := []string{"-mode", "print", "-prompt", prompt}
			if model != "" {
				argsList = append(argsList, "-model", model)
			}

			cmd := exec.Command(rhoBin, argsList...)
			cmd.Env = os.Environ()

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			// Track the task
			id := nextID.Add(1)
			task := &subagentTask{
				ID:        id,
				Task:      truncate(taskStr, 100),
				Status:    "running",
				StartedAt: time.Now().UnixMilli(),
			}
			tasksMu.Lock()
			tasks = append(tasks, task)
			tasksMu.Unlock()

			// Run with timeout
			timer := time.AfterFunc(time.Duration(timeoutSec)*time.Second, func() {
				if cmd.Process != nil {
					cmd.Process.Kill()
				}
			})

			err = cmd.Run()
			timer.Stop()

			tasksMu.Lock()
			if err != nil {
				task.Status = "failed"
				task.Error = fmt.Sprintf("%v\n%s", err, stderr.String())
				task.Duration = formatDuration(time.Since(time.UnixMilli(task.StartedAt)))
				tasksMu.Unlock()
				return fmt.Sprintf("Sub-agent #%d failed: %v\nStderr: %s", id, err, stderr.String()), true, nil
			}

			task.Status = "completed"
			task.Result = stdout.String()
			task.Duration = formatDuration(time.Since(time.UnixMilli(task.StartedAt)))
			tasksMu.Unlock()

			result := fmt.Sprintf("=== SUB-AGENT #%d RESULTS ===\n%s", id, stdout.String())
			return result, false, nil
		})

	// ========================================================================
	// SpawnSubagents – spawn multiple sub-agents for parallel tasks
	// ========================================================================
	ext.Tool("SpawnSubagents", "Spawn multiple sub-agents in parallel to work on independent tasks simultaneously.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"tasks": map[string]interface{}{
					"type":        "array",
					"description": "List of tasks for sub-agents",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"task": map[string]interface{}{
								"type":        "string",
								"description": "The task for this sub-agent",
							},
							"instructions": map[string]interface{}{
								"type":        "string",
								"description": "Additional instructions for this sub-agent",
							},
						},
						"required": []interface{}{"task"},
					},
				},
				"model": map[string]interface{}{
					"type":        "string",
					"description": "Model to use for all sub-agents",
				},
				"timeout": map[string]interface{}{
					"type":        "number",
					"description": "Timeout in seconds for each sub-agent (default: 120)",
				},
			},
			"required": []interface{}{"tasks"},
		}, func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			taskList, _ := args["tasks"].([]interface{})
			model, _ := args["model"].(string)
			timeoutSec := 120
			if t, ok := args["timeout"].(float64); ok && t > 0 {
				timeoutSec = int(t)
			}

			if len(taskList) == 0 {
				return "", true, fmt.Errorf("at least one task is required")
			}

			rhoBin, err := findRhoBinary()
			if err != nil {
				return "", true, fmt.Errorf("cannot find rho binary: %w", err)
			}

			type jobResult struct {
				Index int
				ID    uint64
				Text  string
				Err   string
			}

			ch := make(chan jobResult, len(taskList))
			var wg sync.WaitGroup

			for i, raw := range taskList {
				taskDef, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				taskStr, _ := taskDef["task"].(string)
				instructions, _ := taskDef["instructions"].(string)
				if taskStr == "" {
					continue
				}

				wg.Add(1)
				go func(idx int, taskStr, instructions string) {
					defer wg.Done()

					prompt := taskStr
					if instructions != "" {
						prompt = fmt.Sprintf("%s\n\nInstructions:\n%s", taskStr, instructions)
					}

					argsList := []string{"-mode", "print", "-prompt", prompt}
					if model != "" {
						argsList = append(argsList, "-model", model)
					}

					cmd := exec.Command(rhoBin, argsList...)
					cmd.Env = os.Environ()
					var stdout, stderr bytes.Buffer
					cmd.Stdout = &stdout
					cmd.Stderr = &stderr

					id := nextID.Add(1)
					task := &subagentTask{
						ID:        id,
						Task:      truncate(taskStr, 100),
						Status:    "running",
						StartedAt: time.Now().UnixMilli(),
					}
					tasksMu.Lock()
					tasks = append(tasks, task)
					tasksMu.Unlock()

					timer := time.AfterFunc(time.Duration(timeoutSec)*time.Second, func() {
						if cmd.Process != nil {
							cmd.Process.Kill()
						}
					})
					err := cmd.Run()
					timer.Stop()

					tasksMu.Lock()
					if err != nil {
						task.Status = "failed"
						task.Error = fmt.Sprintf("%v\n%s", err, stderr.String())
						task.Duration = formatDuration(time.Since(time.UnixMilli(task.StartedAt)))
						tasksMu.Unlock()
						ch <- jobResult{Index: idx, ID: id, Err: fmt.Sprintf("%v\n%s", err, stderr.String())}
						return
					}
					task.Status = "completed"
					task.Result = stdout.String()
					task.Duration = formatDuration(time.Since(time.UnixMilli(task.StartedAt)))
					tasksMu.Unlock()

					ch <- jobResult{Index: idx, ID: id, Text: stdout.String()}
				}(i, taskStr, instructions)
			}

			wg.Wait()
			close(ch)

			// Collect results in order
			results := make([]jobResult, len(taskList))
			for r := range ch {
				results[r.Index] = r
			}

			var b strings.Builder
			b.WriteString(fmt.Sprintf("=== PARALLEL SUB-AGENTS (%d tasks) ===\n\n", len(taskList)))
			for i, r := range results {
				b.WriteString(fmt.Sprintf("--- Task #%d (Sub-agent #%d) ---\n", i+1, r.ID))
				if r.Err != "" {
					b.WriteString(fmt.Sprintf("FAILED: %s\n", r.Err))
				} else {
					b.WriteString(r.Text)
				}
				b.WriteString("\n")
			}

			return b.String(), false, nil
		})

	// ========================================================================
	// SubagentStatus – check the status of running/completed sub-agents
	// ========================================================================
	ext.Tool("SubagentStatus", "Check the status of all spawned sub-agents. Returns a summary of running, completed, and failed tasks.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "number",
					"description": "Optional: specific sub-agent ID to check",
				},
			},
		}, func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			tasksMu.Lock()
			defer tasksMu.Unlock()

			specificID, hasID := args["id"].(float64)

			if hasID {
				for _, t := range tasks {
					if t.ID == uint64(specificID) {
						data, _ := json.MarshalIndent(t, "", "  ")
						return string(data), false, nil
					}
				}
				return "", true, fmt.Errorf("sub-agent #%d not found", uint64(specificID))
			}

			var running, completed, failed int
			for _, t := range tasks {
				switch t.Status {
				case "running":
					running++
				case "completed":
					completed++
				case "failed":
					failed++
				}
			}

			var b strings.Builder
			b.WriteString(fmt.Sprintf("Sub-agents: %d total\n", len(tasks)))
			b.WriteString(fmt.Sprintf("  Running:   %d\n", running))
			b.WriteString(fmt.Sprintf("  Completed: %d\n", completed))
			b.WriteString(fmt.Sprintf("  Failed:    %d\n", failed))

			if len(tasks) > 0 {
				b.WriteString("\nRecent tasks:\n")
				start := len(tasks) - 5
				if start < 0 {
					start = 0
				}
				for i := start; i < len(tasks); i++ {
					t := tasks[i]
					b.WriteString(fmt.Sprintf("  #%d [%s] %s (%s)\n", t.ID, t.Status, t.Task, t.Duration))
				}
			}

			return b.String(), false, nil
		})

	ext.Run()
}

// findRhoBinary locates the rho binary by checking common locations.
func findRhoBinary() (string, error) {
	// 1. Check RHO_BIN env var
	if bin := os.Getenv("RHO_BIN"); bin != "" {
		if _, err := os.Stat(bin); err == nil {
			return bin, nil
		}
	}

	// 2. Same directory as this extension binary
	extPath, err := os.Executable()
	if err == nil {
		extDir := filepath.Dir(extPath)
		candidates := []string{"rho", filepath.Join(extDir, "rho")}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				abs, _ := filepath.Abs(c)
				return abs, nil
			}
		}
	}

	// 3. PATH lookup
	if p, err := exec.LookPath("rho"); err == nil {
		return p, nil
	}

	// 4. Common install locations
	candidates := []string{
		"/usr/local/bin/rho",
		filepath.Join(os.Getenv("HOME"), ".local", "bin", "rho"),
		filepath.Join(os.Getenv("HOME"), "go", "bin", "rho"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	return "", fmt.Errorf("rho binary not found. Set RHO_BIN env var or ensure rho is in your PATH")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	return fmt.Sprintf("%.0fm %.0fs", d.Minutes(), d.Seconds()-d.Minutes()*60)
}
