package agentutils

import (
	"path/filepath"
	"strings"
)

// ChromaStyle represents a chroma syntax highlighting style.
type ChromaStyle string

const (
	StyleMonokai  ChromaStyle = "monokai"
	StyleGitHub   ChromaStyle = "github"
	StyleDracula  ChromaStyle = "dracula"
	StyleCatppuccin ChromaStyle = "catppuccin"
	StyleNative   ChromaStyle = "native"
	StyleFriendly ChromaStyle = "friendly"
)

// languageMap maps file extensions to chroma language names.
var languageMap = map[string]string{
	".go":     "go",
	".py":     "python",
	".js":     "javascript",
	".jsx":    "jsx",
	".ts":     "typescript",
	".tsx":    "tsx",
	".rs":     "rust",
	".rb":     "ruby",
	".java":   "java",
	".c":      "c",
	".h":      "c",
	".cpp":    "cpp",
	".hpp":    "cpp",
	".cs":     "csharp",
	".swift":  "swift",
	".kt":     "kotlin",
	".scala":  "scala",
	".php":    "php",
	".sh":     "bash",
	".bash":   "bash",
	".zsh":    "bash",
	".fish":   "fish",
	".ps1":    "powershell",
	".sql":    "sql",
	".html":   "html",
	".css":    "css",
	".scss":   "scss",
	".less":   "less",
	".json":   "json",
	".xml":    "xml",
	".yaml":   "yaml",
	".yml":    "yaml",
	".md":     "markdown",
	".txt":    "text",
	".toml":   "toml",
	".ini":    "ini",
	".cfg":    "ini",
	".dockerfile": "dockerfile",
	".makefile": "makefile",
	".cmake":  "cmake",
	".lua":    "lua",
	".pl":     "perl",
	".r":      "r",
	".m":      "objectivec",
	".mm":     "objectivec",
	".hs":     "haskell",
	".clj":    "clojure",
	".elm":    "elm",
	".erl":    "erlang",
	".ex":     "elixir",
	".exs":    "elixir",
	".vue":    "vue",
	".svelte": "svelte",
	".astro":  "astro",
	".zig":    "zig",
	".nim":    "nim",
	".dart":   "dart",
}

// GetLanguageFromPath returns the programming language name from a file path.
func GetLanguageFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if lang, ok := languageMap[ext]; ok {
		return lang
	}
	// Check by base name
	base := strings.ToLower(filepath.Base(path))
	switch base {
	case "makefile":
		return "makefile"
	case "dockerfile":
		return "dockerfile"
	case "gemfile":
		return "ruby"
	case "procfile":
		return "text"
	}
	return ""
}

// HighlightCode performs syntax highlighting on code.
// Returns the code with ANSI escape sequences for coloring.
// Uses simple heuristic-based highlighting as a fallback when chroma is not available.
func HighlightCode(code, language string, style ChromaStyle) string {
	if language == "" {
		return code
	}
	return highlightWithHeuristics(code, language)
}

