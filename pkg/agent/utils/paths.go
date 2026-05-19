package agentutils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IsLocalPath checks if a path is local (not a URL or remote reference).
func IsLocalPath(p string) bool {
	return !strings.Contains(p, "://") && !strings.HasPrefix(p, "~")
}

// IsParentPath checks if parent is an ancestor directory of child.
func IsParentPath(parent, child string) bool {
	absParent, err := filepath.Abs(parent)
	if err != nil {
		return false
	}
	absChild, err := filepath.Abs(child)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absParent, absChild)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && rel != "."
}

// FormatPathRelativeToCwdOrAbsolute formats a path relative to cwd if possible.
func FormatPathRelativeToCwdOrAbsolute(path, cwd string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return absPath
	}
	rel, err := filepath.Rel(absCwd, absPath)
	if err != nil {
		return absPath
	}
	if strings.HasPrefix(rel, "..") {
		return absPath
	}
	return rel
}

// ShortenPath shortens a path by replacing the home directory with ~.
func ShortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// ExpandTildePath expands ~ to the user's home directory.
func ExpandTildePath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// ResolvePath resolves a potentially relative path against a base directory.
func ResolvePath(path, base string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(base, path))
}

// EnsureTrailingSlash ensures a path ends with a separator.
func EnsureTrailingSlash(path string) string {
	if !strings.HasSuffix(path, string(filepath.Separator)) {
		return path + string(filepath.Separator)
	}
	return path
}

// FindProjectRoot finds the project root by looking for common markers.
func FindProjectRoot(startDir string) string {
	markers := []string{".git", "go.mod", "package.json", "Cargo.toml", "pyproject.toml", "Gemfile", "Makefile"}
	dir := startDir
	for {
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return startDir
		}
		dir = parent
	}
}

// IsHiddenPath returns true if the path or any component starts with a dot.
func IsHiddenPath(path string) bool {
	parts := strings.Split(path, string(filepath.Separator))
	for _, part := range parts {
		if strings.HasPrefix(part, ".") && part != "." && part != ".." {
			return true
		}
	}
	return false
}

// SafeJoin joins path elements and ensures the result is within the base.
func SafeJoin(base string, elems ...string) (string, error) {
	joined := filepath.Join(append([]string{base}, elems...)...)
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absBase, absJoined)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path %q escapes base %q", joined, base)
	}
	return joined, nil
}

// DirSize calculates the total size of a directory recursively.
func DirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// FormatFileSize formats a byte count as a human-readable string.
func FormatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
