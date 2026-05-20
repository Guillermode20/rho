# pi Interactive TUI Map

This is a porting map for pi's interactive terminal UI. It is intentionally detailed because rho needs to mirror the shape of the app, not only a few commands. The main conclusion: pi's interactive mode is a component-driven terminal application with a rich editor, editor-embedded autocomplete, focused selector/dialog replacement, and optional true overlays. A rho port that treats slash commands as plain submitted text is structurally wrong.

## Scope

Primary focus:

- `pi/packages/coding-agent/src/modes/interactive/**`
- `pi/packages/tui/src/**`
- TUI-facing core files under `pi/packages/coding-agent/src/core/**`

Covered flows:

- Root TUI layout
- Component/focus/rendering model
- Editor behavior
- `/` autocomplete
- Slash command dispatch
- Login/logout provider UI
- Model/settings/session selectors
- Extension UI bridge
- Auth/model/session state synchronization
- Porting implications for rho

## High-Level Architecture

pi interactive mode has four layers:

1. Terminal engine: `@earendil-works/pi-tui`
2. App-level interactive shell: `InteractiveMode`
3. Stateful agent/session services: `AgentSession`, `AgentSessionRuntime`, `ModelRegistry`, `AuthStorage`, `SettingsManager`, `ResourceLoader`
4. Domain UI components: editor, selectors, message renderers, footer, dialogs, tool execution views

The runtime is not "chat messages + prompt". It is:

- A differential renderer that owns the terminal.
- A component tree whose children render line arrays.
- A focus manager that sends raw key input to exactly one component.
- An editor component with autocomplete and app action hooks.
- A command router that opens interactive components.
- Session services that mutate model/auth/session state and cause UI rebuilds.

## File Index: Interactive Mode

### `modes/interactive/interactive-mode.ts`

The central app controller. Responsibilities:

- Creates `TUI(new ProcessTerminal())`.
- Creates root containers:
  - `headerContainer`
  - `chatContainer`
  - `pendingMessagesContainer`
  - `statusContainer`
  - `widgetContainerAbove`
  - `widgetContainerBelow`
  - `editorContainer`
- Creates `CustomEditor`.
- Creates `FooterComponent`.
- Wires keybindings and app actions.
- Builds and installs autocomplete provider.
- Handles editor submit.
- Dispatches built-in slash commands.
- Sends normal messages to `AgentSession.prompt()`.
- Renders messages, tools, loaders, status and warnings.
- Opens selectors/dialogs by replacing `editorContainer`.
- Exposes UI primitives to extensions.
- Rebinds UI when the session/runtime changes.

Important method clusters:

- Setup:
  - constructor/setup around TUI, containers, editor, footer
  - `setupAutocompleteProvider()`
  - `setupEditorSubmitHandler()`
  - app action registration via `defaultEditor.onAction(...)`
- Autocomplete:
  - `createBaseAutocompleteProvider()`
  - `getBuiltInCommandConflictDiagnostics()`
  - `prefixAutocompleteDescription()`
- Command submit:
  - slash command `if` chain in `setupEditorSubmitHandler()`
  - bash command handling
  - streaming/compaction behavior
  - normal prompt submission
- Selector/dialog helpers:
  - `showSelector(...)`
  - `showExtensionSelector(...)`
  - `showExtensionInput(...)`
  - `showExtensionEditor(...)`
  - `showExtensionCustom(...)`
- Auth:
  - `showOAuthSelector(...)`
  - `showLoginAuthTypeSelector()`
  - `showLoginProviderSelector(...)`
  - `showApiKeyLoginDialog(...)`
  - `showLoginDialog(...)`
  - `showOAuthLoginSelect(...)`
  - `completeProviderAuthentication(...)`
- Model/session:
  - `handleModelCommand(...)`
  - `showModelSelector(...)`
  - `showModelsSelector(...)`
  - `showUserMessageSelector()`
  - `showTreeSelector(...)`
  - `showSessionSelector()`

### `components/custom-editor.ts`

