# pi Architecture Map

This maps the whole `pi/` reference tree, not just the interactive TUI. It is intended as the porting reference for `rho`: what exists in pi, where it lives, how the packages depend on each other, and which behavior is structural rather than cosmetic.

The interactive terminal surface is covered in greater depth in [`pi-interactive-tui-map.md`](./pi-interactive-tui-map.md). This document treats that as one subsystem inside the larger product.

## 1. Top-Level Shape

`pi` is a TypeScript monorepo. The product is not just a terminal frontend. It is a layered stack:

1. `@earendil-works/pi-ai`: provider-neutral LLM and image API layer.
2. `@earendil-works/pi-agent-core`: generic agent loop and harness.
3. `@earendil-works/pi-tui`: terminal UI rendering/component library.
4. `@earendil-works/pi-coding-agent`: CLI application, sessions, tools, settings, extensions, modes, resources.
5. `@earendil-works/pi-web-ui`: browser chat components and storage.
6. `examples/extensions`: example extension packages that exercise the extension system.

The important architectural point is that `coding-agent` is the product coordinator. It consumes `ai`, `agent-core`, and `tui`; it also exposes SDK, RPC, print/json, and interactive modes. The TUI is not the app. It is one presentation mode over a shared session/runtime core.

### Root Workspace

Key root files:

- `package.json`: workspace definitions and root scripts.
- `README.md`: product overview and supported modes.
- `tsconfig*.json`: TypeScript build/check setup.
- `biome.json`: formatting/lint policy.
- `scripts/*`: smoke tests, release/versioning, profiling, package statistics, session inspection.
- `pi-test.sh`, `pi-test.ps1`, `pi-test.bat`: run-from-source helpers.

Root build order is significant:

1. `packages/tui`
2. `packages/ai`
3. `packages/agent`
4. `packages/coding-agent`
5. `packages/web-ui`

That order reflects dependencies. The coding agent cannot be understood correctly by starting at the TUI alone.

### Root Scripts

Representative scripts:

- `build`: builds all packages in dependency order.
- `dev`: concurrently watches packages.
- `check`: runs Biome, TypeScript, browser smoke checks, and web UI checks.
- `test`: delegates to workspace tests.
- `profile:tui`, `profile:rpc`: profiling entry points.
- `version:*`, `release`, `build:binaries`: release automation.

For `rho`, the equivalent concern is not merely "does Go build?" but whether all product modes still have a coherent shared core.

## 2. Package Dependency Model

The effective dependency graph is:

```text
web-ui
  -> ai
  -> tui utilities/types

coding-agent
  -> ai
  -> agent-core
  -> tui

agent-core
  -> ai

tui
  -> standalone terminal/component primitives

ai
  -> provider adapters, streaming, OAuth, model/image registries
```

The extension examples sit beside the product. They are not core packages, but they are critical compatibility fixtures because they demonstrate supported extension surfaces.

`web-ui` does not depend on `pi-agent-core` in package metadata. It has its own browser-side agent interface, storage, dialogs, and tool/rendering utilities built around the same provider concepts from `pi-ai`.

## 3. `packages/ai`: Provider-Neutral AI Layer

### Role

`packages/ai` defines the model/provider abstraction used everywhere else. It normalizes chat, reasoning, tool calls, streaming events, image content, OAuth credentials, model metadata, and generated provider registries.

This package is the bottom of the AI stack. Higher layers should not know provider-specific wire formats unless explicitly installing/registering providers.

### Important Files

- `src/index.ts`: public exports.
- `src/types.ts`: central type model for messages, context, models, tools, provider APIs, stream events.
- `src/stream.ts`: common stream helpers.
- `src/api-registry.ts`: provider API registration/resolution.
- `src/models.ts`: model registry helpers.
- `src/models.generated.ts`: generated model metadata.
- `src/images.ts`, `src/images.generated.ts`: image model/provider registry.
- `src/oauth.ts`: OAuth entry point.
- `src/providers/*`: concrete provider implementations.
- `src/utils/*`: shared parsing, validation, diagnostics, Unicode sanitation, proxy, event stream, OAuth helpers.

