// Package codecore provides the coding agent's core services: model management,
// keybindings, prompt templates, slash commands, telemetry, and footer data.
package codecore

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/earendil-works/rho/pkg/ai"
)

// ModelRegistry manages model lookup, registration, and metadata.
type ModelRegistry struct {
	mu      sync.RWMutex
	models  []ai.Model
	byName  map[string]ai.Model     // "provider/model" -> Model
	byID    map[string]ai.Model     // model name only -> Model (first match)
	byAPI   map[ai.API][]ai.Model   // API -> models
	byProv  map[ai.Provider][]ai.Model // Provider -> models
	authSt  ModelAuthProvider
}

// ModelAuthProvider resolves API keys for models.
type ModelAuthProvider interface {
	GetAPIKey(provider ai.Provider) (string, bool)
}

// NewModelRegistry creates a new ModelRegistry.
func NewModelRegistry() *ModelRegistry {
	r := &ModelRegistry{
		byName: make(map[string]ai.Model),
		byID:   make(map[string]ai.Model),
		byAPI:  make(map[ai.API][]ai.Model),
		byProv: make(map[ai.Provider][]ai.Model),
	}
	r.registerBuiltins()
	return r
}

// SetAuthProvider sets the auth provider for API key resolution.
func (r *ModelRegistry) SetAuthProvider(ap ModelAuthProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.authSt = ap
}

func (r *ModelRegistry) registerBuiltins() {
	for _, def := range ai.DefaultModels() {
		m := ai.Model{
			API:      def.API,
			Provider: def.Provider,
			Name:     def.Name,
			BaseURL:  def.BaseURL,
		}
		r.registerModel(m)
	}
}

func (r *ModelRegistry) registerModel(m ai.Model) {
	r.models = append(r.models, m)
	key := string(m.Provider) + "/" + m.Name
	r.byName[key] = m
	if _, exists := r.byID[m.Name]; !exists {
		r.byID[m.Name] = m
	}
	r.byAPI[m.API] = append(r.byAPI[m.API], m)
	r.byProv[m.Provider] = append(r.byProv[m.Provider], m)
}

// RegisterModel adds a model to the registry.
func (r *ModelRegistry) RegisterModel(m ai.Model) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registerModel(m)
}

// GetModel returns a model by provider/name combination ("anthropic/claude-sonnet-4-20250514").
func (r *ModelRegistry) GetModel(provider ai.Provider, name string) (ai.Model, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := string(provider) + "/" + name
	m, ok := r.byName[key]
	return m, ok
}

// GetModelByName returns the first model matching the name (ignoring provider).
func (r *ModelRegistry) GetModelByName(name string) (ai.Model, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.byID[name]
	return m, ok
}

// FindModel resolves a model string. Supports:
//   - "provider/model" format
//   - "model" only (returns first match)
//   - Partial name matching (e.g., "sonnet" → "claude-sonnet-4-20250514")
func (r *ModelRegistry) FindModel(s string) (ai.Model, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Exact "provider/model" match
	if strings.Contains(s, "/") {
		if m, ok := r.byName[s]; ok {
			return m, true
		}
	}

	// Exact model name match
	if m, ok := r.byID[s]; ok {
		return m, true
	}

	// Partial match
	s = strings.ToLower(s)
	for _, m := range r.models {
		if strings.Contains(strings.ToLower(m.Name), s) {
			return m, true
		}
		if strings.Contains(strings.ToLower(string(m.Provider)), s) {
			return m, true
		}
	}

	return ai.Model{}, false
}

// GetModels returns all registered models sorted by provider then name.
func (r *ModelRegistry) GetModels() []ai.Model {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ai.Model, len(r.models))
	copy(result, r.models)
	sort.Slice(result, func(i, j int) bool {
		if result[i].Provider != result[j].Provider {
			return string(result[i].Provider) < string(result[j].Provider)
		}
		return result[i].Name < result[j].Name
	})
	return result
}