Thin app-specific subclass of `Editor`.

Responsibilities:

- Owns app keybindings, not low-level editor keybindings.
- Checks extension-registered shortcuts first.
- Handles image paste shortcut.
- Handles app interrupt only when autocomplete is not active.
- Handles app exit only when editor text is empty.
- Calls registered app action handlers for app-level keybindings.
- Falls back to base `Editor.handleInput()`.

This is important: app commands are not all parsed as text. Many app actions are keybinding-driven and run before base text editing.

### Message Components

Files:

- `components/assistant-message.ts`
- `components/user-message.ts`
- `components/tool-execution.ts`
- `components/bash-execution.ts`
- `components/custom-message.ts`
- `components/skill-invocation-message.ts`
- `components/branch-summary-message.ts`
- `components/compaction-summary-message.ts`

Role:

- Render session entries into the chat area.
- Keep tool execution and bash execution as interactive/renderable components, not raw text blocks.
- Support expanding/collapsing tool output.
- Support image display settings and visual truncation.

### Selector Components

Files:

- `components/model-selector.ts`
- `components/scoped-models-selector.ts`
- `components/settings-selector.ts`
- `components/session-selector.ts`
- `components/session-selector-search.ts`
- `components/tree-selector.ts`
- `components/user-message-selector.ts`
- `components/oauth-selector.ts`
- `components/extension-selector.ts`
- `components/show-images-selector.ts`
- `components/thinking-selector.ts`
- `components/theme-selector.ts`
- `components/config-selector.ts`

Common pattern:

- Implement `Component`, usually `Focusable`.
- Own an `Input` or list.
- Render a bordered region in the editor area.
- Handle up/down/enter/escape.
- Call callbacks on select/cancel.
- Ask `TUI` to re-render after state changes.

### Login Components

Files:

- `components/oauth-selector.ts`
- `components/login-dialog.ts`

`OAuthSelectorComponent`:

- Searchable provider picker.
- Shows configuration status.
- Supports login and logout modes.
- Uses embedded `Input` for fuzzy filtering.

`LoginDialogComponent`:

- Replaces editor during API-key or OAuth login.
- Shows URL, prompt, manual redirect input, waiting/progress, or info.
- Owns an `AbortController`.
- Propagates focus to inner `Input`.
- Cancels on select cancel.

### Footer And Status

File: `components/footer.ts`

Footer shows:

- cwd and git branch
- session name
- cumulative input/output/cache token usage
- cumulative cost
- subscription indicator
- context window percent
- model and provider
- thinking level
- extension statuses

It reads from:

- `AgentSession.state`
- `SessionManager`
- `FooterDataProvider`
- `ModelRegistry.isUsingOAuth(model)`

After auth/model changes, interactive mode invalidates footer and refreshes provider counts.

## File Index: pi-tui

### `tui.ts`

Core terminal UI engine.

Exports:

- `Component`
- `Focusable`
- `Container`
- `OverlayOptions`
- `OverlayHandle`
- `TUI`
- cursor marker helpers

Responsibilities:

- Owns terminal.
- Owns root component children.
- Owns focused component.
- Owns input listeners.
- Owns overlay stack.
- Schedules throttled renders.
- Does differential rendering.
- Tracks terminal width/height, previous lines, cursor rows.
- Handles terminal image cleanup.
- Handles hardware cursor marker.

Input dispatch:

1. `terminal.start((data) => this.handleInput(data), ...)`
2. `handleInput()` consumes terminal cell-size responses.
3. Global debug key can intercept.
4. If focus is on a now-hidden overlay, focus is repaired.
5. Key release events are filtered unless component wants them.
6. Focused component receives `handleInput(data)`.
7. TUI requests render.

Focus:

- `setFocus(component)` clears old focus if old is `Focusable`.
- Sets new `focused = true` if component is `Focusable`.
- Components emit cursor markers only when focused.

Overlay stack:

- `showOverlay(component, options)` pushes an entry:
  - component
  - options
  - previous focus
  - hidden flag
  - focus order
