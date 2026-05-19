// Package tools implements the coding agent's tool set for file operations,
// shell execution, search, and text manipulation.
package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/earendil-works/rho/pkg/agent"
)

// ToolFactory creates an agent tool from the given context.
type ToolFactory func(cwd string) agent.AgentTool

// ToolFactories returns all available tool factories.
func ToolFactories() map[string]ToolFactory {
	return map[string]ToolFactory{
		"Read":     NewReadTool,
		"Write":    NewWriteTool,
		"Edit":     NewEditTool,
		"Bash":     NewBashTool,
		"Grep":     NewGrepTool,
		"Find":     NewFindTool,
		"Glob":     NewGlobTool,
		"Ls":       NewLsTool,
	}
}

// AllTools creates all tools bound to the given cwd.
func AllTools(cwd string) []agent.AgentTool {
	var tools []agent.AgentTool
	for name, factory := range ToolFactories() {
		tool := factory(cwd)
		tool.Name = name // Use canonical name
		tools = append(tools, tool)
	}
	return tools
}

// Read tool parameters.
var readParams = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "Path to the file to read (relative or absolute)",
		},
		"offset": map[string]interface{}{
			"type":        "number",
			"description": "Line number to start reading from (1-indexed)",
		},
		"limit": map[string]interface{}{
			"type":        "number",
			"description": "Maximum number of lines to read",
		},
	},
	"required": []interface{}{"path"},
}

// NewReadTool creates a Read tool.
func NewReadTool(cwd string) agent.AgentTool {
	return agent.AgentTool{
		Name:        "Read",
		Description: "Read the contents of a file. Supports text files and images.",
		Parameters:  readParams,
		Execute: func(args map[string]interface{}) (string, bool, error) {
			pathStr, _ := args["path"].(string)
			if pathStr == "" {
				return "", true, fmt.Errorf("path is required")
			}

			absPath := resolvePath(pathStr, cwd)

			// Check file exists
			info, err := os.Stat(absPath)
			if err != nil {
				return "", true, fmt.Errorf("file not found: %s", pathStr)
			}
			if info.IsDir() {
				return "", true, fmt.Errorf("path is a directory: %s", pathStr)
			}

			// Check if it looks like an image
			if isImageFile(absPath) {
				return fmt.Sprintf("[Image: %s (%d bytes)]", filepath.Base(absPath), info.Size()), false, nil
			}

			// Read text file
			file, err := os.Open(absPath)
			if err != nil {
				return "", true, fmt.Errorf("cannot read file: %w", err)
			}
			defer file.Close()

			var offset, limit int
			if o, ok := args["offset"]; ok {
				if of, ok := toInt(o); ok {
					offset = of
				}
			}
			if l, ok := args["limit"]; ok {
				if li, ok := toInt(l); ok {
					limit = li
				}
			}

			return readFileLines(file, offset, limit)
		},
	}
}

func readFileLines(file *os.File, offset, limit int) (string, bool, error) {
	const maxBytes = 50000
	const maxLines = 2000

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var buf bytes.Buffer
	lineNum := 0
	totalBytes := 0
	truncated := false

	for scanner.Scan() {
		lineNum++
		if offset > 0 && lineNum < offset {
			continue
		}
		if limit > 0 && lineNum >= offset+limit {
			break
		}

		line := scanner.Text()
		totalBytes += len(line) + 1 // +1 for newline

		if totalBytes > maxBytes {
			truncated = true
			break
		}
		if lineNum-offset+1 > maxLines {
			truncated = true
			break
		}

		buf.WriteString(fmt.Sprintf("%6d │ %s\n", lineNum, line))
	}

	if err := scanner.Err(); err != nil {
		return "", true, fmt.Errorf("error reading file: %w", err)
	}

	result := buf.String()
	if truncated {
		result += fmt.Sprintf("\n... (truncated at %d bytes / %d lines)", maxBytes, maxLines)
	}

	if result == "" {
		result = "(empty file)"
	}

	return result, false, nil
}

// Write tool parameters.
var writeParams = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "Path to the file to write (relative or absolute)",
		},
		"content": map[string]interface{}{
			"type":        "string",
			"description": "Content to write to the file",
		},
	},
	"required": []interface{}{"path", "content"},
}

