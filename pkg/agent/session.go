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
	LeafID        string `json:"leafId,omitempty"`
	ParentLeafID  string `json:"parentLeafId,omitempty"`
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
	ID            string `json:"id"`
	Name          string `json:"name,omitempty"`
	ParentSession string `json:"parentSession,omitempty"`
	Timestamp     string `json:"timestamp"`
	CWD           string `json:"cwd"`
	MessageCount  int    `json:"messageCount"`
	Preview       string `json:"preview"`
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
// Messages are linked in a chain via ParentID, and the last message ID is stored as LeafID
// in the session header for tree-based context reconstruction.
func (sm *SessionManager) Save(sessionID string, header SessionHeader, messages []AgentMessage) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err := os.MkdirAll(sm.sessionsDir, 0755); err != nil {
		return fmt.Errorf("cannot create sessions dir: %w", err)
	}

	entries := make([]SessionEntry, 0, len(messages)+1)

	// Header
	header.Type = "session"
	header.Version = 4 // bumped for ParentID/LeafID support
	header.LeafID = ""
	entries = append(entries, SessionEntry{
		Type:          "session",
		Version:       header.Version,
		ID:            sessionID,
		Timestamp:     header.Timestamp,
		CWD:           header.CWD,
		ParentSession: header.ParentSession,
	})

	// Messages — build a parentId chain
	// If header.ParentLeafID is set (fork), the first message's parent is the parent session's leaf
	prevID := header.ParentLeafID
	msgCounter := 0
	for i, msg := range messages {
		msgCounter++
		entryID := fmt.Sprintf("msg_%d_%d", time.Now().UnixMilli(), msgCounter)
		ts := fmt.Sprintf("%d", msg.Timestamp)
		if ts == "0" || ts == "" {
			ts = time.Now().Format(time.RFC3339)
		}

		entry := SessionEntry{
			Type:      "message",
			ID:        entryID,
			ParentID:  prevID,
			Timestamp: ts,
			Message:   &msg,
		}
		entries = append(entries, entry)
		prevID = entryID

		// Track leaf — last message that was just appended
		if i == len(messages)-1 {
			header.LeafID = entryID
			// Update the header entry with the leaf ID
			entries[0].FromID = entryID // store leaf reference in header's FromID
		}
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

// Load loads a session from disk, returning messages in file order.
// For tree-based context reconstruction, use LoadContextFromLeaf instead.
func (sm *SessionManager) Load(sessionID string) (SessionHeader, []AgentMessage, error) {
	return sm.loadSession(sessionID)
}

// loadSession is the internal implementation shared by Load and LoadContextFromLeaf.
func (sm *SessionManager) loadSession(sessionID string) (SessionHeader, []AgentMessage, error) {
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
				LeafID:        entry.FromID, // LeafID stored in FromID field
			}
		case "message":
			if entry.Message != nil {
				messages = append(messages, *entry.Message)
			}
		}
	}

	return header, messages, nil
}

// LoadContextFromLeaf reconstructs the conversation context by walking the
// parentId chain backwards from a given leaf entry ID to the root, then
// reversing to produce messages in chronological order.
// If leafID is empty, returns messages in file order (same as Load).
func (sm *SessionManager) LoadContextFromLeaf(sessionID, leafID string) (SessionHeader, []AgentMessage, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	visited := make(map[string]bool)
	return sm.loadContextFromLeafLocked(sessionID, leafID, visited)
}

