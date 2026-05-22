package agentutils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// SpawnOptions configures child process spawning.
type SpawnOptions struct {
	Command string
	Args    []string
	Dir     string
	Env     map[string]string
	Timeout time.Duration
	Stdin   string
}

// SpawnResult captures the result of a spawned process.
type SpawnResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Success  bool
	Duration time.Duration
	Error    error
}

// Spawn spawns a child process with the given options and captures its output.
func Spawn(opts SpawnOptions) *SpawnResult {
	ctx := context.Background()
	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, opts.Command, opts.Args...)
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}

	// Build environment
	cmd.Env = os.Environ()
	for k, v := range opts.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if opts.Stdin != "" {
		cmd.Stdin = strings.NewReader(opts.Stdin)
	}

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := &SpawnResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Errorf("command timed out after %v", opts.Timeout)
			result.ExitCode = -1
			return result
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				result.ExitCode = ws.ExitStatus()
			} else {
				result.ExitCode = 1
			}
			result.Error = err
			return result
		}
		result.Error = err
		result.ExitCode = -1
		return result
	}

	result.Success = true
	result.ExitCode = 0
	return result
}

// SpawnWithStream spawns a process and streams stdout/stderr to callbacks.
func SpawnWithStream(opts SpawnOptions, onStdout func(line string), onStderr func(line string)) *SpawnResult {
	ctx := context.Background()
	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, opts.Command, opts.Args...)
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}
	cmd.Env = os.Environ()
	for k, v := range opts.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	if onStdout != nil && stdoutStr != "" {
		for _, line := range strings.Split(strings.TrimRight(stdoutStr, "\n"), "\n") {
			onStdout(line)
		}
	}
	if onStderr != nil && stderrStr != "" {
		for _, line := range strings.Split(strings.TrimRight(stderrStr, "\n"), "\n") {
			onStderr(line)
		}
	}

	result := &SpawnResult{
		Stdout:   stdoutStr,
		Stderr:   stderrStr,
		Duration: duration,
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Errorf("command timed out after %v", opts.Timeout)
			result.ExitCode = -1
			return result
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				result.ExitCode = ws.ExitStatus()
			} else {
				result.ExitCode = 1
			}
			result.Error = err
			return result
		}
		result.Error = err
		result.ExitCode = -1
		return result
	}

	result.Success = true
	result.ExitCode = 0
	return result
}

// ProcessTree represents a process and its children.
type ProcessTree struct {
	PID      int
	Command  string
	Children []ProcessTree
}

// KillProcessTree kills a process and all its children.
func KillProcessTree(pid int) error {
	// Try to find and kill children first
	children := findChildProcesses(pid)
	for _, child := range children {
		KillProcessTree(child)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGKILL)
}

// findChildProcesses finds direct child PIDs of the given PID.
func findChildProcesses(pid int) []int {
	result := Spawn(SpawnOptions{
		Command: "ps",
		Args:    []string{"--ppid", fmt.Sprintf("%d", pid), "-o", "pid=", "--no-headers"},
	})
	if result.Error != nil {
		return nil
	}
	var children []int
	for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
		var childPID int
		if _, err := fmt.Sscanf(strings.TrimSpace(line), "%d", &childPID); err == nil && childPID > 0 {
			children = append(children, childPID)
		}
	}
	return children
}

// TrackedProcessManager tracks spawned processes for lifecycle management.
type TrackedProcessManager struct {
	mu     sync.Mutex
	procs  map[int]*os.Process
	nextID int
}

// NewTrackedProcessManager creates a new process manager.
func NewTrackedProcessManager() *TrackedProcessManager {
	return &TrackedProcessManager{
		procs: make(map[int]*os.Process),
	}
}

// Track registers a process for tracking.
func (pm *TrackedProcessManager) Track(proc *os.Process) int {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.nextID++
	pm.procs[pm.nextID] = proc
	return pm.nextID
}

// Untrack removes a process from tracking.
func (pm *TrackedProcessManager) Untrack(id int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.procs, id)
}

// KillAll kills all tracked processes.
func (pm *TrackedProcessManager) KillAll() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for id, proc := range pm.procs {
		proc.Kill()
		delete(pm.procs, id)
	}
}