### Provider Implementations

Provider files include:

- `anthropic`
- `openai`
- `openai-responses`
- `openai-codex`
- `google`
- `google-vertex`
- `bedrock`
- `mistral`
- `cloudflare`
- `github-copilot`
- `faux`
- image providers under `providers/images`

Provider adapters are responsible for translating the common `Context` and tool schema model into provider-specific payloads and translating provider events back into the common stream shape.

### Core Concepts

The central abstractions are:

- `Model`: provider/model metadata, mode support, context limits, capabilities.
- `Context`: the provider request context, including messages, tools, thinking controls, cache policy, session identity, headers, transport, and callbacks.
- `Tool`: normalized tool definition schema passed to providers.
- `AssistantMessageEventStream`: async stream of normalized assistant/tool/thinking events.
- `stream`, `complete`, `streamSimple`, `completeSimple`: convenience APIs over registered providers.
- API registry: maps provider IDs to runtime implementations.

### Streaming Contract

The stream contract matters because the agent loop depends on it:

- Provider adapters emit normalized events.
- Tool calls are represented as structured events, not textual scraping.
- Thinking/reasoning blocks are first-class when a provider supports them.
- Errors after a stream starts are encoded in the event stream where possible.
- Providers should avoid throwing after yielding begins.

### OAuth/Auth Surface

`ai` includes OAuth support for provider credentials and provider-specific OAuth flows. Files include provider OAuth implementations and helpers for PKCE, local callback handling, provider metadata, and token storage integration points.

The coding-agent layer consumes this through auth storage and login UI/commands. The lower layer supplies the credential-flow mechanics.

### Image Generation

Image generation is not bolted onto the TUI. It is part of `ai`:

- image model registry
- image provider registry
- image generation types
- OpenRouter/image provider support

This matters for `rho` because image support belongs in the model/provider layer first, then presentation layers can expose it.

### Test Coverage Themes

Tests cover:

- provider compatibility
- SSE/event parsing
- OpenAI/Anthropic/Google/Mistral/Bedrock behavior
- thinking/reasoning payloads
- OAuth flows
- tool-call ID normalization
- image generation
- model registry behavior
- validation and diagnostics
- cache retention and cross-provider handoff

## 4. `packages/agent`: Generic Agent Core

### Role

`packages/agent` is a generic agent runtime built on `pi-ai`. It does not own the terminal app. It owns the loop semantics: send messages to a provider, stream assistant output, execute requested tools, continue turns, emit lifecycle events, compact/session harness support.

### Important Files

- `src/agent-loop.ts`: core stream -> tool execution -> continuation loop.
- `src/agent.ts`: stateful Agent wrapper with subscriptions, queues, abort/follow-up steering.
- `src/types.ts`: event, tool, hook, and loop type definitions.
- `src/harness/*`: reusable harness pieces: sessions, compaction, system prompt, prompt templates, skills, storage, environment utilities.

### Agent Loop

The loop is event-driven. It emits events such as:

- `agent_start`
- `agent_end`
- `turn_start`
- `turn_end`
- `message_start`
- `message_update`
- `message_end`
- `tool_execution_start`
- `tool_execution_update`
- `tool_execution_end`

The loop coordinates:

- provider streaming
- assistant message assembly
- tool-call detection
- tool execution
- parallel or sequential tool policy
- continuation after tool results
- termination policy
- hooks before/after tool calls
- abort behavior
- turn-level stop/continue decisions

### Agent Class

The `Agent` wrapper adds statefulness over the loop:

- event subscription
- message queueing
- follow-up submission
- steering/interruption
- abort handling
- lifecycle management

This gives higher layers a stable API without embedding loop state directly into UI code.

### Harness

The harness is a reusable layer around the core loop:

- compaction
- session persistence
- JSONL/memory storage
- system prompt building
- prompt templates
- skills
- environment/node helpers
- shell output truncation

In `rho`, analogous logic should not be hidden in `cmd/rho/interactive.go`. It belongs in shared agent/session services.

### Test Coverage Themes

Tests cover:

- agent loop behavior
- agent class behavior
- end-to-end tool continuation
- session storage
- compaction
- system prompt construction
- harness storage and truncation

## 5. `packages/tui`: Terminal Component Library

### Role

`packages/tui` is a standalone terminal UI library. It provides raw terminal handling, differential rendering, overlays, focus, input parsing, editor behavior, autocomplete, selectable/settings lists, markdown rendering, keybindings, image protocols, and utilities.

It is not the coding agent. The coding agent builds an interactive application on top of it.

### Important Files

- `src/tui.ts`: root engine, render loop, overlays, focus, scheduling.
- `src/terminal.ts`: terminal raw mode, screen control, protocol support.
- `src/components/editor.ts`: multi-line editor.
- `src/input.ts`: input/key parsing.
- `src/select-list.ts`: selectable list component.
- `src/settings-list.ts`: settings editor component.
- `src/autocomplete.ts`: autocomplete and fuzzy matching.
- `src/markdown.ts`: terminal markdown renderer.
- `src/keybindings.ts`, `src/keys.ts`: keybinding manager and matching.
- `src/terminal-image.ts`: Kitty/iTerm2 terminal image support.
- `src/stdin-buffer.ts`: buffered stdin reader.
- `src/utils.ts`, `src/fuzzy.ts`, `src/kill-ring.ts`, `src/undo-stack.ts`: support utilities.

### Design Role

The terminal library supplies primitives:

- components
- containers
- focus
- overlays
- key events
- rendering
- lifecycle/disposal

The coding-agent interactive mode supplies product behavior:

- slash commands
- provider/login selectors
- session/model/theme/settings selectors
- chat transcript rendering
- tool execution components
- status/footer/header widgets
- extension UI surfaces

See the companion TUI map for detailed interactive behavior and porting implications.

### Test Coverage Themes

Tests cover:

- editor behavior
- autocomplete
- overlays
- differential rendering
- keybindings
- select/settings lists
- markdown/wrapping/truncation
- terminal image handling

## 6. `packages/coding-agent`: Product Coordinator

### Role

`packages/coding-agent` is the main product package. It turns the lower-level AI, agent, and TUI packages into a usable coding agent with CLI modes, sessions, tools, settings, auth, resources, extensions, prompt templates, skills, themes, package management, export, diagnostics, and SDK access.

This is the largest and most important package to map for `rho`.

### CLI Entry

Key files:

- `src/cli.ts`
- `src/main.ts`

`cli.ts` performs process-level setup and delegates to `main`.

`main.ts` owns:

- argument parsing
- stdin/file/image argument handling
- cwd/project resolution
- service creation
- model/provider resolution
- session/runtime setup
- tool setup
- extension loading
- mode dispatch

### Product Modes

The package supports multiple modes over the same core:

- interactive TUI mode
- print mode
- JSON mode
- RPC mode
- SDK/programmatic mode

The critical design point is that these are not separate apps. They are different frontends over shared session/runtime services.

### Print Mode

Print mode runs one-shot prompts. It still uses the session/runtime layer:

- creates/binds an agent session
- applies extensions
- streams/collects events
- renders text or structured output
- persists when configured

Print mode is therefore a compatibility check for core agent behavior independent of the TUI.

### JSON Mode

JSON mode is pipe-friendly. It accepts structured input and returns structured output. It is closer to automation than interactive chat.

Porting implication: user-facing TUI affordances should not be the only way to exercise the agent.

### RPC Mode

RPC mode exposes the coding agent through a JSON-RPC/JSONL protocol. It supports external integrations and extension UI bridging.

Important behavior:

- session lifecycle exposed over protocol
- requests/responses/events are structured
- extension UI requests can be bridged through JSON rather than terminal overlays
- errors must be protocol-safe

RPC mode is a major reason product logic must live below the TUI.

### SDK Mode

The public SDK is centered around `createAgentSession()` in `core/sdk.ts`.

The SDK exports:

- session creation
- built-in tool factories
- core types/options
- programmatic agent session control

The SDK implies that session/runtime behavior must be usable without CLI globals and without terminal assumptions.

## 7. Coding-Agent Core Runtime

### `core/agent-session.ts`

This is the central shared lifecycle abstraction. It coordinates:

- current messages and session state
- persistence
- model and thinking settings
- compaction
- tools
- bash mode/permissions
- system prompt/resource context
- prompt templates
- skills
- extension binding
- event subscription
- turn execution
- session switching/forking/import/resume support

In a Go port, this is the kind of object that should sit behind interactive, print, JSON, RPC, and SDK modes.

### `core/agent-session-runtime.ts`

The runtime owns the current session and services. It supports:

- creating sessions
- resuming sessions
- importing sessions
- switching sessions
- forking sessions
- rebinding UI/extension contexts
- lifecycle notifications

The runtime is the bridge between application-level commands and concrete session instances.

### `core/agent-session-services.ts`

Services are cwd-bound and application-bound:

- auth storage
- settings manager
- model registry
- resource loader
- diagnostics
- extension provider registration

This file is important because it prevents every mode from manually constructing scattered services.

### `core/sdk.ts`

The SDK factory wraps the same services/session model for programmatic use.

Porting implication: if `rho` has SDK aspirations, avoid baking behavior into CLI-only code.

### Other Core Files

Relevant core files include:

- `auth-storage.ts`: API keys and OAuth credential persistence.
- `model-registry.ts`: available models and provider registrations.
- `model-resolver.ts`: resolves requested/default model/provider combinations.
- `settings-manager.ts`: scoped settings.
- `session-manager.ts`: session listing/loading/saving/import/forking.
- `resource-loader.ts`: project/user/default resources.
- `system-prompt.ts`: prompt assembly.
- `prompt-templates.ts`: template loading/selection.
- `skills.ts`: skills loading and selection.
- `telemetry.ts`: pi-side telemetry/diagnostic hooks where present.
- `timings.ts`, `diagnostics.ts`: runtime observability.
- `footer-data-provider.ts`: shared footer state for UI.
- `provider-display-names.ts`: display labels for providers.
- `session-cwd.ts`, `source-info.ts`: context/source metadata.

## 8. Built-In Tools

### Tool Package

Tool files live under `packages/coding-agent/src/core/tools`.

Built-ins include:

- `read`
- `write`
- `edit`
- `bash`
- `grep`
- `find`
- `ls`

The defaults distinguish mutation tools from read-only tools. Tool definitions include:

- schema
- execution function
- rendering metadata
- prompt snippets/guidelines
- permission behavior
- diagnostics

### Read Tool

The read tool handles:

- text files
- images
- truncation
- syntax highlighting
- special labels for docs/skills/context
- non-vision fallbacks
- image resizing and MIME handling

The read tool is not just `cat`.

### Edit Tool

The edit tool handles:

- exact text replacement
- diff preview/rendering
- line ending preservation
- mutation queue coordination
- failure messages when target text is ambiguous or absent

The edit tool is not just "write the new file." Its UX and error behavior affect agent reliability.

### Bash Tool

The bash tool sits inside a permission and output-management framework:

- command execution
- shell integration
- truncation/guarding
- user bash hooks
- sandbox/permission extension hooks
- rendered execution state

### File Mutation Queue

The tool system includes mutation queueing to avoid conflicting writes. This is a core behavior for coding agents, especially when multiple edits or tools may target the same file in one turn.

## 9. Resources: Context, Skills, Themes, Templates, Settings

### Resource Loader