- If capturing and visible, focuses overlay.
- Returns handle:
  - `hide()`
  - `setHidden(hidden)`
  - `isHidden()`
  - `focus()`
  - `unfocus()`
  - `isFocused()`
- `compositeOverlays()` renders base content then overlays positioned by width, max height, anchors, margins, row/col and visibility callbacks.

Most built-in pi selectors do not use true overlays. They replace the editor area. Overlays are used by extension custom UI and any component that should visually float over base content.

### `components/editor.ts`

Rich multi-line editor. Key responsibilities:

- Multi-line text state.
- Cursor line/column.
- Wrapping and scrolling.
- Undo snapshots with coalescing.
- kill-ring/yank-like editing.
- history navigation.
- bracketed paste handling.
- large paste marker handling.
- `@` file attachment completion.
- slash command completion.
- forced file completion on Tab.
- fake cursor and hardware cursor marker.
- embedded autocomplete list rendering.

Autocomplete state:

- `autocompleteProvider?: AutocompleteProvider`
- `autocompleteList?: SelectList`
- `autocompleteState: "regular" | "force" | null`
- `autocompletePrefix`
- abort/debounce/request token fields

Render behavior:

- Renders top border.
- Renders visible wrapped editor lines.
- Renders bottom border.
- If autocomplete is active, renders `SelectList` directly below the editor.

Input behavior:

- If autocomplete active:
  - Escape cancels.
  - Up/down move `SelectList`.
  - Tab applies selected item and stays in editor.
  - Enter applies selected item.
  - If selected prefix starts with `/`, Enter falls through to submit.
- Tab without autocomplete:
  - slash command context -> slash completion
  - otherwise forced file completion
- On character insert:
  - `/` at start of message triggers autocomplete.
  - `@` or `#` at token boundary triggers autocomplete.
  - letters/digits in slash or symbol completion context update autocomplete.

Important slash context checks:

- `isInSlashCommandContext(textBeforeCursor)`
- `isSlashMenuAllowed()`
- `isAtStartOfMessage()`

### `autocomplete.ts`

Defines completion contracts and provider.

Types:

- `SlashCommand`
- `AutocompleteItem`
- `AutocompleteSuggestions`
- `AutocompleteProvider`

`CombinedAutocompleteProvider` responsibilities:

- Slash command completion.
- Slash command argument completion.
- `@` file completion.
- normal/forced path completion.
- fuzzy filtering.
- `fd`-backed filesystem walking.

Slash behavior:

- If text before cursor starts with `/` and contains no space:
  - build command items from registered commands
  - fuzzy filter by command name
  - return prefix as the whole slash text, like `/log`
- If text is `/command args`:
  - find command
  - call `getArgumentCompletions(argumentText)` if present
- `applyCompletion()` for command names writes `/${item.value} `.

### `components/select-list.ts`

Generic list used by autocomplete and some pickers.

Responsibilities:

- Stores all items and filtered items.
- Tracks selected index.
- Renders visible window with scrolling.
- Supports description column.
- Handles up/down/enter/escape.
- Calls `onSelect`, `onCancel`, `onSelectionChange`.

The editor's slash autocomplete uses this directly.

### `components/input.ts`

Single-line input used inside selectors/dialogs.

Used by:

- `OAuthSelectorComponent`
- `ModelSelectorComponent`
- `ExtensionInputComponent`
- `LoginDialogComponent`
- session selectors/searches

### `terminal.ts`, `stdin-buffer.ts`, `keys.ts`, `keybindings.ts`

Terminal/key infrastructure:

- raw terminal lifecycle
- Kitty keyboard protocol support
- key parse/match helpers
- configurable keybindings
- input buffering
- cursor and screen control

rho needs equivalent behavior if it wants pi-level interactivity.

## Core State Files That Matter To TUI

### `core/slash-commands.ts`

Defines the autocomplete metadata list:

