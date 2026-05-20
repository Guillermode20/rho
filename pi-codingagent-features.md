# pi Coding Agent (`@earendil-works/pi-coding-agent`) — Comprehensive Feature List

> Package version: **0.75.3** | Package name: `@earendil-works/pi-coding-agent`

---

## 1. OVERVIEW

The **pi coding agent** is a CLI + SDK for AI-powered coding sessions. It provides a fully extensible AI agent that can read, write, and edit files, execute bash commands, search code, and manage multi-turn conversations with LLMs. It ships with **three run modes** (interactive/TUI, print/single-shot, RPC/headless), a **rich extension system**, **7 built-in tools**, **session management with tree/branching**, **context compaction**, **skill system**, **prompt templates**, and **full SDK for programmatic embedding**.

---

## 2. RUN MODES

### 2.1 Interactive Mode (`InteractiveMode`)
- Full TUI (terminal UI) using the `@earendil-works/pi-tui` library
- Editor with autocomplete, slash commands, syntax highlighting
- Streaming message display with real-time rendering
- Tool call display with progress updates, timing, and expand/collapse
- Model selector, scoped models selector, thinking level selector
- Session selector, tree selector, user message selector
- Settings selector, theme selector, image display toggle
- Footer with token usage, model info, git branch
- Keyboard shortcuts (Ctrl+P to cycle models, etc.)
- Window/title setting
- Overlay dialogs (confirm, input, login, select)
- Widget system (above/below editor)

### 2.2 Print Mode (`runPrintMode`)
- Single-shot prompt → text output on stdout
- `pi -p "prompt"` for text output
- `pi --mode json "prompt"` for JSON event stream (including header)
- Piped stdin support (auto-switches from interactive to print)
- `@file` argument expansion
- Multiple sequential prompts via remaining args
- Signal handling for graceful exit

### 2.3 RPC Mode (`runRpcMode`)
- Headless JSON-line protocol via stdin/stdout
- Commands: `prompt`, `steer`, `follow_up`, `abort`, `new_session`
- State: `get_state` (model, thinking level, streaming, compacting, queue info)
- Model: `set_model`, `cycle_model`, `get_available_models`
- Thinking: `set_thinking_level`, `cycle_thinking_level`
- Queue modes: `set_steering_mode`, `set_follow_up_mode`
- Compaction: `compact`, `set_auto_compaction`
- Retry: `set_auto_retry`, `abort_retry`
- Bash: `bash`, `abort_bash`
- Session: `get_session_stats`, `export_html`, `switch_session`, `fork`, `clone`, `get_fork_messages`, `get_last_assistant_text`, `set_session_name`
- Messages: `get_messages`
- Commands: `get_commands` (extension commands, prompt templates, skills)
- Extension UI: transparent RPC bridge for `select`, `confirm`, `input`, `editor`, `notify`, `setStatus`, `setWidget`, `setTitle`, `setEditorText`

---

## 3. BUILT-IN TOOLS (7 tools)

All tools support **pluggable operations** (override execution to delegate to remote/SSH/container backends).

### 3.1 `read` — Read file contents
- Text files with line range (`offset`/`limit`)
- Images (jpg, png, gif, webp) — auto-resized, sent as base64 attachments
- Auto-truncation to max lines (2000) and max bytes (50KB)
- `truncateHead` — head-based truncation for reading
- Continuation hints (`Use offset=N to continue`)
- Syntax highlighting in TUI display
- Pi docs/skills/resources compact rendering
- Plugable `ReadOperations` interface

### 3.2 `bash` — Execute bash commands
- Full shell execution in working directory
- Streaming output with throttled updates (100ms throttle)
- Truncation to last 2000 lines or 50KB
- Full output saved to temp file if truncated
- Timeout support (optional)
- Exit code handling with error reporting
- Process tree kill on abort/timeout
- Detached child process tracking
- Pluggable `BashOperations` interface
- `BashSpawnHook` for command/env/cwd transformation
- Command prefix support
- Shell path configuration

