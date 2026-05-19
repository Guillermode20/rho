package codecore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/earendil-works/rho/pkg/agent"
)

type SessionInfo struct {
	ID           string `json:"id"`
	Timestamp    string `json:"timestamp"`
	MessageCount int    `json:"messageCount"`
	Preview      string `json:"preview"`
}

type SessionManager struct {
	dir string
}

func NewSessionManager(dir string) *SessionManager { return &SessionManager{dir: dir} }

func (sm *SessionManager) List() ([]SessionInfo, error) {
	os.MkdirAll(sm.dir, 0755)
	entries, err := os.ReadDir(sm.dir)
	if err != nil {
		return nil, err
	}
	var sessions []SessionInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		info, err := sm.readInfo(id)
		if err != nil {
			continue
		}
		sessions = append(sessions, info)
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].Timestamp > sessions[j].Timestamp })
	return sessions, nil
}

func (sm *SessionManager) Load(id string) ([]agent.AgentMessage, error) {
	data, err := os.ReadFile(filepath.Join(sm.dir, id+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found: %s", id)
		}
		return nil, err
	}
	var msgs []agent.AgentMessage
	if err := json.Unmarshal(data, &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

func (sm *SessionManager) Save(id string, messages []agent.AgentMessage) error {
	os.MkdirAll(sm.dir, 0755)
	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(sm.dir, id+".json"), data, 0644)
}

func (sm *SessionManager) Delete(id string) error {
	if err := os.Remove(filepath.Join(sm.dir, id+".json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (sm *SessionManager) Fork(parentID, newID string, newMessages []agent.AgentMessage) error {
	msgs, err := sm.Load(parentID)
	if err != nil {
		return err
	}
	return sm.Save(newID, append(msgs, newMessages...))
}

func (sm *SessionManager) readInfo(id string) (SessionInfo, error) {
	msgs, err := sm.Load(id)
	if err != nil {
		return SessionInfo{}, err
	}
	info := SessionInfo{ID: id, MessageCount: len(msgs)}
	for _, m := range msgs {
		if m.Content != "" && info.Preview == "" {
			preview := m.Content
			if len(preview) > 80 {
				preview = preview[:80] + "..."
			}
			info.Preview = preview
		}
	}
	if len(msgs) > 0 {
		info.Timestamp = fmt.Sprintf("%d", msgs[0].Timestamp)
	}
	return info, nil
}