func (sm *SessionManager) loadContextFromLeafLocked(sessionID, leafID string, visited map[string]bool) (SessionHeader, []AgentMessage, error) {
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

	// Build lookup maps
	entryByID := make(map[string]SessionEntry)
	var header SessionHeader
	for _, entry := range entries {
		entryByID[entry.ID] = entry
		if entry.Type == "session" {
			header = SessionHeader{
				Type:          entry.Type,
				Version:       entry.Version,
				ID:            entry.ID,
				Timestamp:     entry.Timestamp,
				CWD:           entry.CWD,
				ParentSession: entry.ParentSession,
				LeafID:        entry.FromID,
			}
		}
	}

	if leafID == "" {
		// No leaf specified — return all messages in file order
		var messages []AgentMessage
		for _, entry := range entries {
			if entry.Type == "message" && entry.Message != nil {
				messages = append(messages, *entry.Message)
			}
		}
		if header.ParentSession != "" {
			parentHeader, parentMessages, err := sm.loadContextFromLeafLocked(header.ParentSession, header.LeafID, visited)
			if err == nil {
				_ = parentHeader
				allMessages := make([]AgentMessage, 0, len(parentMessages)+len(messages))
				allMessages = append(allMessages, parentMessages...)
				allMessages = append(allMessages, messages...)
				messages = allMessages
			}
		}
		return header, messages, nil
	}

	// Walk backwards from leafID through parentId chain
	type chainNode struct {
		entryID string
		msg     AgentMessage
	}
	var chain []chainNode
	currentID := leafID

	for currentID != "" {
		e, ok := entryByID[currentID]
		if !ok {
			break
		}
		if visited[currentID] {
			return header, nil, fmt.Errorf("circular parent reference detected at entry %s in session %s", currentID, sessionID)
		}
		visited[currentID] = true
		if e.Type == "message" && e.Message != nil {
			chain = append(chain, chainNode{entryID: e.ID, msg: *e.Message})
		}
		currentID = e.ParentID
	}

	// Reverse the chain to get chronological order (root -> leaf)
	var messages []AgentMessage
	if len(chain) > 0 {
		messages = make([]AgentMessage, len(chain))
		for i, node := range chain {
			messages[len(chain)-1-i] = node.msg
		}
	}

	if currentID != "" && header.ParentSession != "" {
		_, parentMessages, err := sm.loadContextFromLeafLocked(header.ParentSession, currentID, visited)
		if err == nil {
			allMessages := make([]AgentMessage, 0, len(parentMessages)+len(messages))
			allMessages = append(allMessages, parentMessages...)
			allMessages = append(allMessages, messages...)
			messages = allMessages
		}
	} else if len(chain) == 0 {
		// Fallback: no chain found, return all messages in file order
		for _, entry := range entries {
			if entry.Type == "message" && entry.Message != nil {
				messages = append(messages, *entry.Message)
			}
		}
		if header.ParentSession != "" {
			_, parentMessages, err := sm.loadContextFromLeafLocked(header.ParentSession, header.LeafID, visited)
			if err == nil {
				allMessages := make([]AgentMessage, 0, len(parentMessages)+len(messages))
				allMessages = append(allMessages, parentMessages...)
				allMessages = append(allMessages, messages...)
				messages = allMessages
			}
		}
	} else if header.ParentSession != "" {
		_, parentMessages, err := sm.loadContextFromLeafLocked(header.ParentSession, header.LeafID, visited)
		if err == nil {
			allMessages := make([]AgentMessage, 0, len(parentMessages)+len(messages))
			allMessages = append(allMessages, parentMessages...)
			allMessages = append(allMessages, messages...)
			messages = allMessages
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
				info.Name = e.FromID // FromID used as session name
				info.ParentSession = e.ParentSession
				// Check if there's a name stored in FromID
				if e.FromID != "" && !strings.HasPrefix(e.FromID, "session_") {
					info.Name = e.FromID
				}
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
// Unlike the old deep-copy approach, this stores only the new messages with
// ParentID pointing back to the fork point, and the header's ParentSession
// links to the parent session file. Context reconstruction via LoadContextFromLeaf
// walks the parentSession chain to include ancestor messages.
func (sm *SessionManager) ForkSession(parentID, newID string, newMessages []AgentMessage) error {
	header, _, err := sm.Load(parentID)
	if err != nil {
		return fmt.Errorf("cannot load parent session: %w", err)
	}

	header.ID = newID
	header.ParentSession = parentID
	header.Timestamp = time.Now().Format(time.RFC3339)

	// Set ParentLeafID to link the first new message's ParentID to the parent session's leaf
	// so the chain connects across session files for context reconstruction
	if len(newMessages) > 0 && header.LeafID != "" {
		header.ParentLeafID = header.LeafID
	}

	// Only store new messages (delta), not a deep copy of parent messages
	return sm.Save(newID, header, newMessages)
}

// CurrentSessionID generates a new session ID.
func CurrentSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}