### 3.3 `edit` — Precise file editing
- Exact text replacement with `edits[].oldText` / `edits[].newText`
- Multiple disjoint replacements in one call
- BOM handling, line ending detection/conservation (`\n`, `\r\n`, `\r`)
- Unified diff generation
- First-changed-line tracking (for editor navigation)
- Legacy single-edit input compatibility (`oldText`/`newText` at top level)
- Preview rendering in TUI (green for success, red for error, gray for pending)
- File mutation queue (sequentializes writes to same file)
- Auto-rescue: JSON-parse `edits` if sent as string
- Pluggable `EditOperations` interface
- `prepareArguments` compatibility shim

### 3.4 `write` — Write/overwrite files
- Creates file + parent directories automatically
- Syntax-highlighted preview in TUI
- Streaming incremental syntax highlight update
- File mutation queue integration
- Pluggable `WriteOperations` interface

### 3.5 `grep` — Search file contents
- Uses `ripgrep` (fd) backend with auto-download
- Regex or literal search
- Case-insensitive option
- Glob filtering
- Context lines (before/after)
- Match limit (default: 100)
- JSON streaming from rg, rich context formatting
- Long line truncation (500 chars)
- Byte truncation of output
- `.gitignore` respect
- Pluggable `GrepOperations` interface
- Fallback from binary to custom `readFile` operations

### 3.6 `find` — Search files by glob
- Uses `fd` backend with auto-download
- Glob pattern matching (`**/*.ts`, etc.)
- Result limit (default: 1000)
- `--full-path` mode with automatic `**/` prefix for path-containing patterns
- `.gitignore` and `.git/info/exclude` respect (via `--no-require-git`)
- Byte truncation
- Pluggable `FindOperations` interface (custom glob)

### 3.7 `ls` — List directory contents
- Alphabetical, case-insensitive sorting
- Directory suffix (`/` appended)
- Entry limit (default: 500)
- Dotfiles included
- Byte truncation
- Pluggable `LsOperations` interface

---

## 4. CORE ARCHITECTURE

### 4.1 `AgentSession`
Central class managing the full agent lifecycle:
- Agent state access (model, thinking level, tools, streaming status)
- Message queue management (steer/follow-up/nextTurn)
- Event subscription with automatic session persistence
- Model management (set, cycle through scoped/all models)
- Thinking level management (set, cycle, clamp to model capabilities)
- Compaction (manual, threshold-based, overflow recovery)
- Bash execution with streaming
- Session switching and branching
- Extension command dispatch
- Skill command expansion (`/skill:name args`)
- Prompt template expansion
- Auto-retry with exponential backoff
- Steering/follow-up queue modes (`all`, `one-at-a-time`)
- Session statistics
- HTML export
- Tool enable/disable by name

### 4.2 `AgentSessionRuntime`
Owns current session + cwd-bound services; provides session replacement:
- `switchSession()` — switch to existing session file
- `newSession()` — create fresh session
- `fork()` — branch from entry (with `position: "before"|"at"`)
- `importFromJsonl()` — import session from JSONL file
- `dispose()` — graceful teardown with shutdown events
- Extension rebinding on session replacement

### 4.3 `AgentSessionServices`
Cwd-bound services infrastructure:
- `AuthStorage` — API key and OAuth credential management
- `SettingsManager` — global + project settings
- `ModelRegistry` — model discovery, API key resolution
- `ResourceLoader` — extensions, skills, prompts, themes, context files

### 4.4 `SettingsManager`
- Global settings: `~/.pi/agent/settings.json`
- Project settings: `.pi/settings.json` (overrides global)
- Deep merge between scopes
- File locking for concurrent access
- All settings with typed interfaces

### 4.5 `SessionManager`
Manages session files (JSONL format):
- Session tree structure (parentId links form a tree)
- Session versioning (current: v3)
- Create, open, continue-recent, fork, in-memory sessions
- List sessions (local project, global across projects)
- Tree traversal, getBranch, getEntry, getLeafId
- Label management on entries
- Session name metadata
- Session information (count, dates, first message)
- Migration support

### 4.6 `ModelRegistry`
- Provider registration/unregistration
- Model discovery and filtering
- API key + OAuth credential resolution
- Environment variable fallback
- `models.json` persistence
- Scoped model resolution

### 4.7 `AuthStorage`
- File-backed (`auth.json`, 0600 permissions)
- In-memory backend option
- File locking with retry + async variants
- Runtime API key overrides (from CLI `--api-key`)
- Environment variable fallback
- OAuth credential storage and refresh
- Fallback resolver support

