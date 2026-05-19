package tui

import (
	"sort"
	"strings"
	"unicode"
)

// ============================================================================
// Fuzzy Matching
// ============================================================================

// FuzzyMatch represents a fuzzy search result.
type FuzzyMatch struct {
	Index int     `json:"index"`
	Score float64 `json:"score"`
	Value string  `json:"value"`
}

// FuzzyFilter filters and scores items by fuzzy match.
func FuzzyFilter(query string, items []string) []FuzzyMatch {
	if query == "" {
		var matches []FuzzyMatch
		for i, item := range items {
			matches = append(matches, FuzzyMatch{
				Index: i,
				Score: 0,
				Value: item,
			})
		}
		return matches
	}

	query = strings.ToLower(query)
	queryRunes := []rune(query)

	var matches []FuzzyMatch
	for i, item := range items {
		score := fuzzyScore(queryRunes, []rune(strings.ToLower(item)))
		if score > 0 {
			matches = append(matches, FuzzyMatch{
				Index: i,
				Score: score,
				Value: item,
			})
		}
	}

	// Sort by score descending
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].Value < matches[j].Value
	})

	return matches
}

// FuzzyMatchQuery checks if the query matches the target using fuzzy matching.
func FuzzyMatchQuery(query, target string) bool {
	return fuzzyScore([]rune(strings.ToLower(query)), []rune(strings.ToLower(target))) > 0
}

// fuzzyScore calculates a match score between query and target runes.
// Returns 0 if no match, positive score if matched.
func fuzzyScore(query, target []rune) float64 {
	if len(query) == 0 {
		return 0
	}
	if len(query) > len(target) {
		return 0
	}

	qi := 0
	score := 0.0
	consecutive := 0
	firstChar := true

	for ti := 0; ti < len(target) && qi < len(query); ti++ {
		if target[ti] == query[qi] {
			qi++
			consecutive++

			// Bonus for consecutive matches
			score += float64(consecutive) * 10

			// Bonus for matching at word boundaries
			if firstChar || ti == 0 || !unicode.IsLetter(target[ti-1]) {
				score += 20
			}

			// Bonus for matching after separator
			if ti > 0 && (target[ti-1] == '_' || target[ti-1] == '-' || target[ti-1] == '.') {
				score += 15
			}

			firstChar = false
		} else {
			consecutive = 0
		}
	}

	if qi < len(query) {
		return 0 // Not all query characters matched
	}

	return score
}

// ============================================================================
// Autocomplete
// ============================================================================

// AutocompleteItem represents a single autocomplete suggestion.
type AutocompleteItem struct {
	Value       string `json:"value"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

// AutocompleteProvider provides autocomplete suggestions.
type AutocompleteProvider interface {
	GetSuggestions(prefix string) []AutocompleteItem
}

// AutocompleteSuggestions is a list of suggestions with metadata.
type AutocompleteSuggestions struct {
	Items    []AutocompleteItem `json:"items"`
	Total    int                `json:"total"`
	HasMore  bool               `json:"hasMore"`
}

// StaticAutocompleteProvider provides suggestions from a static list.
type StaticAutocompleteProvider struct {
	items []AutocompleteItem
}

// NewStaticAutocompleteProvider creates a static autocomplete provider.
func NewStaticAutocompleteProvider(items []AutocompleteItem) *StaticAutocompleteProvider {
	return &StaticAutocompleteProvider{items: items}
}

func (p *StaticAutocompleteProvider) GetSuggestions(prefix string) []AutocompleteItem {
	if prefix == "" {
		return p.items
	}
	var filtered []AutocompleteItem
	lower := strings.ToLower(prefix)
	for _, item := range p.items {
		if strings.HasPrefix(strings.ToLower(item.Value), lower) {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) == 0 {
		// Fall back to fuzzy match
		values := make([]string, len(p.items))
		for i, item := range p.items {
			values[i] = item.Value
		}
		matches := FuzzyFilter(prefix, values)
		for _, m := range matches {
			filtered = append(filtered, p.items[m.Index])
		}
	}
	return filtered
}

// CombinedAutocompleteProvider combines multiple providers.
type CombinedAutocompleteProvider struct {
	providers []AutocompleteProvider
}

// NewCombinedAutocompleteProvider combines multiple providers.
func NewCombinedAutocompleteProvider(providers ...AutocompleteProvider) *CombinedAutocompleteProvider {
	return &CombinedAutocompleteProvider{providers: providers}
}

func (p *CombinedAutocompleteProvider) GetSuggestions(prefix string) []AutocompleteItem {
	var all []AutocompleteItem
	seen := make(map[string]bool)
	for _, provider := range p.providers {
		for _, item := range provider.GetSuggestions(prefix) {
			if !seen[item.Value] {
				all = append(all, item)
				seen[item.Value] = true
			}
		}
	}
	return all
}

// ============================================================================
// Slash Command Provider
// ============================================================================

// SlashCommand represents a slash command for autocomplete.
type SlashCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// SlashCommandProvider provides slash command autocomplete suggestions.
type SlashCommandProvider struct {
	commands []SlashCommand
}

// NewSlashCommandProvider creates a slash command provider.
func NewSlashCommandProvider(commands []SlashCommand) *SlashCommandProvider {
	return &SlashCommandProvider{commands: commands}
}

func (p *SlashCommandProvider) GetSuggestions(prefix string) []AutocompleteItem {
	// Only suggest when the line starts with /
	if !strings.HasPrefix(prefix, "/") {
		return nil
	}

	cmdPrefix := strings.TrimPrefix(prefix, "/")
	var items []AutocompleteItem
	for _, cmd := range p.commands {
		if strings.HasPrefix(cmd.Name, cmdPrefix) {
			items = append(items, AutocompleteItem{
				Value:       "/" + cmd.Name,
				Label:       "/" + cmd.Name,
				Description: cmd.Description,
				Icon:        ">",
			})
		}
	}
	return items
}