- `settings`
- `model`
- `scoped-models`
- `export`
- `import`
- `share`
- `copy`
- `name`
- `session`
- `changelog`
- `hotkeys`
- `fork`
- `clone`
- `tree`
- `login`
- `logout`
- `new`
- `compact`
- `resume`
- `reload`
- `quit`

This file is not the command executor. It is the command catalog for autocomplete.

### `core/keybindings.ts`

Combines TUI keybindings with app keybindings.

Important app actions:

- `app.interrupt`
- `app.clear`
- `app.exit`
- `app.suspend`
- `app.thinking.cycle`
- `app.model.cycleForward`
- `app.model.cycleBackward`
- `app.model.select`
- `app.tools.expand`
- `app.thinking.toggle`
- `app.editor.external`
- `app.message.followUp`
- `app.message.dequeue`
- `app.clipboard.pasteImage`
- `app.session.new`
- `app.session.tree`
- `app.session.fork`
- `app.session.resume`
- tree filter/label actions
- session selector actions
- scoped model selector actions

This means command-like behavior is split across:

- slash command submit
- keyboard action handlers
- selector component input handling

### `core/auth-storage.ts`

Credential storage and resolution.

Storage:

- Default path: `getAgentDir()/auth.json`.
- Uses file locking via `proper-lockfile`.
- Ensures parent dir and file permissions.
- Supports in-memory backend for tests.

Credential types:

- `{ type: "api_key", key }`
- `{ type: "oauth", ...OAuthCredentials }`

Priority in `getApiKey(providerId)`:

1. Runtime override from CLI `--api-key`
2. API key from `auth.json`
3. OAuth token from `auth.json`, with refresh and lock coordination
4. Environment variable
5. Fallback resolver, usually custom provider config

Other behavior:

- `getAuthStatus(provider)` returns status without exposing secrets.
- `login(providerId, callbacks)` runs an OAuth provider login and stores result.
- `logout(provider)` removes stored credential.
- OAuth refresh uses async lock to avoid multiple pi instances racing.

### `core/model-registry.ts`

Model catalog and auth-aware model availability.

Responsibilities:

- Loads built-in models.
- Loads `models.json`.
- Applies provider/model overrides.
- Registers extension providers.
- Registers extension OAuth providers for `/login`.
- Tracks request configs for provider/model headers and API keys.
- Filters available models by configured auth.
- Resolves API key and headers for a model.
- Reports provider auth status for UI.
- Reports provider display names.
- Detects OAuth/subscription use.

Important methods:

- `refresh()`
- `getAvailable()`
- `find(provider, modelId)`
- `hasConfiguredAuth(model)`
- `getApiKeyAndHeaders(model)`
- `getProviderAuthStatus(provider)`
- `getProviderDisplayName(provider)`
- `getApiKeyForProvider(provider)`
- `isUsingOAuth(model)`
- `registerProvider(...)`
- `unregisterProvider(...)`

The login UI depends on this registry to:

- show provider display names
- show configured status
- refresh available models after auth
- select a default model if the current model was unknown/unavailable

### `core/agent-session-services.ts`

Creates cwd-bound services:

- `AuthStorage`
- `SettingsManager`
- `ModelRegistry`
- `ResourceLoader`

Then:

- reloads resources
- applies extension provider registrations to model registry
- applies extension flags
- returns diagnostics instead of directly printing/exiting

This matters because extension-provided providers can add models and OAuth support for `/login`.

### `core/agent-session-runtime.ts`

Owns current `AgentSession` and services.

Responsibilities:

- Switch session
- New session
- Fork session
- Import/resume session
- Emit extension session lifecycle events
- Teardown old session
- Create replacement runtime
- Rebind interactive UI to new session

Interactive mode relies on this for:

- `/new`
- `/resume`
- `/fork`
- `/clone`
- `/tree`
- `/import`

## Root Interactive Layout In Detail

The app is visually composed from containers rather than a monolithic string.

Core containers:

- Header: startup notices, app heading, warnings.
- Chat: rendered conversation entries.
- Pending messages: queued follow-ups or pending bash.
- Status: loaders, transient status lines, warnings/errors.
- Widget above/below: extension/plugin-driven UI placements.
- Editor: current editor or replacement selector/dialog.
- Footer: cwd, stats, context/model/provider/thinking info.

The editor is just one focused component in this structure. During modal flows, the editor is temporarily removed from its container.

Editor replacement matters because it keeps the user "in the same TUI" while changing the focused interaction. Example:

```ts
this.editorContainer.clear();
this.editorContainer.addChild(selector);
this.ui.setFocus(selector);
this.ui.requestRender();
```

Restore:

```ts
this.editorContainer.clear();
this.editorContainer.addChild(this.editor);
this.ui.setFocus(this.editor);
this.ui.requestRender();
```

## Slash Command Autocomplete Flow

### Data Sources

`InteractiveMode.createBaseAutocompleteProvider()` builds commands from:

1. `BUILTIN_SLASH_COMMANDS`
2. Prompt templates
3. Extension commands
4. Skills, if skill commands are enabled

It also adds argument completion for `/model`.

### Trigger

In `Editor.insertCharacter()`:

- If inserted char is `/` and the editor is at start of message, call `tryTriggerAutocomplete()`.
- If typing command characters while already in slash context, call `tryTriggerAutocomplete()` or `updateAutocomplete()`.

### Request

`requestAutocomplete()`:

- checks provider exists
- handles forced/file completion rules
- cancels previous autocomplete request
- increments request token
- optionally debounces symbol completions
- starts async request

`runAutocompleteRequest()`:

- calls provider
- checks request is still current using token, text snapshot, cursor snapshot and abort signal
- if no suggestions, clears UI
- if one forced file result, applies immediately
- otherwise calls `applyAutocompleteSuggestions()`

### Render

Editor stores:

- autocomplete prefix
- `SelectList`
- state regular/force

Then editor render appends list rows below the editor box.

### Selection

While autocomplete is open:

- Escape -> cancel
- Up/down -> list navigation
- Tab -> apply selection, do not submit
- Enter -> apply selection
- If prefix starts with `/`, after applying selection the editor falls through to submit

So `/` opens the command list immediately, and Enter on `/login` submits `/login ` or `/login` depending text state and dispatch path.

## Slash Command Dispatch Flow

`setupEditorSubmitHandler()` installs `defaultEditor.onSubmit = async (text) => { ... }`.

Flow:

1. Trim text.
2. If empty, return.
3. Check built-ins by exact string or prefix.
4. For built-ins:
   - open selector/dialog or run handler
   - clear editor
   - return
5. Check bash command `!` / `!!`.
6. If compaction active, queue or route extension command.
7. If streaming active, steer or queue.
8. Otherwise normal user message:
   - flush pending bash components
   - add to history
   - clear editor
   - call `session.prompt(text)`
   - update UI

Important: built-in slash commands are app-level UI commands, not agent prompts.

## Login Flow: Full Trace

### `/login`

1. User types `/`.
2. Editor shows slash autocomplete list.
3. User selects/types `/login`.
4. Submit handler detects `text === "/login"`.
5. Calls `showOAuthSelector("login")`.
6. Clears editor text.

### Auth type selection

`showOAuthSelector("login")` immediately calls `showLoginAuthTypeSelector()`.

`showLoginAuthTypeSelector()`:

- Creates `ExtensionSelectorComponent`.
- Title: `Select authentication method:`
- Options:
  - `Use a subscription`
  - `Use an API key`
- Uses `showSelector`, replacing editor area.
- On selection:
  - closes selector
  - maps selected option to `oauth` or `api_key`
  - calls `showLoginProviderSelector(authType)`
- On cancel:
  - restores editor

### Provider selection

`showLoginProviderSelector(authType)`:

- Calls `getLoginProviderOptions(authType)`.
- If no providers, show status and return.
- Creates `OAuthSelectorComponent`.
- Passes:
  - mode: `login`
  - auth storage
  - provider options
  - select callback
  - cancel callback
  - status callback from `modelRegistry.getProviderAuthStatus(providerId)`

