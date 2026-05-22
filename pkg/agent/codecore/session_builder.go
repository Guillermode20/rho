package codecore

import (
	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/agent/tools"
	"github.com/earendil-works/rho/pkg/ai"
)

type SessionBuilder struct{}

func NewSessionBuilder() *SessionBuilder { return &SessionBuilder{} }

type CreateAgentSessionOptions struct {
	Model        ai.Model
	SystemPrompt string
	APIKey       string
	CWD          string
}

type CreateAgentSessionResult struct {
	SessionID string
	Agent     *agent.AgentLoop
	Context   agent.AgentContext
}

func (s *SessionBuilder) CreateAgentSession(opts CreateAgentSessionOptions) (*CreateAgentSessionResult, error) {
	loop := agent.NewAgentLoop(agent.AgentLoopConfig{
		Model:        opts.Model,
		SystemPrompt: opts.SystemPrompt,
		APIKey:       opts.APIKey,
	})
	allTools := tools.AllTools(opts.CWD)
	context := agent.AgentContext{
		SystemPrompt: opts.SystemPrompt,
		Model:        opts.Model,
		Tools:        allTools,
	}
	return &CreateAgentSessionResult{
		SessionID: agent.CurrentSessionID(),
		Agent:     loop,
		Context:   context,
	}, nil
}

func (s *SessionBuilder) RunSession(opts CreateAgentSessionOptions, prompt string) ([]agent.AgentMessage, error) {
	result, err := s.CreateAgentSession(opts)
	if err != nil {
		return nil, err
	}
	prompts := []agent.AgentMessage{{Role: ai.RoleUser, Content: prompt}}
	return result.Agent.Run(prompts, result.Context, func(event agent.AgentEvent) error { return nil })
}