`DefaultResourceLoader` loads resources from default, user, and project scopes.

It handles:

- AGENTS/CLAUDE-style context files
- skills
- prompt templates
- themes
- extensions
- package-installed resources
- ancestor project directories
- agent directory resources

The loader is not just filesystem discovery. It defines precedence and scope.

### Settings

Settings are scoped and feed:

- default model/provider
- thinking level
- permissions
- UI preferences
- theme
- package/resource behavior
- session behavior

The interactive settings selector edits settings, but settings themselves live below UI.

### Skills

Skills use YAML/frontmatter metadata and context loading. They affect system prompts and available behavior.

Important distinction: skills are not slash commands. They are prompt/context resources with metadata and selection behavior.

### Prompt Templates

Prompt templates are first-class resources, loaded and selected independently from sessions and slash commands.

### Themes

Themes define terminal UI color/style behavior and can be loaded from resources/packages.

## 10. Extensions

### Role

Extensions are a major product axis. They are not examples only. The core package supports extension loading, lifecycle hooks, tools, slash commands, keybindings, CLI flags, providers, message renderers, and UI contributions.

### Extension Core Files

Relevant files live under:

- `packages/coding-agent/src/core/extensions`

Important areas:

- type definitions
- runtime
- loader
- wrapper/binding
- runner/invocation
- UI context
- package/resource integration

### Lifecycle Hooks

The extension system supports hooks such as:

- session start/shutdown
- agent start/end
- turn start/end
- context injection
- before provider request
- before agent start
- input modification
- tool call/result
- user bash

Exact naming varies by implementation file, but this is the behavioral surface the examples exercise.

### Extension Contributions

Extensions can provide:

- custom tools
- slash commands
- keybindings
- CLI flags
- provider APIs
- OAuth providers
- message renderers
- context providers
- UI widgets
- overlays/dialogs
- custom editors/autocomplete behavior

### Extension UI Context

The extension UI context supports terminal-appropriate interaction primitives:

- select
- confirm
- input
- notify
- status widgets
- header/footer/title contributions
- custom overlays
- custom editor surfaces
- autocomplete
- editor component integration

This is why the interactive TUI needs real overlay/focus/component behavior. `/login`, `/theme`, selectors, extension overlays, and command pickers all use the same class of interaction.

### Example Extensions

The examples are compatibility documentation. They demonstrate:

- hello/weather tools
- slash commands
- Q&A flows
- plan mode
- subagent/handoff patterns
- dynamic tools
- dynamic providers
- dynamic resources
- permission guards
- SSH/user bash interception
- sandbox behavior
- custom editor/header/footer/status widgets
- custom overlays
- games and modal interactions

When porting, examples reveal edge cases that core docs may not state.

## 11. Interactive Mode

Interactive mode lives in `packages/coding-agent/src/modes/interactive` and uses both:

- `packages/tui`
- `packages/coding-agent/src/core/ui`

It includes:

- transcript rendering
- user/assistant message components
- bash/tool execution components
- footer/header/status
- model/session/theme/settings selectors
- login/OAuth dialogs
- extension UI surfaces
- keybinding hints
- custom message/renderers
- dynamic border and visual truncation

This is the area mapped deeply in `docs/pi-interactive-tui-map.md`.

The key conclusion from the deeper map: slash commands, `/login`, model/provider selection, settings, sessions, and extensions are interactive component flows, not plain prompt strings.

## 12. RPC Mode

RPC mode lives under `packages/coding-agent/src/modes/rpc`.

It provides structured integration for external clients:

- request parsing
- response/event output
- session control
- agent turns
- extension UI bridging
- error handling

RPC mode prevents the app from becoming terminal-only internally. UI requests must be abstract enough to be rendered by terminal, browser, or protocol clients.

## 13. Print/JSON Modes

Print and JSON modes live under `packages/coding-agent/src/modes`.

They exercise the same session and extension machinery without an interactive terminal. They are important regression tests for:

- model resolution
- prompt construction
- session persistence
- tool execution
- extension hooks
- output rendering
- non-interactive auth/model failures

For `rho`, a feature implemented only in interactive mode is likely incomplete if print/json/rpc should also see it.

## 14. Package Manager

The coding-agent package includes package management logic:

- install/list/remove/update package resources
- npm/git/local package sources
- package resource discovery
- CLI helpers
- migration/version behavior

Package resources can include extensions, skills, prompt templates, themes, or related assets. This makes the resource loader and package manager part of the same system.

## 15. Sessions

Sessions are not just chat logs. They include:

- message history
- model/provider state
- cwd/source metadata
- tool results
- compaction state
- persistence format
- import/export
- resume/fork/switch workflows

Docs under `packages/coding-agent/docs` cover session format and session workflows. Runtime code handles both interactive selection and non-interactive persistence.

## 16. Compaction

Compaction appears both in agent harness concepts and coding-agent core. It handles context-window management:

- when to compact
- what to summarize
- what to preserve
- how session messages are rewritten
- how tools/context/resources survive compaction

Compaction is a cross-cutting concern. It affects prompt building, session persistence, and model-context limits.

## 17. Auth and Login

Auth spans multiple layers:

- `ai`: OAuth mechanics and provider auth support.
- `coding-agent`: credential storage, provider login commands/UI, model/provider resolution.
- interactive UI: login dialogs/provider selectors/OAuth selectors.
- print/json/rpc: non-interactive auth failure or protocol-visible auth needs.

The `/login` behavior in pi is therefore not "send a message containing `/login`." It is a command flow that opens provider/OAuth/API-key interaction surfaces and updates stored credentials.

## 18. Model Registry and Resolver

Model handling spans:

- generated model metadata in `ai`
- provider API registry in `ai`
- coding-agent model registry
- coding-agent model resolver
- settings/default model/provider
- interactive model selector
- CLI flags
- extension-provided models/providers

Important behaviors:

- provider and model can be selected separately.
- defaults can come from settings.
- models have capability metadata.
- thinking/reasoning settings interact with model capabilities.
- custom providers/extensions can alter availability.

## 19. Web UI Package

### Role

`packages/web-ui` is a browser component package. It is not the terminal app, but it demonstrates that pi’s agent experience is intended to be frontend-agnostic.

### Important Files

- `src/index.ts`: exports.
- `src/ChatPanel.ts`: top-level browser chat panel.
- `src/components/*`: browser chat/message/editor components.
- `src/dialogs/*`: API key, model selector, settings, sessions, custom provider dialogs.
- `src/storage/*`: IndexedDB/app storage/provider keys/sessions/settings.
- `src/tools/*`: browser tool/rendering utilities, artifacts, JS REPL, extract-document.
- `src/sandbox/*`: sandbox runtime providers.
- `src/utils/*`: attachments, auth tokens, i18n, model discovery, proxy, formatting.

### ChatPanel Architecture

`ChatPanel` wires together:

- browser-side agent interface
- message/editor/input components
- artifacts panel
- storage
- model/provider dialogs
- responsive layout
- sandbox providers
- artifact renderers

This package proves that the same conceptual product can be surfaced outside a terminal. The terminal UI should not own agent semantics.

### Storage

Web UI storage uses browser persistence:

- IndexedDB backend
- app storage abstraction
- provider key storage
- session storage
- settings storage
- custom provider storage

This differs from CLI filesystem storage but maps to the same conceptual entities.

## 20. Documentation Inventory

`packages/coding-agent/docs` covers:

- usage
- providers
- models
- sessions
- session format
- settings
- extensions
- skills
- prompt templates
- packages
- themes
- RPC
- SDK
- JSON mode
- keybindings
- compaction
- custom providers
- terminal setup
- platform notes for Windows, tmux, Termux, and similar environments

These docs should be treated as behavioral references, not just marketing text. They describe the surfaces `rho` needs to match or consciously diverge from.