`OAuthSelectorComponent`:

- Renders title.
- Renders search input.
- Renders filtered provider list.
- Displays status indicators:
  - configured
  - subscription configured
  - API key configured
  - env key
  - runtime API key
  - custom/fallback API key
  - models.json key
  - models.json command
  - unconfigured
- Supports fuzzy search.
- Up/down moves selected provider.
- Enter selects.
- Escape cancels/backtracks.

On provider select:

- If provider auth type is `oauth`: `showLoginDialog(provider.id, provider.name)`
- If provider is Bedrock: `showBedrockSetupDialog(...)`
- Else: `showApiKeyLoginDialog(provider.id, provider.name)`

### API key login dialog

`showApiKeyLoginDialog(providerId, providerName)`:

1. Saves previous model.
2. Creates `LoginDialogComponent`.
3. Replaces editor with dialog and focuses dialog.
4. Calls `dialog.showPrompt("Enter API key:")`.
5. Dialog renders prompt and input.
6. User enters key.
7. Stores:
   - `authStorage.set(providerId, { type: "api_key", key })`
8. Restores editor.
9. Calls `completeProviderAuthentication(providerId, providerName, "api_key", previousModel)`.

If key is empty, it errors. If cancelled, restore editor and suppress cancellation error.

### OAuth login dialog

`showLoginDialog(providerId, providerName)`:

1. Finds OAuth provider metadata from `authStorage.getOAuthProviders()`.
2. Determines whether provider uses callback server.
3. Creates `LoginDialogComponent`.
4. Replaces editor with dialog.
5. Creates manual code promise for callback/manual URL.
6. Calls `authStorage.login(providerId, callbacks)`.

Callbacks:

- `onAuth(info)`
  - `dialog.showAuth(info.url, info.instructions)`
  - tries to open browser
  - for callback providers, calls `dialog.showManualInput(...)`
  - for GitHub Copilot, shows waiting state
- `onPrompt(prompt)`
  - `dialog.showPrompt(prompt.message, prompt.placeholder)`
- `onProgress(message)`
  - `dialog.showProgress(message)`
- `onSelect(prompt)`
  - temporarily replaces dialog with `ExtensionSelectorComponent`
  - restores dialog after selection/cancel
- `onManualCodeInput()`
  - returns manual code promise
- `signal`
  - comes from dialog abort controller

On success:

- Restore editor.
- Call `completeProviderAuthentication(providerId, providerName, "oauth", previousModel)`.

### Completing auth

`completeProviderAuthentication(...)`:

- Calls `modelRegistry.refresh()`.
- If previous model was unknown/unavailable:
  - finds provider models
  - selects provider default model if available
  - otherwise tells user to use `/model`
- Updates available provider count.
- Invalidates footer.
- Updates editor border color.
- Shows status:
  - saved key/login complete
  - credential path
  - selected default model if applicable
- Emits/warns about Anthropic subscription usage if relevant.

This is the missing part in simple rho implementations: auth is tied to model availability, footer state, warnings and selected model.

## Logout Flow

`/logout`:

1. Submit handler calls `showOAuthSelector("logout")`.
2. Gets stored credential providers.
3. If none, status explains that logout only removes `/login` credentials.
4. Shows `OAuthSelectorComponent` in logout mode.
5. On select:
   - `authStorage.logout(providerId)`
   - `modelRegistry.refresh()`
   - update provider count
   - show status
6. On cancel, restore editor.

## Model Selector Flow

### `/model`

- Opens `ModelSelectorComponent`.
- It refreshes `modelRegistry`.
- Loads `modelRegistry.getAvailable()`.
- If scoped models are active, can toggle between scoped/all.
- Shows current model first.
- Search input filters by model id/provider/provider-id string.
- Enter selects highlighted model.
- Tab toggles scope if scoped models exist.
- Selection:
  - `session.setModel(model)`
  - footer invalidated
  - editor border updated
  - status updated
  - auth warnings checked

