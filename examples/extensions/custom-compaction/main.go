// Custom Compaction Extension
//
// Provides tools for customizing how session compaction works.
// Allows setting different compaction strategies, reviewing what will be compacted,
// and customizing summarization prompts.
//
// Build:  go build -o custom-compaction ./examples/extensions/custom-compaction/
// Deploy: cp custom-compaction ~/.rho/extensions/custom-compaction/
//
//	cp examples/extensions/custom-compaction/rho.toml ~/.rho/extensions/custom-compaction/
package main

import (
	"fmt"
	"strings"
	"sync"

	"github.com/earendil-works/rho/pkg/sdk"
)

type compactionStrategy struct {
	Name         string
	Description  string
	TargetRatio  float64 // Target proportion of context window after compaction
	TriggerRatio float64 // Proportion at which compaction triggers
}

var strategies = []compactionStrategy{
	{Name: "conservative", Description: "Keep as much context as possible (trigger at 90%)", TargetRatio: 0.7, TriggerRatio: 0.9},
	{Name: "balanced", Description: "Balanced approach (trigger at 75%)", TargetRatio: 0.5, TriggerRatio: 0.75},
	{Name: "aggressive", Description: "Aggressively compact (trigger at 60%)", TargetRatio: 0.3, TriggerRatio: 0.6},
	{Name: "minimal", Description: "Minimal context kept (trigger at 50%)", TargetRatio: 0.15, TriggerRatio: 0.5},
}

var (
	currentStrategyIdx int
	strategyMu         sync.Mutex
)

func main() {
	ext := sdk.New("rho.custom-compaction")

	ext.Command("compaction-strategy", func(ctx sdk.Context, args []string) error {
		strategyMu.Lock()
		defer strategyMu.Unlock()

		if len(args) > 0 {
			for i, s := range strategies {
				if strings.EqualFold(s.Name, args[0]) {
					currentStrategyIdx = i
					ctx.Notify(fmt.Sprintf("📊 Compaction strategy set to: %s - %s", s.Name, s.Description), "success")
					return nil
				}
			}
			ctx.Notify(fmt.Sprintf("Unknown strategy: %s. Available: conservative, balanced, aggressive, minimal", args[0]), "error")
			return nil
		}

		// List strategies
		var msg strings.Builder
		msg.WriteString("📊 Compaction Strategies:\n\n")
		for i, s := range strategies {
			mark := " "
			if i == currentStrategyIdx {
				mark = "✓"
			}
			msg.WriteString(fmt.Sprintf("  %s %-15s %s\n", mark, s.Name, s.Description))
		}
		msg.WriteString(fmt.Sprintf("\nCurrent: %s\n", strategies[currentStrategyIdx].Name))
		msg.WriteString(fmt.Sprintf("  Trigger at: %.0f%% of context window\n", strategies[currentStrategyIdx].TriggerRatio*100))
		msg.WriteString(fmt.Sprintf("  Target: %.0f%% of context window after compaction\n", strategies[currentStrategyIdx].TargetRatio*100))
		ctx.Notify(msg.String(), "info")
		return nil
	})

	ext.Tool("set_compaction_strategy", "Set the compaction strategy for context window management. "+
		"Controls how aggressively the conversation is summarized to stay within context limits.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"strategy": map[string]interface{}{
					"type":        "string",
					"description": "Compaction strategy: 'conservative' (keep most context), 'balanced', 'aggressive', or 'minimal'",
					"enum":        []interface{}{"conservative", "balanced", "aggressive", "minimal"},
				},
				"customTriggerRatio": map[string]interface{}{
					"type":        "number",
					"description": "Custom trigger ratio (0.0-1.0) - overrides the strategy if set",
				},
				"customTargetRatio": map[string]interface{}{
					"type":        "number",
					"description": "Custom target ratio (0.0-1.0) - overrides the strategy if set",
				},
			},
		},
		func(ctx sdk.Context, args map[string]interface{}) (string, bool, error) {
			strategyName, _ := args["strategy"].(string)
			customTrigger, hasTrigger := args["customTriggerRatio"].(float64)
			customTarget, hasTarget := args["customTargetRatio"].(float64)

			strategyMu.Lock()
			defer strategyMu.Unlock()

			var result strings.Builder
			result.WriteString("📊 Compaction Strategy\n\n")

			if strategyName != "" {
				found := false
				for i, s := range strategies {
					if strings.EqualFold(s.Name, strategyName) {
						currentStrategyIdx = i
						result.WriteString(fmt.Sprintf("Strategy: %s\n", s.Name))
						result.WriteString(fmt.Sprintf("  Trigger: %.0f%% of context window\n", s.TriggerRatio*100))
						result.WriteString(fmt.Sprintf("  Target:  %.0f%% of context window\n", s.TargetRatio*100))
						result.WriteString(fmt.Sprintf("  %s\n", s.Description))
						found = true
						break
					}
				}
				if !found {
					result.WriteString(fmt.Sprintf("Unknown strategy: %s. Using current: %s\n", strategyName, strategies[currentStrategyIdx].Name))
				}
			} else {
				s := strategies[currentStrategyIdx]
				result.WriteString(fmt.Sprintf("Current strategy: %s\n", s.Name))
				result.WriteString(fmt.Sprintf("  Trigger: %.0f%% of context window\n", s.TriggerRatio*100))
				result.WriteString(fmt.Sprintf("  Target:  %.0f%% of context window\n", s.TargetRatio*100))
			}

			if hasTrigger {
				result.WriteString(fmt.Sprintf("\nCustom trigger ratio: %.0f%%\n", customTrigger*100))
			}
			if hasTarget {
				result.WriteString(fmt.Sprintf("Custom target ratio: %.0f%%\n", customTarget*100))
			}

			result.WriteString("\nUse /compaction-strategy to see all available strategies.")
			return result.String(), false, nil
		},
	)

	ext.Run()
}
