# rho Agent — Comprehensive Feature Catalog

> **Package:** `github.com/earendil-works/rho/pkg/agent`
> **Source directory:** `pkg/agent/`
> **Total subdirectories:** 14  (auth/, codecore/, compaction/, export/, extensions/, harness/, resources/, rpc/, settings/, skills/, systemprompt/, theme/, tools/, utils/)
> **Generated:** exhaustive analysis of every Go source file

---

## Table of Contents

1. [Root Package (`pkg/agent/`)](#1-root-package-pkgagent)
2. [Auth (`pkg/agent/auth/`)](#2-auth-pkgagentauth)
3. [Compaction (`pkg/agent/compaction/`)](#3-compaction-pkgagentcompaction)
4. [Export (`pkg/agent/export/`)](#4-export-pkgagentexport)
5. [Resources (`pkg/agent/resources/`)](#5-resources-pkgagentresources)
6. [RPC (`pkg/agent/rpc/`)](#6-rpc-pkgagentrpc)
7. [CodeCore (`pkg/agent/codecore/`)](#7-codecore-pkgagentcodecore)
   - 7.1 Agent Session & Runtime
   - 7.2 Services / DI Container
   - 7.3 Auth Guidance
   - 7.4 Bash Executor
   - 7.5 Config & Defaults
   - 7.6 Diagnostics
   - 7.7 Exec (generic command execution)
   - 7.8 Footer Data Provider
   - 7.9 Keybindings
   - 7.10 Messages (helpers + custom types)
   - 7.11 Migrations
   - 7.12 Model Registry
   - 7.13 Model Resolver
   - 7.14 Output Guard
   - 7.15 Package Manager
   - 7.16 Package Manager CLI
   - 7.17 Prompt Templates
   - 7.18 Provider Display Names
   - 7.19 Resource Loader
   - 7.20 Runtime Coordinator
   - 7.21 SDK
   - 7.22 Session CWD Manager
   - 7.23 Session Manager
   - 7.24 Settings Manager
   - 7.25 Slash Commands
   - 7.26 Source Info
   - 7.27 Telemetry
   - 7.28 Timings
   - 7.29 UI Components (`codecore/ui/`)
8. [Extensions (`pkg/agent/extensions/`)](#8-extensions-pkgagentextensions)
9. [Harness (`pkg/agent/harness/`)](#9-harness-pkgagentharness)
10. [Settings (`pkg/agent/settings/`)](#10-settings-pkgagentsettings)
11. [Skills (`pkg/agent/skills/`)](#11-skills-pkgagentskills)
12. [SystemPrompt (`pkg/agent/systemprompt/`)](#12-systemprompt-pkgagentsystemprompt)
13. [Theme (`pkg/agent/theme/`)](#13-theme-pkgagenttheme)
14. [Tools (`pkg/agent/tools/`)](#14-tools-pkgagenttools)
15. [Utils (`pkg/agent/utils/`)](#15-utils-pkgagentutils)

---

# 1. Root Package (`pkg/agent/`)

## Files: `types.go`, `session.go`, `agent_loop.go`

### Types & Data Model
- **AgentToolCall** — Type alias for `ai.ToolCall` representing a single tool invocation from an assistant message.
- **AgentToolResult** — Result of executing a tool (ID, name, content, error flag, optional details).
- **AgentMessage** — Unified message format across all roles (user, assistant, tool-result) with fields: role, content, tool-calls, usage, stop-reason, response-id, API, provider, model, hide-flag, timestamp, error-message.
- **AgentTool** — A tool available to the agent with name, description, JSON schema parameters, and execute callback.
- **AgentContext** — Current execution context: system prompt, model, messages, tools, thinking level.
- **AgentState** — Snapshot of agent state (for UI binding).
- **AgentEvent** — Typed event emitted during agent loop execution (text deltas, tool call start/end/delta, message end, errors).
- **AgentEventCallback** — Callback type for receiving agent events.
- **AgentLoopConfig** — Configuration for the agent loop: model, system prompt, max tokens, temperature, thinking level, tool execution mode, API key, max retries, signal context for cancellation.
- **BeforeToolCallContext / BeforeToolCallResult** — Hook context/result for intercepting tool calls before execution (block + reason).
- **AfterToolCallContext / AfterToolCallResult** — Hook context/result for post-execution tool call override (content, isError, terminate).
- **AgentError** — Typed error with message + wrapped error.
- **ToolExecutionMode** — Enum: `"sequential"` or `"parallel"`.
- **QueueMode** — Enum: `"all"` or `"one-at-a-time"`.

### Agent Loop Engine
- **NewAgentLoop(config)** — Creates a new agent loop.
- **loop.Run(prompts, context, emit)** — Starts agent loop from user prompts.
- **loop.Continue(context, emit)** — Continues from existing context (last message must be user).
- **loop.SetBeforeTool(fn)** — Registers a before-tool-call hook.
- **loop.SetAfterTool(fn)** — Registers an after-tool-call hook.
- **loop.SetStreamFn(fn)** — Replaces the default streaming function.
- **Internal `runLoop`** — Max 20 iterations safety limit, builds LLM context, streams response, executes tool calls, loops for tool-use continuations.
- **Internal `buildLLMContext`** — Converts AgentMessages to AI Messages for the provider.
- **Internal `streamResponse`** — Streams from model, emits text/tool call events, handles abort/error.
- **Internal `executeToolCalls`** — Iterates tool calls, fires before/after hooks, invokes tool execute, emits execution events.
- **Internal `isAborted`** — Checks signal context for cancellation.
- **Internal `extractNewMessages`** — Returns only messages added after the initial count.
- **defaultStreamFn** — Delegates to `providers.StreamSimple` from the AI registry.

### Session Management (`session.go`)
- **SessionHeader** — Metadata: type, version, ID, timestamp, CWD, parent session ID.
- **SessionEntry** — A session file entry (header or message) with type, version, ID, parent, timestamp, CWD, message, summary.
- **SessionInfo** — Summary display for a saved session: ID, timestamp, CWD, message count, preview.
- **SessionManager** — Thread-safe session CRUD with JSON file persistence.
  - **Save** — Persists header + messages as JSON.
  - **Load** — Reads JSON, returns header + messages.
  - **List** — Enumerates session files, returns sorted summary.
  - **Delete** — Removes a session file.
  - **PruneOldSessions** — Removes sessions older than a duration.
  - **ForkSession** — Creates a new session branching from an existing one (copies parent messages + new).
- **CurrentSessionID** — Generates a unique session ID (`session_<unixNano>`).

### Utility Functions
- **JSONSchemaTool** — Convenience constructor for simple AgentTool.
- **ConvertToLLMMessages** — Converts AgentMessages to AI Messages (filters non-convertible roles).

---

# 2. Auth (`pkg/agent/auth/`)

## Files: `auth.go`

### API Key Storage (`AuthStorage`)
- **NewAuthStorage(filePath)** — Creates JSON-file-backed auth storage.
- **SetAPIKey(provider, key)** — Saves an API key for a provider.
- **GetAPIKey(provider)** — Retrieves a stored key.
- **DeleteAPIKey(provider)** — Removes a key.
- **HasAPIKey(provider)** — Checks key existence.
- **GetAllProviders()** — Lists providers with stored keys.
- **Clear()** — Removes all keys.
- Thread-safe with `sync.RWMutex`, persisted to `~/.rho/auth/keys.json` (mode 0600).

### OAuth Credential Store (`OAuthStore`)
- **NewOAuthStore(filePath)** — Creates JSON-file-backed OAuth store.
- **Save(credential)** — Stores OAuth credentials (provider, access token, refresh token, expiry, scopes, token type).
- **Get(provider)** — Retrieves stored credentials.
- **List()** — Returns all stored credentials.
- **HasProvider(provider)** — Existence check.
- **Delete(provider)** — Removes credentials.
- Thread-safe, persisted to `~/.rho/auth/oauth.json`.

---

# 3. Compaction (`pkg/agent/compaction/`)

## Files: `compaction.go`

### Context Window Management
- **TokenEstimator** — Function type for token counting (default: ~4 chars/token).
- **CompactionSettings** — Config: target ratio, trigger ratio, min tokens, max summary length.
- **DefaultCompactionSettings** — Returns `{TargetRatio: 0.5, TriggerRatio: 0.75, MinTokens: 1024, MaxSummary: 512}`.
- **EstimateTokens(text)** — Estimates token count.
- **EstimateContextTokens(messages)** — Estimates total tokens across a conversation.

### Compaction Logic
- **FindCutPoint(messages, contextWindow, settings)** — Locates the best cut point from the end, returns `CutPoint`.
- **ShouldCompact(messages, contextWindow, currentUsage)** — Boolean check if compaction is needed (ratio >= trigger).
- **Compact(messages, cutPoint, summaryFn)** — Executes compaction by summarizing older messages, returns `CompactionResult`.
- **PrepareCompaction(messages, contextWindow)** — Returns `CompactionPreparation` (preview without executing).
- **Compacter** — Stateful compacter with settings and custom estimator.
  - **ShouldCompactWith(messages, contextWindow)** — Checks if compaction is needed.
  - **SetEstimator(est)** — Sets custom token estimator.

### Data Structures
- **Message** — Minimal message interface for compaction (role, content, tool calls, tool name, hide).
- **ToolCall** — Compact tool call representation.
- **CutPoint** — Start/end index + tokens before cut.
- **CompactionResult** — Result: compacted messages, summary, tokens before/after, messages removed.
- **CompactionPreparation** — Pending compaction preview.
- **CompactionEntry** — Stored compaction record with summary, entry IDs, tokens, timestamp, hook flag, details.

---

# 4. Export (`pkg/agent/export/`)

## Files: `export.go`

### HTML Export
- **ExportOptions** — Config: title, include CSS, include JS, dark mode, show timestamps.
- **DefaultExportOptions()** — Sensible defaults.
- **ExportToHTML(messages, path, opts)** — Writes a styled, responsive HTML file from agent messages.
  - Generates full `<!DOCTYPE html>` document with embedded CSS.
  - Supports dark mode via `prefers-color-scheme`.
  - Renders user, assistant, and tool messages with role-based styling.
  - Handles markdown-like formatting: code blocks, inline code, bold, headings.
  - Shows timestamps, model names, tool calls, error messages.
  - Responsive max-width layout.
- **ExportToHTMLString(messages, opts)** — Returns HTML as string (stub).
- **Internal formatting:** `formatContentForHTML`, `escapeNewlines`, `replaceInlineCode`, `replaceBold`, `replaceHeadings`.

---

# 5. Resources (`pkg/agent/resources/`)

## Files: `resources.go`

### Project Resource Discovery
- **ResourceDescriptor** — Describes a discovered resource: path, type ("skill", "prompt", "theme", "context"), source, priority.
- **ResourceLoader** — Scans the project directory for resources.
  - **Discover()** — Finds:
    - Context files: `AGENTS.md`, `CLAUDE.md`, `.rho/context.md`, `.github/AGENTS.md`
    - Skill files: `.rho/skills/*.md`
    - Prompt files: `.rho/prompts/*.md`
    - Theme files: `.rho/themes/*.json`
  - Results sorted by priority descending.
- **ReadProjectContext(projectDir)** — Reads and combines all project context files.
- **LoadProjectContextFiles(projectDir)** — Convenience: discover + read context.

---

# 6. RPC (`pkg/agent/rpc/`)

## Files: `rpc.go`

### JSON-RPC 2.0 Server
- **Server** — JSON-RPC server reading from stdin, writing to stdout.
- **NewServer()** — Creates server with STDIO transport.
- **SetModel(m)** — Sets the AI model.
- **SetAPIKey(key)** — Sets the API key.
- **SetTools(tools)** — Sets available tools.
- **SetSessionManager(mgr)** — Sets session manager.
- **Run()** — Starts the RPC loop, reads JSON-RPC requests line by line.

### Registered RPC Methods
| Method | Description |
|--------|-------------|
| `ping` | Returns `"pong"` |
| `models.list` | Returns all default models |
| `tools.list` | Returns registered tools |
| `session.list` | Lists saved sessions |
| `session.load` | Loads a session by ID |
| `session.delete` | Deletes a session |
| `chat` | Sends a user message, streams response, returns full text |
| `tool.execute` | Executes a specific tool by name with args |
| `agent.run` | Simple agent run (streams response) |

### Error Codes
- `ErrParse = -32700`, `ErrInvalidReq = -32600`, `ErrMethod = -32601`, `ErrParams = -32602`, `ErrInternal = -32603`

---

# 7. CodeCore (`pkg/agent/codecore/`)

## 7.1 Agent Session & Runtime

### `agent_session.go`
- **AgentSession** — Simple session wrapper around AgentLoop.
  - **NewAgentSession(config)** — Creates a session with model, system prompt, API key, CWD.
  - **Start() / Stop()** — Lifecycle management.
  - **SendMessage(content)** — Sends user message, returns assistant response.
  - **GetMessages()** — Returns all session messages.
  - **GetStats()** — Returns `SessionStats` (turns, tokens, cost, duration, tool calls).
  - **Save(path) / LoadSession(path)** — JSON serialization.

### `agent_session_runtime.go`
- **AgentSessionRuntime** — Full-featured session runtime integrating agent loop, extensions, compaction, settings, session persistence.
  - **NewAgentSessionRuntime(sessionID, opts)** — Constructor with all dependencies.
  - **Start() / Stop()** — Fires extension hooks (`SessionStart`, `SessionShutdown`).
  - **SendMessage(content)** — Full pipeline: builds context with tools + extension tools, installs hooks, runs agent loop, tracks stats, auto-saves session, fires `AgentStart`/`AgentEnd` events.
  - **GetStats() / GetMessages() / GetSessionID()** — Accessors.
  - **AddDiagnostic(type, message) / GetDiagnostics()** — Runtime diagnostics.
- **AgentSessionRuntimeDiagnostic** — Diagnostic message (info, warning, error).

### `agent_session_services.go`
- **AgentSessionServices** — Dependency injection container for all session services.
  - **NewAgentSessionServices(opts)** — Creates and wires:
    - ModelRegistry (populated with default models)
    - AuthStorage + OAuthStore
    - SessionManager
    - SettingsManager
    - DiagnosticsCollector
    - TimeTracker
    - TelemetryCollector
    - Extensions Runtime (loads from extension dirs)
  - **CreateAgentSessionFromServices(opts)** — Creates a session, merges extension tools.

## 7.2 Auth Guidance (`auth_guidance.go`)
- **MissingAPIKeyMessage(provider)** — Human-readable guidance for missing API keys per provider.
- **FormatNoModelsAvailableMessage(provider)** — Message when no models available.
- **FormatAuthStatusMessage(available, missing)** — Formatted auth status checklist.

## 7.3 Bash Executor (`bash_executor.go`)
- **BashExecutor** — Shell command execution with streaming.
  - **NewBashExecutor(cwd)** — Creates executor (default shell: `/bin/bash`, timeout: 5min).
  - **Execute(command)** — Runs command, captures stdout/stderr/exit code/duration/timed-out/PID.
  - **ExecuteWithStreaming(command, onStdout, onStderr)** — Streaming variant.
  - **AddSpawnHook(hook)** — Adds pre-execution hooks.
  - **appendEnv()** — Builds environment.
- **BashResult** — Result struct (stdout, stderr, combined, exit code, duration, timed out, PID).
- **BashSpawnHook** — Hook type for modifying exec.Cmd before start.
- **KillProcessTree(pid)** — Kills a process group by PID.

## 7.4 Config & Defaults (`config.go`, `defaults.go`)
- **VERSION** — Current version string `"0.2.0"`.
- **GetAgentDir / GetSessionsDir / GetConfigDir / GetExtensionsDir / GetPackageDir / GetCacheDir / GetLogDir** — Standard XDG-like directory paths under `~/.rho/`.
- **ExpandTildePath(p)** — Expands `~` to home.
- **IsBunBinary()** — Always false (Go version).
- **GetRelativePath(path, base)** — Relative path computation.
- **EnsureDir(dir)** — MkdirAll.
- **AppDataDir / TempSessionsDir** — Data directory helpers.
- **FormatPath(p)** — Display-format path (shortens home to `~`).
- **ValidateDir(dir)** — Checks directory existence and writability.
- **IsLocalPath(p)** — URL vs local path detection.
- **Defaults:**
  - `DefaultModelName = "claude-sonnet-4-20250514"`
  - `DefaultProvider = "anthropic"`
  - `DefaultMaxTokens = 8192`
  - `DefaultTemperature = 0.0`
  - `DefaultSystemPrompt` — Full coding agent system prompt.
  - `DefaultConfig()` — Returns full default configuration map.
  - `DefaultModel()` — Returns default `ai.Model`.
  - `DefaultToolSettings()` — Tool limits (max bytes, lines, output, timeout).
  - `DefaultThinkingLevel()` — `"off"`.
  - `DefaultCompactionSettings()` — Threshold-based compaction config.
  - `DefaultSessionSettings()` — Auto-save, max sessions, pruning config.

## 7.5 Diagnostics (`diagnostics.go`)
- **DiagnosticsCollector** — Collects categorized diagnostic messages.
  - **Report(diag) / ReportError / ReportWarning / ReportInfo** — Add diagnostics.
  - **Drain() / GetAll()** — Retrieve and optionally clear.
  - **HasErrors() / Count(type)** — Query diagnostics.
  - **Clear()** — Reset.
- **Diagnostic** — Struct: type (error/warning/info), scope, code, details.
- **FormatDiagnostic / FormatDiagnostics** — Human-readable formatting.
- **SortDiagnostics** — Sort by severity then message.
- **DrainErrors / DrainWarnings** — Convenience drain functions.

## 7.6 Exec (`exec.go`)
- **ExecOptions** — Config for command execution: CWD, timeout, env, extra paths, shell, merge stderr, signal.
- **ExecResult** — Result: stdout, stderr, exit code, duration (ms), error string, success flag.
- **ExecCommand(command, opts)** — Execute a command with full options.
- **ShellConfig** — Shell path, args, env.
- **GetDefaultShell()** — Detects user's shell (SHELL env, platform default).
- **GetShellConfig()** — Returns shell config with args per shell type.
- **GetShellEnv(extraPaths...)** — Builds environment for shell execution.

## 7.7 Footer Data Provider (`footer_data_provider.go`)
- **FooterData** — Status bar data: git branch, model name, provider, token count, context window %, thinking level, extension statuses, message count, cost, streaming flag, error, version, CWD.
- **ReadonlyFooterDataProvider** — Thread-safe provider for TUI footer.
  - **SetGitBranch / SetModel / SetTokenCount / SetThinkingLevel / SetMessageCount / SetStreaming / SetCost / SetCWD / SetError / SetExtensionStatus** — All the setters.
  - **GetData()** — Returns a copy of current data.
  - **GetSegments()** — Returns typed segments for TUI rendering.
  - **RenderText(width)** — Plain text rendering.
  - **RenderSegments()** — Segments for colored rendering.
- **FooterSegment** — Single segment: text, color, bold.

## 7.8 Keybindings (`keybindings.go`)
- **AppKeybinding** — Named keybinding with ID, key, description, category, default key.
- **ReservedKeybindingIDs** — 18 system-reserved binding IDs (interrupt, clear, exit, suspend, model cycle, thinking toggle, etc.).
- **DefaultKeybindings()** — Returns all 18 default bindings.
- **KeybindingsManager** — Thread-safe keybinding resolution with overrides.
  - **GetBinding(id) / GetKey(id)** — Lookup.
  - **SetOverride(id, newKey)** — Override a keybinding.
  - **ResetBinding(id)** — Restore to default.
  - **GetConflicts()** — Detect keybinding conflicts.
  - **LoadBindingsFromFile / SaveBindingsToFile** — JSON persistence.
  - **IsReservedBinding(id)** — Check if binding is system-reserved.
- **KeybindingConflict** — Conflict descriptor.
- **FormatKeybinding(key)** — Formats keys with Unicode symbols (⌃, ⇧, ⌥, ⌘, etc.).

## 7.9 Messages (`messages.go`)
- **Message Creation Helpers:**
  - **CreateUserMessage(text)** — Creates a user message with timestamp.
  - **CreateAssistantMessage(text, modelName, usage)** — Creates assistant message with usage.
  - **CreateToolResultMessage(toolCallID, toolName, content, isError)** — Creates tool result.
- **Formatting Helpers:**
  - **FormatMessageForDisplay(msg, width)** — Formats for TUI display.
  - **TruncateMessage(msg, maxLen)** — Truncates for previews.
  - **FormatToolCall(tc)** — Formats tool call for display.
- **Custom Message Types:**
  - **CustomMessage** — User-defined message type (customType, content, display, details).
  - **BashExecutionMessage** — Bash command execution record.
  - **CompactionSummaryMessage** — Compaction event record.
  - **BranchSummaryMessage** — Branch summary record.
  - **CreateCustomMessage / CreateBashExecutionMessage / CreateCompactionSummaryMessage / CreateBranchSummaryMessage** — Constructors.
  - **FormatCustomMessage(cmsg)** — Formats custom messages for display.

## 7.10 Migrations (`migrations.go`)
- **Migration** — Version migration descriptor (from, to, description, migrate function).
- **RegisterMigration(m)** — Register a migration.
- **RunMigrations(sessionsDir)** — Apply pending migrations to all session files.
- **migrateFile(path)** — Migrate a single file through applicable migrations.
- **detectVersion(data)** — Read version from JSON.
- **ShowDeprecationWarnings()** — Warns about old config paths (`~/.pi`, `~/.pi-config.json`).
- Built-in migrations: v1→v2 (add timestamps), v2→v3 (add model info) — both pass-through.

## 7.11 Model Registry (`model_registry.go`)
- **ModelRegistry** — Thread-safe registry for AI models.
  - **NewModelRegistry()** — Creates registry with built-in models from `ai.DefaultModels()`.
  - **SetAuthProvider(ap)** — Sets API key resolver.
  - **RegisterModel(m)** — Add a model.
  - **GetModel(provider, name)** — Lookup by provider+name.
  - **GetModelByName(name)** — Lookup by name only.
  - **FindModel(s)** — Flexible resolution: `"provider/model"`, model name, partial name match.
  - **GetModels()** — All models sorted by provider then name.
  - **GetProviders()** — Unique provider list.
  - **GetModelsByProvider / GetModelsByAPI** — Filtered lists.
  - **SearchModels(query)** — Multi-token search.
  - **ModelCount()** — Total count.
  - **String()** — Summary (e.g., `"ModelRegistry(anthropic:5 openai:3)"`).
- **ModelMetadata** — Context window, max tokens, thinking support, thinking levels, input types, cost.
- **GetModelMetadata(m)** — Lookup metadata from default model definitions.
- **GetSupportedThinkingLevels(model)** — Which thinking levels a model supports.
- **ClampThinkingLevel(model, level)** — Clamp to nearest supported level.
- **ModelAuthProvider** — Interface for API key resolution.

## 7.12 Model Resolver (`model_resolver.go`)
- **ScopedModel** — Model with scope info (CLI, config, session, default).
- **ResolveModelResult** — Model + scope + source description.
- **ResolveCLIModel(registry, modelArg, providerArg)** — Resolves from CLI args with fallbacks.
- **ResolveModelScope(cliModel, sessionModel, configModel)** — Determines most specific scope.
- **AutoDetectProviderFromModel(modelName)** — Guesses provider from model name prefixes.
- **AutoDetectAPIFromProvider(provider)** — Maps provider to default API type.
- **FormatModel / FormatModelShort** — Display formatting.

## 7.13 Output Guard (`output_guard.go`)
- **OutputGuard** — Captures stdout/stderr for safe TUI integration.
  - **TakeOverStdout()** — Redirects stdout/stderr to pipe.
  - **RestoreStdout()** — Restores original FDs.
  - **GetCapturedOutput()** — Returns captured data.
  - **IsActive()** — Check if redirect is active.

## 7.14 Package Manager (`package_manager.go`)
- **PackageInfo** — Extension package metadata (name, version, source type, description, enabled).
- **PackageSource** — Enum: `local`, `npm`, `gopath`, `git`.
- **PackageManager** — Manages extension packages.
  - **ListInstalledPackages()** — Enumerate installed extensions.
  - **InstallPackage(name, source)** — Create extension directory with manifest.
  - **RemovePackage(name)** — Remove extension.
  - **EnablePackage / DisablePackage** — Toggle extension state.
  - **ResolvePackagePath(name, dirs)** — Find package in search dirs.

## 7.15 Package Manager CLI (`package_manager_cli.go`)
- **PackageManagerCLI** — CLI frontend for package management.
  - **HandleCommand(args)** — Routes subcommands.
  - **Subcommands:** `install`/`i`/`add`, `uninstall`/`remove`/`rm`, `list`/`ls`, `update`/`upgrade`, `info`.
  - **install(args)** — Installs from name/URL, creates manifest.
  - **uninstall(args)** — Removes extension directory.
  - **list()** — Lists installed extensions with descriptions.
  - **update(args)** — Stub for update logic.
  - **info(args)** — Shows extension manifest details.

## 7.16 Prompt Templates (`prompt_templates.go`)
- **PromptTemplate** — Reusable prompt template (name, description, template text, category).
- **PromptTemplateStore** — Manages loading and formatting templates.
  - **NewPromptTemplateStore(dirs)** — Loads built-in + filesystem templates.
  - **LoadFromDir(dir)** — Loads `.tpl` files with companion `.json` metadata.
  - **Get(name)** — Get template by name.
  - **List() / ListByCategory(category)** — Enumerate.
  - **FormatTemplate(template, vars)** — Renders with `{{.Key}}` substitution.
  - **FormatTemplateString(tpl, vars)** — Direct string rendering.
- **Built-in templates:** `summarize`, `compact`, `branch-summary`, `plan`, `code-review`, `explain`.

## 7.17 Provider Display Names (`provider_display_names.go`)
- **ProviderDisplayInfo** — Name, short name, icon character, description, URL, docs URL.
- Complete mapping for all 17 supported providers (Anthropic, OpenAI, Google, Vertex AI, DeepSeek, Mistral, Groq, Fireworks, Together, xAI, OpenRouter, Vercel AI Gateway, Amazon Bedrock, GitHub Copilot, Cerebras, CrofAI, Azure OpenAI, OpenAI Codex).
- **GetProviderDisplayName / GetProviderShortName / GetProviderIcon / GetProviderDisplayInfo** — Lookups.
- **FormatProviderWithIcon** — Display with icon prefix.
- **AllProviderDisplayInfos** — Returns all known provider infos.

## 7.18 Resource Loader (codecore-level) (`resource_loader.go`)
- **ResolvedResource** — Path, content, kind, priority.
- **ResolvedPaths** — Skill paths, prompt paths, theme paths.
- **ResourceLoader** (codecore version) — Scans project `~/.rho/` and project `.rho/` directories.
  - **LoadProjectContextFiles()** — Finds AGENTS.md, CLAUDE.md, `.rho/instructions.md`, `.rho/context.md`, `.rho/project.md`.
  - **LoadSkillFiles(dir) / LoadPromptFiles(dir) / LoadThemeFiles(dir)** — Walk directories for matching files.
  - **ResolveResources()** — Combines user and project skill/prompt/theme paths.
- **LoadProjectContextFiles(cwd)** — Convenience function.

## 7.19 Runtime Coordinator (`runtime_coordinator.go`)
- **RuntimeCoordinator** — Central coordination layer for interactive mode.
  - **NewRuntimeCoordinator(...)** — Constructor with all core services.
  - **StatusText(activity)** — Builds status line.
  - **UpdateStatus(activity)** — Triggers UI callback.
- **Model Management:**
  - **SelectModel(modelDef)** — Changes model, resolves auth token.
  - **CurrentModelInfo()** — Returns `"provider/model"`.
- **Auth Management:**
  - **AvailableProviderNames()** — Sorted list of known providers.
  - **IsKnownProvider(provider)** — Checks recognition.
  - **SaveAPIKey(provider, key)** — Saves and applies.
  - **RemoveCredentials(provider)** — Removes both API key and OAuth.
  - **HasOAuthCredentials(provider)** — Checks OAuth store.
  - **GetOAuthStatus(provider)** — Human-readable OAuth status.
  - **ExchangeAndStoreOAuthCode(...)** — Exchanges auth code for tokens, persists.
- **Settings:**
  - **GetSetting / SetSetting / SettingBool / SettingString** — Typed setting access.
- **Theme:**
  - **ActiveThemeName() / SelectTheme(name)** — Theme control.
- **Session:**
  - **SaveCurrentSession / ResumeSession / StartNewSession / ForkSession / ListSessions** — Session lifecycle.
  - **SessionName / SetSessionName** — Named sessions.
- **Session Tree:**
  - **SessionTreeNode** — Tree node for session hierarchy.
  - **BuildSessionTree(allSessions)** — Builds parent/child tree.
  - **FlattenSessionTree(roots)** — Flattens tree to list.
- **Extension Status:**
  - **SetExtensionStatus / GetExtensionStatuses** — Key-value extension statuses.
- **Helpers:** `DefaultAuthKeysPath()`, `DefaultOAuthPath()`, `shortenPath()`.

## 7.20 SDK (`sdk.go`)
- **SDK** — Simple programmatic API for creating and running agent sessions.
  - **NewSDK()** — Creates SDK instance.
  - **CreateAgentSession(opts)** — Creates agent loop + context with all tools.
  - **RunSession(opts, prompt)** — Create + run in one call.

## 7.21 Session CWD Manager (`session_cwd.go`)
- **SessionCwdManager** — Per-session working directory tracking.
  - **Set(sessionID, cwd) / Get(sessionID) / Resolve(sessionID)** — CWD resolution.
- **MissingSessionCwdError** — Typed error for missing CWD.
- **GetMissingSessionCwdIssue(sessionID)** — User-friendly guidance.
- **FormatMissingSessionCwdPrompt(sessionID)** — Interactive prompt.
- **ResolveSessionCwd(sessionCwd, globalCwd)** — Path resolution logic.
- **ValidateSessionCwd(cwd)** — Validates directory exists.

## 7.22 Session Manager (codecore version) (`session_manager.go`)
- **SessionManager** (simplified) — JSON file-based CRUD.
  - **List()** — Lists sessions with preview, sorted by timestamp.
  - **Load(id) / Save(id, messages) / Delete(id)** — Standard CRUD.
  - **Fork(parentID, newID, newMessages)** — Branch a session.
- **SessionInfo** — Display info for session listing.

## 7.23 Settings Manager (codecore) (`settings_manager.go`)
- **SettingsManager** — Scoped settings with JSON persistence.
  - **NewSettingsManager(userDir, projectDir)** — Loads from defaults, user, project files.
  - **Get(key) / GetString / GetBool / GetInt** — Typed reads with scope hierarchy (project > user > defaults).
  - **SetUser(key, value) / SetProject(key, value)** — Persisted writes.
  - **GetAll()** — Merged view of all scopes.
  - **DrainErrors()** — Returns load errors.
- **SettingsError** — Error with scope.

## 7.24 Slash Commands (`slash_commands.go`)
- **SlashCommand** — Command definition: name, aliases, description, usage, category, handler, hidden.
- **SlashCommandContext** — Execution context: CWD, session manager, model registry, model, system prompt, settings, extension runner, notify callback.
- **SlashCommandManager** — Registry and execution.
  - **NewSlashCommandManager()** — Creates with built-in commands.
  - **Register(cmd)** — Add command.
  - **Get(name)** — Resolve by name or alias.
  - **Execute(ctx, input)** — Parse and execute `/command`.
  - **List() / ListByCategory(category)** — Enumerate.
  - **AutocompleteSuggestions(prefix)** — Returns matching command names.
- **Built-in slash commands (28 total):**
  - `/help` — Show help (with category grouping)
  - `/login`, `/logout` — Provider auth
  - `/compact` — Summarize conversation
  - `/fork` — Branch session
  - `/sessions` — List/switch/manage sessions
  - `/model` — View/change model
  - `/think` — Set thinking level
  - `/settings` — Open settings panel
  - `/theme` — Change UI theme
  - `/scoped-models` — Model cycling scope
  - `/export` — Export session as HTML
  - `/import` — Import JSONL session
  - `/share` — Share as GitHub gist
  - `/copy` — Copy last assistant message
  - `/name` — Set session display name
  - `/session` — Show session info
  - `/changelog` — Show changelog
  - `/hotkeys` — Show keyboard shortcuts
  - `/extensions` — Manage extensions
  - `/clone` — Duplicate session
  - `/tree` — Session tree navigation
  - `/new` — New session
  - `/resume` — Resume saved session
  - `/reload` — Reload config
  - `/quit` — Exit
  - `/clear` — Clear screen
  - `/cost` — Show token usage/cost
  - `/context` — Show context usage

## 7.25 Source Info (`source_info.go`)
- **SourceInfo** — Provenance tracking for tools and messages.
  - **Type** — `user`, `tool`, `extension`, `system`.
  - **Name, Version, Timestamp, Metadata, ToolCallID** — Tracking fields.
- **SourceInfoType** — Enum (user, tool, extension, system).
- **SourceInfoList** — List with query methods.
  - **Add, FindByType, FindByName, Last** — Collection operations.
- **Constructors:** `CreateSyntheticSourceInfo`, `CreateToolSourceInfo`, `CreateExtensionSourceInfo`, `CreateUserSourceInfo`.

## 7.26 Telemetry (`telemetry.go`)
- **TelemetryCollector** — Opt-in telemetry with JSONL persistence.
  - **NewTelemetryCollector(rhoDir)** — Disabled by default, checks for `.telemetry-opt-in` marker.
  - **SetEnabled(enabled)** — Toggle with opt-in marker file.
  - **IsEnabled()** — Status check.
  - **SetSessionID(id)** — Correlate events.
  - **Record(eventType, data) / RecordEvent(event)** — Record events (batched, flushed every 10).
  - **RecordSessionMetrics / RecordModelUsage / RecordToolExecution** — Specialized recorders.
  - **RecordError / RecordStartup** — Error and startup events.
  - **Flush()** — Force write to JSONL file.
  - **GetStats()** — Returns TelemetryStats.
- **TelemetryEvent** — Event: type, timestamp, session ID, data map.
- **TelemetryStats** — Snapshot: enabled, event count, session ID, uptime, pending flush.

## 7.27 Timings (`timings.go`)
- **TimeTracker** — Performance measurement.
  - **NewTimeTracker()** — Create tracker.
  - **Start(label) / Stop(label)** — Measure labeled operations.
  - **Record(label, duration)** — Direct recording.
  - **GetRecords(label) / GetAllLabels() / TotalDuration(label)** — Query.
  - **Reset() / PrintTimings()** — Clear and summary.
- **TimingRecord** — Label, duration, start time.
- **Global Convenience Functions:**
  - `Time(label)` / `TimeEnd(label)` — Start/stop using global tracker.
  - `PrintTimings()` / `ResetTimings()`.
  - `TimeFunc(label, fn)` / `TimeFuncResult[T](label, fn)` — Function timing wrappers.

## 7.28 UI Components (`codecore/ui/`)

### 7.28.1 `index.go`
- **KeybindingHintItem** — Key + description pair.

### 7.28.2 `armin.go` — Animated Robot Character
- **Armin** — Cute ASCII robot with 5 animated states:
  - `ArminIdle`, `ArminThinking`, `ArminHappy`, `ArminWorking`, `ArminError`
- **SetState / SetMessage / Start / Stop / Render** — Lifecycle and rendering.
- Each state has multiple animation frames with distinct ASCII art.

### 7.28.3 `daxnuts.go` — Animated Cat Character
- **Daxnuts** — ASCII cat with 5 emotional states:
  - `DaxnutIdle`, `DaxnutHappy`, `DaxnutConfused`, `DaxnutExcited`, `DaxnutSleepy`
- **SetState / SetMessage / Start / Stop / Render** — Lifecycle.
- Multiple animation frames per state.

### 7.28.4 `assistant_message.go`
- **AssistantMessage** — Renders assistant messages with:
  - Model name header + usage info
  - Markdown rendering of content
  - Tool call listing `🔧 toolName(args)`
  - Expand/collapse toggle
  - Thinking block visibility toggle
  - Error display
  - Stop reason display
- **CompactionSummaryMessage** — Renders compaction events in bordered box.
- **BranchSummaryMessage** — Renders branch summary events.

### 7.28.5 `user_message.go`
- **UserMessage** — Renders user messages with "You:" header, markdown content, image placeholder.
- **UserMessageSelector** — Select from multiple user messages to edit/resend.

### 7.28.6 `bash_execution.go`
- **BashExecution** — Renders bash command execution:
  - Command header with exit code and duration
  - Output display (expandable/collapsible)
  - Running indicator with spinner
  - Truncation notice
- **ToolExecution** — Generic tool execution display (name, args, duration, error/success icon, expand/collapse).
- **ExpandableContent** — Generic expandable/collapsible content.
- **ConsoleBlock** — Renders console output with scroll truncation.
- **SkillInvocationMessage** — Skill invocation display.
- **ProgressBar** — Simple progress bar component.

### 7.28.7 `countdown_timer.go`
- **CountdownTimerComponent** — Animated countdown overlay:
  - Bordered dialog with title and message
  - Color-changing progress bar (green → yellow → red)
  - Time display
  - Confirm (Enter) / Dismiss (Esc) callbacks
  - Auto-dismiss on timeout
- **TimedConfirm** — Convenience: countdown timer with boolean result callback.

### 7.28.8 `custom_message.go`
- **MessageRendererFunc** — Function type for custom message rendering.
- **RegisterMessageRenderer** — Global registry for extension renderers.
- **CustomMessageComponent** — Dispatches to registered renderers, falls back to generic bordered display.
- **GenericMessageComponent** — Standardized rendering for any agent message role.

### 7.28.9 `diff.go`
- **DiffRenderer** — Side-by-side diff with syntax highlighting:
  - Uses external `diff -u` tool when available, falls back to internal LCS-based diff.
  - Color-coded hunks (cyan headers, green additions, red deletions).
  - Stats line (additions, deletions).
- **VisualTruncate** — Smart content truncation with show-more toggle.
- **truncateToVisualLines** — Line-based truncation.

### 7.28.10 `dynamic_border.go`
- **BorderStyle** — Enum: `BorderSingle`, `BorderDouble`, `BorderRounded`, `BorderBold`, `BorderAnimated`, `BorderHidden`.
- **DynamicBorder** — Animated/configurable border around any child component:
  - Title display in top border.
  - 6 visual styles.
- **LoginDialog** — OAuth login flow display (waiting → code received → completed → error states).
- **OAuthSelector** — Provider selection list (with navigation).
- **CountdownTimer** — Simple countdown timer display.
- **CustomMessage** — Renders custom message using extension renderers.
- **ExtensionSelector** — Extension list with enable/disable toggling.

### 7.28.11 `earendil_announcement.go`
- **EarendilAnnouncement** — Startup announcement component:
  - Version info display.
  - Release notes.
  - Tips section.
  - Dismissible.

### 7.28.12 `extension_editor.go`
- **EditorFactory** — Factory type for custom editors.
- **ExtensionEditor** — Wraps custom editor implementations:
  - Multi-line input fallback.
  - Submit/cancel callbacks.
  - Pluggable via extension.

### 7.28.13 `extension_input.go`
- **InputFactory** — Factory type for custom inputs.
- **ExtensionInput** — Wraps custom input implementations:
  - Placeholder support.
  - Submit/cancel/change callbacks.
  - Pluggable via extension.

### 7.28.14 `footer.go`
- **Footer** — Status bar component displaying:
  - Git branch (with icon)
  - Session name
  - Model/provider
  - Thinking level
  - Token count + context percentage
  - Running cost
  - Auth status (API key vs OAuth)
  - Extension statuses
  - Help hint
  - All with ANSI coloring and proper spacing.
- **KeybindingHint** — Keybinding hints bar.
- **BorderedLoader** — Bordered loading indicator with spinner.

### 7.28.15 `keybinding_hints.go`
- **KeybindingHints** — Renders keybinding hints in reverse-video style:
  - Default hints (Enter, Esc, Tab, navigation, shortcuts).
  - Truncation to fit width.

### 7.28.16 `login_dialog.go`
- **LoginDialogComponent** — Full-featured login overlay:
  - States: idle, waiting, success, failed, expired.
  - Spinner animation.
  - Login URL display.
  - Client ID display.
  - Timeout countdown.
  - Retry on failure.
- **SetOnCancel / SetOnRetry** — Callbacks.

### 7.28.17 `model_selector.go`
- **ModelSelector** — Model selection dialog:
  - Provider-grouped display.
  - Search/filter by typing.
  - Keyboard navigation (arrows, pgup/down, home/end).
- **ThinkingSelector** — Thinking level selector for reasoning models.
- **ShowImagesSelector** — Toggle for inline image display.

### 7.28.18 `oauth_selector.go`
- **OAuthProviderInfo** — Provider metadata for selector.
- **OAuthSelectorComponent** — OAuth provider selection dialog:
  - Status icons (logged in vs not).
  - Scrolling for many providers.
  - Keyboard navigation.

### 7.28.19 `session_selector.go`
- **SessionSelector** — Session browsing dialog:
  - Search/filter.
  - Active session indicator.
  - Timestamp and message count.
  - Preview text.
  - Scrolling for many sessions.
- **SessionSelectorSearch** — Searchable variant.
- **TreeSelector** — Session tree navigation:
  - Indented tree display with connectors.
  - Branch markers (◇) and current position (●).
  - Depth-based indentation.

### 7.28.20 `settings_selector.go`
- **SettingType** — Enum: `boolean`, `string`, `number`, `select`.
- **SettingDef** — Setting definition: key, label, description, type, value, default, options, min/max, category.
- **SettingsSelector** — Interactive settings editor:
  - Category grouping.
  - Search/filter.
  - Value toggling (bool, select cycling, number increment).
  - Inline descriptions for selected item.

### 7.28.21 `theme_selector.go`
- **ThemeInfo** — Theme metadata (name, path, built-in, current, description).
- **ThemeSelector** — Theme selection dialog:
  - Current theme indicator (●).
  - Built-in label.

### 7.28.22 `tool_execution.go`
- **ToolExecutionState** — Enum: `pending`, `running`, `completed`, `failed`, `blocked`.
- **ToolExecutionComponent** — Single tool call lifecycle display:
  - State-specific icons and colors.
  - Spinner animation while running.
  - Expandable output (max 20 lines).
  - Duration display.
  - Blocked/hook-rejection display.
- **ToolExecutionGroup** — Groups multiple tool executions.
- **formatDuration** — Human-readable duration formatting.

### 7.28.23 `tree_selector.go`
- **TreeNode** — Session tree node (ID, label, summary, timestamp, branch/current flags, children, depth, message).
- **TreeSelectorComponent** — Full tree navigation overlay:
  - Flattened tree with tree-drawing characters.
  - Scrolling for deep trees.
  - Page up/down, home/end navigation.
  - Select (jump) and navigate (preview) callbacks.
- **BuildTreeFromSession(messages)** — Builds tree from session messages.

### 7.28.24 `visual_truncate.go`
- **VisualTruncateResult** — Truncation result (text, truncated flag, line count).
- **TruncateToVisualLines** — Truncates text to N visual lines at a given width.

---

# 8. Extensions (`pkg/agent/extensions/`)

## Files: `types.go`, `runtime.go`, `loader.go`, `index.go`, `wrapper.go`

### Extension System Architecture
Extensions can: register custom tools, subscribe to 13 lifecycle events, register slash commands, register custom AI providers, intercept/modify messages, block/modify tool calls, register keyboard shortcuts and CLI flags, provide custom message renderers, load resources (skills, prompts, themes).

### Types (`types.go`)
- **ToolDefinition** — Custom extension tool (name, label, description, prompt snippet/guidelines, parameters JSON schema, execute callback).
- **DefineTool** — Convenience constructor.
- **Lifecycle Events:**
  - `SessionStartEvent` / `SessionShutdownEvent` — Session lifecycle.
  - `AgentStartEvent` / `AgentEndEvent` — Agent loop lifecycle.
  - `TurnStartEvent` / `TurnEndEvent` — Per-turn events.
  - `ContextEvent` — Before LLM call (can modify messages).
  - `BeforeProviderRequestEvent` / `AfterProviderResponseEvent` — Provider request interception.
  - `BeforeAgentStartEvent` — Before agent processing.
  - `InputEvent` — User input handling (continue/transform/handled).
  - `ToolCallEvent` / `ToolCallEventResult` — Pre-execution tool call interception.
  - `ToolResultEvent` — Post-execution notification.
  - `UserBashEvent` — User bash command execution.
- **ExtensionUIContext** — UI primitives for extensions: Select, Confirm, Input, Notify, SetStatus.
- **ExtensionContext** — Full context passed to handlers: UI, HasUI, CWD, Model, Abort, Shutdown, AgentLoop, ExtensionRuntime.
- **SlashCommand** (extension) — Name, description, args, handler.
- **RegisteredCommand** — Registered command metadata.
- **ProviderConfig** — Custom AI provider definition (name, API type, base URL, API key, models).
- **MessageRenderer** — Custom message type renderer.
- **ExtensionFlag / ExtensionShortcut** — CLI flags and keyboard shortcuts.
- **ExtensionDef** — Complete extension definition: name, description, version, all event handlers, registered items.
- **ExtensionError** — Typed error with extension name.

### Runtime (`runtime.go`)
- **Runtime** — Extension lifecycle management and event dispatch.
  - **Register(ext)** — Add extension.
  - **GetAllExtensions()** — List all.
  - **GetCustomTools() / GetCustomProviders() / GetSlashCommands() / GetKeybindings() / GetCLIFlags() / GetMessageRenderers()** — Aggregated lists.
  - **GetEventBus()** — Inter-extension communication.
  - **Event Dispatch:**
    - `FireSessionStart` / `FireSessionShutdown`
    - `FireAgentStart` / `FireAgentEnd`
    - `FireTurnStart` / `FireTurnEnd`
    - `FireContext` — Can modify messages (pipeline).
    - `FireBeforeProviderRequest` — Can modify payload.
    - `FireBeforeAgentStart`
    - `FireInput` — First handler with non-"continue" action wins.
    - `FireToolCall` — First block wins.
    - `FireToolResult`
    - `FireUserBash`
    - `HandleSlashCommand` — Finds and executes.
- **EventBus** — Pub/sub for inter-extension communication.
  - **On(eventType, handler)** — Subscribe, returns unsubscribe function.
  - **Emit(eventType, event)** — Publish.

### Loader (`loader.go`)
- **LoadExtensions(dirs, runtime)** — Discovers and loads extensions from directories:
  - JSON config files (`extension.json`) with tool/provider/command definitions.
  - Go plugins (`.so` files exporting `RegisterExtension`).
  - Versioned directories (`name@version/`).
  - Hidden directories ignored.
- **LoadExtensionsResult** — Loaded names, errors, skipped.
- **BuiltinExtensions(runtime)** — No built-in extensions yet.
- **LoadExtensionConfig(path)** — Loads extension JSON config.
- **RegisterExtensionFromConfig(runtime, name, cfg)** — Creates extension from raw config map.

### Wrapper (`wrapper.go`)
- **Wrapper** — Wraps agent loop with extension hooks.
  - **WrapBeforeToolCall()** — Returns before-tool hook firing extension events.
  - **WrapAfterToolCall()** — Returns after-tool hook firing extension events.
- **ApplyToAgentLoop(runtime, extCtx, loop)** — Applies all extension hooks to a loop.
- **InstallHooks(runtime, extCtx, agentLoop)** — Alias for compatibility.
- **MergeTools(builtin, extensionTools)** — Merges with extension precedence.
- **MergeAllProviders(builtin, customProviders)** — Merges providers (dedup).

---

# 9. Harness (`pkg/agent/harness/`)

## Files: `types.go`, `agent_harness.go`, `messages.go`, `prompt_templates.go`, `skills.go`, `system_prompt.go`, `env/nodejs.go`, `utils/truncate.go`

### Types (`types.go`)
- **Result[TValue, TError]** — Type-safe result with Ok/Error.
- **FileError** — Typed filesystem error with code, message, path.
  - Codes: `aborted`, `not_found`, `permission_denied`, `not_directory`, `is_directory`, `invalid`, `not_supported`, `unknown`.
- **ExecutionError** — Typed execution error.
  - Codes: `aborted`, `timeout`, `shell_unavailable`, `spawn_error`, `callback_error`, `unknown`.
- **CompactionError** — Typed compaction error.
  - Codes: `aborted`, `summarization_failed`, `invalid_session`, `unknown`.
- **BranchSummaryError** — Typed branch summary error.
- **SessionError** — Typed session error.
  - Codes: `not_found`, `invalid_session`, `invalid_entry`, `invalid_fork_target`, `storage`, `unknown`.
- **AgentHarnessError** — Typed harness error.
  - Codes: `busy`, `invalid_state`, `invalid_argument`, `session`, `hook`, `auth`, `compaction`, `branch_summary`, `unknown`.
- **FileInfo** — Metadata: name, path, kind (file/directory/symlink), size, modification time.
- **FileSystem** — Interface for filesystem abstraction (14 methods: CWD, AbsolutePath, JoinPath, ReadTextFile, ReadTextLines, ReadBinaryFile, WriteFile, AppendFile, FileInfo, ListDir, CanonicalPath, Exists, CreateDir, Remove, CreateTempDir, CreateTempFile, Cleanup).
- **ExecutionEnvExecOptions** — Command execution config (CWD, env, timeout, abort signal, stdout/stderr callbacks).
- **Shell** — Interface: Exec, Cleanup.
- **ExecResult** — Stdout, stderr, exit code.
- **ExecutionEnv** — Combined FileSystem + Shell interface.
- **Skill** — Loaded skill with frontmatter (name, description, content, file path, disable-model-invocation).
- **SkillFrontmatter** — YAML frontmatter fields.
- **SkillDiagnostic** — Warning from skill loading.
- **PromptTemplate** — Reusable prompt template.
- **AgentHarnessResources** — Prompt templates + skills.
- **AgentHarnessStreamOptions / AgentHarnessStreamOptionsPatch** — Provider request configuration.
- **Session tree entry types:**
  - `SessionTreeEntryBase` — Type, ID, parent ID, timestamp.
  - `MessageEntry`, `ThinkingLevelChangeEntry`, `ModelChangeEntry`, `CompactionEntry` (with summary, tokens, hook flag), `BranchSummaryEntry`, `CustomEntry`, `CustomMessageEntry`, `LabelEntry`, `LeafEntry`.
  - `SessionTreeEntry` — Union type.
- **SessionContext** — Messages, thinking level, model ref.
- **SessionStorage** — Interface: GetMetadata, GetLeafID, SetLeafID, CreateEntryID, AppendEntry, GetEntry, GetEntries.
- **CompactionSettings / CompactionPreparation / CompactResult** — Compaction types.
- **AgentHarnessPhase** — Enum: `idle`, `turn`, `compaction`, `branch_summary`, `retry`.
- **Harness-specific events:** `QueueUpdateEvent`, `SavePointEvent`, `AbortEvent`, `SettledEvent`.
- **Event constants:** `EventQueueUpdate`, `EventSavePoint`, `EventAbort`, `EventSettled`, `EventBeforeAgentStart`, `EventContext`, `EventBeforeProviderReq`, `EventBeforeProviderPayload`, `EventAfterProviderResp`, `EventToolCall`, `EventToolResult`, `EventSessionBeforeCompact`, `EventSessionCompact`, `EventSessionBeforeTree`, `EventSessionTree`, `EventModelSelect`, `EventThinkingLevelSelect`, `EventResourcesUpdate`.
- **TreePreparation / NavigateTreeResult** — Tree navigation types.

### Agent Harness (`agent_harness.go`)
- **AgentHarness** — Configurable wrapper around AgentLoop with session management, skills, and event hooks.
  - **NewAgentHarness(config)** — Constructor.
  - **Phase() / SessionID() / TurnIndex()** — State accessors.
  - **On(eventType, handler)** — Register event listener (returns unsubscribe).
  - **SetResources(skills, templates)** — Set harness resources.
  - **GetSkills() / GetPromptTemplates()** — Accessors.
  - **Run(prompt, userImages, tools)** — Start agent with prompt, return messages.
  - **Continue(context, tools)** — Continue from existing context.
  - **Abort()** — Abort current execution.
  - **IsBusy()** — Check if processing.

### Messages (`messages.go`)
- **CustomMessageType** — Enum: `bash_execution`, `compaction_summary`, `branch_summary`, `skill_invocation`, `custom_notification`.
- **CreateBashExecutionMessage** — Creates tool result message for bash execution.
- **CreateCompactionSummaryMessage** — Creates hidden assistant message with compaction summary.
- **CreateBranchSummaryMessage** — Creates branch summary message.
- **CreateSkillInvocationMessage** — Creates skill invocation user message.
- **CreateCustomMessage** — Creates generic custom message.
- **FormatToolResult / FormatAgentMessage** — Display formatting.
- **IsCustomMessage / GetCustomMessageType** — Custom message detection.

### Prompt Templates (`prompt_templates.go`)
- **TemplateEngine** — Go template engine with FuncMap (upper, lower, trim, join).
  - **Register(name, content) / Execute(name, data)** — Template lifecycle.
- **FormatPromptTemplate** — Simple `{{.Variable}}` substitution.
- **BuildSystemPromptFromTemplate** — Builds system prompt from registered template.
- **DefaultSystemPromptTemplate** — Default system prompt with skills, tools, guidelines.
- **FormatSkillInvocation / FormatSkillXMLBlock / FormatSkillsForSystemPrompt** — Skill formatting.
- **FormatPromptTemplateInvocation** — Template with args.

### Skills (`skills.go`)
- **LoadSkills(env, dirs)** — Loads skills from directories recursively:
  - Looks for `SKILL.md` files and other `.md` files.
  - Parses YAML-like frontmatter.
  - Deduplicates by path.
- **loadSkillFromFile / parseSkillContent / parseSimpleFrontmatter** — Parsing pipeline.
- **FormatSkillsForPrompt** — Formats skills for system prompt inclusion.
- **FindSkill(skills, name)** — Case-insensitive lookup.
- **SkillNames(skills)** — Returns visible skill names.

### System Prompt (`system_prompt.go`)
- **SystemPromptSection** — Named section with content, order, enabled.
- **SystemPromptBuilder** — Builds structured system prompts:
  - **AddSection / RemoveSection / EnableSection** — Section management.
  - **Build()** — Assembles enabled sections in order.
  - **Reset() / SectionNames()** — Control.
- **DefaultSections(sysPrompt, skills, tools)** — Returns sections: identity, skills, tools, guidelines.
- **BuildSystemPrompt / BuildSystemPromptWithContext** — Full prompt assembly.
- **GenerateBranchSummary(messages, maxLen)** — Generates branch summary text.

### Environment (`env/nodejs.go`)
- **GetEnv(key, fallback)** — Env var with fallback.
- **GetEnvBool(key, defaultVal)** — Truthy env var parsing.
- **HomeDir / ConfigDir / DataDir / CacheDir** — Platform-specific directories (Linux, macOS, Windows).
- **ExpandPath(path)** — Expand `~`.
- **IsTerminal()** — Check if stdin is a terminal.
- **Platform() / Arch()** — Node.js-style platform/arch naming.
- **PID() / CWD() / Environ()** — Process info.

### Utils (`utils/truncate.go`)
- **TruncationResult** — Truncated, original/final size, message.
- **TruncationOptions** — Max bytes, max lines, add ellipsis.
- **DefaultTruncationOptions** — 50KB max, 2000 lines, ellipsis.
- **TruncateHead / TruncateTail** — End/beginning truncation.
- **TruncateLine** — Single line truncation.
- **FormatSize / FormatLineCount** — Human-readable formatting.

---

# 10. Settings (`pkg/agent/settings/`)

## Files: `settings.go`

### Scoped Settings Manager
- **SettingsManager** — Configuration with scoped overrides.
  - **NewSettingsManager(configDir)** — Creates with defaults.
  - **SetDefaults()** — 20+ default settings:
    - Model: `model`, `provider`, `maxTokens` (8192), `temperature` (0.7), `thinkingLevel` ("off").
    - UI: `theme` ("default"), `showImages` (true), `hardwareCursor` (false), `clearOnShrink` (true), `showLineNumbers` (true).
    - Tools: `autoResizeImages`, `maxReadBytes` (50K), `maxReadLines` (2000), `maxBashOutput` (100K), `bashTimeout` (0).
    - Session: `autoSave` (true), `autoCompact` (true), `compactTargetTokens` (40K), `compactMaxTokens` (80K), `sessionMaxAge` ("30d").
    - Extensions: `extensionsDir`, `autoLoadExtensions` (true).
    - Feature flags: `kittyProtocol` (true), `modifyOtherKeys` (true).
  - **AddProjectDir(dir)** — Add project for scoped settings.
  - **Load()** — Loads from user config (`~/.rho/config.json`) and project config (`.rho/config.json`), plus environment variables.
  - **Get(key) / GetString / GetInt / GetBool** — Typed access.
  - **Set(key, value, scope)** — Persisted write per scope.
  - **GetScope(key)** — Returns setting's scope.
  - **All()** — All settings.
  - **DrainErrors()** — Load errors.
- **SettingsScope** — Enum: `default`, `user`, `project`.
- **SettingsError** — Scope + message.
- **ShortName(key)** — Shortens dotted config keys.

### Environment Variable Overrides
| Env var | Setting key |
|---------|-------------|
| `RHO_MODEL` | model |
| `RHO_PROVIDER` | provider |
| `RHO_API_KEY` | apiKey |
| `RHO_TEMPERATURE` | temperature |
| `RHO_MAX_TOKENS` | maxTokens |
| `RHO_THEME` | theme |
| `RHO_EXTENSIONS_DIR` | extensionsDir |
| `RHO_BASH_TIMEOUT` | bashTimeout |

---

# 11. Skills (`pkg/agent/skills/`)

## Files: `skills.go`

### Skill Loading & Management
- **Skill** — Loaded skill: name, description, content, source file, tags, glob pattern.
- **SkillFrontmatter** — Parsed frontmatter fields.
- **LoadSkills(dirs)** — Loads from multiple directories.
- **LoadSkillsFromDir(dir)** — Scans for `.md` and `.mdx` files with YAML frontmatter.
- **parseSkillFile(path)** — Reads and parses a single skill file (frontmatter: name, description, tags, glob).
- **FormatSkillsForPrompt(skills)** — Formats skills for system prompt.
- **FilterSkillsByGlob(skills, filePath)** — Filters skills matching a file path glob.
- **ParseFrontmatter(content)** — Generic frontmatter parser (returns map + body).
- **StripFrontmatter(content)** — Removes frontmatter.
- **AllTags(skills)** — Returns unique sorted tags across all skills.
- **LoadSkillsResult** — Loaded skills, errors, skipped count.

---

# 12. SystemPrompt (`pkg/agent/systemprompt/`)

## Files: `builder.go`

### System Prompt Assembly
- **BuildOptions** — Assembly options: user prompt, base template, skills, project context, tools, extension instructions, CWD, model/provider name.
- **DefaultBaseTemplate()** — Returns full base system prompt for the coding agent.
- **Build(opts)** — Assembles complete system prompt from all sources in order:
  1. Base template
  2. Project context (AGENTS.md, etc.)
  3. Working directory
  4. Skills (formatted)
  5. Tools list
  6. Extension instructions
  7. Model/provider info
  8. User system prompt (highest priority)
- **TokenEstimate(prompt)** — Rough token count (~4 chars/token).
- **FormatToolList(tools)** — Formats tools for system prompt inclusion.

---

# 13. Theme (`pkg/agent/theme/`)

## Files: `theme.go`

### Theme System
- **ThemeColor** — Color definition: name, ANSI code, hex, bold/italic/underline.
- **Theme** — Complete visual theme: name, description, dark flag, author, color map, style map (ANSI format strings).
- **Built-in themes:**
  - **DefaultTheme()** — Light theme with blue accent, green success, etc.
  - **DraculaTheme()** — Dark theme (Dracula palette).
  - **CatppuccinTheme()** — Dark theme (Catppuccin Mocha palette).
- **ThemeManager** — Full theme lifecycle:
  - **NewThemeManager(themeDir)** — Loads built-in + filesystem themes.
  - **LoadThemes()** — Loads JSON theme files from directory.
  - **GetTheme(name)** — Lookup.
  - **SetActive(name)** — Switch active theme.
  - **Active() / ActiveName()** — Current theme access.
  - **ListThemes()** — Available theme names.
  - **Style(styleName)** — Get ANSI escape sequence.
  - **ApplyStyle(styleName, text)** — Wrap text with style + reset.
- **Reset()** — ANSI reset sequence.
- **SaveTheme(t, path)** — Serialize theme to JSON.

---

# 14. Tools (`pkg/agent/tools/`)

## Files: `tools.go`

### Agent Tool Implementations
8 built-in tools:

| Tool | Description | Parameters |
|------|-------------|------------|
| **Read** | Read file contents (text + image detection). Supports offset/limit for partial reads. | `path` (req), `offset`, `limit` |
| **Write** | Create/overwrite files. Auto-creates parent directories. | `path` (req), `content` (req) |
| **Edit** | Exact text replacement in files. Unique oldText required. | `path` (req), `oldText` (req), `newText` (req) |
| **Bash** | Execute shell commands. Configurable timeout. Max output 100KB. | `command` (req), `timeout` |
| **Grep** | Search file contents. Supports glob filtering, case-insensitive mode. Respects `.gitignore`-like skip dirs. Max 100 matches. | `pattern` (req), `path`, `glob`, `ignoreCase` |
| **Find** | Glob pattern file search. Configurable result limit. | `pattern` (req), `path`, `limit` |
| **Glob** | Double-star-aware glob search (delegates to Find). | `pattern` (req), `path` |
| **Ls** | Directory listing with sizes, sorted, with `/` for dirs. | `path`, `limit` |

### Tool Infrastructure
- **ToolFactory** — Function type `func(cwd string) AgentTool`.
- **ToolFactories()** — Returns map of all factory functions.
- **AllTools(cwd)** — Creates all tools bound to a working directory.
- **Helpers:**
  - `resolvePath(pathStr, cwd)` — Cleans and resolves relative/absolute paths.
  - `isImageFile(path)` — Detects image file extensions.
  - `isBinaryExt(name)` — Detects binary file extensions.
  - `toInt(v)` — Flexible numeric conversion (float64, int, int64, json.Number).
  - `readFileLines(file, offset, limit)` — Line-based file reading with byte/line limits.
  - `grepFiles(searchPath, pattern, globPattern, ignoreCase)` — Recursive grep implementation.
  - `findFiles(searchPath, pattern, maxResults)` — Recursive find implementation.
- **Limits:** Read: 50KB / 2000 lines max. Bash: 100KB max output. Grep: 100 matches max. Find: 100 results default. Ls: 500 entries default.

---

# 15. Utils (`pkg/agent/utils/`)

## Files: `ansi.go`, `changelog.go`, `child_process.go`, `clipboard.go`, `clipboard_native.go`, `clipboard_image.go`, `exif_orientation.go`, `frontmatter.go`, `fs_watch.go`, `git.go`, `html.go`, `image_convert.go`, `image_resize.go`, `mime.go`, `paths.go`, `photon.go`, `pi_user_agent.go`, `self_update.go`, `shell.go`, `sleep.go`, `syntax_highlight.go`, `tools_manager.go`, `version_check.go`, `windows_self_update.go`, `windows_self_update_stub.go`

### ANSI (`ansi.go`)
- **StripANSI(s)** — Removes all ANSI escape sequences.
- **StripSGR(s)** — Removes only SGR (color/style) codes.
- **CategorizeANSI(s)** — Classifies ANSI content type (plain, SGR, cursor, erase, hyperlink, image, other).
- **ExtractHyperlinks(s)** — Extracts OSC 8 hyperlinks (text + URL pairs).
- **WrapANSI(text, maxWidth)** — Wraps text respecting ANSI boundaries.
- **HasANSI(s)** — Check for ANSI presence.
- **ANSILength(s)** — Visible (non-ANSI) length.

### Changelog (`changelog.go`)
- **ChangelogEntry** — Version, date, changes list.
- **ChangeItem** — Type (added/changed/fixed/removed/deprecated/security), message.
- **ParseChangelog(content)** — Parses markdown changelog.
- **FormatChangelog(entries, maxEntries)** — Formats entries as markdown.
- **LatestVersion(entries)** — Returns latest version string.
- **CompareVersions(v1, v2)** — Semver comparison.
- **SortChangelogEntries(entries)** — Sort by version descending.

### Child Process (`child_process.go`)
- **SpawnOptions** — Command, args, dir, env, timeout, stdin.
- **SpawnResult** — Stdout, stderr, exit code, success, duration, error.
- **Spawn(opts)** — Spawn process with timeout.
- **SpawnWithStream(opts, onStdout, onStderr)** — Streaming variant.
- **ProcessTree** — Process with children.
- **KillProcessTree(pid)** — Recursive process tree kill.
- **TrackedProcessManager** — Process lifecycle manager.
  - **Track(proc) / Untrack(id) / KillAll()** — Track and cleanup.

### Clipboard (`clipboard.go`, `clipboard_native.go`, `clipboard_image.go`)
- **Clipboard** — Cross-platform clipboard handler.
  - **DefaultClipboard()** — Platform-appropriate (pbcopy/pbpaste on macOS, xclip/wl-clipboard on Linux, PowerShell on Windows).
  - **Primary** (read), **Write** — Operations.
  - Fallback: temp file, OSC 52 terminal escape.
- **NativeClipboard** — OSC 52 terminal clipboard.
  - **WriteOSC52(text) / WriteOSC52WithSelection(text, selection)** — Write via terminal.
  - **SupportsOSC52()** — Terminal detection (kitty, tmux, iTerm2, Windows Terminal, JetBrains).
- **ClipboardFile** — File-based clipboard fallback.
- **ClipboardImage** — Image clipboard operations.
  - **EncodeForClipboard / EncodeForKittyClipboard / DecodeFromClipboard** — Base64 encode/decode.
- **WriteImageToFile** — Writes image to temp file.

### EXIF Orientation (`exif_orientation.go`)
- **EXIFOrientation** — 8 standard orientation flags (Normal, FlipH, Rotate180, FlipV, Rotate90, etc.).
- **ReadEXIFOrientation(data)** — Reads from JPEG/WebP/TIFF bytes.
- **Degrees() / NeedsFlip() / AutoRotateDimensions()** — Orientation utilities.

### Frontmatter (`frontmatter.go`)
- **Frontmatter** — Parsed frontmatter: raw text, key-value data, content body, has flag.
- **ParseFrontmatter(content)** — Parses `---` or `+++` delimited frontmatter.
- **StripFrontmatter(content)** — Removes frontmatter.
- **ExtractFrontmatterValue / HasFrontmatterField** — Single value access.
- **BuildFrontmatter(data)** — Builds YAML frontmatter string.

### File System Watcher (`fs_watch.go`)
- **FileWatcher** — Poll-based file watcher with debouncing.
  - **NewFileWatcher(interval, debounce)** — Constructor.
  - **Watch(path) / Unwatch(path)** — Add/remove paths.
  - **OnChange(fn)** — Change callback.
  - **Start() / Stop()** — Polling lifecycle.
- **WatchDir(root, onChange)** — Recursive directory watcher.
- **WatchFiles(paths, onChange)** — Multi-file watcher.

### Git (`git.go`)
- **GitInfo** — Branch, commit, short commit, message, has changes, ahead/behind, remote, root.
- **GetGitBranch(dir)** — Current branch name.
- **GetGitInfo(dir)** — Full git repository info.
- **IsGitRepo(dir)** — Check if inside git repo.
- **GitDiff(dir, staged)** — Get diff output.
- **GitCommit(dir, message)** — Create commit.
- **GitAdd(dir, files...)** — Stage files.
- **GitLog(dir, count)** — Recent commit log.
- **GitCheckpointer** — Auto-commit before agent actions.
  - **NewGitCheckpointer(dir)** — Constructor.
  - **SetAutoPush(push)** — Enable auto-push.
  - **Checkpoint(message)** — Create checkpoint commit.
  - **Restore()** — Restore working tree.
- **FormatGitStatus(info)** — Display formatting.

### HTML (`html.go`)
- **StripHTMLTags(s)** — Removes HTML tags, scripts, styles, comments.
- **EscapeHTML / UnescapeHTML** — HTML entity handling.
- **WrapHTML(title, body)** — Wraps in basic HTML document.
- **ExtractTextFromHTML** — Visible text extraction.
- **HTMLToMarkdown(s)** — Converts HTML to Markdown (headers, bold, links, images, code, lists, etc.).

### Image Convert (`image_convert.go`)
- **ImageConverter** — Format conversion.
  - **Convert(data, fromFormat, toFormat)** — PNG ↔ JPEG conversion.
  - **CompressJPEG(data, quality)** — JPEG recompression.
  - **CompressPNG(data)** — PNG best compression.
  - **DetectFormat(data, path)** — Format detection (magic bytes + extension).
- **ConvertImageFile(srcPath, dstPath)** — File-based conversion.

### Image Resize (`image_resize.go`)
- **ResizeOptions** — MaxWidth, MaxHeight, MaxSize, Quality.
- **DefaultResizeOptions()** — 2000x2000, 85% quality.
- **ImageResizer** — Bilinear interpolation resize.
  - **Resize(data, opts)** — Resize image data.
  - **calculateDimensions / bilinearResize / bilinearPixel** — Internal algorithm.
- **ResizeImageFile(path, opts)** — Resize file in place.

### MIME (`mime.go`)
- **DetectMIMEType(path)** — Extension-based MIME detection (80+ mappings).
- **IsImageMIME / IsTextMIME / IsBinaryMIME** — Type classification.
- **SupportedImageMIMETypes()** — Returns `[image/png, image/jpeg, image/gif, image/webp]`.
- **FileExtensionFromMIME / MIMETypeForExtension** — Bidirectional lookup.

### Paths (`paths.go`)
- **IsLocalPath / IsParentPath** — Path relationship checks.
- **FormatPathRelativeToCwdOrAbsolute / ShortenPath / ExpandTildePath / ResolvePath** — Path formatting.
- **EnsureTrailingSlash / FindProjectRoot / IsHiddenPath** — Path utilities.
- **SafeJoin(base, elems...)** — Path traversal protection.
- **DirSize / FormatFileSize** — Size utilities.

### Photon WASM (`photon.go`)
- **PhotonWASM** — WASM-based image processing stub (not available in pure Go).
- **Filter** — Enum: grayscale, sepia, invert, brightness, contrast, blur, sharpen.
- **FilterImage** — Go standard library fallback (grayscale via RGB conversion).

### User Agent (`pi_user_agent.go`)
- **SetAppInfo(name, version)** — Set app identity.
- **GetUserAgent() / GetUserAgentWithExtra(extra)** — HTTP user agent strings.
- **AppName() / AppVersion()** — Metadata.
- **PiUserAgent()** — Pi/rho API user agent.

### Self Update (`self_update.go`, `windows_self_update.go`, `windows_self_update_stub.go`)
- **SelfUpdater** — Update checker and installer.
  - **NewSelfUpdater(currentVersion, updateURL)** — Constructor.
  - **CheckForUpdates()** — Checks GitHub releases API for newer version.
  - **ApplyUpdate(info)** — Downloads and replaces the running binary.
  - **NotifyIfUpdateAvailable()** — Returns notification string.
  - **ShouldCheck()** — 24-hour interval check.
- **WindowsSelfUpdate** — Windows-specific update handling:
  - `ReplaceRunning(newBinary)` — Creates rename script for in-place update.
  - `IsMarkedForCleanup()` / `CleanupWindowsSelfUpdateQuarantine()` — Cleanup.
  - `ScheduleUpdate(path)` — Uses `MoveFileEx` with `MOVEFILE_DELAY_UNTIL_REBOOT`.
  - `WaitForUpdateCompletion(maxWait)` / `IsUpdatePending()` — Status.
- Non-Windows stub returns no-ops.

### Shell (`shell.go`)
- **ShellType** — Enum: bash, zsh, fish, pwsh, cmd, sh, unknown.
- **ShellConfig** — Shell path, args, type, env.
- **GetShellConfig()** — Platform-aware shell detection.
- **GetDefaultShell()** — Returns default shell path.
- **GetShellEnv(shellType)** — Shell-optimized environment variables.
- **DetectShellType(shellPath)** — Detect type from path.
- **IsShellAvailable(shellPath)** — Check installation.
- **GetInteractiveShellArgs(shellType)** — Args for interactive use.

### Sleep/Backoff/Retry (`sleep.go`)
- **Sleep(d) / SleepWithContext(ctx, d)** — Blocking sleep.
- **BackoffConfig** — Initial, max, factor, jitter.
- **DefaultBackoffConfig()** — 1s initial, 60s max, 2x factor, ±25% jitter.
- **BackoffSleep(attempt, cfg) / BackoffSleepWithContext** — Exponential backoff.
- **RetryConfig** — Max retries + backoff.
- **DefaultRetryConfig()** — 3 retries, default backoff.
- **RetryWithBackoff(ctx, cfg, fn)** — Retry loop with backoff.

### Syntax Highlighting (`syntax_highlight.go`)
- **ChromaStyle** — Enum: monokai, github, dracula, catppuccin, native, friendly.
- **GetLanguageFromPath(path)** — Extension-to-language mapping (50+ mappings).
- **HighlightCode(code, language, style)** — Syntax highlighting with ANSI codes.
- **highlightWithHeuristics** — Keyword-based highlighting (keywords, strings, comments, numbers, builtins, punctuation).
- Language-specific keyword sets for Go, Python, and common languages.

### Tools Manager (`tools_manager.go`)
- **ToolStatus** — Enum: `enabled`, `disabled`.
- **ToolInfo** — Name, description, status, source (builtin/extension), order.
- **ToolsManager** — Tool lifecycle management.
  - **Register(name, description, source)** — Add tool.
  - **Enable(name) / Disable(name)** — Toggle.
  - **IsEnabled(name) / GetStatus(name)** — Query.
  - **GetOrdered() / GetAll()** — List tools.
  - **SetOrder(names)** — Custom ordering.
  - **FilterTools(tools)** — Filter AgentTools by enabled state.
  - **SortTools(tools)** — Sort by manager's order.
  - **String()** — Formatted summary.

### Version Check (`version_check.go`)
- **ReleaseInfo** — Tag, name, body, HTML URL from GitHub releases.
- **UpdateChecker** — GitHub API-based version checking.
  - **NewUpdateChecker(currentVersion, repoOwner, repoName)** — Constructor.
  - **CheckLatest()** — Fetches latest release from GitHub API.
  - **IsNewerVersionAvailable(latestVersion)** — Comparison.
- **FormatUpdateNotification** — Human-readable update notification.
- **CheckForUpdates(currentVersion, repoOwner, repoName)** — Full check + notification.

---

## Summary Statistics

| Category | Count |
|----------|-------|
| **Go source files** | ~80+ across all subdirectories |
| **Subdirectories** | 14 |
| **Built-in agent tools** | 8 (Read, Write, Edit, Bash, Grep, Find, Glob, Ls) |
| **Built-in slash commands** | 27 |
| **Extensions lifecycle hooks** | 13 |
| **Built-in themes** | 3 (Default, Dracula, Catppuccin) |
| **Supported AI providers** | 17 |
| **Built-in prompt templates** | 6 |
| **Utility packages** | 25 files in utils/ |
| **RPC methods** | 7 |
| **UI components** | 25+ in codecore/ui/ |

---

*This document was generated by exhaustively reading every Go source file in `pkg/agent/` and all its subdirectories.*