## 21. Testing Map

### AI Tests

AI tests validate:

- provider payload compatibility
- streaming/SSE parsing
- thinking/reasoning behavior
- OAuth
- image generation
- registry behavior
- validation and diagnostics

### Agent Tests

Agent tests validate:

- loop continuation
- tool execution ordering
- event emission
- Agent wrapper state behavior
- harness sessions
- compaction
- system prompts

### TUI Tests

TUI tests validate:

- editor features
- autocomplete
- overlays/focus
- keybindings
- select/settings components
- markdown/wrapping
- terminal images
- rendering behavior

### Coding-Agent Tests

Coding-agent tests validate:

- agent sessions
- settings
- sessions/import/export
- model resolution
- auth
- extensions
- tools
- resource loading
- package manager
- print/json/rpc modes
- interactive UI components

### Web UI Tests/Checks

Web UI checks validate:

- browser bundle/component behavior
- storage/components/dialogs
- smoke behavior

## 22. Porting Implications for `rho`

### Do Not Collapse Product Logic Into TUI

The pi structure strongly separates:

- provider APIs
- generic agent loop
- coding-agent session/runtime
- presentation modes
- terminal components

If `rho` puts slash commands, login, model selection, settings, and tool execution semantics directly in `cmd/rho/interactive.go`, it will be hard to support print/json/rpc/sdk consistently.

### Shared Session Runtime Is Central

The most important target is a shared session/runtime layer equivalent to pi’s coding-agent core:

- mode-independent session object
- mode-independent model/auth/settings/resource resolution
- mode-independent tool registry
- mode-independent extension hooks
- mode-independent event stream

The TUI should subscribe to and command that runtime.

### Slash Commands Are Product Commands

Slash commands are not plain chat messages. They are registered command definitions with:

- names/aliases
- descriptions
- argument behavior
- command handlers
- UI flows where needed
- extension contributions

The autocomplete list and command picker are presentation details over that registry.

### Login Is a Flow

`/login` should drive:

- provider selection
- OAuth/API-key path selection
- provider-specific credential flow
- credential storage
- feedback/status
- model availability refresh

It should be callable from interactive UI and meaningful in non-interactive modes as an auth command or failure path.

### Extensions Need Abstract UI Primitives

Extensions should request interactions through abstract UI APIs such as select/confirm/input/notify/status/custom overlay. The terminal can render those with overlays. RPC can serialize them. A future web frontend can render dialogs.

### Model/Provider Registry Needs Capability Metadata

Model choice is not a string dropdown only. The registry should carry:

- provider
- model ID
- display name
- reasoning/thinking support
- context limits
- tool support
- image support
- auth requirements
- custom provider data

### Resource Loading Defines Product Behavior

AGENTS files, skills, prompt templates, themes, packages, and extensions are resources. They should be loaded with scope and precedence, not by one-off path checks.

### Tools Need Rendering and Prompt Metadata

Tools should not only expose an execution function. Pi’s tools include:

- schema
- prompt guidance
- renderers
- permission behavior
- mutation coordination
- truncation/diff behavior

The UI renders tool state, but the tool definition carries part of that contract.

### Multiple Modes Are a Regression Guard

Any behavior that exists only in interactive mode should be questioned. The same underlying runtime should support:

- interactive
- print
- json
- rpc
- sdk

Different modes can render differently, but the agent semantics should not fork.

## 23. Suggested `rho` Mapping Targets

To align `rho` with pi, map or build these layers in order:

1. Provider registry and model capability metadata.
2. Shared agent/session runtime independent of CLI/TUI.
3. Tool registry with execution, rendering metadata, and mutation coordination.
4. Resource loader for project/user/default context, skills, templates, themes, extensions.
5. Settings and auth stores with scoped resolution.
6. Slash command registry shared across modes.
7. Extension runtime and abstract UI context.
8. Interactive TUI as a subscriber/controller over shared runtime.
9. Print/json/rpc modes over the same runtime.
10. SDK surface over the same runtime.