---

## 5. EXTENSION SYSTEM

### 5.1 Extension Lifecycle Events
Extensions can subscribe to **30+ event types**:

| Event | Description |
|-------|-------------|
| `resources_discover` | Provide additional resource paths (skills, prompts, themes) |
| `session_start` | Session started/loaded/reloaded |
| `session_before_switch` | Before switching to another session (cancellable) |
| `session_before_fork` | Before forking (cancellable) |
| `session_before_compact` | Before context compaction (cancellable, custom compaction) |
| `session_compact` | After compaction |
| `session_shutdown` | Runtime teardown (quit, reload, new, resume, fork) |
| `session_before_tree` | Before tree navigation (cancellable) |
| `session_tree` | After tree navigation |
| `context` | Before LLM call — modify messages |
| `before_provider_request` | Before API request — modify payload |
| `after_provider_response` | After API response received |
| `before_agent_start` | Before agent loop — add custom messages, override system prompt |
| `agent_start` | Agent loop started |
| `agent_end` | Agent loop ended (with messages) |
| `turn_start` | Turn started (with turnIndex + timestamp) |
| `turn_end` | Turn ended (with tool results) |
| `message_start` | Any message started |
| `message_update` | Streaming assistant message update |
| `message_end` | Message finalized (can replace message) |
| `tool_execution_start` | Tool execution started |
| `tool_execution_update` | Tool execution partial/streaming update |
| `tool_execution_end` | Tool execution finished |
| `model_select` | Model changed |
| `thinking_level_select` | Thinking level changed |
| `tool_call` | Before tool execution (cancellable, argument patching) |
| `tool_result` | After tool execution (can modify result) |
| `user_bash` | User !/!! bash command (can intercept) |
| `input` | User input before processing (transform or handle) |

### 5.2 Extension API (`pi` object)
Extensions receive a typed `pi` API object:

**Registration:**
- `pi.on(event, handler)` — subscribe to events
- `pi.registerTool(tool)` — register LLM-callable tool with TypeBox schema
- `pi.registerCommand(name, options)` — register slash command with autocomplete
- `pi.registerShortcut(keyId, options)` — register keyboard shortcut
- `pi.registerFlag(name, options)` — register CLI flag (boolean/string)
- `pi.getFlag(name)` — read registered CLI flag value
- `pi.registerMessageRenderer(customType, renderer)` — custom message rendering
- `pi.registerProvider(name, config)` — register/override model provider (with OAuth, custom API)
- `pi.unregisterProvider(name)` — remove a provider

**Actions:**
- `pi.sendMessage(message, options)` — send custom message (steer/followUp/nextTurn)
- `pi.sendUserMessage(content, options)` — send user message, triggers turn
- `pi.appendEntry(customType, data)` — persist extension state in session
- `pi.setSessionName(name)` / `pi.getSessionName()`
- `pi.setLabel(entryId, label)` — label/bookmark entries
- `pi.exec(command, args)` — execute shell command
- `pi.getActiveTools()` / `pi.getAllTools()` / `pi.setActiveTools(toolNames)`
- `pi.getCommands()` — available slash commands
- `pi.setModel(model)` — change model
- `pi.getThinkingLevel()` / `pi.setThinkingLevel(level)`
- `pi.events` — EventBus for extension-to-extension communication

### 5.3 Tool Definition Interface
Custom tools can define:
- `name` — unique tool name
- `label` — human-readable
- `description` — for LLM
- `promptSnippet` — one-line for system prompt
- `promptGuidelines` — guideline bullets for system prompt
- `parameters` — TypeBox schema
- `execute()` — execution function with abort signal, streaming update, context
- `renderCall()` / `renderResult()` — custom TUI rendering with shared state
- `renderShell: "default"|"self"` — custom framing control
- `prepareArguments()` — compatibility shim
- `executionMode: "sequential"|"parallel"`