// NewWriteTool creates a Write tool.
func NewWriteTool(cwd string) agent.AgentTool {
	return agent.AgentTool{
		Name:        "Write",
		Description: "Write content to a file. Creates the file if it doesn't exist, overwrites if it does. Automatically creates parent directories.",
		Parameters:  writeParams,
		Execute: func(args map[string]interface{}) (string, bool, error) {
			pathStr, _ := args["path"].(string)
			content, _ := args["content"].(string)
			if pathStr == "" {
				return "", true, fmt.Errorf("path is required")
			}

			absPath := resolvePath(pathStr, cwd)

			// Create parent directories
			parentDir := filepath.Dir(absPath)
			if err := os.MkdirAll(parentDir, 0755); err != nil {
				return "", true, fmt.Errorf("cannot create directory %s: %w", parentDir, err)
			}

			// Write file
			if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
				return "", true, fmt.Errorf("cannot write file: %w", err)
			}

			return fmt.Sprintf("Written %d bytes to %s", len(content), pathStr), false, nil
		},
	}
}

// Edit tool parameters.
var editParams = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "Path to the file to edit (relative or absolute)",
		},
		"oldText": map[string]interface{}{
			"type":        "string",
			"description": "Exact text to replace (must be unique in the file)",
		},
		"newText": map[string]interface{}{
			"type":        "string",
			"description": "Replacement text",
		},
	},
	"required": []interface{}{"path", "oldText", "newText"},
}

// NewEditTool creates an Edit tool with exact text replacement.
func NewEditTool(cwd string) agent.AgentTool {
	return agent.AgentTool{
		Name:        "Edit",
		Description: "Edit a file using exact text replacement. Old text must be unique.",
		Parameters:  editParams,
		Execute: func(args map[string]interface{}) (string, bool, error) {
			pathStr, _ := args["path"].(string)
			oldText, _ := args["oldText"].(string)
			newText, _ := args["newText"].(string)

			if pathStr == "" || oldText == "" {
				return "", true, fmt.Errorf("path and oldText are required")
			}

			absPath := resolvePath(pathStr, cwd)

			data, err := os.ReadFile(absPath)
			if err != nil {
				return "", true, fmt.Errorf("cannot read file: %w", err)
			}

			content := string(data)

			// Count occurrences
			count := strings.Count(content, oldText)
			if count == 0 {
				return "", true, fmt.Errorf("oldText not found in the file")
			}
			if count > 1 {
				return "", true, fmt.Errorf("oldText occurs %d times; must be unique", count)
			}

			newContent := strings.Replace(content, oldText, newText, 1)

			if err := os.WriteFile(absPath, []byte(newContent), 0644); err != nil {
				return "", true, fmt.Errorf("cannot write file: %w", err)
			}

			return fmt.Sprintf("Applied edit to %s", pathStr), false, nil
		},
	}
}

// Bash tool parameters.
var bashParams = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"command": map[string]interface{}{
			"type":        "string",
			"description": "Bash command to execute",
		},
		"timeout": map[string]interface{}{
			"type":        "number",
			"description": "Timeout in seconds (optional, no default timeout)",
		},
	},
	"required": []interface{}{"command"},
}

// NewBashTool creates a Bash tool.
func NewBashTool(cwd string) agent.AgentTool {
	return agent.AgentTool{
		Name:        "Bash",
		Description: "Execute a bash command and capture its output. Returns stdout and stderr.",
		Parameters:  bashParams,
		Execute: func(args map[string]interface{}) (string, bool, error) {
			command, _ := args["command"].(string)
			if command == "" {
				return "", true, fmt.Errorf("command is required")
			}

			timeoutSec := 0
			if t, ok := args["timeout"]; ok {
				if tf, ok := toInt(t); ok {
					timeoutSec = tf
				}
			}

			ctx := context.Background()
			var cancel context.CancelFunc
			if timeoutSec > 0 {
				ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
				defer cancel()
			}

			cmd := exec.CommandContext(ctx, "bash", "-c", command)
			cmd.Dir = cwd

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()

			output := ""
			if stdout.Len() > 0 {
				output += stdout.String()
			}
			if stderr.Len() > 0 {
				if output != "" {
					output += "\n"
				}
				output += stderr.String()
			}

			const maxOutput = 100000
			if len(output) > maxOutput {
				output = output[:maxOutput] + fmt.Sprintf("\n... (truncated at %d bytes)", maxOutput)
			}

			if err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					return output + fmt.Sprintf("\nCommand timed out after %d seconds", timeoutSec), false, nil
				}
				return output, true, fmt.Errorf("command failed: %w", err)
			}

			return output, false, nil
		},
	}
}

