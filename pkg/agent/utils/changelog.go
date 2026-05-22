package agentutils

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ChangelogEntry represents a single changelog entry.
type ChangelogEntry struct {
	Version string       `json:"version"`
	Date    string       `json:"date"`
	Changes []ChangeItem `json:"changes"`
}

// ChangeItem represents a single change in a changelog.
type ChangeItem struct {
	Type    string `json:"type"` // "added", "changed", "fixed", "removed", "deprecated", "security"
	Message string `json:"message"`
}

// ParseChangelog parses a markdown changelog into structured entries.
func ParseChangelog(content string) []ChangelogEntry {
	var entries []ChangelogEntry
	re := regexp.MustCompile(`##\s+\[?(\d+\.\d+\.\d+[^\]]*)\]?\s*(?:-?\s*(\d{4}-\d{2}-\d{2}))?`)
	lines := strings.Split(content, "\n")
	var current *ChangelogEntry
	var currentType string

	for _, line := range lines {
		if m := re.FindStringSubmatch(line); m != nil {
			if current != nil {
				entries = append(entries, *current)
			}
			current = &ChangelogEntry{
				Version: strings.TrimSpace(m[1]),
				Date:    strings.TrimSpace(m[2]),
			}
			currentType = ""
			continue
		}
		if current == nil {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "### ") {
			currentType = strings.ToLower(strings.TrimPrefix(trimmed, "### "))
			continue
		}
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			msg := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(trimmed, "- "), "* "))
			current.Changes = append(current.Changes, ChangeItem{
				Type:    currentType,
				Message: msg,
			})
		}
	}
	if current != nil {
		entries = append(entries, *current)
	}
	return entries
}

// FormatChangelog formats changelog entries as a markdown string.
func FormatChangelog(entries []ChangelogEntry, maxEntries int) string {
	if maxEntries <= 0 || maxEntries > len(entries) {
		maxEntries = len(entries)
	}
	var b strings.Builder
	for i, entry := range entries[:maxEntries] {
		if i > 0 {
			b.WriteString("\n")
		}
		dateStr := ""
		if entry.Date != "" {
			dateStr = " - " + entry.Date
		}
		b.WriteString(fmt.Sprintf("## %s%s\n", entry.Version, dateStr))
		typeChanges := make(map[string][]string)
		var types []string
		for _, ch := range entry.Changes {
			t := ch.Type
			if t == "" {
				t = "other"
			}
			if _, ok := typeChanges[t]; !ok {
				types = append(types, t)
			}
			typeChanges[t] = append(typeChanges[t], ch.Message)
		}
		for _, t := range types {
			title := strings.Title(t)
			b.WriteString(fmt.Sprintf("### %s\n", title))
			for _, msg := range typeChanges[t] {
				b.WriteString(fmt.Sprintf("- %s\n", msg))
			}
		}
	}
	return b.String()
}

// LatestVersion extracts the latest version string from changelog entries.
func LatestVersion(entries []ChangelogEntry) string {
	if len(entries) == 0 {
		return ""
	}
	return entries[0].Version
}

// CompareVersions compares two semver strings. Returns -1 if v1 < v2, 0 if equal, 1 if v1 > v2.
func CompareVersions(v1, v2 string) int {
	re := regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)
	m1 := re.FindStringSubmatch(v1)
	m2 := re.FindStringSubmatch(v2)
	if m1 == nil && m2 == nil {
		return 0
	}
	if m1 == nil {
		return -1
	}
	if m2 == nil {
		return 1
	}
	for i := 1; i <= 3; i++ {
		n1, n2 := 0, 0
		fmt.Sscanf(m1[i], "%d", &n1)
		fmt.Sscanf(m2[i], "%d", &n2)
		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
	}
	return 0
}

// SortChangelogEntries sorts entries by version descending.
func SortChangelogEntries(entries []ChangelogEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return CompareVersions(entries[i].Version, entries[j].Version) > 0
	})
}
