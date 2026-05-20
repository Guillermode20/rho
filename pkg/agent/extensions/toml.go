package extensions

import (
	"strings"
)

// Manifest represents the structure of rho.toml
type Manifest struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	Version      string               `json:"version"`
	Description  string               `json:"description"`
	Entry        ManifestEntry        `json:"entry"`
	Capabilities ManifestCapabilities `json:"capabilities"`
	Skills       []ManifestSkill      `json:"skills"`
	Tools        []ManifestTool       `json:"tools"`
}

type ManifestEntry struct {
	Command string `json:"command"`
}

type ManifestCapabilities struct {
	Tools    bool `json:"tools"`
	Skills   bool `json:"skills"`
	Prompts  bool `json:"prompts"`
	Commands bool `json:"commands"`
}

type ManifestSkill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type ManifestTool struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

// ParseTOML parses a basic subset of TOML for rho.toml manifest.
func ParseTOML(content string) (*Manifest, error) {
	manifest := &Manifest{}
	lines := strings.Split(content, "\n")

	var currentSection string
	var currentArrayTable string
	var currentArrayMap map[string]interface{}

	flushCurrentArray := func() {
		if currentArrayTable != "" && currentArrayMap != nil {
			if currentArrayTable == "skills" {
				skill := ManifestSkill{
					ID:          getString(currentArrayMap["id"]),
					Name:        getString(currentArrayMap["name"]),
					Description: getString(currentArrayMap["description"]),
				}
				manifest.Skills = append(manifest.Skills, skill)
			} else if currentArrayTable == "tools" {
				tool := ManifestTool{
					ID:          getString(currentArrayMap["id"]),
					Description: getString(currentArrayMap["description"]),
				}
				manifest.Tools = append(manifest.Tools, tool)
			}
			currentArrayMap = nil
		}
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for array table headers: [[name]]
		if strings.HasPrefix(line, "[[") && strings.HasSuffix(line, "]]") {
			flushCurrentArray()
			currentArrayTable = strings.TrimSpace(line[2 : len(line)-2])
			currentSection = ""
			currentArrayMap = make(map[string]interface{})
			continue
		}

		// Check for table headers: [name]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flushCurrentArray()
			currentSection = strings.TrimSpace(line[1 : len(line)-1])
			currentArrayTable = ""
			continue
		}

		// Key-value pair
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		valStr := strings.TrimSpace(parts[1])

		// Strip inline comment if any
		if idx := strings.Index(valStr, "#"); idx >= 0 {
			beforeHash := valStr[:idx]
			if strings.Count(beforeHash, "\"")%2 == 0 {
				valStr = strings.TrimSpace(beforeHash)
			}
		}

		// Parse val
		var val interface{}
		if strings.HasPrefix(valStr, "\"") && strings.HasSuffix(valStr, "\"") {
			val = valStr[1 : len(valStr)-1]
		} else if valStr == "true" {
			val = true
		} else if valStr == "false" {
			val = false
		} else {
			val = valStr
		}

		if currentArrayTable != "" && currentArrayMap != nil {
			currentArrayMap[key] = val
		} else if currentSection == "entry" {
			if key == "command" {
				manifest.Entry.Command = getString(val)
			}
		} else if currentSection == "capabilities" {
			bVal := getBool(val)
			switch key {
			case "tools":
				manifest.Capabilities.Tools = bVal
			case "skills":
				manifest.Capabilities.Skills = bVal
			case "prompts":
				manifest.Capabilities.Prompts = bVal
			case "commands":
				manifest.Capabilities.Commands = bVal
			}
		} else if currentSection == "" {
			// Top-level fields
			switch key {
			case "id":
				manifest.ID = getString(val)
			case "name":
				manifest.Name = getString(val)
			case "version":
				manifest.Version = getString(val)
			case "description":
				manifest.Description = getString(val)
			}
		}
	}

	flushCurrentArray()
	return manifest, nil
}

func getString(val interface{}) string {
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

func getBool(val interface{}) bool {
	if b, ok := val.(bool); ok {
		return b
	}
	return false
}