// Grep tool parameters.
var grepParams = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"pattern": map[string]interface{}{
			"type":        "string",
			"description": "Search pattern (regex or literal string)",
		},
		"path": map[string]interface{}{
			"type":        "string",
			"description": "Directory or file to search (default: current directory)",
		},
		"glob": map[string]interface{}{
			"type":        "string",
			"description": "Filter files by glob pattern, e.g. '*.ts' or '**/*.spec.ts'",
		},
		"ignoreCase": map[string]interface{}{
			"type":        "boolean",
			"description": "Case-insensitive search (default: false)",
		},
	},
	"required": []interface{}{"pattern"},
}

// NewGrepTool creates a Grep tool.
func NewGrepTool(cwd string) agent.AgentTool {
	return agent.AgentTool{
		Name:        "Grep",
		Description: "Search file contents for a pattern. Returns matching lines with file paths and line numbers.",
		Parameters:  grepParams,
		Execute: func(args map[string]interface{}) (string, bool, error) {
			pattern, _ := args["pattern"].(string)
			if pattern == "" {
				return "", true, fmt.Errorf("pattern is required")
			}

			searchPath := cwd
			if p, ok := args["path"]; ok {
				if ps, ok := p.(string); ok {
					searchPath = resolvePath(ps, cwd)
				}
			}

			globPattern, _ := args["glob"].(string)
			ignoreCase, _ := args["ignoreCase"].(bool)

			return grepFiles(searchPath, pattern, globPattern, ignoreCase)
		},
	}
}

func grepFiles(searchPath, pattern, globPattern string, ignoreCase bool) (string, bool, error) {
	const maxMatches = 100

	var buf bytes.Buffer
	matchCount := 0

	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			// Skip hidden dirs and common ignore dirs
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			if info.Name() == "node_modules" || info.Name() == "vendor" || info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip binary-looking files
		if isBinaryExt(info.Name()) {
			return nil
		}

		// Apply glob filter
		if globPattern != "" {
			matched, err := filepath.Match(globPattern, info.Name())
			if err != nil || !matched {
				return nil
			}
		}

		if matchCount >= maxMatches {
			return filepath.SkipDir
		}

		// Read file and search
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			var found bool
			if ignoreCase {
				found = strings.Contains(strings.ToLower(line), strings.ToLower(pattern))
			} else {
				found = strings.Contains(line, pattern)
			}

			if found {
				matchCount++
				relPath := path
				if strings.HasPrefix(path, searchPath) {
					relPath = path[len(searchPath):]
					if strings.HasPrefix(relPath, "/") {
						relPath = relPath[1:]
					}
				}
				buf.WriteString(fmt.Sprintf("%s:%d: %s\n", relPath, lineNum, strings.TrimSpace(line)))
				if matchCount >= maxMatches {
					return filepath.SkipDir
				}
			}
		}

		return nil
	})

	if err != nil {
		return "", true, fmt.Errorf("search failed: %w", err)
	}

	result := buf.String()
	if result == "" {
		result = "No matches found."
	} else if matchCount >= maxMatches {
		result += fmt.Sprintf("\n... (%d+ matches, truncated)", maxMatches)
	}

	return result, false, nil
}

// Find tool parameters.
var findParams = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"pattern": map[string]interface{}{
			"type":        "string",
			"description": "Glob pattern to match files, e.g. '*.ts', '**/*.json'",
		},
		"path": map[string]interface{}{
			"type":        "string",
			"description": "Directory to search in (default: current directory)",
		},
		"limit": map[string]interface{}{
			"type":        "number",
			"description": "Maximum number of results (default: 100)",
		},
	},
	"required": []interface{}{"pattern"},
}

// NewFindTool creates a Find tool.
func NewFindTool(cwd string) agent.AgentTool {
	return agent.AgentTool{
		Name:        "Find",
		Description: "Search for files by glob pattern. Returns matching file paths.",
		Parameters:  findParams,
		Execute: func(args map[string]interface{}) (string, bool, error) {
			pattern, _ := args["pattern"].(string)
			if pattern == "" {
				return "", true, fmt.Errorf("pattern is required")
			}

			searchPath := cwd
			if p, ok := args["path"]; ok {
				if ps, ok := p.(string); ok {
					searchPath = resolvePath(ps, cwd)
				}
			}

			maxResults := 100
			if l, ok := args["limit"]; ok {
				if li, ok := toInt(l); ok && li > 0 {
					maxResults = li
				}
			}

			return findFiles(searchPath, pattern, maxResults)
		},
	}
}

