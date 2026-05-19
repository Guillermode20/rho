package codecore

import (
	"fmt"
	"time"
)

// SourceInfoType describes the type of a source.
type SourceInfoType string

const (
	SourceInfoTypeUser      SourceInfoType = "user"
	SourceInfoTypeTool      SourceInfoType = "tool"
	SourceInfoTypeExtension SourceInfoType = "extension"
	SourceInfoTypeSystem    SourceInfoType = "system"
)

// SourceInfo tracks provenance information for tools and messages.
type SourceInfo struct {
	// Type of source.
	Type SourceInfoType `json:"type"`
	// Name of the source (tool name, extension name, "user", etc.).
	Name string `json:"name"`
	// Version of the source, if applicable.
	Version string `json:"version,omitempty"`
	// Timestamp when the source was created.
	Timestamp int64 `json:"timestamp"`
	// Additional metadata.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// ID of the tool call that produced this source.
	ToolCallID string `json:"toolCallId,omitempty"`
}

// CreateSyntheticSourceInfo creates a synthetic source info for testing or fallback.
func CreateSyntheticSourceInfo(name string, sourceType SourceInfoType) *SourceInfo {
	return &SourceInfo{
		Type:      sourceType,
		Name:      name,
		Timestamp: time.Now().UnixMilli(),
		Metadata:  make(map[string]interface{}),
	}
}

// CreateToolSourceInfo creates a source info for a tool call.
func CreateToolSourceInfo(toolName string, toolCallID string) *SourceInfo {
	return &SourceInfo{
		Type:       SourceInfoTypeTool,
		Name:       toolName,
		Timestamp:  time.Now().UnixMilli(),
		ToolCallID: toolCallID,
		Metadata:   make(map[string]interface{}),
	}
}

// CreateExtensionSourceInfo creates a source info for an extension.
func CreateExtensionSourceInfo(extName, extVersion string) *SourceInfo {
	return &SourceInfo{
		Type:      SourceInfoTypeExtension,
		Name:      extName,
		Version:   extVersion,
		Timestamp: time.Now().UnixMilli(),
		Metadata:  make(map[string]interface{}),
	}
}

// CreateUserSourceInfo creates a source info for a user message.
func CreateUserSourceInfo() *SourceInfo {
	return &SourceInfo{
		Type:      SourceInfoTypeUser,
		Name:      "user",
		Timestamp: time.Now().UnixMilli(),
	}
}

// String returns a human-readable representation of the source info.
func (si *SourceInfo) String() string {
	switch si.Type {
	case SourceInfoTypeUser:
		return "user"
	case SourceInfoTypeTool:
		if si.ToolCallID != "" {
			return fmt.Sprintf("tool:%s(%s)", si.Name, si.ToolCallID)
		}
		return fmt.Sprintf("tool:%s", si.Name)
	case SourceInfoTypeExtension:
		if si.Version != "" {
			return fmt.Sprintf("ext:%s@%s", si.Name, si.Version)
		}
		return fmt.Sprintf("ext:%s", si.Name)
	case SourceInfoTypeSystem:
		return fmt.Sprintf("system:%s", si.Name)
	default:
		return si.Name
	}
}

// AddMetadata adds a metadata key-value pair.
func (si *SourceInfo) AddMetadata(key string, value interface{}) {
	if si.Metadata == nil {
		si.Metadata = make(map[string]interface{})
	}
	si.Metadata[key] = value
}

// SourceInfoList is a list of source infos.
type SourceInfoList []*SourceInfo

// Add adds a source info to the list.
func (l *SourceInfoList) Add(si *SourceInfo) {
	*l = append(*l, si)
}

// FindByType finds source infos by type.
func (l SourceInfoList) FindByType(sourceType SourceInfoType) []*SourceInfo {
	var result []*SourceInfo
	for _, si := range l {
		if si.Type == sourceType {
			result = append(result, si)
		}
	}
	return result
}

// FindByName finds source infos by name.
func (l SourceInfoList) FindByName(name string) []*SourceInfo {
	var result []*SourceInfo
	for _, si := range l {
		if si.Name == name {
			result = append(result, si)
		}
	}
	return result
}

// Last returns the last source info in the list.
func (l SourceInfoList) Last() *SourceInfo {
	if len(l) == 0 {
		return nil
	}
	return l[len(l)-1]
}
