// Package compaction manages context window limits by summarizing old messages.
package compaction

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// TokenEstimator estimates token counts for text.
type TokenEstimator func(text string) int

// DefaultTokenEstimator estimates tokens at ~4 characters per token.
func DefaultTokenEstimator(text string) int {
	if text == "" {
		return 0
	}
	return len(text)/4 + 1
}

// CompactionSettings controls when and how compaction occurs.
type CompactionSettings struct {
	TargetRatio  float64 `json:"targetRatio"`
	TriggerRatio float64 `json:"triggerRatio"`
	MinTokens    int     `json:"minTokens"`
	MaxSummary   int     `json:"maxSummary"`
}

// DefaultCompactionSettings returns sensible defaults.
func DefaultCompactionSettings() CompactionSettings {
	return CompactionSettings{
		TargetRatio:  0.5,
		TriggerRatio: 0.75,
		MinTokens:    1024,
		MaxSummary:   512,
	}
}

// EstimateTokens estimates token count for a string.
func EstimateTokens(text string) int {
	return DefaultTokenEstimator(text)
}

// EstimateContextTokens estimates total tokens for a conversation.
func EstimateContextTokens(messages []Message) int {
	total := 0
	for _, m := range messages {
		total += EstimateTokens(m.Content)
		for _, tc := range m.ToolCalls {
			total += EstimateTokens(tc.Name)
		}
	}
	return total
}

// Message is a minimal message interface for compaction.
type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
	ToolName  string     `json:"toolName,omitempty"`
	Hide      bool       `json:"hide,omitempty"`
}

// ToolCall represents a tool call.
type ToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// CutPoint describes where to split for compaction.
type CutPoint struct {
	StartIndex   int `json:"startIndex"`
	EndIndex     int `json:"endIndex"`
	TokensBefore int `json:"tokensBefore"`
}

// FindCutPoint finds the best place to cut for compaction.
func FindCutPoint(messages []Message, contextWindow int, settings CompactionSettings) *CutPoint {
	if len(messages) == 0 {
		return nil
	}

	totalTokens := EstimateContextTokens(messages)
	triggerTokens := int(float64(contextWindow) * settings.TriggerRatio)

	if totalTokens < triggerTokens {
		return nil
	}

	targetTokens := int(float64(contextWindow) * settings.TargetRatio)
	runningTokens := 0
	cutIdx := len(messages)

	for i := len(messages) - 1; i >= 0; i-- {
		runningTokens += EstimateTokens(messages[i].Content)
		if runningTokens > targetTokens {
			cutIdx = i + 1
			break
		}
		cutIdx = i
	}

	tokensBefore := 0
	for i := 0; i < cutIdx; i++ {
		tokensBefore += EstimateTokens(messages[i].Content)
	}

	return &CutPoint{
		StartIndex:   0,
		EndIndex:     cutIdx,
		TokensBefore: tokensBefore,
	}
}

// ShouldCompact checks whether compaction is needed.
func ShouldCompact(messages []Message, contextWindow int, currentUsage int) bool {
	if contextWindow <= 0 {
		return false
	}
	ratio := float64(currentUsage) / float64(contextWindow)
	return ratio >= DefaultCompactionSettings().TriggerRatio
}

// CompactionResult describes the result of compaction.
type CompactionResult struct {
	Messages        []Message `json:"messages"`
	Summary         string    `json:"summary"`
	TokensBefore    int       `json:"tokensBefore"`
	TokensAfter     int       `json:"tokensAfter"`
	MessagesRemoved int       `json:"messagesRemoved"`
}

// SummaryFn generates a summary from text.
type SummaryFn func(text string) (string, error)

