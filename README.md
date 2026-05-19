# ρ (rho)

A lightweight extensible TUI coding agent. Go translation of [pi](https://github.com/earendil-works/pi).

```
172 Go files · 36,223 lines · 0 external deps · 16 MB static binary
```

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
| **RPC** | `./rho -mode rpc` | JSON-RPC over stdin/stdout for external tools |

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

## Features

- **TUI** — Differential rendering, overlays, focus management, Kitty keyboard protocol, Unicode/CJK support
- **Editor** — Multi-line with syntax highlighting, undo/redo, kill ring, autocomplete with fuzzy matching
- **AI Providers** — Anthropic, OpenAI, Google, Mistral, Azure, Bedrock, Cloudflare, GitHub Copilot (11 total)
- **Tools** — Read, Write, Edit (exact text replacement), Bash, Grep, Find, Glob, Ls
- **Extensions** — 12 lifecycle hooks, custom tools, custom providers, slash commands, keybindings
- **Sessions** — JSON persistence, branching, fork, prune, compaction
- **Skills** — YAML frontmatter skills loaded from `.rho/skills/`
- **Themes** — Loadable JSON themes (default, dracula, catppuccin)
- **Auth** — File-backed API keys + OAuth credential store
- **Export** — HTML session export with templates
- **RPC** — JSON-RPC protocol for external tool integration

## Requirements

- Go 1.26+
- A terminal with Unicode support (Kitty keyboard protocol optional)

## License

MIT