### 5.4 Extension UI Context
Extensions get UI methods (mode-dependent implementation):
- `select(title, options)` — selector dialog
- `confirm(title, message)` — confirmation dialog
- `input(title, placeholder)` — text input dialog
- `notify(message, type)` — notification
- `onTerminalInput(handler)` — raw terminal input
- `setStatus(key, text)` — footer status
- `setWorkingMessage()` / `setWorkingVisible()` / `setWorkingIndicator()`
- `setHiddenThinkingLabel()`
- `setWidget(key, content, placement)` — above/below editor widgets
- `setFooter(factory)` / `setHeader(factory)` — custom footer/header
- `setTitle(title)` — terminal title
- `custom(factory, options)` — custom TUI component with overlay support
- `pasteToEditor()` / `setEditorText()` / `getEditorText()`
- `editor(title, prefill)` — multi-line editor
- `addAutocompleteProvider(factory)` — stacked autocomplete
- `setEditorComponent(factory)` / `getEditorComponent()`
- `theme` / `getAllThemes()` / `getTheme()` / `setTheme()`
- `getToolsExpanded()` / `setToolsExpanded(expanded)`

### 5.5 Extension Command Context
Extended context for command handlers adds:
- `waitForIdle()` — wait for streaming to finish
- `newSession(options)` — create new session (with `parentSession`, `setup`, `withSession`)
- `fork(entryId, options)` — fork session
- `navigateTree(targetId, options)` — tree navigation with summarization
- `switchSession(path, options)` — switch sessions
- `reload()` — reload extensions, skills, prompts, themes

### 5.6 Extension Discovery & Loading
- **jiti**-based TypeScript loader (compiles on-the-fly)
- `discoverAndLoadExtensions()` — automatic discovery from:
  1. Project-local: `.pi/extensions/*`
  2. Global: `~/.pi/agent/extensions/*`
  3. Explicit paths from CLI/config
- Subdirectory support: `index.ts`, `package.json` with `pi.extensions` manifest
- Virtual modules for Bun binary (bundled dependencies)
- jiti aliases for Node.js/development
- `loadExtensionFromFactory()` — inline factory support

### 5.7 Registered Commands
Extensions can register slash commands with:
- Name and description
- Argument autocomplete provider
- Handler receiving `(args, ctx)` with full command context
- Deduplication with invocation name suffixing for conflicts

### 5.8 Extension Shortcuts
- Register keybindings with conflict detection
- Reserved keybindings cannot be overridden (interrupt, clear, exit, model cycle, etc.)

---

## 6. SESSION MANAGEMENT

### 6.1 Session File Format (JSONL)
- One JSON object per line: header + entries
- Version 3 (current)
- Thread ID for branched sessions
- Parent ID linking for tree structure

### 6.2 Entry Types
| Type | Description |
|------|-------------|
| `message` | User/assistant/toolResult message |
| `thinking_level_change` | Thinking level change |
| `model_change` | Model switch |
| `compaction` | Compaction summary |
| `branch_summary` | Branch navigation summary |
| `custom` | Extension state (not sent to LLM) |
| `custom_message` | Extension message (sent to LLM) |
| `label` | User-defined entry label |
| `session_info` | Session metadata (name) |

### 6.3 Session Operations
- **Create**: new session in current project
- **Open**: resume existing session
- **Fork**: branch from a previous entry (`position: "before"|"at"`)
- **Clone**: duplicate session at current position
- **Continue**: resume most recent session in project
- **Import**: from JSONL file
- **Resume**: interactive session picker
- **Switch**: between sessions
- **In-memory**: ephemeral sessions
- **List**: local project and global session listing

### 6.4 Tree Navigation
- Session tree structure with parent/child relationships
- `navigateTree(targetId)` — switch to different branch
- Optional summarization of traversed messages
- Entry labeling/bookmarking

---

## 7. CONTEXT COMPACTION

### 7.1 Compaction Algorithm
- Token estimation (chars/4 heuristic)
- Cut point detection — walks backwards from newest
- Finds valid cut points (never at tool results)
- Handles mid-turn splits (keeps turn prefix as context)
- `keepRecentTokens` budget (default: 20K tokens)
- `reserveTokens` headroom (default: 16K tokens)

### 7.2 Summarization
- LLM-generated structured summary using **Goal/Progress/Decisions/Next Steps** format
- Support for incremental summaries (update from previous)
- Turn prefix summarization for split turns
- File operation tracking (read/modified files)
- Custom instructions for summary focus
- Configurable thinking level during summarization

