package codecore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PromptTemplate represents a reusable prompt template.
type PromptTemplate struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Template    string `json:"template"`
	Category    string `json:"category,omitempty"`
}

// PromptTemplateStore manages loading and formatting prompt templates.
type PromptTemplateStore struct {
	templates map[string]*PromptTemplate
	dirs      []string
}

// NewPromptTemplateStore creates a new template store.
func NewPromptTemplateStore(dirs []string) *PromptTemplateStore {
	store := &PromptTemplateStore{
		templates: make(map[string]*PromptTemplate),
		dirs:      dirs,
	}
	store.loadBuiltins()
	for _, dir := range dirs {
		store.LoadFromDir(dir)
	}
	return store
}

func (s *PromptTemplateStore) loadBuiltins() {
	builtins := []PromptTemplate{
		{
			Name:        "summarize",
			Description: "Summarize the conversation context for compaction",
			Category:    "system",
			Template: `Please summarize the following conversation between a user and an AI assistant.
Focus on the key facts, decisions, and any code changes made.
Be concise but thorough.

{{.Content}}`,
		},
		{
			Name:        "compact",
			Description: "Compaction system prompt for continuing after summarization",
			Category:    "system",
			Template: `The conversation history above has been summarized to stay within context limits.
The summary captures key facts, decisions, and changes made.
Continue the conversation naturally, referring to the summary when needed.

Previous summary: {{.Summary}}`,
		},
		{
			Name:        "branch-summary",
			Description: "Summarize a session branch for tree navigation",
			Category:    "system",
			Template: `Summarize the following conversation branch.
Capture the key decisions, changes, and outcomes.

Branch entries:
{{.Content}}`,
		},
		{
			Name:        "plan",
			Description: "Planning mode template",
			Category:    "user",
			Template: `You are in planning mode. Before making any changes, create a detailed plan.
Consider:
1. What needs to be done
2. Which files need to be modified
3. Potential risks and edge cases
4. Testing strategy

Then wait for approval before executing.`,
		},
		{
			Name:        "code-review",
			Description: "Code review prompt template",
			Category:    "user",
			Template: `Please review the following code changes:

{{.Content}}

Focus on:
- Correctness
- Performance
- Security
- Best practices
- Potential bugs`,
		},
		{
			Name:        "explain",
			Description: "Explain code or concepts",
			Category:    "user",
			Template: `Please explain the following in simple terms:

{{.Content}}

Assume I'm familiar with basic programming concepts but not an expert in this specific area.`,
		},
	}

	for _, t := range builtins {
		s.templates[t.Name] = &t
	}
}

// LoadFromDir loads templates from a directory of template files.
// Files should be named <name>.tpl and contain the template text.
// A companion <name>.json can provide metadata.
func (s *PromptTemplateStore) LoadFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot read template dir %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tpl") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".tpl")

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		tpl := &PromptTemplate{
			Name:     name,
			Template: string(data),
		}

		// Try to load metadata from companion JSON file
		metaPath := filepath.Join(dir, name+".json")
		if metaData, err := os.ReadFile(metaPath); err == nil {
			var meta struct {
				Description string `json:"description"`
				Category    string `json:"category"`
			}
			if err := json.Unmarshal(metaData, &meta); err == nil {
				tpl.Description = meta.Description
				tpl.Category = meta.Category
			}
		}

		s.templates[name] = tpl
	}

	return nil
}

// Get returns a template by name.
func (s *PromptTemplateStore) Get(name string) (*PromptTemplate, bool) {
	t, ok := s.templates[name]
	return t, ok
}

// List returns all templates sorted by name.
func (s *PromptTemplateStore) List() []*PromptTemplate {
	var result []*PromptTemplate
	for _, t := range s.templates {
		result = append(result, t)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// ListByCategory returns templates in a category.
func (s *PromptTemplateStore) ListByCategory(category string) []*PromptTemplate {
	var result []*PromptTemplate
	for _, t := range s.templates {
		if t.Category == category {
			result = append(result, t)
		}
	}
	return result
}

// FormatTemplate renders a template with variable substitution.
// Variables are passed as a map. The template uses {{.Key}} syntax.
func FormatTemplate(template *PromptTemplate, vars map[string]string) string {
	result := template.Template
	for key, val := range vars {
		result = strings.ReplaceAll(result, "{{."+key+"}}", val)
	}
	// Remove any unresolved variables
	result = removeUnresolved(result)
	return result
}

// FormatTemplateString renders a template string with variables.
func FormatTemplateString(tpl string, vars map[string]string) string {
	result := tpl
	for key, val := range vars {
		result = strings.ReplaceAll(result, "{{."+key+"}}", val)
	}
	result = removeUnresolved(result)
	return result
}

func removeUnresolved(s string) string {
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '{' && i+1 < len(s) && s[i+1] == '{' {
			end := strings.Index(s[i:], "}}")
			if end >= 0 {
				i += end + 1
				continue
			}
		}
		result.WriteByte(s[i])
	}
	return result.String()
}