// highlightWithHeuristics provides basic syntax highlighting using ANSI codes.
func highlightWithHeuristics(code, language string) string {
	// Simple keyword-based highlighting
	reset := "\x1b[0m"
	keyword := "\x1b[1;34m"   // bold blue
	str := "\x1b[32m"         // green
	comment := "\x1b[2;37m"   // dim white
	number := "\x1b[35m"      // magenta
	builtin := "\x1b[33m"     // yellow
	punctuation := "\x1b[37m" // white

	keywords := getKeywords(language)
	builtins := getBuiltins(language)

	lines := strings.Split(code, "\n")
	var highlighted []string

	for _, line := range lines {
		var out strings.Builder
		i := 0
		runes := []rune(line)
		for i < len(runes) {
			// String literals
			if runes[i] == '"' || runes[i] == '\'' || runes[i] == '`' {
				quote := runes[i]
				out.WriteString(str)
				out.WriteRune(runes[i])
				i++
				for i < len(runes) && runes[i] != quote {
					if runes[i] == '\\' && i+1 < len(runes) {
						out.WriteRune(runes[i])
						i++
					}
					out.WriteRune(runes[i])
					i++
				}
				if i < len(runes) {
					out.WriteRune(runes[i])
					i++
				}
				out.WriteString(reset)
				continue
			}

			// Comments
			if strings.HasPrefix(line[i:], "//") || strings.HasPrefix(line[i:], "#") ||
				strings.HasPrefix(line[i:], "--") {
				out.WriteString(comment)
				out.WriteString(line[i:])
				out.WriteString(reset)
				break
			}

			// Numbers
			if runes[i] >= '0' && runes[i] <= '9' {
				out.WriteString(number)
				for i < len(runes) && (runes[i] >= '0' && runes[i] <= '9' || runes[i] == '.' || runes[i] == 'x' || runes[i] >= 'a' && runes[i] <= 'f') {
					out.WriteRune(runes[i])
					i++
				}
				out.WriteString(reset)
				continue
			}

			// Identifiers
			if (runes[i] >= 'a' && runes[i] <= 'z') || (runes[i] >= 'A' && runes[i] <= 'Z') || runes[i] == '_' {
				var word strings.Builder
				start := i
				for i < len(runes) && ((runes[i] >= 'a' && runes[i] <= 'z') || (runes[i] >= 'A' && runes[i] <= 'Z') || (runes[i] >= '0' && runes[i] <= '9') || runes[i] == '_') {
					word.WriteRune(runes[i])
					i++
				}
				w := word.String()
				if keywords[w] {
					out.WriteString(keyword)
					out.WriteString(w)
					out.WriteString(reset)
				} else if builtins[w] {
					out.WriteString(builtin)
					out.WriteString(w)
					out.WriteString(reset)
				} else {
					out.WriteString(w)
				}
				_ = start
				continue
			}

			// Punctuation and operators
			if strings.ContainsRune("{}[]();,.+-*/=&|!<>?:", runes[i]) {
				out.WriteString(punctuation)
				out.WriteRune(runes[i])
				out.WriteString(reset)
				i++
				continue
			}

			out.WriteRune(runes[i])
			i++
		}
		highlighted = append(highlighted, out.String())
	}

	return strings.Join(highlighted, "\n")
}

func getKeywords(language string) map[string]bool {
	common := map[string]bool{
		"if": true, "else": true, "for": true, "while": true, "return": true,
		"break": true, "continue": true, "switch": true, "case": true, "default": true,
		"try": true, "catch": true, "finally": true, "throw": true,
		"import": true, "export": true, "from": true, "as": true,
		"class": true, "function": true, "def": true, "var": true,
		"let": true, "const": true, "new": true, "this": true, "super": true,
		"extends": true, "implements": true, "interface": true, "type": true,
		"enum": true, "package": true, "namespace": true, "using": true,
		"public": true, "private": true, "protected": true, "static": true,
		"void": true, "null": true, "true": true, "false": true, "nil": true,
		"in": true, "of": true, "async": true, "await": true, "yield": true,
		"with": true, "where": true, "select": true,
	}
	switch language {
	case "go":
		common["defer"] = true
		common["go"] = true
		common["chan"] = true
		common["range"] = true
		common["map"] = true
		common["struct"] = true
		common["func"] = true
		common["interface"] = true
	case "python":
		common["elif"] = true
		common["except"] = true
		common["finally"] = true
		common["with"] = true
		common["lambda"] = true
		common["pass"] = true
		common["raise"] = true
		common["yield"] = true
	}
	return common
}

func getBuiltins(language string) map[string]bool {
	common := map[string]bool{
		"print": true, "len": true, "range": true, "type": true,
		"string": true, "int": true, "float": true, "bool": true,
		"byte": true, "rune": true, "error": true,
	}
	switch language {
	case "go":
		common["make"] = true
		common["append"] = true
		common["copy"] = true
		common["delete"] = true
		common["close"] = true
		common["panic"] = true
		common["recover"] = true
	case "python":
		common["range"] = true
		common["enumerate"] = true
		common["zip"] = true
		common["map"] = true
		common["filter"] = true
		common["sorted"] = true
		common["reversed"] = true
		common["any"] = true
		common["all"] = true
		common["sum"] = true
		common["min"] = true
		common["max"] = true
		common["abs"] = true
		common["isinstance"] = true
		common["hasattr"] = true
		common["getattr"] = true
		common["setattr"] = true
		common["open"] = true
		common["super"] = true
		common["property"] = true
		common["staticmethod"] = true
		common["classmethod"] = true
	}
	return common
}
