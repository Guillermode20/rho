# ρ (rho)

**A lightweight extensible TUI coding agent.** Go translation of [pi](https://github.com/earendil-works/pi).

```
172 Go files · 36,223 lines · 0 external deps · 16 MB static binary
```

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/earendil-works/rho)](https://goreportcard.com/report/github.com/earendil-works/rho)

---

## Quick Start

```bash
# Build
go build -o rho ./cmd/rho

# Set your API key
export ANTHROPIC_API_KEY="sk-ant-..."

# Run
./rho
```

Press `Ctrl+N` for a new session, `Ctrl+E` to open the editor, `Ctrl+D` to toggle debug. Type `/help` for slash commands.

## Modes

| Mode | Command | Use Case |
|------|---------|----------|
| **Interactive** (default) | `./rho` | Full TUI agent with message list, editor, overlays |
| **Print** | `./rho -mode print -prompt "hello"` | One-shot Q&A to stdout |
| **JSON** | `./rho -mode json -prompt '...'` | Machine-readable JSON output |
| **RPC** | `./rho -mode rpc` | JSON-RPC over stdin/stdout for external tool integration |

## Models

Set the corresponding env var for your provider:

```bash
# Anthropic (default)
export ANTHROPIC_API_KEY="sk-ant-..."
./rho -model claude-sonnet-4-20250514

# OpenAI
export OPENAI_API_KEY="sk-proj-..."
./rho -model gpt-4o -provider openai

# Google
export GOOGLE_API_KEY="..."
./rho -model gemini-2.5-pro-exp-03-25

# Full list
./rho -list-models
```

**Supported providers:** Anthropic, OpenAI, Google, Mistral, DeepSeek, Amazon Bedrock, Azure OpenAI, Cloudflare, GitHub Copilot, Groq, Crof — 11 total.

## Features

### 🖥️ Terminal UI
- Differential rendering — only redraws what changed
- Overlays — model selector, session picker, theme picker, settings
- Focus management — message list, editor, command palette
- Kitty keyboard protocol — enhanced key reporting
- Full Unicode/CJK support
- Markdown rendering
- Terminal image protocol (Kitty / iTerm2)
- Animated loaders

### ✏️ Editor
- Multi-line editing with syntax highlighting
- Undo/redo with undo stack
- Kill ring (circular clipboard)
- Autocomplete with fuzzy matching
- Keybinding system (Emacs-style, configurable)

### 🤖 AI Providers
- **Anthropic** — Messages API with SSE streaming and thinking blocks
- **OpenAI** — Completions API + Responses API + Codex
- **Google** — Generative AI + Vertex AI
- **Mistral** — Conversations API
- **Amazon Bedrock** — Converse Stream API
- **Azure OpenAI** — Responses API
- **Cloudflare** — AI Gateway
- **GitHub Copilot** — OAuth-based auth
- **DeepSeek** — OpenAI-compatible
- **Groq** — OpenAI-compatible
- **Crof** — OpenAI-compatible
- OAuth PKCE flow (Anthropic, GitHub Copilot, OpenAI Codex)

### 🛠️ Built-in Tools
- **Read** — Read file contents with line ranges
- **Write** — Write or overwrite files
- **Edit** — Exact text replacement (diff-based)
- **Bash** — Execute shell commands with timeout
- **Grep** — Pattern search across files
- **Find** — Glob-based file search
- **Ls** — List directory contents
- **Glob** — Advanced glob pattern matching

### 🧩 Extension System
Extensions hook into **12 lifecycle events**, and can provide:

- Custom tools
- Custom AI providers
- Slash commands
- Keybindings
- CLI flags
- Message renderers
- Context injection

See [AGENTS.md](AGENTS.md) for the full extension API, or browse [example extensions](examples/extensions/) — including games, subagent orchestration, plan mode, and SSH permissions.

### 📦 Sessions
- JSON session persistence with branching support
- Fork, prune, and compaction for context window management
- Session picker to resume previous sessions
- HTML export with templates

### 🎨 Themes
Loadable JSON themes — includes `default`, `dracula`, and `catppuccin` out of the box. Create your own with any 16-color palette.

### 🔐 Authentication
- File-backed API key storage
- OAuth credential store with automatic token refresh
- Per-provider key management via `/login` and `/logout` commands
- Environment variable fallback

### ⚡ Additional Features
- **Skills** — YAML frontmatter skills loaded from `.rho/skills/`
- **Settings** — Scoped settings (default / user / project)
- **Extensions as Go plugins** (`.so`) or declarative JSON configs
- **JSON-RPC protocol** for external tool integration
- **Output guard** — prevents runaway responses
- **Telemetry** — deliberately omitted for privacy

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    cmd/rho/main.go                   │
│  CLI entry point · flag parsing · mode dispatcher   │
├─────────────────────────────────────────────────────┤
│                    pkg/agent/                        │
│  Agent loop · session management · tools             │
│  extensions · skills · system prompt · auth          │
│  compaction · export · RPC                           │
├─────────────────────────────────────────────────────┤
│                     pkg/ai/                          │
│  Model registry · providers (11) · streaming         │
│  OAuth · image generation                            │
├─────────────────────────────────────────────────────┤
│                     pkg/tui/                         │
│  Terminal UI engine · editor · markdown rendering    │
│  components · keybindings · fuzzy matching           │
├─────────────────────────────────────────────────────┤
│              examples/extensions/                    │
│  29 example extensions showcasing the plugin API     │
└─────────────────────────────────────────────────────┘
```

**Key design decisions:**
- **Zero external Go dependencies** — only `golang.org/x/sys`, `golang.org/x/text`, and `github.com/charmbracelet/...` (for TUI rendering)
- **Static binary** — single 16 MB self-contained executable
- **No JavaScript runtime** — extensions are Go plugins (`.so`) or JSON configs
- **No telemetry** — rho does not call home

## Requirements

- **Go 1.26+** (to build from source)
- A terminal with Unicode support
- Kitty keyboard protocol is optional but recommended for the best experience

## Installation

```bash
# From source
git clone https://github.com/earendil-works/rho.git
cd rho
go build -o rho ./cmd/rho
sudo cp rho /usr/local/bin/

# Or just download the binary
curl -LO https://github.com/earendil-works/rho/releases/latest/download/rho-linux-amd64
chmod +x rho-linux-amd64
sudo mv rho-linux-amd64 /usr/local/bin/rho
```

## Configuration

rho stores configuration in `~/.rho/`:

| Path | Purpose |
|------|---------|
| `~/.rho/auth/keys.json` | Stored API keys |
| `~/.rho/auth/oauth.json` | OAuth credentials |
| `~/.rho/settings/` | Scoped settings (default/user/project) |
| `~/.rho/themes/` | JSON theme files |
| `~/.rho/sessions/` | Session persistence |
| `~/.rho/skills/` | YAML skill files |

## Example: Custom Extension

```go
package main

import "github.com/earendil-works/rho/pkg/agent/extensions"

var Extension = extensions.ExtensionDef{
	Name:        "hello-weather",
	Description: "Adds a weather tool",
	Tools: []extensions.ToolDef{{
		Name:        "get_weather",
		Description: "Get the current weather for a location",
		Parameters:  ...,
		Execute: func(ctx context.Context, params map[string]interface{}) (string, error) {
			// ...
			return "Sunny, 72°F", nil
		},
	}},
}
```

See [examples/extensions/](examples/extensions/) for complete working examples.

## Community

- [Discord](https://discord.com/invite/3cU7Bz4UPx) — chat, support, showcase
- [GitHub Issues](https://github.com/earendil-works/rho/issues) — bug reports, feature requests
- [GitHub Discussions](https://github.com/earendil-works/rho/discussions) — Q&A, ideas

## Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on reporting bugs, requesting features, and submitting pull requests.

All participants are expected to follow our [Code of Conduct](CODE_OF_CONDUCT.md).

## License

MIT — see [LICENSE](LICENSE) for details.

rho is a Go translation of [pi](https://github.com/earendil-works/pi) (TypeScript) by [Mario Zechner](https://github.com/badlogic).
