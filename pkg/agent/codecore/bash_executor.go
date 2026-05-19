package codecore

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

type BashResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Combined string
	Duration time.Duration
	TimedOut bool
	PID      int
}

type BashSpawnHook func(cmd *exec.Cmd) error

type BashExecutor struct {
	CWD        string
	Env        []string
	SpawnHooks []BashSpawnHook
	Timeout    time.Duration
	ShellPath  string
}

func NewBashExecutor(cwd string) *BashExecutor {
	return &BashExecutor{CWD: cwd, ShellPath: "/bin/bash", Timeout: 5 * time.Minute}
}

func (be *BashExecutor) Execute(command string) (*BashResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), be.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, be.ShellPath, "-c", command)
	cmd.Dir = be.CWD
	cmd.Env = be.appendEnv()
	for _, hook := range be.SpawnHooks {
		if err := hook(cmd); err != nil {
			return nil, err
		}
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)
	result := &BashResult{
		Stdout: stdout.String(), Stderr: stderr.String(),
		Combined: stdout.String() + stderr.String(), Duration: duration,
		PID: cmd.ProcessState.Pid(),
	}
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.TimedOut = true
			result.ExitCode = -1
			return result, nil
		}
		if ee, ok := err.(*exec.ExitError); ok {
			result.ExitCode = ee.ExitCode()
			return result, nil
		}
		return nil, err
	}
	result.ExitCode = cmd.ProcessState.ExitCode()
	return result, nil
}

func (be *BashExecutor) appendEnv() []string {
	if len(be.Env) > 0 {
		return be.Env
	}
	return os.Environ()
}

func (be *BashExecutor) ExecuteWithStreaming(command string, onStdout, onStderr func(string)) (*BashResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), be.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, be.ShellPath, "-c", command)
	cmd.Dir = be.CWD
	cmd.Env = os.Environ()
	for _, hook := range be.SpawnHooks {
		if err := hook(cmd); err != nil {
			return nil, err
		}
	}
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	done := make(chan bool, 2)
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			stdoutBuf.WriteString(line + "\n")
			if onStdout != nil {
				onStdout(line)
			}
		}
		done <- true
	}()
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			stderrBuf.WriteString(line + "\n")
			if onStderr != nil {
				onStderr(line)
			}
		}
		done <- true
	}()
	<-done
	<-done
	err := cmd.Wait()
	result := &BashResult{
		Stdout: stdoutBuf.String(), Stderr: stderrBuf.String(),
		Combined: stdoutBuf.String() + stderrBuf.String(),
		PID:      cmd.ProcessState.Pid(),
	}
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			result.ExitCode = ee.ExitCode()
			return result, nil
		}
		return nil, err
	}
	result.ExitCode = cmd.ProcessState.ExitCode()
	return result, nil
}

func (be *BashExecutor) AddSpawnHook(hook BashSpawnHook) {
	be.SpawnHooks = append(be.SpawnHooks, hook)
}

func KillProcessTree(pid int) error {
	return exec.Command("kill", "-TERM", fmt.Sprintf("-%d", pid)).Run()
}
