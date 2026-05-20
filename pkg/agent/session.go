package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// SessionHeader contains metadata about a session.
type SessionHeader struct {
	Type          string `json:"type"`
	Version       int    `json:"version"`
	ID            string `json:"id"`
	Timestamp     string `json:"timestamp"`
	CWD           string `json:"cwd"`
	ParentSession string `json:"parentSession,omitempty"`
}

// SessionEntry is a single entry in a session file.
type SessionEntry struct {
	Type          string        `json:"type"`
	Version       int           `json:"version,omitempty"`
	ID            string        `json:"id"`
	ParentID      string        `json:"parentId"`
	Timestamp     string        `json:"timestamp"`
	CWD           string        `json:"cwd,omitempty"`
	ParentSession string        `json:"parentSession,omitempty"`
	Message       *AgentMessage `json:"message,omitempty"`
	Summary       string        `json:"summary,omitempty"`
	FromID        string        `json:"fromId,omitempty"`
}

// SessionInfo summarizes a saved session.
type SessionInfo struct {
	ID           string `json:"id"`
	Timestamp    string `json:"timestamp"`
	CWD          string `json:"cwd"`
	MessageCount int    `json:"messageCount"`
	Preview      string `json:"preview"`
}

// SessionManager manages session persistence.
type SessionManager struct {
	sessionsDir string
	mu          sync.Mutex
}

// NewSessionManager creates a session manager.
func NewSessionManager(sessionsDir string) *SessionManager {
	return &SessionManager{
		sessionsDir: sessionsDir,
	}
}

// Save persists a session to disk.
func (sm *SessionManager) Save(sessionID string, header SessionHeader, messages []AgentMessage) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err := os.MkdirAll(sm.sessionsDir, 0755); err != nil {
		return fmt.Errorf("cannot create sessions dir: %w", err)
	}

	entries := make([]SessionEntry, 0, len(messages)+1)

	// Header
	header.Type = "session"
	header.Version = 3
	entries = append(entries, SessionEntry{
		Type:          "session",
		Version:       header.Version,
		ID:            sessionID,
		Timestamp:     header.Timestamp,
		CWD:           header.CWD,
		ParentSession: header.ParentSession,
	})

	// Messages
	for _, msg := range messages {
		entry := SessionEntry{
			Type:      "message",
			ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
			Timestamp: fmt.Sprintf("%d", msg.Timestamp),
			Message:   &msg,
		}
		if entry.Timestamp == "0" || entry.Timestamp == "" {
			entry.Timestamp = time.Now().Format(time.RFC3339)
		}
		entries = append(entries, entry)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal session: %w", err)
	}

	filename := filepath.Join(sm.sessionsDir, sessionID+".json")
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("cannot write session file: %w", err)
	}

	return nil
}

// Load loads a session from disk.
func (sm *SessionManager) Load(sessionID string) (SessionHeader, []AgentMessage, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	filename := filepath.Join(sm.sessionsDir, sessionID+".json")
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return SessionHeader{}, nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return SessionHeader{}, nil, fmt.Errorf("cannot read session: %w", err)
	}

	var entries []SessionEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return SessionHeader{}, nil, fmt.Errorf("cannot parse session: %w", err)
	}

	var header SessionHeader
	var messages []AgentMessage

	for _, entry := range entries {
		switch entry.Type {
		case "session":
			header = SessionHeader{
				Type:          entry.Type,
				Version:       entry.Version,
				ID:            entry.ID,
				Timestamp:     entry.Timestamp,
				CWD:           entry.CWD,
				ParentSession: entry.ParentSession,
			}
		case "message":
			if entry.Message != nil {
				messages = append(messages, *entry.Message)
			}
		}
	}

	return header, messages, nil
}

// List returns all saved sessions.
func (sm *SessionManager) List() ([]SessionInfo, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err := os.MkdirAll(sm.sessionsDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create sessions dir: %w", err)
	}

	entries, err := os.ReadDir(sm.sessionsDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read sessions dir: %w", err)
	}

	var sessions []SessionInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		sessionID := strings.TrimSuffix(entry.Name(), ".json")

		// Quick-read header
		data, err := os.ReadFile(filepath.Join(sm.sessionsDir, entry.Name()))
		if err != nil {
			continue
		}

		var entries []SessionEntry
		if err := json.Unmarshal(data, &entries); err != nil {
			continue
		}

		var info SessionInfo
		info.ID = sessionID
		info.MessageCount = 0

		for _, e := range entries {
			switch e.Type {
			case "session":
				info.Timestamp = e.Timestamp
				info.CWD = e.CWD
			case "message":
				info.MessageCount++
				if info.Preview == "" && e.Message != nil && e.Message.Content != "" {
					preview := e.Message.Content
					if len(preview) > 80 {
						preview = preview[:80] + "..."
					}
					info.Preview = preview
				}
			}
		}

		sessions = append(sessions, info)
	}

	// Sort by timestamp descending
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Timestamp > sessions[j].Timestamp
	})

	return sessions, nil
}

// Delete removes a session.
func (sm *SessionManager) Delete(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	filename := filepath.Join(sm.sessionsDir, sessionID+".json")
	if err := os.Remove(filename); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot delete session: %w", err)
	}
	return nil
}

// PruneOldSessions removes sessions older than the given duration.
func (sm *SessionManager) PruneOldSessions(maxAge time.Duration) (int, error) {
	sessions, err := sm.List()
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, s := range sessions {
		t, err := time.Parse(time.RFC3339, s.Timestamp)
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			if err := sm.Delete(s.ID); err == nil {
				removed++
			}
		}
	}

	return removed, nil
}

// ForkSession creates a new session branching from an existing one.
func (sm *SessionManager) ForkSession(parentID, newID string, newMessages []AgentMessage) error {
	header, messages, err := sm.Load(parentID)
	if err != nil {
		return fmt.Errorf("cannot load parent session: %w", err)
	}

	header.ID = newID
	header.ParentSession = parentID
	header.Timestamp = time.Now().Format(time.RFC3339)

	allMessages := append(messages, newMessages...)

	return sm.Save(newID, header, allMessages)
}

// CurrentSessionID generates a new session ID.
func CurrentSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}