// Compact compacts messages by summarizing old ones.
func Compact(messages []Message, cutPoint *CutPoint, summaryFn SummaryFn) (*CompactionResult, error) {
	if cutPoint == nil {
		return nil, fmt.Errorf("no cut point specified")
	}

	var toSummarize []Message
	for i := cutPoint.StartIndex; i < cutPoint.EndIndex && i < len(messages); i++ {
		toSummarize = append(toSummarize, messages[i])
	}

	summaryText := formatForSummary(toSummarize)

	var summary string
	var err error
	if summaryFn != nil {
		summary, err = summaryFn(summaryText)
		if err != nil {
			summary = generateSimpleSummary(toSummarize)
		}
	} else {
		summary = generateSimpleSummary(toSummarize)
	}

	compacted := make([]Message, 0, 1+len(messages)-cutPoint.EndIndex+cutPoint.StartIndex)
	compacted = append(compacted, Message{
		Role:    "assistant",
		Content: fmt.Sprintf("[Previous conversation summarized: %s]", summary),
	})

	for i := cutPoint.EndIndex; i < len(messages); i++ {
		compacted = append(compacted, messages[i])
	}

	return &CompactionResult{
		Messages:        compacted,
		Summary:         summary,
		TokensBefore:    cutPoint.TokensBefore,
		TokensAfter:     EstimateContextTokens(compacted),
		MessagesRemoved: len(toSummarize),
	}, nil
}

// PrepareCompaction creates a preparation without executing.
func PrepareCompaction(messages []Message, contextWindow int) *CompactionPreparation {
	cutPoint := FindCutPoint(messages, contextWindow, DefaultCompactionSettings())
	if cutPoint == nil {
		return nil
	}
	return &CompactionPreparation{
		CutPoint:            cutPoint,
		MessageCount:        len(messages),
		TokensBefore:        cutPoint.TokensBefore,
		MessagesToSummarize: messages[cutPoint.StartIndex:cutPoint.EndIndex],
	}
}

// CompactionPreparation describes pending compaction.
type CompactionPreparation struct {
	CutPoint            *CutPoint `json:"cutPoint"`
	MessageCount        int       `json:"messageCount"`
	TokensBefore        int       `json:"tokensBefore"`
	TokensAfter         int       `json:"tokensAfter"`
	MessagesToSummarize []Message `json:"messagesToSummarize"`
}

func formatForSummary(messages []Message) string {
	var parts []string
	for _, m := range messages {
		switch m.Role {
		case "user":
			parts = append(parts, "User: "+m.Content)
		case "assistant":
			parts = append(parts, "Assistant: "+m.Content)
			for _, tc := range m.ToolCalls {
				parts = append(parts, fmt.Sprintf("  [Tool: %s]", tc.Name))
			}
		case "toolResult":
			c := m.Content
			if len(c) > 200 {
				c = c[:200] + "..."
			}
			parts = append(parts, fmt.Sprintf("  [%s result: %s]", m.ToolName, c))
		}
	}
	return strings.Join(parts, "\n")
}

func generateSimpleSummary(messages []Message) string {
	if len(messages) == 0 {
		return "Empty conversation"
	}
	userCount, assistantCount, toolCount := 0, 0, 0
	for _, m := range messages {
		switch m.Role {
		case "user":
			userCount++
		case "assistant":
			assistantCount++
		case "toolResult":
			toolCount++
		}
	}
	return fmt.Sprintf("%d user messages, %d assistant responses, %d tool calls",
		userCount, assistantCount, toolCount)
}

// CompactionEntry represents a stored compaction record.
type CompactionEntry struct {
	Type      string          `json:"type"`
	Summary   string          `json:"summary"`
	EntryIDs  []string        `json:"entryIds"`
	Tokens    int             `json:"tokens"`
	Timestamp int64           `json:"timestamp"`
	FromHook  bool            `json:"fromHook,omitempty"`
	Details   json.RawMessage `json:"details,omitempty"`
}

// Compacter manages compaction state.
type Compacter struct {
	mu        sync.Mutex
	estimator TokenEstimator
	settings  CompactionSettings
}

// NewCompacter creates a new compacter.
func NewCompacter(settings CompactionSettings) *Compacter {
	return &Compacter{
		estimator: DefaultTokenEstimator,
		settings:  settings,
	}
}

// SetEstimator sets a custom token estimator.
func (c *Compacter) SetEstimator(est TokenEstimator) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.estimator = est
}

// ShouldCompactWith checks if compaction is needed for the given messages.
func (c *Compacter) ShouldCompactWith(messages []Message, contextWindow int) bool {
	total := EstimateContextTokens(messages)
	threshold := int(float64(contextWindow) * c.settings.TriggerRatio)
	return total >= threshold
}
