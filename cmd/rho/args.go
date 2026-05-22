package main

import (
	"fmt"
	"os"
	"strings"
)

// Args represents the parsed CLI arguments.
type Args struct {
	Provider           string
	Model              string
	APIKey             string
	SystemPrompt       string
	AppendSystemPrompt []string
	Thinking           string
	Continue           bool
	Resume             bool
	Help               bool
	Version            bool
	Mode               string // text, json, rpc, interactive
	NoSession          bool
	Session            string
	Fork               string
	SessionDir         string
	Models             []string
	Tools              []string
	NoTools            bool
	NoBuiltinTools     bool
	Extensions         []string
	NoExtensions       bool
	Print              bool
	Export             string
	NoSkills           bool
	Skills             []string
	PromptTemplates    []string
	NoPromptTemplates  bool
	Themes             []string
	NoThemes           bool
	NoContextFiles     bool
	ListModels         string // search pattern, or "true" if list all
	Offline            bool
	Verbose            bool
	Messages           []string
	FileArgs           []string
	UnknownFlags       map[string]interface{}
}

// ParseArgs parses the CLI arguments sequentially to support POSIX-compliant formats
// and unknown extension flags without raising errors.
func ParseArgs(args []string) Args {
	result := Args{
		UnknownFlags: make(map[string]interface{}),
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--help" || arg == "-h" {
			result.Help = true
		} else if arg == "--version" || arg == "-v" {
			result.Version = true
		} else if arg == "--mode" && i+1 < len(args) {
			i++
			mode := args[i]
			if mode == "text" || mode == "json" || mode == "rpc" || mode == "interactive" {
				result.Mode = mode
			}
		} else if arg == "--continue" || arg == "-c" {
			result.Continue = true
		} else if arg == "--resume" || arg == "-r" {
			result.Resume = true
		} else if arg == "--provider" && i+1 < len(args) {
			i++
			result.Provider = args[i]
		} else if arg == "--model" && i+1 < len(args) {
			i++
			result.Model = args[i]
		} else if arg == "--api-key" && i+1 < len(args) {
			i++
			result.APIKey = args[i]
		} else if arg == "--system-prompt" && i+1 < len(args) {
			i++
			result.SystemPrompt = args[i]
		} else if arg == "--append-system-prompt" && i+1 < len(args) {
			i++
			result.AppendSystemPrompt = append(result.AppendSystemPrompt, args[i])
		} else if arg == "--no-session" {
			result.NoSession = true
		} else if arg == "--session" && i+1 < len(args) {
			i++
			result.Session = args[i]
		} else if arg == "--fork" && i+1 < len(args) {
			i++
			result.Fork = args[i]
		} else if arg == "--session-dir" && i+1 < len(args) {
			i++
			result.SessionDir = args[i]
		} else if arg == "--models" && i+1 < len(args) {
			i++
			result.Models = splitAndTrim(args[i])
		} else if arg == "--no-tools" || arg == "-nt" {
			result.NoTools = true
		} else if arg == "--no-builtin-tools" || arg == "-nbt" {
			result.NoBuiltinTools = true
		} else if (arg == "--tools" || arg == "-t") && i+1 < len(args) {
			i++
			result.Tools = splitAndTrim(args[i])
		} else if arg == "--thinking" && i+1 < len(args) {
			i++
			level := args[i]
			if isValidThinkingLevel(level) {
				result.Thinking = level
			} else {
				fmt.Fprintf(os.Stderr, "Warning: Invalid thinking level %q. Valid values: off, minimal, low, medium, high, xhigh\n", level)
			}
		} else if arg == "--print" || arg == "-p" {
			result.Print = true
			if i+1 < len(args) {
				next := args[i+1]
				if !strings.HasPrefix(next, "@") && (!strings.HasPrefix(next, "-") || strings.HasPrefix(next, "---")) {
					result.Messages = append(result.Messages, next)
					i++
				}
			}
		} else if arg == "--export" && i+1 < len(args) {
			i++
			result.Export = args[i]
		} else if (arg == "--extension" || arg == "-e") && i+1 < len(args) {
			i++
			result.Extensions = append(result.Extensions, args[i])
		} else if arg == "--no-extensions" || arg == "-ne" {
			result.NoExtensions = true
		} else if arg == "--skill" && i+1 < len(args) {
			i++
			result.Skills = append(result.Skills, args[i])
		} else if arg == "--prompt-template" && i+1 < len(args) {
			i++
			result.PromptTemplates = append(result.PromptTemplates, args[i])
		} else if arg == "--theme" && i+1 < len(args) {
			i++
			result.Themes = append(result.Themes, args[i])
		} else if arg == "--no-skills" || arg == "-ns" {
			result.NoSkills = true
		} else if arg == "--no-prompt-templates" || arg == "-np" {
			result.NoPromptTemplates = true
		} else if arg == "--no-themes" {
			result.NoThemes = true
		} else if arg == "--no-context-files" || arg == "-nc" {
			result.NoContextFiles = true
		} else if arg == "--list-models" {
			result.ListModels = "true"
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") && !strings.HasPrefix(args[i+1], "@") {
				i++
				result.ListModels = args[i]
			}
		} else if arg == "--verbose" {
			result.Verbose = true
		} else if arg == "--offline" {
			result.Offline = true
		} else if strings.HasPrefix(arg, "@") {
			result.FileArgs = append(result.FileArgs, arg[1:])
		} else if strings.HasPrefix(arg, "--") {
			eqIndex := strings.Index(arg, "=")
			if eqIndex != -1 {
				name := arg[2:eqIndex]
				val := arg[eqIndex+1:]
				result.UnknownFlags[name] = val
			} else {
				name := arg[2:]
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") && !strings.HasPrefix(args[i+1], "@") {
					result.UnknownFlags[name] = args[i+1]
					i++
				} else {
					result.UnknownFlags[name] = true
				}
			}
		} else if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") {
			fmt.Fprintf(os.Stderr, "Error: Unknown option: %s\n", arg)
		} else {
			result.Messages = append(result.Messages, arg)
		}
	}

	return result
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func isValidThinkingLevel(level string) bool {
	switch level {
	case "off", "minimal", "low", "medium", "high", "xhigh":
		return true
	}
	return false
}
