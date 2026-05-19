package extensions

import (
	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/ai"
)

// Wrapper wraps an agent loop with extension hooks.
type Wrapper struct {
	runtime *Runtime
	ctx     ExtensionContext
}

// NewWrapper creates a new extension wrapper.
func NewWrapper(runtime *Runtime, ctx ExtensionContext) *Wrapper {
	return &Wrapper{
		runtime: runtime,
		ctx:     ctx,
	}
}

// WrapBeforeToolCall returns a before-tool-call hook that fires extension events.
func (w *Wrapper) WrapBeforeToolCall() func(ctx agent.BeforeToolCallContext) (*agent.BeforeToolCallResult, error) {
	return func(ctx agent.BeforeToolCallContext) (*agent.BeforeToolCallResult, error) {
		event := ToolCallEvent{
			ToolCallID: ctx.ToolCall.ID,
			ToolName:   ctx.ToolCall.Name,
			Input:      ctx.ToolCall.Arguments,
		}
		result, err := w.runtime.FireToolCall(w.ctx, event)
		if err != nil {
			return nil, err
		}
		if result != nil && result.Block {
			return &agent.BeforeToolCallResult{
				Block:  true,
				Reason: result.Reason,
			}, nil
		}
		return nil, nil
	}
}

// WrapAfterToolCall returns an after-tool-call hook that fires extension events.
func (w *Wrapper) WrapAfterToolCall() func(ctx agent.AfterToolCallContext) (*agent.AfterToolCallResult, error) {
	return func(ctx agent.AfterToolCallContext) (*agent.AfterToolCallResult, error) {
		event := ToolResultEvent{
			ToolCallID: ctx.ToolCall.ID,
			ToolName:   ctx.ToolCall.Name,
			Input:      ctx.ToolCall.Arguments,
			Content:    ctx.Result.Content,
			IsError:    ctx.Result.IsError,
		}
		if err := w.runtime.FireToolResult(w.ctx, event); err != nil {
			return nil, err
		}
		return nil, nil
	}
}

// ApplyToAgentLoop applies extension hooks to an agent loop.
func ApplyToAgentLoop(runtime *Runtime, extCtx ExtensionContext, loop *agent.AgentLoop) {
	wrapper := NewWrapper(runtime, extCtx)
	loop.SetBeforeTool(wrapper.WrapBeforeToolCall())
	loop.SetAfterTool(wrapper.WrapAfterToolCall())
}

// InstallHooks sets up session lifecycle hooks.
func InstallHooks(runtime *Runtime, extCtx ExtensionContext, agentLoop *agent.AgentLoop) {
	ApplyToAgentLoop(runtime, extCtx, agentLoop)
}

// MergeTools merges extension custom tools with built-in tools.
// Extension tools take precedence by name.
func MergeTools(builtin []agent.AgentTool, extensionTools []agent.AgentTool) []agent.AgentTool {
	byName := make(map[string]agent.AgentTool)
	for _, t := range builtin {
		byName[t.Name] = t
	}
	for _, t := range extensionTools {
		byName[t.Name] = t
	}
	result := make([]agent.AgentTool, 0, len(byName))
	for _, t := range byName {
		result = append(result, t)
	}
	return result
}

// MergeAllProviders merges built-in and extension custom providers.
func MergeAllProviders(builtin []ai.Model, customProviders []ProviderConfig) []ai.Model {
	seen := make(map[string]bool)
	var merged []ai.Model
	for _, m := range builtin {
		key := string(m.Provider) + "/" + m.Name
		if !seen[key] {
			merged = append(merged, m)
			seen[key] = true
		}
	}
	for _, cp := range customProviders {
		for _, m := range cp.Models {
			key := string(m.Provider) + "/" + m.Name
			if !seen[key] {
				merged = append(merged, m)
				seen[key] = true
			}
		}
	}
	return merged
}