### 7.3 Compaction Modes
- **Manual**: via `/compact` command
- **Threshold-based**: auto-triggered when context window > threshold
- **Overflow recovery**: triggered after context overflow errors
- **Extension-triggered**: via `ctx.compact(options)`

### 7.4 Compaction Settings
- `enabled` (default: true)
- `reserveTokens` (default: 16384)
- `keepRecentTokens` (default: 20000)

---

## 8. SYSTEM PROMPT ENGINE

- Custom prompt support (replaces default)
- Append prompt support (adds to default)
- Tool list generation with one-line snippets
- Tool-specific guideline bullets
- Project context files (AGENTS.md, CLAUDE.md etc.)
- Skill formatting for prompt
- Date and cwd injection
- Pi documentation references (README, docs/, examples/)

---

## 9. SKILL SYSTEM

- Markdown files with YAML frontmatter
- Skill fields: `name`, `description`, `disable-model-invocation`
- Loaded from project-local: `.pi/skills/`
- Loaded from global: `~/.pi/agent/skills/`
- `/skill:name args` command expansion
- Model invocation support (trigger agent turn from skill)
- Skill block parsing (`<skill name="..." location="...">`)
- Validation (name length ≤64, description ≤1024)

---

## 10. PROMPT TEMPLATES

- Markdown files with YAML frontmatter
- Template fields: `name`, `description`, `argumentHint`
- Arg substitution: `$1`, `$2`, `$@`, `$ARGUMENTS`, `${@:N}`, `${@:N:L}` (bash-style slicing)
- Quoted string parsing
- Loaded from `.pi/prompts/` and `~/.pi/agent/prompts/`

---

## 11. CLI ARGUMENTS

| Flag | Description |
|------|-------------|
| `-h`, `--help` | Show help |
| `-v`, `--version` | Show version |
| `--mode text\|json\|rpc` | Output mode |
| `-p`, `--print <prompt>` | Single-shot mode |
| `-c`, `--continue` | Continue most recent session |
| `-r`, `--resume` | Interactive session picker |
| `--provider <name>` | LLM provider |
| `--model <pattern>` | Model (supports `provider/pattern` and `pattern:thinking`) |
| `--api-key <key>` | API key |
| `--system-prompt <text>` | Custom system prompt |
| `--append-system-prompt <text>` | Append to system prompt |
| `--thinking <level>` | Thinking level (off/minimal/low/medium/high/xhigh) |
| `--no-session` | Ephemeral session |
| `--session <id\|path>` | Resume specific session |
| `--fork <id\|path>` | Fork from session |
| `--session-dir <path>` | Custom session storage |
| `--models <patterns>` | Scoped models for cycling |
| `--no-tools`, `-nt` | Disable all tools |
| `--no-builtin-tools`, `-nbt` | Disable built-in tools only |
| `--tools`, `-t <names>` | Tool allowlist |
| `--export <file>` | Export session to HTML |
| `--extension`, `-e <path>` | Load extension |
| `--no-extensions`, `-ne` | Skip extensions |
| `--skill <path>` | Load skill |
| `--no-skills`, `-ns` | Skip skills |
| `--prompt-template <path>` | Load prompt template |
| `--no-prompt-templates`, `-np` | Skip prompt templates |
| `--theme <path>` | Load theme |
| `--no-themes` | Skip themes |
| `--no-context-files` | Skip project context files |
| `--list-models [pattern]` | List available models |
| `--offline` | Offline mode (skip version check) |
| `--verbose` | Verbose logging |
| `@<file>` | Read message from file |

### Extension CLI Flags
Extensions can register custom CLI flags (`registerFlag()`), which appear in help and are parsed from unknown `--key=value` args.

---

## 12. BUILT-IN SLASH COMMANDS

