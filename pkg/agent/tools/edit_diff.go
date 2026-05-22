package tools

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/earendil-works/rho/pkg/agent"
)

// ============================================================================
// EditDiff tool — patch-based file editing with fuzzy matching
// ============================================================================
//
// Ported from pi's edit-diff.ts. Supports:
//   - Multiple simultaneous edits in one call
//   - Fuzzy matching (Unicode normalization, trailing whitespace stripping)
//   - Overlap detection
//   - Diff output with line numbers
//   - Line ending preservation
// ============================================================================

// EditDiffParams is the JSON schema for the EditDiff tool.
var editDiffParams = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "Path to the file to edit (relative or absolute)",
		},
		"edits": map[string]interface{}{
			"type":        "array",
			"description": "One or more targeted replacements. Each edit is matched against the original file, not incrementally. Do not include overlapping or nested edits.",
			"items": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"oldText": map[string]interface{}{
						"type":        "string",
						"description": "Exact text for one targeted replacement. It must be unique in the original file and must not overlap with any other edits[].oldText in the same call.",
					},
					"newText": map[string]interface{}{
						"type":        "string",
						"description": "Replacement text for this targeted edit.",
					},
				},
				"required": []interface{}{"oldText", "newText"},
				"additionalProperties": false,
			},
		},
	},
	"required": []interface{}{"path", "edits"},
}

// NewEditDiffTool creates an EditDiff tool with diff-based editing and fuzzy matching.
func NewEditDiffTool(cwd string) agent.AgentTool {
	return agent.AgentTool{
		Name:        "EditDiff",
		Description: "Edit a file using diff-based replacements. Supports multiple edits in one call. Old text is matched with fuzzy normalization (trailing whitespace, Unicode quotes/dashes). Returns a diff of changes.",
		Parameters:  editDiffParams,
		Execute: func(args map[string]interface{}) (string, bool, error) {
			pathStr, _ := args["path"].(string)
			if pathStr == "" {
				return "", true, fmt.Errorf("path is required")
			}

			editsRaw, ok := args["edits"].([]interface{})
			if !ok || len(editsRaw) == 0 {
				return "", true, fmt.Errorf("at least one edit is required")
			}

			absPath := resolvePath(pathStr, cwd)

			// Read the file
			rawContent, err := os.ReadFile(absPath)
			if err != nil {
				return "", true, fmt.Errorf("cannot read file %s: %w", pathStr, err)
			}

			content := string(rawContent)

			// Strip BOM
			bom, text := stripBOM(content)
			_ = bom // preserve BOM for write-back

			// Detect and normalize line endings
			lineEnding := detectLineEnding(text)
			normalizedContent := normalizeToLF(text)

			// Parse edits
			edits := make([]editDef, 0, len(editsRaw))
			for i, raw := range editsRaw {
				em, ok := raw.(map[string]interface{})
				if !ok {
					return "", true, fmt.Errorf("edits[%d]: invalid edit object", i)
				}
				oldText, _ := em["oldText"].(string)
				newText, _ := em["newText"].(string)
				if oldText == "" {
					return "", true, fmt.Errorf("edits[%d].oldText must not be empty", i)
				}
				edits = append(edits, editDef{
					oldText: normalizeToLF(oldText),
					newText: normalizeToLF(newText),
				})
			}

			// Apply edits to normalized content
			result, err := applyEditsToNormalizedContent(normalizedContent, edits, pathStr)
			if err != nil {
				return "", true, err
			}

			// Restore line endings
			newContent := restoreLineEndings(result.newContent, lineEnding)

			// Write the file
			finalContent := bom + newContent
			if err := os.WriteFile(absPath, []byte(finalContent), 0644); err != nil {
				return "", true, fmt.Errorf("cannot write file %s: %w", pathStr, err)
			}

			// Generate diff string
			diffResult := generateDiffString(result.baseContent, result.newContent, 4)

			output := fmt.Sprintf("Applied %d edit(s) to %s\n\n%s", len(edits), pathStr, diffResult.diff)
			if diffResult.firstChangedLine != nil {
				output += fmt.Sprintf("\nFirst changed line: %d", *diffResult.firstChangedLine)
			}

			return output, false, nil
		},
	}
}

