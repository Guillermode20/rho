package codecore

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/ai"
)

type AgentSessionConfig struct {
	Model         ai.Model         `json:"model"`
	SystemPrompt  string           `json:"systemPrompt,omitempty"`
	APIKey        string           `json:"apiKey,omitempty"`
	CWD           string           `json:"cwd,omitempty"`
	ThinkingLevel ai.ThinkingLevel `json:"thinkingLevel,omitempty"`
}

type SessionStats struct {
	TotalTurns    int           `json:"totalTurns"`
	TotalTokens   int           `json:"totalTokens"`
	TotalCost     float64       `json:"totalCost"`
	StartTime     time.Time     `json:"startTime"`
	Duration      time.Duration `json:"duration"`
	ToolCallCount int           `json:"toolCallCount"`
}

type AgentSession struct {
	mu       sync.RWMutex
	config   AgentSessionConfig
	messages []agent.AgentMessage
	agent    *agent.AgentLoop
	context  agent.AgentContext
	started  bool
	stopped  bool
	stats    SessionStats
}

func NewAgentSession(config AgentSessionConfig) *AgentSession {
	loop := agent.NewAgentLoop(agent.AgentLoopConfig{
		Model:        config.Model,
		SystemPrompt: config.SystemPrompt,
		APIKey:       config.APIKey,
	})
	return &AgentSession{
		config: config,
		agent:  loop,
		stats:  SessionStats{StartTime: time.Now()},
	}
}

func (s *AgentSession) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return fmt.Errorf("session already started")
	}
	s.started = true
	return nil
}

func (s *AgentSession) SendMessage(content string) (*agent.AgentMessage, error) {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil, fmt.Errorf("session stopped")
	}
	s.mu.Unlock()

	prompt := agent.AgentMessage{Role: ai.RoleUser, Content: content, Timestamp: time.Now().UnixMilli()}
	context := agent.AgentContext{SystemPrompt: s.config.SystemPrompt, Model: s.config.Model}
	var response *agent.AgentMessage

	results, err := s.agent.Run([]agent.AgentMessage{prompt}, context, func(agent.AgentEvent) error { return nil })
	if err != nil {
		return nil, err
	}
	for i := len(results) - 1; i >= 0; i-- {
		if results[i].Role == ai.RoleAssistant {
			r := results[i]
			response = &r
			break
		}
	}
	s.mu.Lock()
	s.messages = append(s.messages, prompt)
	if response != nil {
		s.messages = append(s.messages, *response)
	}
	s.stats.TotalTurns++
	s.stats.TotalTokens += len(response.Content) / 2
	s.mu.Unlock()
	return response, nil
}

func (s *AgentSession) GetMessages() []agent.AgentMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r := make([]agent.AgentMessage, len(s.messages))
	copy(r, s.messages)
	return r
}

func (s *AgentSession) GetStats() SessionStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

func (s *AgentSession) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	s.stats.Duration = time.Since(s.stats.StartTime)
}

func (s *AgentSession) Save(path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data := map[string]interface{}{
		"config": s.config, "messages": s.messages, "stats": s.stats,
		"savedAt": time.Now().Format(time.RFC3339),
	}
	d, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, d, 0644)
}

func LoadSession(path string) (*AgentSession, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Config   AgentSessionConfig   `json:"config"`
		Messages []agent.AgentMessage `json:"messages"`
		Stats    SessionStats         `json:"stats"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	s := NewAgentSession(raw.Config)
	s.messages = raw.Messages
	s.stats = raw.Stats
	return s, nil
}