### `/model <query>`

- First tries exact model reference match.
- If found, selects directly.
- Otherwise opens selector with search prefilled.

### `/model` autocomplete argument completions

In `createBaseAutocompleteProvider()`:

- model command receives `getArgumentCompletions(prefix)`.
- Candidates come from scoped models if present, otherwise registry available models.
- Fuzzy filter by id and provider.
- Completion value is `provider/id`.
- Label is model id; description is provider.

## Settings Selector Flow

`/settings` opens `SettingsSelectorComponent`.

It mutates both persisted settings and live session/UI state:

- auto compaction toggles session and footer
- image settings update existing tool components
- skill command enablement rebuilds autocomplete
- steering/follow-up mode updates session
- transport updates settings and agent
- thinking level updates session/footer/border
- theme preview and theme persistence invalidate full UI
- hide thinking block rerenders assistant messages
- hardware cursor changes TUI behavior
- editor padding and autocomplete max visible update current editor
- clear-on-shrink updates TUI
- warning config updates settings

This means settings cannot be implemented as a static message list. It is a live control surface.

## Session/Tree/Fork/Resume Flow

Session-related commands use selectors and `AgentSessionRuntime`.

Commands:

- `/new`
- `/resume`
- `/fork`
- `/clone`
- `/tree`
- `/import`

Core:

- `AgentSessionRuntime` owns current session and services.
- Session replacement emits extension lifecycle events.
- It tears down current session before applying new runtime.
- It calls `rebindSession` so interactive mode points at the new session.

Selectors:

- `UserMessageSelectorComponent`: chooses message to fork from.
- `TreeSelectorComponent`: navigates session tree and can prompt for branch summarization.
- `SessionSelectorComponent`: lists/resumes sessions; includes search and management actions.

Tree navigation can chain selectors:

- tree selector
- summarize branch selector
- custom summary editor
- loader/status component while summarizing
- restore or rerender chat after navigation

## Extension UI Bridge

Interactive mode exposes UI to extensions. This is not optional if rho wants plugin parity.

Extension UI methods:

- select
- confirm
- input
- editor
- custom
- notify
- set status
- set editor text
- get editor text
- register autocomplete provider wrapper
- set custom editor component
- register shortcuts

Patterns:

- `showExtensionSelector()` replaces editor with `ExtensionSelectorComponent`.
- `showExtensionInput()` replaces editor with `ExtensionInputComponent`.
- `showExtensionEditor()` replaces editor with `ExtensionEditorComponent`.
- `showExtensionCustom()` can either replace editor or use `ui.showOverlay()`.

Extension custom component restoration:

- saves editor text
- restores editor component
- restores saved text
- returns focus to editor

Custom editor replacement:

- saves current editor text
- creates extension editor
- wires submit/change callbacks
- copies text and style options
- installs autocomplete provider if supported
- copies app action handlers if compatible
- replaces active editor

## Keybindings And Actions

Low-level TUI keybindings are in `pi-tui`.

App-level keybindings are in `core/keybindings.ts` and routed through `CustomEditor`.

Default app examples:

- Escape: interrupt/cancel or autocomplete cancel
- Ctrl+C: clear editor
- Ctrl+D: exit if editor empty
- Ctrl+Z: suspend
- Shift+Tab: cycle thinking
- Ctrl+P / Shift+Ctrl+P: cycle model
- Ctrl+L: open model selector
- Ctrl+O: expand tools
- Ctrl+T: toggle thinking blocks
- Ctrl+G: external editor
- Alt+Enter: queue follow-up
- Alt+Up: restore queued messages
- Ctrl+V or Alt+V: paste image depending platform

Selector-specific actions:

- tree fold/unfold/filter/label actions
- session selector path/sort/rename/delete actions
- scoped model selector save/enable/clear/provider/reorder actions

Important design point: command behavior is not only slash commands. The editor must support action hooks and selectors must support context-specific keybindings.

## Auth/Model Availability Coupling

