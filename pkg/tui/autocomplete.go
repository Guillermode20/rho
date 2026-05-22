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


