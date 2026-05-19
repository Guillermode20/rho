package codecore

import (
	"fmt"
	"os"
	"path/filepath"
)

// MissingSessionCwdError is returned when a session has no working directory.
type MissingSessionCwdError struct {
	SessionID string
	Message   string
}

func (e *MissingSessionCwdError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("session %s has no working directory", e.SessionID)
}

// SessionCwdManager manages per-session working directories.
type SessionCwdManager struct {
	cwdMap map[string]string
}

// NewSessionCwdManager creates a new session CWD manager.
func NewSessionCwdManager() *SessionCwdManager {
	return &SessionCwdManager{
		cwdMap: make(map[string]string),
	}
}

// Set sets the working directory for a session.
func (m *SessionCwdManager) Set(sessionID, cwd string) {
	m.cwdMap[sessionID] = cwd
}

// Get returns the working directory for a session.
func (m *SessionCwdManager) Get(sessionID string) (string, bool) {
	cwd, ok := m.cwdMap[sessionID]
	return cwd, ok
}

// Resolve resolves the working directory for a session, falling back to the current directory.
func (m *SessionCwdManager) Resolve(sessionID string) (string, error) {
	if cwd, ok := m.cwdMap[sessionID]; ok {
		return cwd, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", &MissingSessionCwdError{
			SessionID: sessionID,
			Message:   fmt.Sprintf("cannot resolve working directory: %v", err),
		}
	}
	return cwd, nil
}

// GetMissingSessionCwdIssue returns a user-friendly prompt for missing CWD.
func GetMissingSessionCwdIssue(sessionID string) string {
	return fmt.Sprintf(`Session "%s" has no working directory set.

This can happen when:
- The session was created from a different machine
- The session file was manually edited
- The original working directory was deleted

To continue, please set a working directory for this session.`, sessionID)
}

// FormatMissingSessionCwdPrompt formats a prompt asking the user to set a CWD.
func FormatMissingSessionCwdPrompt(sessionID string) string {
	return fmt.Sprintf(`No working directory set for session "%s".

Options:
1. Use the current directory (%s)
2. Specify a different directory
3. Cancel

Please choose an option or enter a path:`,
		sessionID, mustGetwd())
}

func mustGetwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

// ResolveSessionCwd resolves the session CWD from various sources.
func ResolveSessionCwd(sessionCwd, globalCwd string) string {
	if sessionCwd != "" {
		if filepath.IsAbs(sessionCwd) {
			return sessionCwd
		}
		// Relative path - resolve against global CWD
		return filepath.Join(globalCwd, sessionCwd)
	}
	if globalCwd != "" {
		return globalCwd
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

// ValidateSessionCwd checks that the session CWD is valid.
func ValidateSessionCwd(cwd string) error {
	info, err := os.Stat(cwd)
	if err != nil {
		return fmt.Errorf("session working directory is invalid: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("session working directory is not a directory: %s", cwd)
	}
	return nil
}