pi only shows "available" models when auth exists.

`ModelRegistry.getAvailable()` filters models by:

- stored auth
- runtime override
- environment key
- fallback/custom config
- provider request config from models.json

Login changes auth, which changes model availability, which changes:

- `/model` selector contents
- autocomplete model arguments
- footer provider count
- selected model fallback
- warnings

So rho should not implement `/login` as only "save key". It must refresh registry and UI state after saving.

## State Synchronization Points

After model selection:

- `session.setModel(model)`
- `footer.invalidate()`
- `updateEditorBorderColor()`
- `showStatus(...)`
- subscription warning checks

After login:

- `modelRegistry.refresh()`
- maybe select provider default model
- `updateAvailableProviderCount()`
- `footer.invalidate()`
- `updateEditorBorderColor()`
- status/error/warning display

After settings changes:

- mutate settings/session/tool components
- rebuild autocomplete if skill command setting changes
- invalidate/rebuild UI where necessary

After session switch/fork/resume:

- old session shutdown
- new services/session
- rebind interactive mode
- rebuild chat and footer
- restore or set editor text where needed

## pi TUI Patterns To Port To Go

### Required Primitives

rho needs these before serious feature parity:

- `Component` interface returning rendered lines.
- `Focusable` interface.
- `Container`.
- `TUI` with raw terminal input, focus dispatch and differential rendering.
- `showOverlay`/overlay stack.
- Rich editor with:
  - multi-line state
  - cursor
  - history
  - undo
  - paste handling
  - autocomplete provider
  - embedded select list rendering
- `Input`.
- `SelectList`.
- app-level `CustomEditor` wrapper.
- selector replacement helper.

### Required App Flows

The first real rho parity pass should implement:

1. `/` command autocomplete in editor, with descriptions.
2. `/model` command argument completion.
3. Built-in command submit dispatcher.
4. `showSelector` editor replacement.
5. Login auth type selector.
6. Login provider selector.
7. API key login dialog.
8. OAuth login dialog or at least a structurally compatible placeholder.
9. Logout provider selector.
10. Model selector.
11. Settings selector skeleton.
12. Footer state tied to model/auth/session.

### Avoid These False Starts

- Do not print "Available commands" as an assistant message and call that command UI.
- Do not make `/login anthropic` the primary UX.
- Do not echo/prompt for secrets in the chat transcript.
- Do not run slash commands through the agent.
- Do not make the TUI a Bubble Tea screen with only a text field unless it has equivalent component/focus/autocomplete/selector behavior.
- Do not treat auth storage as independent from model registry and footer state.

## rho Gap Checklist

Current rho is missing pi-level behavior if any of these are absent:

- `/` immediately opens a command suggestion list.
- Suggestions are embedded below the editor and navigable.
- Enter on a suggestion can complete and dispatch.
- Built-in commands have descriptions.
- `/model` has model argument completions.
- `/login` opens auth type selector.
- `/login` provider selection is searchable/navigable.
- API key entry happens in a focused dialog replacing the editor.
- OAuth login shows URL/progress/manual input in a focused dialog.
- `/logout` opens provider selector.
- Model selector is searchable and auth-filtered.
- Settings are live controls.
- Session/tree/fork/resume flows are selector-based.
- Extension UI can replace editor or open overlays.
- Footer updates after auth/model/session changes.

## Implementation Order Recommendation For rho

1. Rebuild TUI primitives or adapt current ones to match pi contracts.
2. Implement editor-embedded autocomplete using `SelectList`.
3. Port `CombinedAutocompleteProvider` logic.
4. Port built-in slash command catalog.
5. Wire command submit dispatcher to app UI actions.
6. Port selector replacement helper.
7. Port auth storage shape and model registry refresh behavior.
8. Implement login auth type/provider/API-key dialogs.
9. Implement model selector.
10. Implement settings/session selectors after the primitive pattern is stable.

The point is to get the interactive substrate right first. Once that exists, the individual command handlers become much less risky.