func findFiles(searchPath, pattern string, maxResults int) (string, bool, error) {
	var buf bytes.Buffer
	count := 0

	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if count >= maxResults {
			return filepath.SkipDir
		}

		matched, err := filepath.Match(pattern, info.Name())
		if err != nil {
			return nil
		}
		if !matched {
			return nil
		}

		relPath := path
		if strings.HasPrefix(path, searchPath) {
			relPath = path[len(searchPath):]
			if strings.HasPrefix(relPath, "/") {
				relPath = relPath[1:]
			}
		}
		if relPath == "" {
			relPath = "."
		}

		count++
		if info.IsDir() {
			buf.WriteString(relPath + "/\n")
		} else {
			buf.WriteString(relPath + "\n")
		}

		return nil
	})

	if err != nil {
		return "", true, fmt.Errorf("find failed: %w", err)
	}

	result := buf.String()
	if result == "" {
		result = "No files found matching pattern: " + pattern
	} else if count >= maxResults {
		result += fmt.Sprintf("... (%d results, truncated)", count)
	}

	return result, false, nil
}

// Glob tool parameters.
var globParams = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"pattern": map[string]interface{}{
			"type":        "string",
			"description": "Glob pattern to match, e.g. '**/*.go'",
		},
		"path": map[string]interface{}{
			"type":        "string",
			"description": "Directory to search in (default: current directory)",
		},
	},
	"required": []interface{}{"pattern"},
}

// NewGlobTool creates a Glob tool.
func NewGlobTool(cwd string) agent.AgentTool {
	return agent.AgentTool{
		Name:        "Glob",
		Description: "Search for files by glob pattern (double-star aware). Returns matching file paths relative to search directory.",
		Parameters:  globParams,
		Execute: func(args map[string]interface{}) (string, bool, error) {
			// Delegates to Find for now
			return NewFindTool(cwd).Execute(args)
		},
	}
}

// Ls tool parameters.
var lsParams = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "Directory to list (default: current directory)",
		},
		"limit": map[string]interface{}{
			"type":        "number",
			"description": "Maximum number of entries to return (default: 500)",
		},
	},
}

// NewLsTool creates an Ls tool.
func NewLsTool(cwd string) agent.AgentTool {
	return agent.AgentTool{
		Name:        "Ls",
		Description: "List directory contents. Returns entries sorted alphabetically, with '/' suffix for directories.",
		Parameters:  lsParams,
		Execute: func(args map[string]interface{}) (string, bool, error) {
			dirPath := cwd
			if p, ok := args["path"]; ok {
				if ps, ok := p.(string); ok {
					dirPath = resolvePath(ps, cwd)
				}
			}

			maxEntries := 500
			if l, ok := args["limit"]; ok {
				if li, ok := toInt(l); ok && li > 0 {
					maxEntries = li
				}
			}

			entries, err := os.ReadDir(dirPath)
			if err != nil {
				return "", true, fmt.Errorf("cannot read directory: %w", err)
			}

			var buf bytes.Buffer
			count := 0
			for _, entry := range entries {
				if count >= maxEntries {
					buf.WriteString(fmt.Sprintf("... (%d entries, truncated)", len(entries)))
					break
				}
				name := entry.Name()
				if entry.IsDir() {
					name += "/"
				}
				// Get file info for size
				info, err := entry.Info()
				if err == nil {
					buf.WriteString(fmt.Sprintf("%-40s %8d\n", name, info.Size()))
				} else {
					buf.WriteString(name + "\n")
				}
				count++
			}

			return buf.String(), false, nil
		},
	}
}

// Helpers

func resolvePath(pathStr, cwd string) string {
	if filepath.IsAbs(pathStr) {
		return filepath.Clean(pathStr)
	}
	return filepath.Clean(filepath.Join(cwd, pathStr))
}

func isImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".ico":
		return true
	}
	return false
}

func isBinaryExt(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".exe", ".bin", ".a", ".so", ".dll", ".dylib", ".o":
		return true
	case ".png", ".jpg", ".jpeg", ".gif", ".ico", ".webp":
		return true
	case ".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar":
		return true
	case ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt":
		return true
	case ".mp3", ".mp4", ".avi", ".mov", ".wav":
		return true
	case ".ttf", ".otf", ".woff", ".woff2":
		return true
	case ".pyc", ".class", ".jar":
		return true
	}
	return false
}

func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i), true
		}
		return 0, false
	default:
		return 0, false
	}
}