| Command | Description |
|---------|-------------|
| `/settings` | Open settings menu |
| `/model` | Select model (selector UI) |
| `/scoped-models` | Enable/disable models for Ctrl+P cycling |
| `/export [path]` | Export session (HTML default, or .html/.jsonl) |
| `/import <path>` | Import and resume session from JSONL |
| `/share` | Share session as secret GitHub gist |
| `/copy` | Copy last agent message to clipboard |
| `/name <name>` | Set session display name |
| `/session` | Show session info and stats |
| `/changelog` | Show changelog entries |
| `/hotkeys` | Show all keyboard shortcuts |
| `/fork` | Create fork from previous user message |
| `/clone` | Duplicate session at current position |
| `/tree` | Navigate session tree |
| `/login <provider>` | Configure provider authentication |
| `/logout <provider>` | Remove provider authentication |
| `/new` | Start new session |
| `/compact` | Manually compact context |
| `/resume` | Resume different session |
| `/reload` | Reload extensions, skills, prompts, themes |
| `/quit` | Quit pi |

---

## 13. SETTINGS

### 13.1 Settings Categories
| Setting | Default | Description |
|---------|---------|-------------|
| `defaultProvider` | — | Default LLM provider |
| `defaultModel` | — | Default model ID |
| `defaultThinkingLevel` | — | Default thinking level |
| `transport` | `"auto"` | Provider transport |
| `steeringMode` | — | Queue mode: all / one-at-a-time |
| `followUpMode` | — | Follow-up mode: all / one-at-a-time |
| `theme` | — | Theme name |
| `shellPath` | — | Custom shell path |
| `shellCommandPrefix` | — | Bash command prefix |
| `quietStartup` | false | Suppress startup output |
| `collapseChangelog` | false | Condensed update changelog |
| `enableInstallTelemetry` | true | Anonymous usage ping |
| `enableSkillCommands` | true | /skill: commands |
| `doubleEscapeAction` | `"tree"` | Action on empty editor double-escape |
| `treeFilterMode` | `"default"` | Default tree filter |
| `editorPaddingX` | 0 | Editor horizontal padding |
| `autocompleteMaxVisible` | 5 | Autocomplete dropdown size |
| `showHardwareCursor` | false | Terminal cursor for IME |
| `npmCommand` | — | Custom npm command (argv array) |
| `sessionDir` | — | Custom session storage directory |

**Compaction settings** (`compaction.*`):
- `enabled` (true), `reserveTokens` (16384), `keepRecentTokens` (20000)

**Branch summary settings** (`branchSummary.*`):
- `reserveTokens` (16384), `skipPrompt` (false)

**Retry settings** (`retry.*`):
- `enabled` (true), `maxRetries` (3), `baseDelayMs` (2000)
- `retry.provider.timeoutMs`, `retry.provider.maxRetries`, `retry.provider.maxRetryDelayMs` (60000)

**Image settings** (`images.*`):
- `autoResize` (true), `blockImages` (false)

**Terminal settings** (`terminal.*`):
- `showImages` (true), `imageWidthCells` (60), `clearOnShrink` (false), `showTerminalProgress` (false)

**Thinking budgets** (`thinkingBudgets.*`):
- `minimal`, `low`, `medium`, `high` — custom token budgets per level

**Warning settings** (`warnings.*`):
- `anthropicExtraUsage` (true)

**Markdown settings** (`markdown.*`):
- `codeBlockIndent` ("  ")

### 13.2 Package Sources (`packages[]`)
- String: load all resources from npm/git package
- Object: filter which resources (extensions/skills/prompts/themes) to load

### 13.3 Resource Paths
- `extensions[]`, `skills[]`, `prompts[]`, `themes[]` — local file/directory paths
- `enabledModels[]` — model patterns for cycling

---

## 14. CONFIGURATION & PATHS

### 14.1 Agent Directory (`~/.pi/agent/`)
```
~/.pi/agent/
├── settings.json       # Global settings
├── auth.json           # API keys + OAuth tokens (0600)
├── models.json         # Model definitions
├── bin/                # Managed binaries (fd, rg)
├── tools/              # Custom tool directories
├── prompts/            # Prompt templates
├── themes/             # Custom themes
├── extensions/         # Global extensions
├── skills/             # Global skills
└── sessions/           # Session files
```

### 14.2 Project Directory (`.pi/`)
```
<project>/.pi/
├── settings.json       # Project settings overrides
├── extensions/         # Project-local extensions
├── skills/             # Project-local skills
├── prompts/            # Project-local prompt templates
└── themes/             # Project-local themes
```

