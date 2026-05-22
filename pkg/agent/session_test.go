package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSessionManagerLoadContextFromLeafRecursive(t *testing.T) {
	// Create a temp sessions directory
	tmpDir, err := os.MkdirTemp("", "rho-test-sessions-*")
	if err != nil {
		t.Fatalf("failed to create temp sessions dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	sm := NewSessionManager(tmpDir)

	// Create Session A (root)
	// msg-a1 (root) -> msg-a2 (leaf)
	sessionAEntries := []SessionEntry{
		{
			Type:    "session",
			Version: 1,
			ID:      "session-a",
			FromID:  "msg-a2", // LeafID in header
		},
		{
			Type:     "message",
			ID:       "msg-a1",
			ParentID: "",
			Message: &AgentMessage{
				Content: "Hello from A1",
			},
		},
		{
			Type:     "message",
			ID:       "msg-a2",
			ParentID: "msg-a1",
			Message: &AgentMessage{
				Content: "Hello from A2",
			},
		},
	}
	writeSessionFile(t, tmpDir, "session-a", sessionAEntries)

	// Create Session B (branched from Session A at msg-a2)
	// msg-a2 -> msg-b1 -> msg-b2 (leaf)
	sessionBEntries := []SessionEntry{
		{
			Type:          "session",
			Version:       1,
			ID:            "session-b",
			ParentSession: "session-a",
			FromID:        "msg-b2",
		},
		{
			Type:     "message",
			ID:       "msg-b1",
			ParentID: "msg-a2",
			Message: &AgentMessage{
				Content: "Hello from B1",
			},
		},
		{
			Type:     "message",
			ID:       "msg-b2",
			ParentID: "msg-b1",
			Message: &AgentMessage{
				Content: "Hello from B2",
			},
		},
	}
	writeSessionFile(t, tmpDir, "session-b", sessionBEntries)

	// Create Session C (branched from Session B at msg-b2)
	// msg-b2 -> msg-c1 -> msg-c2 (leaf)
	sessionCEntries := []SessionEntry{
		{
			Type:          "session",
			Version:       1,
			ID:            "session-c",
			ParentSession: "session-b",
			FromID:        "msg-c2",
		},
		{
			Type:     "message",
			ID:       "msg-c1",
			ParentID: "msg-b2",
			Message: &AgentMessage{
				Content: "Hello from C1",
			},
		},
		{
			Type:     "message",
			ID:       "msg-c2",
			ParentID: "msg-c1",
			Message: &AgentMessage{
				Content: "Hello from C2",
			},
		},
	}
	writeSessionFile(t, tmpDir, "session-c", sessionCEntries)

	// Load context from Session C, starting from leaf "msg-c2"
	header, messages, err := sm.LoadContextFromLeaf("session-c", "msg-c2")
	if err != nil {
		t.Fatalf("LoadContextFromLeaf failed: %v", err)
	}

	if header.ID != "session-c" {
		t.Errorf("expected session-c header, got %q", header.ID)
	}

	expectedContents := []string{
		"Hello from A1",
		"Hello from A2",
		"Hello from B1",
		"Hello from B2",
		"Hello from C1",
		"Hello from C2",
	}

	if len(messages) != len(expectedContents) {
		t.Fatalf("expected %d messages, got %d", len(expectedContents), len(messages))
	}

	for i, msg := range messages {
		if msg.Content != expectedContents[i] {
			t.Errorf("messages[%d]: expected content %q, got %q", i, expectedContents[i], msg.Content)
		}
	}
}

func writeSessionFile(t *testing.T, dir, sessionID string, entries []SessionEntry) {
	filename := filepath.Join(dir, sessionID+".json")
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("failed to marshal entries: %v", err)
	}
	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}
}
