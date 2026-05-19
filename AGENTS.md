# rho — Agent Guide

rho is a lightweight extensible TUI coding agent written in Go, translated from [pi](https://github.com/earendil-works/pi) (TypeScript).

## Project Structure

```
.
├── cmd/rho/                  # CLI entry point
│   ├── main.go               # Flag parsing, model resolution, 4 modes
│   └── interactive.go        # Full TUI interactive mode
├── pkg/
│   ├── tui/                  # Terminal UI library (differential rendering)
│   │   ├── tui.go            # Core TUI engine (overlays, focus, render)
│   │   ├── terminal.go       # Raw terminal, raw mode, Kitty protocol
│   │   ├── editor.go         # Multi-line editor with undo/kill-ring
│   │   ├── components.go     # Component interface + Container
│   │   ├── markdown.go       # Markdown rendering
│   │   ├── select_list.go    # Selectable list component
│   │   ├── settings_list.go  # Settings editor component
│   │   ├── autocomplete.go   # Autocomplete with fuzzy matching
│   │   ├── loader.go         # Animated loader component
│   │   ├── image.go          # Terminal image protocol (Kitty/iTerm2)
│   │   ├── keybindings.go    # Keybinding manager
│   │   ├── keys.go           # Key parse/match utilities
│   │   ├── stdin_buffer.go   # Buffered stdin reader
│   │   ├── fuzzy.go          # Fuzzy string matching
│   │   ├── kill_ring.go      # Circular kill ring for editor
│   │   ├── undo_stack.go     # Undo/redo stack for editor
│   │   ├── editor_component.go # Pluggable editor interface
│   │   └── utils.go          # ANSI width, wrap, truncate
│   ├── ai/                   # AI provider framework
│   │   ├── types.go          # Message, content block, provider types
│   │   ├── models.go         # Model registry, 11 built-in models
│   │   ├── stream.go         # SSE stream reader
│   │   ├── images.go         # Image generation types + registry
│   │   ├── oauth.go          # OAuth PKCE flow for providers
│   │   ├── providers/        # Provider implementations
│   │   │   ├── anthropic.go      # Anthropic Messages API (SSE, thinking)
│   │   │   ├── openai.go         # OpenAI Completions + Responses API
│   │   │   ├── google.go         # Google Generative AI
│   │   │   ├── google_vertex.go  # Google Vertex AI
│   │   │   ├── azure_openai.go   # Azure OpenAI
│   │   │   ├── bedrock.go        # Amazon Bedrock
│   │   │   ├── mistral.go        # Mistral AI
│   │   │   ├── openai_codex.go   # OpenAI Codex
│   │   │   ├── cloudflare.go     # Cloudflare AI Gateway
│   │   │   ├── github_copilot.go # GitHub Copilot
│   │   │   ├── faux.go           # Mock provider (testing)
│   │   │   ├── images/           # Image generation providers
│   │   │   └── registry.go       # Provider registration
│   │   └── aiutils/          # AI utilities
│   │       ├── event_stream.go   # Streaming event emitter
│   │       ├── json_parse.go     # JSON repair/parse helpers
│   │       ├── sanitize_unicode.go
│   │       ├── validation.go
│   │       ├── diagnostics.go
│   │       └── ...
│   ├── agent/                # Agent core loop + services
│   │   ├── agent_loop.go     # Stream → tool exec → continue loop
│   │   ├── types.go          # Agent types, AgentTool, hooks
│   │   ├── session.go        # Session persistence (JSON)
│   │   ├── extensions/       # Extension system
│   │   │   ├── runtime.go    # Runtime with 12 lifecycle hooks
│   │   │   ├── loader.go     # .so plugin + JSON config loader
│   │   │   ├── wrapper.go    # Extension wrapping for agent loop
│   │   │   ├── types.go      # ExtensionDef, Hook types
│   │   │   └── index.go      # EventBus
│   │   ├── tools/            # Built-in tools
│   │   │   └── tools.go      # Read, Write, Edit, Bash, Grep, Find, Ls
│   │   ├── skills/           # YAML frontmatter skills system
│   │   ├── compaction/       # Context window management
│   │   ├── systemprompt/     # System prompt builder
│   │   ├── auth/             # API key + OAuth credential storage
│   │   ├── settings/         # Scoped settings (default/user/project)
│   │   ├── theme/            # JSON color themes
│   │   ├── resources/        # Load AGENTS.md, CLAUDE.md, skills, themes
│   │   ├── rpc/              # JSON-RPC protocol
│   │   ├── export/           # HTML session export
│   │   ├── codecore/         # Coding agent core
│   │   │   ├── sdk.go        # Public SDK for programmatic use
│   │   │   ├── agent_session.go / _runtime.go / _services.go
│   │   │   ├── config.go / defaults.go
│   │   │   ├── model_registry.go / model_resolver.go
│   │   │   ├── bash_executor.go
│   │   │   ├── keybindings.go / slash_commands.go
│   │   │   ├── settings_manager.go / session_manager.go
│   │   │   ├── telemetry.go / timings.go / diagnostics.go
│   │   │   ├── prompt_templates.go / messages.go
│   │   │   ├── package_manager*.go / migrations.go
│   │   │   ├── output_guard.go / auth_guidance.go
│   │   │   ├── resource_loader.go / footer_data_provider.go
│   │   │   ├── provider_display_names.go / session_cwd.go / source_info.go
│   │   │   └── ui/           # Interactive mode TUI components
│   │   │       ├── assistant_message.go, user_message.go
│   │   │       ├── bash_execution.go, tool_execution.go
│   │   │       ├── footer.go, keybinding_hints.go
│   │   │       ├── model_selector.go, session_selector.go
│   │   │       ├── settings_selector.go, theme_selector.go
│   │   │       ├── tree_selector.go, diff.go, dynamic_border.go
│   │   │       ├── login_dialog.go, oauth_selector.go
│   │   │       ├── countdown_timer.go, custom_message.go
│   │   │       ├── extension_editor.go, extension_input.go
│   │   │       ├── armin.go, daxnuts.go, earendil_announcement.go
│   │   │       └── visual_truncate.go, index.go
│   │   ├── harness/          # Agent harness (for SDK users)
│   │   └── utils/            # 24 utility packages
│   │       ├── ansi.go, git.go, shell.go, paths.go
│   │       ├── clipboard*.go, frontmatter.go, html.go
│   │       ├── child_process.go, fs_watch.go, sleep.go
│   │       ├── mime.go, image_convert.go, image_resize.go
│   │       ├── syntax_highlight.go, tools_manager.go
│   │       └── ...
│   └── ... (tests)
├── examples/extensions/      # 29 example extensions
│   ├── example-extension.go   # Complete demo (17 hook registrations)
│   ├── hello_weather.go       # Hello world + weather tool
│   ├── guard_extensions.go    # Permission gates, SSH, plan mode
│   ├── interactive_extensions.go  # Games, editors, modals
│   └── planning_extensions.go # Plan mode, handoff, subagent
└── rho                        # Built binary (16MB static)
```

## Key Design Decisions

- **Single dependency**: Only `golang.org/x/term` — no external Go deps
- **Static binary**: `go build -o rho ./cmd/rho` produces a 16MB binary with zero runtime deps
- **Extensions**: Go plugins (.so) or declarative JSON configs — no JS runtime
- **No web UI**: Terminal-only — browser components are out of scope
- **No telemetry**: Deliberately omitted for privacy

## How to Build

```bash
go build -o rho ./cmd/rho
```

## How to Run

```bash
# Set your API key
export ANTHROPIC_API_KEY=sk-ant-...

# Interactive mode (default)
./rho

# One-shot prompt
./rho -mode print -prompt "List all Go files"

# List available models
./rho -list-models

# Use a specific model
./rho -model gpt-4o -provider openai

# JSON mode (pipe-friendly)
./rho -mode json -prompt '{"messages":[...]}'

# RPC mode (for external tool integration)
./rho -mode rpc
```

## Available Models

| Model | Provider | Type |
|-------|----------|------|
| claude-sonnet-4-20250514 | Anthropic | reasoning |
| claude-3-5-sonnet-20241022 | Anthropic | chat |
| claude-3-opus-20240229 | Anthropic | chat |
| claude-3-haiku-20240307 | Anthropic | chat |
| gpt-4o | OpenAI | chat |
| gpt-4o-mini | OpenAI | chat |
| o3-mini | OpenAI | reasoning |
| gemini-2.5-pro-exp-03-25 | Google | reasoning |
| gemini-2.0-flash | Google | chat |
| deepseek-chat | DeepSeek (via OpenAI) | chat |
| mistral-large-2411 | Mistral | chat |

## Extension System

Extensions hook into 12 lifecycle events:

- `SessionStart` / `SessionShutdown`
- `AgentStart` / `AgentEnd`
- `TurnStart` / `TurnEnd`
- `Context` — inject context into system prompt
- `BeforeProviderRequest` — modify request payload
- `BeforeAgentStart`
- `Input` — modify user input
- `ToolCall` / `ToolResult`
- `UserBash` — intercept bash commands

Extensions can also provide: custom tools, custom AI providers, slash commands, keybindings, CLI flags, and message renderers.