### 14.3 Environment Variables
- `PI_CODING_AGENT_DIR` — custom agent config directory
- `PI_CODING_AGENT_SESSION_DIR` — custom session directory
- `PI_OFFLINE` — offline mode
- `PI_PACKAGE_DIR` — custom package assets directory
- `PI_SHARE_VIEWER_URL` — custom share viewer URL
- `PI_STARTUP_BENCHMARK` — startup timing benchmark

---

## 15. UTILITIES

### 15.1 String & Text
- `ansi.ts` — ANSI escape code stripping
- `frontmatter.ts` — YAML frontmatter parsing/stripping
- `syntax-highlight.ts` — code syntax highlighting via highlight.js
- `shell.ts` — shell config, env, process tree kill, detached child tracking
- `sleep.ts` — async sleep

### 15.2 File System
- `paths.ts` — path utilities, canonicalization, relative path formatting
- `git.ts` — git URL parsing
- `fs-watch.ts` — file system watching

### 15.3 Image Processing
- `image-convert.ts` — image format conversion
- `image-resize.ts` — image resizing (2000x2000 max)
- `mime.ts` — MIME type detection from file
- `clipboard-image.ts` — clipboard image reading
- `exif-orientation.ts` — EXIF orientation handling

### 15.4 System & Platform
- `child-process.ts` — child process spawning with wait
- `windows-self-update.ts` — Windows self-update quarantine cleanup
- `version-check.ts` — new version detection
- `pi-user-agent.ts` — User-Agent for HTTP requests

### 15.5 Clipboard
- `clipboard.ts` — unified clipboard (cross-platform via `@mariozechner/clipboard`)
- `clipboard-native.ts` — native clipboard fallback
- `clipboard-image.ts` — image clipboard operations

### 15.6 Other
- `changelog.ts` — changelog parsing (from CHANGELOG.md)
- `mime.ts` — image MIME type detection
- `photon.ts` — photon node WASM integration for image processing
- `tools-manager.ts` — automatic download of `fd` and `rg` binaries
- `html.ts` — HTML utilities

---

## 16. EXPORT & SHARING

### 16.1 HTML Export
- Full session export to standalone HTML
- ANSI-to-HTML conversion for tool output
- Template-based rendering (template.html, template.css, template.js)
- Custom tool rendering support
- Skill block rendering
- XSS protection

### 16.2 GitHub Gist Sharing
- `/share` command — share session as secret gist
- URL format: `https://pi.dev/session/#<gistId>`

---

## 17. KEYBINDINGS SYSTEM

- App-level keybindings (interrupt, clear, exit, suspend, thinking toggle/cycle, model cycle/select, tools expand, editor external, follow-up)
- TUI-level keybindings (submit, confirm, cancel, copy, delete-to-line-end)
- Extension shortcut registration with conflict detection
- Reserved shortcuts (cannot be overridden by extensions)
- Keybindings manager with configurable bindings
- Keybinding hints in UI

---

## 18. THEME SYSTEM

- Built-in themes (JSON files in `src/modes/interactive/theme/`)
- Custom themes from `~/.pi/agent/themes/` and `.pi/themes/`
- Theme selector in interactive mode
- Theme export
- Theme change watcher
- System theme detection (macOS light/dark)
- Theme colors for all UI components (tool titles, output, errors, etc.)
- `getLanguageFromPath()` for syntax highlighting
- `highlightCode()` for inline code rendering

---

## 19. TELEMETRY

- Anonymous install telemetry (version/update ping)
- Disabled via `enableInstallTelemetry: false`
- OpenRouter attribution headers (`HTTP-Referer`, `X-OpenRouter-Title`)
- Cloudflare attribution headers (`User-Agent`)

---

## 20. AUTO-UPDATE & PACKAGE MANAGEMENT

- Install method detection (bun-binary, npm, pnpm, yarn, bun)
- Self-update command generation per method
- Global package root detection
- Package manager CLI (`pi install <package>`, `pi uninstall <package>`)
- npm/git package source resolution via `PackageManager`
- Package manifest (`pi.extensions`, `pi.skills`, `pi.prompts`, `pi.themes` in package.json)

---

## 21. MIGRATIONS

- `migrations.ts` — auth provider migration
- Session file migration (version bumps)
- Keybinding migration
- Deprecation warnings

---

## 22. ERROR HANDLING & DIAGNOSTICS