// GetProviders returns all unique provider names.
func (r *ModelRegistry) GetProviders() []ai.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var provs []ai.Provider
	for p := range r.byProv {
		provs = append(provs, p)
	}
	sort.Slice(provs, func(i, j int) bool { return string(provs[i]) < string(provs[j]) })
	return provs
}

// GetModelsByProvider returns all models for a given provider.
func (r *ModelRegistry) GetModelsByProvider(provider ai.Provider) []ai.Model {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ai.Model, len(r.byProv[provider]))
	copy(result, r.byProv[provider])
	return result
}

// GetModelsByAPI returns all models for a given API type.
func (r *ModelRegistry) GetModelsByAPI(api ai.API) []ai.Model {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ai.Model, len(r.byAPI[api]))
	copy(result, r.byAPI[api])
	return result
}

// ModelMetadata returns additional metadata for a model.
type ModelMetadata struct {
	ContextWindow    int                  `json:"contextWindow"`
	MaxTokens        int                  `json:"maxTokens"`
	SupportsThinking bool                 `json:"supportsThinking"`
	ThinkingLevels   []ai.ThinkingLevel   `json:"thinkingLevels"`
	InputTypes       []string             `json:"inputTypes"`
	Cost             ai.CostPerMillion    `json:"cost"`
}

// GetModelMetadata returns metadata for a model by looking it up in the definition list.
func GetModelMetadata(m ai.Model) *ModelMetadata {
	for _, def := range ai.DefaultModels() {
		if def.Name == m.Name && def.Provider == m.Provider {
			levels := make([]ai.ThinkingLevel, 0)
			for _, l := range def.ThinkingLevels {
				levels = append(levels, ai.ThinkingLevel(l))
			}
			return &ModelMetadata{
				ContextWindow:    def.ContextWindow,
				MaxTokens:        def.MaxTokens,
				SupportsThinking: def.Reasoning,
				ThinkingLevels:   levels,
				InputTypes:       def.Input,
				Cost:             def.Cost,
			}
		}
	}
	return nil
}

// GetSupportedThinkingLevels returns which thinking levels a model supports.
func GetSupportedThinkingLevels(model ai.Model) []ai.ThinkingLevel {
	meta := GetModelMetadata(model)
	if meta == nil || !meta.SupportsThinking {
		return []ai.ThinkingLevel{}
	}
	if len(meta.ThinkingLevels) > 0 {
		return meta.ThinkingLevels
	}
	return []ai.ThinkingLevel{
		ai.ThinkingMinimal,
		ai.ThinkingLow,
		ai.ThinkingMedium,
		ai.ThinkingHigh,
	}
}

// ClampThinkingLevel clamps a requested thinking level to the nearest supported level.
func ClampThinkingLevel(model ai.Model, level ai.ThinkingLevel) ai.ThinkingLevel {
	levels := GetSupportedThinkingLevels(model)
	if len(levels) == 0 {
		return ""
	}
	for _, l := range levels {
		if l == level {
			return level
		}
	}
	// Fall back to highest supported level
	return levels[len(levels)-1]
}

// SearchModels searches models by query string.
func (r *ModelRegistry) SearchModels(query string) []ai.Model {
	r.mu.RLock()
	defer r.mu.RUnlock()
	q := strings.ToLower(query)
	parts := strings.Fields(q)

	var results []ai.Model
	for _, m := range r.models {
		match := true
		for _, part := range parts {
			if !strings.Contains(strings.ToLower(m.Name), part) &&
				!strings.Contains(strings.ToLower(string(m.Provider)), part) {
				match = false
				break
			}
		}
		if match {
			results = append(results, m)
		}
	}
	return results
}

// ModelCount returns the total number of registered models.
func (r *ModelRegistry) ModelCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.models)
}

// String returns a human-readable summary of the registry.
func (r *ModelRegistry) String() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	providers := make(map[ai.Provider]int)
	for _, m := range r.models {
		providers[m.Provider]++
	}
	var parts []string
	for p, c := range providers {
		parts = append(parts, fmt.Sprintf("%s:%d", p, c))
	}
	sort.Strings(parts)
	return fmt.Sprintf("ModelRegistry(%s)", strings.Join(parts, " "))
}