// ============================================================================
// Fuzzy matching — normalize Unicode for approximate matching
// ============================================================================

// normalizeForFuzzyMatch normalizes text for fuzzy matching by applying
// progressive transformations: strip trailing whitespace per line,
// normalize smart quotes/hyphens/spaces to ASCII equivalents.
func normalizeForFuzzyMatch(text string) string {
	// Normalize to NFKC
	text = nfkcNormalize(text)

	// Strip trailing whitespace per line
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	text = strings.Join(lines, "\n")

	// Smart single quotes → '
	text = strings.NewReplacer(
		"\u2018", "'", "\u2019", "'", "\u201A", "'", "\u201B", "'",
		"\u201C", "\"", "\u201D", "\"", "\u201E", "\"", "\u201F", "\"",
		"\u2010", "-", "\u2011", "-", "\u2012", "-",
		"\u2013", "-", "\u2014", "-", "\u2015", "-", "\u2212", "-",
		"\u00A0", " ",
	).Replace(text)

	// Special spaces → regular space
	var b strings.Builder
	for _, r := range text {
		if isSpecialSpace(r) {
			b.WriteRune(' ')
		} else {
			b.WriteRune(r)
		}
	}

	return b.String()
}

// nfkcNormalize performs basic NFKC normalization (canonical decomposition + compatibility).
func nfkcNormalize(text string) string {
	// Simple NFKC-like normalization for common Unicode characters.
	// This handles the most common cases from pi's normalize("NFKC").
	var b strings.Builder
	for _, r := range text {
		switch {
		case r >= 0x2000 && r <= 0x206F: // General Punctuation block
			b.WriteRune(normalizePunct(r))
		case r >= 0x2100 && r <= 0x214F: // Letterlike Symbols
			b.WriteRune(normalizeLetterlike(r))
		case r >= 0x2150 && r <= 0x218F: // Number Forms
			b.WriteRune(r) // pass through
		case r >= 0x2190 && r <= 0x21FF: // Arrows
			b.WriteRune(r) // pass through
		case r >= 0x2200 && r <= 0x22FF: // Mathematical Operators
			b.WriteRune(r) // pass through
		case r >= 0x2460 && r <= 0x24FF: // Enclosed Alphanumerics
			b.WriteRune(r) // pass through
		case r >= 0x2500 && r <= 0x257F: // Box Drawing
			b.WriteRune(r) // pass through
		case r >= 0x2580 && r <= 0x259F: // Block Elements
			b.WriteRune(r) // pass through
		case r >= 0x25A0 && r <= 0x25FF: // Geometric Shapes
			b.WriteRune(r) // pass through
		case r >= 0x2600 && r <= 0x26FF: // Miscellaneous Symbols
			b.WriteRune(r) // pass through
		case r >= 0x2700 && r <= 0x27BF: // Dingbats
			b.WriteRune(r) // pass through
		case r >= 0x3000 && r <= 0x303F: // CJK Symbols
			if r == 0x3000 {
				b.WriteRune(' ') // ideographic space → space
			} else {
				b.WriteRune(r)
			}
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func normalizePunct(r rune) rune {
	switch r {
	case 0x2010, 0x2011, 0x2012, 0x2013, 0x2014, 0x2015:
		return '-' // all dashes/hyphens
	case 0x2018, 0x2019, 0x201A, 0x201B:
		return '\'' // smart single quotes
	case 0x201C, 0x201D, 0x201E, 0x201F:
		return '"' // smart double quotes
	case 0x2026:
		return '.' // ellipsis (partial)
	case 0x202F:
		return ' ' // narrow no-break space
	case 0x205F:
		return ' ' // medium mathematical space
	default:
		return r
	}
}

func normalizeLetterlike(r rune) rune {
	switch r {
	case 0x2122:
		return 'T' // TM → T
	case 0x2126:
		return 'O' // Ohm → O
	default:
		return r
	}
}

func isSpecialSpace(r rune) bool {
	return (r >= 0x2002 && r <= 0x200A) || r == 0x202F || r == 0x205F || r == 0x3000 || r == 0x00A0
}

// ============================================================================
// Line ending handling
// ============================================================================

func detectLineEnding(content string) string {
	if strings.Contains(content, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

func normalizeToLF(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text
}

func restoreLineEndings(text string, ending string) string {
	if ending == "\r\n" {
		return strings.ReplaceAll(text, "\n", "\r\n")
	}
	return text
}

// ============================================================================
// BOM handling
// ============================================================================

func stripBOM(content string) (bom string, text string) {
	if len(content) >= 3 && content[0] == 0xEF && content[1] == 0xBB && content[2] == 0xBF {
		return content[:3], content[3:]
	}
	return "", content
}

// ============================================================================
// Edit application
// ============================================================================

type editDef struct {
	oldText string
	newText string
}

type matchedEdit struct {
	editIndex   int
	matchIndex  int
	matchLength int
	newText     string
}

type appliedEditsResult struct {
	baseContent string
	newContent  string
}

func fuzzyMapRune(r rune) (rune, bool) {
	switch r {
	case 0x2018, 0x2019, 0x201A, 0x201B:
		return '\'', true
	case 0x201C, 0x201D, 0x201E, 0x201F:
		return '"', true
	case 0x2010, 0x2011, 0x2012, 0x2013, 0x2014, 0x2015, 0x2212:
		return '-', true
	case 0x2026:
		return '.', true // ellipsis
	case 0x202F, 0x205F, 0x3000, 0x00A0:
		return ' ', true
	case 0x2122:
		return 'T', true
	case 0x2126:
		return 'O', true
	}
	if r >= 0x2002 && r <= 0x200A {
		return ' ', true
	}
	return r, false
}

func normalizeForFuzzyWithMapping(text string) (string, []int) {
	var normalized strings.Builder
	mapping := make([]int, 0, len(text)+1)

	lines := strings.Split(text, "\n")
	lineStartOffset := 0

	for idx, line := range lines {
		trimmedLine := strings.TrimRight(line, " \t")

		for byteOffsetInLine, r := range trimmedLine {
			mappedRune, _ := fuzzyMapRune(r)
			normalized.WriteRune(mappedRune)
			
			runeBuf := [utf8.UTFMax]byte{}
			n := utf8.EncodeRune(runeBuf[:], mappedRune)
			
			origByteOffset := lineStartOffset + byteOffsetInLine
			for i := 0; i < n; i++ {
				mapping = append(mapping, origByteOffset)
			}
		}

		if idx < len(lines)-1 {
			normalized.WriteByte('\n')
			mapping = append(mapping, lineStartOffset+len(line))
			lineStartOffset += len(line) + 1
		}
	}

	mapping = append(mapping, len(text))
	return normalized.String(), mapping
}

func applyEditsToNormalizedContent(normalizedContent string, edits []editDef, path string) (*appliedEditsResult, error) {
	// Validate: no empty oldText
	for i, e := range edits {
		if len(e.oldText) == 0 {
			return nil, fmt.Errorf("edits[%d].oldText must not be empty in %s", i, path)
		}
	}

	// Match edits against the base content (always normalizedContent)
	matchedEdits := make([]matchedEdit, 0, len(edits))
	for i, e := range edits {
		m := fuzzyFindText(normalizedContent, e.oldText)
		if !m.found {
			return nil, getNotFoundError(path, i, len(edits))
		}

		occurrences := countOccurrences(normalizedContent, e.oldText)
		if occurrences > 1 {
			return nil, getDuplicateError(path, i, len(edits), occurrences)
		}

		matchedEdits = append(matchedEdits, matchedEdit{
			editIndex:   i,
			matchIndex:  m.index,
			matchLength: m.matchLength,
			newText:     e.newText,
		})
	}

	// Sort by match index (ascending)
	sort.Slice(matchedEdits, func(i, j int) bool {
		return matchedEdits[i].matchIndex < matchedEdits[j].matchIndex
	})

	// Check for overlaps
	for i := 1; i < len(matchedEdits); i++ {
		prev := matchedEdits[i-1]
		curr := matchedEdits[i]
		if prev.matchIndex+prev.matchLength > curr.matchIndex {
			return nil, fmt.Errorf(
				"edits[%d] and edits[%d] overlap in %s. Merge them into one edit or target disjoint regions.",
				prev.editIndex, curr.editIndex, path,
			)
		}
	}

	// Apply replacements in reverse order (so offsets remain stable)
	newContent := normalizedContent
	for i := len(matchedEdits) - 1; i >= 0; i-- {
		edit := matchedEdits[i]
		newContent = newContent[:edit.matchIndex] + edit.newText + newContent[edit.matchIndex+edit.matchLength:]
	}

	if normalizedContent == newContent {
		return nil, getNoChangeError(path, len(edits))
	}

	return &appliedEditsResult{
		baseContent: normalizedContent,
		newContent:  newContent,
	}, nil
}

// fuzzyFindText finds oldText in content, trying exact match first, then fuzzy match.
type fuzzyMatchResult struct {
	found                 bool
	index                 int
	matchLength           int
	usedFuzzyMatch        bool
	contentForReplacement string
}

func fuzzyFindText(content string, oldText string) fuzzyMatchResult {
	// Try exact match first
	exactIndex := strings.Index(content, oldText)
	if exactIndex != -1 {
		return fuzzyMatchResult{
			found:                 true,
			index:                 exactIndex,
			matchLength:           len(oldText),
			usedFuzzyMatch:        false,
			contentForReplacement: content,
		}
	}

	// Try fuzzy match
	fuzzyContent, mapping := normalizeForFuzzyWithMapping(content)
	fuzzyOldText := normalizeForFuzzyMatch(oldText)
	fuzzyIndex := strings.Index(fuzzyContent, fuzzyOldText)

	if fuzzyIndex == -1 {
		return fuzzyMatchResult{
			found: false,
			index: -1,
		}
	}

	origStart := mapping[fuzzyIndex]
	origEnd := mapping[fuzzyIndex+len(fuzzyOldText)]

	return fuzzyMatchResult{
		found:                 true,
		index:                 origStart,
		matchLength:           origEnd - origStart,
		usedFuzzyMatch:        true,
		contentForReplacement: content,
	}
}

func countOccurrences(content string, oldText string) int {
	fuzzyContent := normalizeForFuzzyMatch(content)
	fuzzyOldText := normalizeForFuzzyMatch(oldText)
	return strings.Count(fuzzyContent, fuzzyOldText)
}

// ============================================================================
// Error messages (matching pi's error style)
// ============================================================================

func getNotFoundError(path string, editIndex int, totalEdits int) error {
	if totalEdits == 1 {
		return fmt.Errorf(
			"Could not find the exact text in %s. The old text must match exactly including all whitespace and newlines.",
			path,
		)
	}
	return fmt.Errorf(
		"Could not find edits[%d] in %s. The oldText must match exactly including all whitespace and newlines.",
		editIndex, path,
	)
}

func getDuplicateError(path string, editIndex int, totalEdits int, occurrences int) error {
	if totalEdits == 1 {
		return fmt.Errorf(
			"Found %d occurrences of the text in %s. The text must be unique. Please provide more context to make it unique.",
			occurrences, path,
		)
	}
	return fmt.Errorf(
		"Found %d occurrences of edits[%d] in %s. Each oldText must be unique. Please provide more context to make it unique.",
		occurrences, editIndex, path,
	)
}

func getNoChangeError(path string, totalEdits int) error {
	if totalEdits == 1 {
		return fmt.Errorf(
			"No changes made to %s. The replacement produced identical content. This might indicate an issue with special characters or the text not existing as expected.",
			path,
		)
	}
	return fmt.Errorf(
		"No changes made to %s. The replacements produced identical content.",
		path,
	)
}

// ============================================================================
// Diff generation
// ============================================================================

type diffResult struct {
	diff            string
	firstChangedLine *int
}

// generateDiffString produces a unified diff with line numbers and context lines.
func generateDiffString(oldContent, newContent string, contextLines int) diffResult {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// If both end with \n, remove the trailing empty element
	if len(oldLines) > 0 && oldLines[len(oldLines)-1] == "" {
		oldLines = oldLines[:len(oldLines)-1]
	}
	if len(newLines) > 0 && newLines[len(newLines)-1] == "" {
		newLines = newLines[:len(newLines)-1]
	}

	// Compute diff
	diff := computeLineDiff(oldLines, newLines)

	maxLineNum := len(oldLines)
	if len(newLines) > maxLineNum {
		maxLineNum = len(newLines)
	}
	lineNumWidth := 1
	for maxLineNum >= 10 {
		maxLineNum /= 10
		lineNumWidth++
	}
	maxLineNum = len(oldLines)
	if len(newLines) > maxLineNum {
		maxLineNum = len(newLines)
	}

	var buf bytes.Buffer
	oldLineNum := 1
	newLineNum := 1
	lastWasChange := false
	var firstChangedLine *int

	for i, part := range diff {
		if part.added || part.removed {
			// Capture first changed line
			if firstChangedLine == nil && part.added {
				l := newLineNum
				firstChangedLine = &l
			} else if firstChangedLine == nil && part.removed {
				l := newLineNum
				firstChangedLine = &l
			}

			for _, line := range part.lines {
				if part.added {
					ln := padNum(newLineNum, lineNumWidth)
					buf.WriteString(fmt.Sprintf("+%s %s\n", ln, line))
					newLineNum++
				} else {
					ln := padNum(oldLineNum, lineNumWidth)
					buf.WriteString(fmt.Sprintf("-%s %s\n", ln, line))
					oldLineNum++
				}
			}
			lastWasChange = true
		} else {
			// Context lines
			nextIsChange := i < len(diff)-1 && (diff[i+1].added || diff[i+1].removed)
			hasLeadingChange := lastWasChange
			hasTrailingChange := nextIsChange

			if hasLeadingChange && hasTrailingChange {
				if len(part.lines) <= contextLines*2 {
					for _, line := range part.lines {
						ln := padNum(oldLineNum, lineNumWidth)
						buf.WriteString(fmt.Sprintf(" %s %s\n", ln, line))
						oldLineNum++
						newLineNum++
					}
				} else {
					leading := part.lines[:contextLines]
					trailing := part.lines[len(part.lines)-contextLines:]
					skipped := len(part.lines) - len(leading) - len(trailing)

					for _, line := range leading {
						ln := padNum(oldLineNum, lineNumWidth)
						buf.WriteString(fmt.Sprintf(" %s %s\n", ln, line))
						oldLineNum++
						newLineNum++
					}
					buf.WriteString(fmt.Sprintf(" %s ...\n", padNum(0, lineNumWidth)))
					oldLineNum += skipped
					newLineNum += skipped
					for _, line := range trailing {
						ln := padNum(oldLineNum, lineNumWidth)
						buf.WriteString(fmt.Sprintf(" %s %s\n", ln, line))
						oldLineNum++
						newLineNum++
					}
				}
			} else if hasLeadingChange {
				shown := part.lines
				if len(shown) > contextLines {
					shown = shown[:contextLines]
				}
				skipped := len(part.lines) - len(shown)
				for _, line := range shown {
					ln := padNum(oldLineNum, lineNumWidth)
					buf.WriteString(fmt.Sprintf(" %s %s\n", ln, line))
					oldLineNum++
					newLineNum++
				}
				if skipped > 0 {
					buf.WriteString(fmt.Sprintf(" %s ...\n", padNum(0, lineNumWidth)))
					oldLineNum += skipped
					newLineNum += skipped
				}
			} else if hasTrailingChange {
				skipped := 0
				if len(part.lines) > contextLines {
					skipped = len(part.lines) - contextLines
				}
				if skipped > 0 {
					buf.WriteString(fmt.Sprintf(" %s ...\n", padNum(0, lineNumWidth)))
					oldLineNum += skipped
					newLineNum += skipped
				}
				for _, line := range part.lines[skipped:] {
					ln := padNum(oldLineNum, lineNumWidth)
					buf.WriteString(fmt.Sprintf(" %s %s\n", ln, line))
					oldLineNum++
					newLineNum++
				}
			} else {
				oldLineNum += len(part.lines)
				newLineNum += len(part.lines)
			}
			lastWasChange = false
		}
	}

	return diffResult{
		diff:            strings.TrimRight(buf.String(), "\n"),
		firstChangedLine: firstChangedLine,
	}
}

type diffPart struct {
	added   bool
	removed bool
	lines   []string
}

func computeLineDiff(oldLines, newLines []string) []diffPart {
	// Simple LCS-based diff
	m := len(oldLines)
	n := len(newLines)

	// Build LCS table
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				if dp[i-1][j] > dp[i][j-1] {
					dp[i][j] = dp[i-1][j]
				} else {
					dp[i][j] = dp[i][j-1]
				}
			}
		}
	}

	// Backtrack to build diff
	var parts []diffPart
	i, j := m, n
	var currentPart *diffPart

	flushPart := func() {
		if currentPart != nil && len(currentPart.lines) > 0 {
			// Merge with previous part if same type
			if len(parts) > 0 {
				last := parts[len(parts)-1]
				if last.added == currentPart.added && last.removed == currentPart.removed {
					parts[len(parts)-1].lines = append(parts[len(parts)-1].lines, currentPart.lines...)
					currentPart = nil
					return
				}
			}
			parts = append(parts, *currentPart)
			currentPart = nil
		}
	}

	addPart := func(added, removed bool, line string) {
		if currentPart != nil && (currentPart.added != added || currentPart.removed != removed) {
			flushPart()
		}
		if currentPart == nil {
			currentPart = &diffPart{added: added, removed: removed}
		}
		currentPart.lines = append(currentPart.lines, line)
	}

	// Collect operations in reverse
	type op struct {
		added, removed bool
		line           string
	}
	var ops []op

	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			ops = append(ops, op{added: false, removed: false, line: oldLines[i-1]})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			ops = append(ops, op{added: true, removed: false, line: newLines[j-1]})
			j--
		} else if i > 0 {
			ops = append(ops, op{added: false, removed: true, line: oldLines[i-1]})
			i--
		}
	}

	// Reverse to get correct order
	for k := len(ops) - 1; k >= 0; k-- {
		op := ops[k]
		if !op.added && !op.removed {
			addPart(false, false, op.line)
		} else if op.added && !op.removed {
			addPart(true, false, op.line)
		} else if !op.added && op.removed {
			addPart(false, true, op.line)
		}
	}
	flushPart()

	return parts
}

func padNum(num int, width int) string {
	if num == 0 {
		return strings.Repeat(" ", width)
	}
	s := fmt.Sprintf("%d", num)
	for len(s) < width {
		s = " " + s
	}
	return s
}

// ============================================================================
// ComputeEditsDiff — preview only (no write)
// ============================================================================

// ComputeEditsDiff computes the diff for one or more edit operations without applying them.
// Used for preview rendering in the TUI.
func ComputeEditsDiff(path, cwd string, edits []editDef) (*diffResult, error) {
	absPath := resolvePath(path, cwd)

	rawContent, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("could not edit file: %s. Cannot read: %w", path, err)
	}

	content := string(rawContent)
	_, text := stripBOM(content)
	normalizedContent := normalizeToLF(text)

	// Normalize edit oldText
	normalizedEdits := make([]editDef, len(edits))
	for i, e := range edits {
		normalizedEdits[i] = editDef{
			oldText: normalizeToLF(e.oldText),
			newText: normalizeToLF(e.newText),
		}
	}

	result, err := applyEditsToNormalizedContent(normalizedContent, normalizedEdits, path)
	if err != nil {
		return nil, err
	}

	diffRes := generateDiffString(result.baseContent, result.newContent, 4)
	return &diffRes, nil
}