- `AgentSessionRuntimeDiagnostic` — typed diagnostics (info/warning/error)
- Extension error reporting via `ExtensionErrorListener`
- Settings load error collection
- Resource loader error collection (extension load failures)
- Output guard for stdout management in non-interactive modes
- Startup timing benchmarks

---

## 23. SDK EXPORTS (Programmatic API)

The package is fully usable as an SDK:

```typescript
import {
  createAgentSession,
  createAgentSessionServices,
  createAgentSessionRuntime,
  AgentSession,
  AgentSessionRuntime,
  SettingsManager,
  SessionManager,
  AuthStorage,
  ModelRegistry,
  InteractiveMode,
  runPrintMode,
  RpcClient,
  // Tools
  createCodingTools,
  createReadOnlyTools,
  createBashTool,
  createReadTool,
  createEditTool,
  createWriteTool,
  createGrepTool,
  createFindTool,
  createLsTool,
  createBashToolDefinition,
  createEditToolDefinition,
  createReadToolDefinition,
  createWriteToolDefinition,
  createGrepToolDefinition,
  createFindToolDefinition,
  createLsToolDefinition,
  withFileMutationQueue,
  // Extensions
  defineTool,
  discoverAndLoadExtensions,
  ExtensionRunner,
  wrapRegisteredTool,
  wrapRegisteredTools,
  // Compaction
  compact,
  shouldCompact,
  findCutPoint,
  generateSummary,
  estimateContextTokens,
  serializeConversation,
  // Skills
  loadSkills,
  formatSkillsForPrompt,
  // Prompt templates
  expandPromptTemplate,
  // Settings
  CompactionSettings,
  ImageSettings,
  RetrySettings,
  // Session
  SessionManager,
  SessionEntry,
  // Theme
  Theme,
  initTheme,
  highlightCode,
  getLanguageFromPath,
  // Components
  renderDiff,
  ToolExecutionComponent,
  FooterComponent,
  UserMessageComponent,
  AssistantMessageComponent,
  // Clipboard
  copyToClipboard,
  // Frontmatter
  parseFrontmatter,
  stripFrontmatter,
  // Shell
  getShellConfig,
} from "@earendil-works/pi-coding-agent";
```

---

## 24. EXAMPLES (60+ shipped examples)

The package ships with an extensive `examples/` directory demonstrating:

**Extension examples** (60+):
`auto-commit-on-exit`, `bash-spawn-hook`, `bookmark`, `border-status-editor`, `built-in-tool-renderer`, `claude-rules`, `commands`, `confirm-destructive`, `custom-compaction`, `custom-footer`, `custom-header`, `custom-provider-anthropic`, `custom-provider-gitlab-duo`, `dirty-repo-guard`, `doom-overlay` (full DOOM game!), `dynamic-resources`, `dynamic-tools`, `event-bus`, `file-trigger`, `git-checkpoint`, `github-issue-autocomplete`, `handoff`, `hello`, `hidden-thinking-label`, `inline-bash`, `input-transform`, `interactive-shell`, `mac-system-theme`, `message-renderer`, `minimal-mode`, `modal-editor`, `model-status`, `notify`, `overlay-qa-tests`, `overlay-test`, `permission-gate`, `pirate`, `plan-mode`, `preset`, `prompt-customizer`, `protected-paths`, `provider-payload`, `qna`, `question`, `questionnaire`, `rainbow-editor`, `reload-runtime`, `rpc-demo`, `sandbox`, `send-user-message`, `session-name`, `shutdown-command`, `snake`, `space-invaders`, `ssh`, `status-line`, `structured-output`, `subagent`, `summarize`, `system-prompt-header`, `tic-tac-toe`, `timed-confirm`, `titlebar-spinner`, `todo`, `tool-override`, `tools`, `trigger-compact`, `truncated-tool`, `widget-placement`, `with-deps`, `working-indicator`, `working-message-test`

**SDK examples** (13):
`01-minimal`, `02-custom-model`, `03-custom-prompt`, `04-skills`, `05-tools`, `06-extensions`, `07-context-files`, `08-prompt-templates`, `09-api-keys-and-oauth`, `10-settings`, `11-sessions`, `12-full-control`, `13-session-runtime`
