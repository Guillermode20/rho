package agentutils

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// GitInfo contains information about the current git repository.
type GitInfo struct {
	Branch      string `json:"branch"`
	Commit      string `json:"commit"`
	ShortCommit string `json:"shortCommit"`
	Message     string `json:"message,omitempty"`
	HasChanges  bool   `json:"hasChanges"`
	Ahead       int    `json:"ahead"`
	Behind      int    `json:"behind"`
	Remote      string `json:"remote,omitempty"`
	Root        string `json:"root,omitempty"`
}

// GetGitBranch returns the current git branch name.
func GetGitBranch(dir string) string {
	result := Spawn(SpawnOptions{
		Command: "git",
		Args:    []string{"rev-parse", "--abbrev-ref", "HEAD"},
		Dir:     dir,
	})
	if result.Error != nil {
		return ""
	}
	return strings.TrimSpace(result.Stdout)
}

// GetGitInfo gathers information about the git repository in the given directory.
func GetGitInfo(dir string) *GitInfo {
	info := &GitInfo{}

	branch := Spawn(SpawnOptions{Command: "git", Args: []string{"rev-parse", "--abbrev-ref", "HEAD"}, Dir: dir})
	if branch.Error != nil {
		return info
	}
	info.Branch = strings.TrimSpace(branch.Stdout)

	commit := Spawn(SpawnOptions{Command: "git", Args: []string{"rev-parse", "HEAD"}, Dir: dir})
	if commit.Error == nil {
		info.Commit = strings.TrimSpace(commit.Stdout)
		if len(info.Commit) > 7 {
			info.ShortCommit = info.Commit[:7]
		}
	}

	msg := Spawn(SpawnOptions{Command: "git", Args: []string{"log", "-1", "--format=%s"}, Dir: dir})
	if msg.Error == nil {
		info.Message = strings.TrimSpace(msg.Stdout)
	}

	changes := Spawn(SpawnOptions{Command: "git", Args: []string{"status", "--porcelain"}, Dir: dir})
	if changes.Error == nil && strings.TrimSpace(changes.Stdout) != "" {
		info.HasChanges = true
	}

	revList := Spawn(SpawnOptions{Command: "git", Args: []string{"rev-list", "--left-right", "--count", "HEAD...@{upstream}"}, Dir: dir})
	if revList.Error == nil {
		parts := strings.Fields(revList.Stdout)
		if len(parts) >= 2 {
			fmt.Sscanf(parts[0], "%d", &info.Behind)
			fmt.Sscanf(parts[1], "%d", &info.Ahead)
		}
	}

	remote := Spawn(SpawnOptions{Command: "git", Args: []string{"remote", "get-url", "origin"}, Dir: dir})
	if remote.Error == nil {
		info.Remote = strings.TrimSpace(remote.Stdout)
	}

	root := Spawn(SpawnOptions{Command: "git", Args: []string{"rev-parse", "--show-toplevel"}, Dir: dir})
	if root.Error == nil {
		info.Root = strings.TrimSpace(root.Stdout)
	}

	return info
}

// IsGitRepo returns true if the directory is inside a git repository.
func IsGitRepo(dir string) bool {
	result := Spawn(SpawnOptions{
		Command: "git",
		Args:    []string{"rev-parse", "--git-dir"},
		Dir:     dir,
	})
	return result.Success
}

// GitDiff returns the git diff for the repo.
func GitDiff(dir string, staged bool) string {
	args := []string{"diff"}
	if staged {
		args = []string{"diff", "--staged"}
	}
	result := Spawn(SpawnOptions{Command: "git", Args: args, Dir: dir})
	if result.Error != nil {
		return ""
	}
	return result.Stdout
}

// GitCommit creates a git commit with the given message.
func GitCommit(dir, message string) error {
	result := Spawn(SpawnOptions{
		Command: "git",
		Args:    []string{"commit", "-m", message},
		Dir:     dir,
	})
	return result.Error
}

// GitAdd stages all files.
func GitAdd(dir string, files ...string) error {
	args := []string{"add"}
	if len(files) == 0 {
		args = append(args, ".")
	} else {
		args = append(args, files...)
	}
	result := Spawn(SpawnOptions{Command: "git", Args: args, Dir: dir})
	return result.Error
}

// GitLog returns recent git log entries.
func GitLog(dir string, count int) string {
	if count <= 0 {
		count = 5
	}
	result := Spawn(SpawnOptions{
		Command: "git",
		Args:    []string{"log", "--oneline", fmt.Sprintf("-%d", count)},
		Dir:     dir,
	})
	if result.Error != nil {
		return ""
	}
	return result.Stdout
}

// GitCheckpointer auto-commits changes before agent actions.
type GitCheckpointer struct {
	dir             string
	branch          string
	autoPush        bool
	mu              sync.Mutex
	checkpointCount int
}

// NewGitCheckpointer creates a new git checkpointer.
func NewGitCheckpointer(dir string) *GitCheckpointer {
	return &GitCheckpointer{
		dir:    dir,
		branch: GetGitBranch(dir),
	}
}

// SetAutoPush enables automatic pushing after checkpoints.
func (gc *GitCheckpointer) SetAutoPush(push bool) {
	gc.autoPush = push
}

// Checkpoint creates an automatic checkpoint commit.
func (gc *GitCheckpointer) Checkpoint(message string) error {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	if !IsGitRepo(gc.dir) {
		return fmt.Errorf("not a git repository")
	}

	gc.checkpointCount++

	// Stage all changes
	if err := GitAdd(gc.dir); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	// Check if there's anything to commit
	status := Spawn(SpawnOptions{Command: "git", Args: []string{"status", "--porcelain"}, Dir: gc.dir})
	if status.Error == nil && strings.TrimSpace(status.Stdout) == "" {
		return nil // Nothing to commit
	}

	msg := fmt.Sprintf("rho checkpoint %d: %s", gc.checkpointCount, message)
	if err := GitCommit(gc.dir, msg); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}

	if gc.autoPush {
		go func() {
			Spawn(SpawnOptions{
				Command: "git",
				Args:    []string{"push"},
				Dir:     gc.dir,
				Timeout: 30 * time.Second,
			})
		}()
	}

	return nil
}

// Restore restores working tree to the last checkpoint.
func (gc *GitCheckpointer) Restore() error {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	result := Spawn(SpawnOptions{
		Command: "git",
		Args:    []string{"checkout", "--", "."},
		Dir:     gc.dir,
	})
	return result.Error
}

// FormatGitStatus formats git information for display.
func FormatGitStatus(info *GitInfo) string {
	if info == nil || info.Branch == "" {
		return ""
	}
	parts := []string{info.Branch}
	if info.HasChanges {
		parts = append(parts, "*")
	}
	if info.Ahead > 0 || info.Behind > 0 {
		parts = append(parts, fmt.Sprintf("+%d-%d", info.Ahead, info.Behind))
	}
	return strings.Join(parts, " ")
}