This order follows pi’s architecture more closely than starting from terminal rendering.

## 24. High-Risk Gaps To Watch In `rho`

These are the places where a Go port can appear functional while missing pi’s product behavior:

- `/` only inserts a character instead of opening command autocomplete.
- `/login` is treated as a prompt rather than a command flow.
- provider/model selection exists only as CLI flags.
- tools execute but do not emit renderable lifecycle events.
- sessions persist text only, losing metadata and tool state.
- settings are global only, with no scoped precedence.
- extensions can add tools but not UI/keybindings/commands/providers.
- print/json/rpc bypass extension hooks.
- compaction rewrites messages without preserving tool/resource context.
- resource loading ignores ancestor/project/user/default precedence.
- model registry lacks capability/auth/thinking metadata.
- terminal UI owns app semantics instead of rendering shared events.

## 25. Reference File Index

Use this as a starting index when checking pi behavior:

```text
pi/package.json
pi/README.md

pi/packages/ai/src/index.ts
pi/packages/ai/src/types.ts
pi/packages/ai/src/api-registry.ts
pi/packages/ai/src/models.ts
pi/packages/ai/src/stream.ts
pi/packages/ai/src/oauth.ts
pi/packages/ai/src/providers/*
pi/packages/ai/src/utils/*

pi/packages/agent/src/agent-loop.ts
pi/packages/agent/src/agent.ts
pi/packages/agent/src/types.ts
pi/packages/agent/src/harness/*

pi/packages/tui/src/tui.ts
pi/packages/tui/src/terminal.ts
pi/packages/tui/src/components/editor.ts
pi/packages/tui/src/autocomplete.ts
pi/packages/tui/src/select-list.ts
pi/packages/tui/src/settings-list.ts
pi/packages/tui/src/keybindings.ts
pi/packages/tui/src/keys.ts

pi/packages/coding-agent/src/cli.ts
pi/packages/coding-agent/src/main.ts
pi/packages/coding-agent/src/core/sdk.ts
pi/packages/coding-agent/src/core/agent-session.ts
pi/packages/coding-agent/src/core/agent-session-runtime.ts
pi/packages/coding-agent/src/core/agent-session-services.ts
pi/packages/coding-agent/src/core/tools/*
pi/packages/coding-agent/src/core/extensions/*
pi/packages/coding-agent/src/core/resource-loader.ts
pi/packages/coding-agent/src/core/settings-manager.ts
pi/packages/coding-agent/src/core/session-manager.ts
pi/packages/coding-agent/src/core/model-registry.ts
pi/packages/coding-agent/src/core/model-resolver.ts
pi/packages/coding-agent/src/modes/interactive/*
pi/packages/coding-agent/src/modes/print-mode.ts
pi/packages/coding-agent/src/modes/rpc/*
pi/packages/coding-agent/docs/*
pi/packages/coding-agent/examples/*

pi/packages/web-ui/src/index.ts
pi/packages/web-ui/src/ChatPanel.ts
pi/packages/web-ui/src/components/*
pi/packages/web-ui/src/dialogs/*
pi/packages/web-ui/src/storage/*
pi/packages/web-ui/src/tools/*
pi/packages/web-ui/src/utils/*
```

## 26. Bottom Line

Pi is a multi-mode coding-agent platform. The terminal UI is only one frontend.

The architecture to preserve in `rho` is:

```text
providers/models/images/oauth
  -> generic agent loop
  -> coding-agent session/runtime/services
  -> tools/resources/settings/auth/extensions
  -> interactive / print / json / rpc / sdk frontends
```

The practical consequence: before changing the TUI, map which shared runtime behavior it should call. A fully interactive `/` command picker and `/login` dialog are symptoms of the deeper design: commands, auth, providers, resources, and extension UI are all registered product surfaces rendered by the active frontend.
